package sim

type Event struct {
	Tick       int64  `json:"tick"`
	Type       string `json:"type"`
	Actor      int    `json:"actor"`
	ActorType  string `json:"actorType"`
	Target     int    `json:"target,omitempty"`
	TargetType string `json:"targetType,omitempty"`
	Amount     int    `json:"amount,omitempty"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Msg        string `json:"msg"`
}
