package loopback

import (
	"context"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/transport"
)

func TestPortInjectsInternalPacketOnLocalLoop(t *testing.T) {
	out := make(chan transport.Packet, 1)
	p := New(out)
	if err := p.Send(context.Background(), transport.Packet{PortID: "wrong", Channel: 7, Data: []byte("frame")}); err != nil {
		t.Fatal(err)
	}
	got := <-out
	if got.InterfaceID != "internal" || got.PortID != transport.LocalLoopPortID || got.Channel != 0 || !got.Internal || string(got.Data) != "frame" {
		t.Fatalf("packet=%+v", got)
	}
}

func TestPortHonorsCancellationWhenReceivePipelineIsFull(t *testing.T) {
	p := New(make(chan transport.Packet))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Send(ctx, transport.Packet{Data: []byte("frame")}); err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}
