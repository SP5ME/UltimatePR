package transport

import "context"

type Packet struct {
	PortID  string
	Channel uint8
	Data    []byte
}

type Port interface {
	ID() string
	Run(context.Context, chan<- Packet) error
	Send(context.Context, Packet) error
}
