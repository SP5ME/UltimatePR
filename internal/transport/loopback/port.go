package loopback

import (
	"context"
	"errors"

	"github.com/packet-radio/ultimatepr/internal/transport"
)

// Port injects transmitted AX.25 packets into the normal receive pipeline.
// Addressed AX.25 frames provide the two logical endpoints, so the port does
// not reflect a frame back to its sender or create a second protocol path.
type Port struct {
	id  string
	out chan<- transport.Packet
}

func New(out chan<- transport.Packet) *Port {
	return &Port{id: transport.LocalLoopPortID, out: out}
}

func (p *Port) ID() string { return p.id }

func (p *Port) Status() transport.Status {
	return transport.Status{ID: p.id, Type: "Local AX.25", Connected: true, Enabled: true}
}

func (p *Port) Run(ctx context.Context, _ chan<- transport.Packet) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *Port) Send(ctx context.Context, pkt transport.Packet) error {
	if p == nil || p.out == nil {
		return errors.New("local AX.25 loopback unavailable")
	}
	pkt.InterfaceID = "internal"
	pkt.PortID = p.id
	pkt.Channel = 0
	pkt.Internal = true
	select {
	case p.out <- pkt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
