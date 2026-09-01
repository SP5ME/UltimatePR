package session

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/service"
)

func TestInboundRegistryEchoService(t *testing.T) {
	sent := make(chan []byte, 16)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error {
		sent <- append([]byte(nil), b...)
		return nil
	}}, nil)
	m.t1, m.n2, m.paclen = 50*time.Millisecond, 2, 128
	registry := service.NewRegistry()
	seen := make(chan service.ServiceContext, 1)
	if err := registry.Register(service.ServiceRegistration{
		Service: service.Func{ServiceID: "echo", Handler: func(ctx service.ServiceContext) error {
			seen <- ctx
			buf := make([]byte, 128)
			n, err := ctx.Reader.Read(buf)
			if err != nil {
				return err
			}
			_, _ = ctx.Writer.Write(append([]byte("ECHO:"), buf[:n]...))
			return nil
		}},
		Callsign: ax25.Address{Callsign: "SP5ECH"}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	m.SetRegistry(registry)
	local := ax25.Address{Callsign: "SP5ECH", CommandResponse: true}
	remote := ax25.Address{Callsign: "SP5ME"}
	if !m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM, PollFinal: true}) {
		t.Fatal("SABM not handled")
	}
	ua := decodeSent(t, <-sent)
	if ua.Type != ax25.TypeUA {
		t.Fatalf("UA=%+v", ua)
	}
	ctx := <-seen
	if ctx.LocalCall.String() != local.String() || ctx.RemoteCall.String() != remote.String() || ctx.PortID != "radio" || ctx.EntryType != service.EntryAX25 {
		t.Fatalf("context=%+v", ctx)
	}
	pid := byte(0xF0)
	if !m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeI, NS: 0, NR: 1, PID: &pid, Payload: []byte("hello")}) {
		t.Fatal("I frame not handled")
	}
	var reply ax25.Frame
	for {
		reply = decodeSent(t, <-sent)
		if reply.Type == ax25.TypeI {
			break
		}
	}
	if !bytes.Equal(reply.Payload, []byte("ECHO:hello")) {
		t.Fatalf("reply=%q", reply.Payload)
	}
	if ctx.Reader == nil || ctx.Writer == nil || ctx.Cancel == nil || ctx.Disconnect == nil {
		t.Fatal("incomplete ServiceContext")
	}
}
