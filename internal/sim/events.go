package sim

type Event struct {
	Tick          int64  `json:"tick"`
	Type          string `json:"type"`
	Actor         int    `json:"actor"`
	ActorSpecies  string `json:"actorSpecies"`
	Target        int    `json:"target,omitempty"`
	TargetSpecies string `json:"targetSpecies,omitempty"`
	Msg           string `json:"msg"`
}
