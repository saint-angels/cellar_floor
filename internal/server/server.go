package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
	"cellarfloor/internal/sim"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var validScales = map[int]bool{0: true, 1: true, 8: true, 64: true}

type Server struct {
	cfg   *data.Config
	world *sim.World
	hub   *Hub
	scale atomic.Int64
	mu    sync.Mutex // guards world during snapshot vs tick

	players map[string]*Player // guarded by mu
	pending []sim.Event        // guarded by mu; drained into the next tick

	admin string // when set, reset/timescale/advance require this token
}

// adminOK allows world-level controls: open when no token is configured
// (local play), token-gated on public deployments.
func (s *Server) adminOK(token string) bool {
	return s.admin == "" || token == s.admin
}

func Run(cfg *data.Config, w *sim.World, addr, staticDir string) error {
	s := &Server{cfg: cfg, world: w, hub: NewHub(), admin: os.Getenv("CELLAR_ADMIN_TOKEN")}
	s.scale.Store(1)
	players, err := LoadPlayers(playersPath(cfg.Sim.SavePath))
	if err != nil {
		log.Printf("load players: %v (starting empty)", err)
		players = map[string]*Player{}
	}
	s.players = players

	go s.tickLoop()
	go s.autosaveLoop()
	go s.saveOnInterrupt()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	mux.HandleFunc("/ws", s.handleWS)
	s.registerAPI(mux)
	log.Printf("cellar floor listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) tickLoop() {
	interval := time.Duration(float64(time.Second) / s.cfg.Sim.TickRate)
	t := time.NewTicker(interval)
	for range t.C {
		s.safeTick()
	}
}

func (s *Server) safeTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tick panic recovered: %v", r)
		}
	}()
	scale := int(s.scale.Load())
	if scale == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var events []sim.Event
	removed := []int{}
	for i := 0; i < scale; i++ {
		events = append(events, s.world.Step()...)
		removed = append(removed, s.world.Removed...)
	}
	if len(s.pending) > 0 {
		events = append(s.pending, events...)
		s.pending = nil
	}
	msg := BuildTick(s.world, events, scale, s.owners())
	msg.Removed = removed
	if len(msg.Events) > 200 {
		msg.Events = msg.Events[len(msg.Events)-200:]
	}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal tick: %v", err)
		return
	}
	s.hub.Broadcast(b)
}

func (s *Server) autosaveLoop() {
	if s.cfg.Sim.AutosaveMinutes <= 0 {
		return
	}
	t := time.NewTicker(time.Duration(s.cfg.Sim.AutosaveMinutes) * time.Minute)
	for range t.C {
		s.save()
	}
}

func (s *Server) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := SaveWorld(s.world, s.cfg.Sim.SavePath); err != nil {
		log.Printf("save: %v", err)
	} else {
		log.Printf("world saved at tick %d", s.world.Tick)
	}
	if err := SavePlayers(s.players, playersPath(s.cfg.Sim.SavePath)); err != nil {
		log.Printf("save players: %v", err)
	}
}

// resetWorld swaps in a freshly generated world. Player records survive;
// their dwarves do not, so everyone resolves to the dead state and can
// rejoin from the death screen.
func (s *Server) resetWorld() []byte {
	s.mu.Lock()
	s.world = gen.Generate(time.Now().UnixNano(), s.cfg)
	// entity ids restart with the new world; a kept DwarfID could collide
	// with an unrelated new entity and make owners() flip names
	for _, p := range s.players {
		p.DwarfID = 0
		// the counters restart at zero too, so old snapshots would make the
		// next recap go negative; scrub them alongside the id
		p.SeenTick = 0
		p.SeenBlocks = 0
		p.SeenGold = 0
		p.SeenMold = 0
	}
	log.Printf("world reset: %d entities", len(s.world.Entities))
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners()))
	s.mu.Unlock()
	s.save()
	if err != nil {
		log.Printf("marshal reset snapshot: %v", err)
		return nil
	}
	return snap
}

func (s *Server) saveOnInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	s.save()
	os.Exit(0)
}

func (s *Server) handleWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}
	c := &Client{conn: conn, send: make(chan []byte, 64)}

	s.mu.Lock()
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load()), s.owners()))
	s.mu.Unlock()
	if err != nil {
		log.Printf("marshal snapshot: %v", err)
	} else {
		c.send <- snap
	}

	s.hub.Register(c)

	go func() { // writer
		for b := range c.send {
			if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				break
			}
		}
		c.conn.Close()
	}()

	go func() { // reader
		defer s.hub.Unregister(c)
		for {
			_, b, err := c.conn.ReadMessage()
			if err != nil {
				return
			}
			var m ClientMsg
			if json.Unmarshal(b, &m) != nil {
				log.Printf("bad client message ignored: %s", b)
				continue
			}
			switch {
			case m.Type == "timescale" && validScales[m.Scale] && s.adminOK(m.Admin):
				s.scale.Store(int64(m.Scale))
			case m.Type == "reset" && s.adminOK(m.Admin):
				if b := s.resetWorld(); b != nil {
					s.hub.Broadcast(b)
				}
			case (m.Type == "hello" || m.Type == "spawn") && m.Player != "":
				s.mu.Lock()
				var pm PlayerMsg
				var recap *RecapMsg
				if m.Type == "hello" {
					pm = s.playerMsg(m.Player)
					// a known player returning gets an away-summary computed
					// under the same lock hold; the snapshot advances inside
					recap = s.recapFor(m.Player)
				} else {
					pm = s.spawnDwarf(m.Player, m.Name)
				}
				s.mu.Unlock()
				if b, err := json.Marshal(pm); err == nil {
					select {
					case c.send <- b:
					default:
					}
				}
				if recap != nil {
					if b, err := json.Marshal(recap); err == nil {
						select {
						case c.send <- b:
						default:
						}
					}
				}
			case m.Type == "claim" && m.Player != "":
				s.mu.Lock()
				pm := s.claimUpgrade(m.Player)
				s.mu.Unlock()
				if b, err := json.Marshal(pm); err == nil {
					select {
					case c.send <- b:
					default:
					}
				}
			case m.Type == "torch" && m.Player != "":
				s.mu.Lock()
				pm := s.placeTorch(m.Player, m.X, m.Y)
				s.mu.Unlock()
				if b, err := json.Marshal(pm); err == nil {
					select {
					case c.send <- b:
					default:
					}
				}
			}
		}
	}()
}
