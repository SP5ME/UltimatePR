package session

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/packet-radio/modernbbs/internal/ax25"
)

func TestInboundAX25Service(t *testing.T) {
	sent := make(chan []byte, 32)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil }}, nil)
	m.t1, m.n2, m.paclen = 50*time.Millisecond, 2, 128
	local := ax25.Address{Callsign: "SP5ABC", SSID: 7}
	remote := ax25.Address{Callsign: "SP5ME", SSID: 1}
	m.Register(local, func(call string, r io.Reader, w io.Writer) {
		if call != "SP5ME-1" {
			t.Errorf("call=%s", call)
		}
		_, _ = io.WriteString(w, "NODE> ")
		b := make([]byte, 16)
		n, _ := r.Read(b)
		_, _ = io.WriteString(w, "RX:"+string(b[:n]))
	})

	if !m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM, PollFinal: true}) {
		t.Fatal("SABM not handled")
	}
	ua := decodeSent(t, <-sent)
	if ua.Type != ax25.TypeUA || ua.Destination.String() != remote.String() {
		t.Fatalf("UA=%+v", ua)
	}

	first := decodeSent(t, <-sent)
	if first.Type != ax25.TypeI || string(first.Payload) != "NODE> " {
		t.Fatalf("first=%+v", first)
	}
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeRR, NR: 1})

	pid := byte(0xF0)
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeI, NS: 0, NR: 1, PID: &pid, Payload: []byte("H\r")})
	_ = decodeSent(t, <-sent) // RR for inbound I frame
	reply := decodeSent(t, <-sent)
	if !strings.Contains(string(reply.Payload), "RX:H") {
		t.Fatalf("reply=%q", reply.Payload)
	}
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeRR, NR: 2})
}

func TestInboundIgnoresUnknownDestination(t *testing.T) {
	m := NewInboundMux(nil, nil)
	if m.Handle("radio", ax25.Frame{Destination: ax25.Address{Callsign: "OTHER"}, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeSABM}) {
		t.Fatal("unknown service handled")
	}
}

func decodeSent(t *testing.T, b []byte) ax25.Frame {
	t.Helper()
	f, err := ax25.Decode(bytes.Clone(b))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
