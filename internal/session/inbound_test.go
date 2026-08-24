package session

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestInboundAX25Service(t *testing.T) {
	sent := make(chan []byte, 32)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil }}, nil)
	m.t1, m.n2, m.paclen = 50*time.Millisecond, 2, 128
	local := ax25.Address{Callsign: "SP5ABC", SSID: 7}
	remote := ax25.Address{Callsign: "SP5ME", SSID: 1}
	inboundPath := []ax25.Address{{Callsign: "DIGI1", SSID: 1, Repeated: true}, {Callsign: "DIGI2", SSID: 2, Repeated: true}}
	m.Register(local, func(call string, r io.Reader, w io.Writer) {
		if call != "SP5ME-1" {
			t.Errorf("call=%s", call)
		}
		_, _ = io.WriteString(w, "NODE> ")
		b := make([]byte, 16)
		n, _ := r.Read(b)
		_, _ = io.WriteString(w, "RX:"+string(b[:n]))
	})

	if !m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Digipeaters: inboundPath, Type: ax25.TypeSABM, PollFinal: true}) {
		t.Fatal("SABM not handled")
	}
	ua := decodeSent(t, <-sent)
	if ua.Type != ax25.TypeUA || ua.Destination.String() != remote.String() {
		t.Fatalf("UA=%+v", ua)
	}
	assertReversePath(t, ua.Digipeaters)

	first := decodeSent(t, <-sent)
	if first.Type != ax25.TypeI || string(first.Payload) != "NODE> " {
		t.Fatalf("first=%+v", first)
	}
	assertReversePath(t, first.Digipeaters)
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeRR, NR: 1})

	pid := byte(0xF0)
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeI, NS: 0, NR: 1, PID: &pid, Payload: []byte("H\r")})
	_ = decodeSent(t, <-sent) // RR for inbound I frame
	reply := decodeSent(t, <-sent)
	if !strings.Contains(string(reply.Payload), "RX:H") {
		t.Fatalf("reply=%q", reply.Payload)
	}
	assertReversePath(t, reply.Digipeaters)
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeRR, NR: 2})
}

func assertReversePath(t *testing.T, got []ax25.Address) {
	t.Helper()
	if len(got) != 2 || got[0].String() != "DIGI2-2" || got[1].String() != "DIGI1-1" {
		t.Fatalf("reverse digipeater path=%v", got)
	}
	for _, digi := range got {
		if digi.Repeated {
			t.Fatalf("reverse path contains repeated H bit: %v", got)
		}
	}
}

func TestInboundIgnoresUnknownDestination(t *testing.T) {
	m := NewInboundMux(nil, nil)
	if m.Handle("radio", ax25.Frame{Destination: ax25.Address{Callsign: "OTHER"}, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeSABM}) {
		t.Fatal("unknown service handled")
	}
}

func TestInboundDisconnectedServiceRespondsDM(t *testing.T) {
	sent := make(chan []byte, 1)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	local := ax25.Address{Callsign: "SP5ABC", SSID: 7}
	m.Register(local, func(string, io.Reader, io.Writer) {})
	f := ax25.Frame{Destination: local, Source: ax25.Address{Callsign: "SP5ME", CommandResponse: false}, Type: ax25.TypeDISC, PollFinal: true}
	if !m.Handle("radio", f) {
		t.Fatal("local disconnected frame not handled")
	}
	dm := decodeSent(t, <-sent)
	if dm.Type != ax25.TypeDM || !dm.PollFinal {
		t.Fatalf("DM=%+v", dm)
	}
}

func TestInboundAnswersSupervisoryPoll(t *testing.T) {
	sent := make(chan []byte, 8)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	local := ax25.Address{Callsign: "SP5ABC", SSID: 7}
	remote := ax25.Address{Callsign: "SP5ME"}
	m.Register(local, func(string, io.Reader, io.Writer) { time.Sleep(100 * time.Millisecond) })
	sabm := ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM, PollFinal: true}
	if !m.Handle("radio", sabm) {
		t.Fatal("SABM not handled")
	}
	_ = <-sent // UA
	rr := ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeRR, PollFinal: true}
	rr.Destination.CommandResponse = true
	m.Handle("radio", rr)
	answer := decodeSent(t, <-sent)
	if answer.Type != ax25.TypeRR || !answer.PollFinal {
		t.Fatalf("RR response=%+v", answer)
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
