package server

import (
	"encoding/json"
	"os"

	"cellarfloor/internal/data"
	"cellarfloor/internal/sim"
)

func SaveWorld(w *sim.World, path string) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadWorld(path string, cfg *data.Config) (*sim.World, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var w sim.World
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	w.SetConfig(cfg)
	return &w, nil
}
