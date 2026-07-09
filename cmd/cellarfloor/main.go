package main

import (
	"errors"
	"flag"
	"io/fs"
	"log"

	"cellarfloor/internal/data"
	"cellarfloor/internal/gen"
	"cellarfloor/internal/server"
	"cellarfloor/internal/sim"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "data", "data directory")
	staticDir := flag.String("static", "client/dist", "client build directory")
	seed := flag.Int64("seed", 2026, "world seed when no save exists")
	fresh := flag.Bool("fresh", false, "ignore existing save and regenerate")
	flag.Parse()

	cfg, err := data.Load(*dataDir)
	if err != nil {
		log.Fatalf("data: %v", err)
	}

	var w *sim.World
	if !*fresh {
		w, err = server.LoadWorld(cfg.Sim.SavePath, cfg)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("save file unreadable (%v), regenerating", err)
		}
	}
	if w == nil {
		w = gen.Generate(*seed, cfg)
		log.Printf("generated world from seed %d: %d entities", *seed, len(w.Entities))
	} else {
		log.Printf("loaded world at tick %d: %d entities", w.Tick, len(w.Entities))
	}

	log.Fatal(server.Run(cfg, w, *addr, *staticDir))
}
