package transport

import "context"

type Packet struct {
	// InterfaceID identifies the physical/network adapter. PortID is the
	// logical packet-radio port selected after interface mapping.
	InterfaceID string
	PortID      string
	Channel     uint8
	// Internal marks packets produced by an in-process transport such as the
	// local AX.25 loopback. They still use the normal dispatcher pipeline but
	// must not be treated as RF observations.
	Internal bool
	Data     []byte
}

const LocalLoopPortID = "local-loop"

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
