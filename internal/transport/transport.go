package transport

import "context"

type Packet struct {
	PortID  string
	Channel uint8
	Data    []byte
}

type Port interface {
	ID() string
	Status() Status
	Run(context.Context, chan<- Packet) error
	Send(context.Context, Packet) error
}

type Status struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Enabled   bool   `json:"enabled"`
}
