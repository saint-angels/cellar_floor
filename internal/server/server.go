package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"cellarfloor/internal/data"
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
}

func Run(cfg *data.Config, w *sim.World, addr, staticDir string) error {
	s := &Server{cfg: cfg, world: w, hub: NewHub()}
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
	msg := BuildTick(s.world, events, scale)
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

func (s *Server) saveOnInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
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
	snap, err := json.Marshal(BuildSnapshot(s.world, int(s.scale.Load())))
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
			if m.Type == "timescale" && validScales[m.Scale] {
				s.scale.Store(int64(m.Scale))
			}
		}
	}()
}
