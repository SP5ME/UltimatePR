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
	local.CommandResponse = true
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
	if first.Type != ax25.TypeI || !isCommand(first) || string(first.Payload) != "NODE> " {
		t.Fatalf("first=%+v", first)
	}
	assertReversePath(t, first.Digipeaters)
	m.Handle("radio", ax25.Frame{Destination: ax25.Address{Callsign: "SP5ABC", SSID: 7}, Source: ax25.Address{Callsign: "SP5ME", SSID: 1, CommandResponse: true}, Type: ax25.TypeRR, NR: 1})

	pid := byte(0xF0)
	m.Handle("radio", ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeI, NS: 0, NR: 1, PID: &pid, Payload: []byte("H\r")})
	_ = decodeSent(t, <-sent) // RR for inbound I frame
	reply := decodeSent(t, <-sent)
	if !strings.Contains(string(reply.Payload), "RX:H") {
		t.Fatalf("reply=%q", reply.Payload)
	}
	assertReversePath(t, reply.Digipeaters)
	m.Handle("radio", ax25.Frame{Destination: ax25.Address{Callsign: "SP5ABC", SSID: 7}, Source: ax25.Address{Callsign: "SP5ME", SSID: 1, CommandResponse: true}, Type: ax25.TypeRR, NR: 2})
}

func TestInboundPacketService(t *testing.T) {
	sent := make(chan []byte, 8)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil }}, nil)
	m.t1, m.n2 = 50*time.Millisecond, 2
	local := ax25.Address{Callsign: "N0NODE"}
	remote := ax25.Address{Callsign: "N0PEER"}
	received := make(chan []byte, 1)
	m.RegisterPacket(local, func(route InboundRoute, pid byte, data []byte, send func(context.Context, byte, []byte) error) {
		if route.Remote != remote.String() || pid != 0xCF {
			t.Errorf("route=%+v pid=%x", route, pid)
		}
		received <- append([]byte(nil), data...)
		go func() { _ = send(context.Background(), pid, []byte("reply")) }()
	})
	sabm := ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM, PollFinal: true}
	sabm.Destination.CommandResponse = true
	if !m.Handle("radio", sabm) {
		t.Fatal("packet SABM not handled")
	}
	ua := decodeSent(t, <-sent)
	if ua.Type != ax25.TypeUA {
		t.Fatalf("UA=%+v", ua)
	}
	pid := byte(0xCF)
	dataFrame := ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeI, NS: 0, PID: &pid, Payload: []byte("request")}
	dataFrame.Destination.CommandResponse = true
	if !m.Handle("radio", dataFrame) {
		t.Fatal("packet I not handled")
	}
	if string(<-received) != "request" {
		t.Fatal("packet payload not delivered")
	}
	_ = decodeSent(t, <-sent) // AX.25 RR for the received packet.
	reply := decodeSent(t, <-sent)
	if reply.Type != ax25.TypeI || reply.PID == nil || *reply.PID != pid || string(reply.Payload) != "reply" {
		t.Fatalf("reply=%+v", reply)
	}
	m.Handle("radio", ax25.Frame{Destination: local, Source: ax25.Address{Callsign: remote.Callsign, CommandResponse: true}, Type: ax25.TypeRR, NR: 1})
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
	f := ax25.Frame{Destination: ax25.Address{Callsign: "SP5ABC", SSID: 7, CommandResponse: true}, Source: ax25.Address{Callsign: "SP5ME", CommandResponse: false}, Type: ax25.TypeDISC, PollFinal: true}
	if !m.Handle("radio", f) {
		t.Fatal("local disconnected frame not handled")
	}
	dm := decodeSent(t, <-sent)
	if dm.Type != ax25.TypeDM || !dm.PollFinal {
		t.Fatalf("DM=%+v", dm)
	}
}

func TestInboundDisconnectedDMAlwaysSetsFinal(t *testing.T) {
	sent := make(chan []byte, 1)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	local := ax25.Address{Callsign: "SP5ABC"}
	m.Register(local, func(string, io.Reader, io.Writer) {})
	disc := ax25.Frame{Destination: local, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeDISC}
	disc.Destination.CommandResponse = true
	m.Handle("radio", disc)
	dm := decodeSent(t, <-sent)
	if dm.Type != ax25.TypeDM || !dm.PollFinal || !isResponse(dm) {
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
	sabm.Destination.CommandResponse = true
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

func TestInboundAnswersXIDWithoutStartingService(t *testing.T) {
	sent := make(chan []byte, 1)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	local := ax25.Address{Callsign: "SP5ABC", SSID: 7}
	m.Register(local, func(string, io.Reader, io.Writer) { t.Fatal("XID started data service") })
	payload, err := ax25.EncodeXID(nil)
	if err != nil {
		t.Fatal(err)
	}
	xid := ax25.Frame{Destination: local, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeXID, PollFinal: true, Payload: payload}
	xid.Destination.CommandResponse = true
	if !m.Handle("radio", xid) {
		t.Fatal("XID not handled")
	}
	response := decodeSent(t, <-sent)
	if response.Type != ax25.TypeXID || !response.PollFinal || !isResponse(response) {
		t.Fatalf("XID response=%+v", response)
	}
	if _, err := ax25.DecodeXID(response.Payload); err != nil {
		t.Fatalf("invalid XID response: %v", err)
	}
	parameters, _ := ax25.DecodeXID(response.Payload)
	if len(parameters) != 6 || parameters[3].Identifier != 8 || len(parameters[3].Value) != 1 || parameters[3].Value[0] != 1 {
		t.Fatalf("XID did not advertise actual k=1 capabilities: %v", parameters)
	}
}

func TestInboundEchoesTESTCommand(t *testing.T) {
	sent := make(chan []byte, 1)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	local := ax25.Address{Callsign: "SP5ABC"}
	m.Register(local, func(string, io.Reader, io.Writer) {})
	test := ax25.Frame{Destination: local, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeTEST, PollFinal: true, Payload: []byte("probe")}
	test.Destination.CommandResponse = true
	if !m.Handle("radio", test) {
		t.Fatal("TEST not handled")
	}
	response := decodeSent(t, <-sent)
	if response.Type != ax25.TypeTEST || !response.PollFinal || !isResponse(response) || string(response.Payload) != "probe" {
		t.Fatalf("TEST response=%+v", response)
	}
}

func TestInboundServiceUsesDISCHandshakeOnClose(t *testing.T) {
	sent := make(chan []byte, 8)
	m := NewInboundMux(map[string]Sender{"radio": func(_ context.Context, b []byte) error { sent <- b; return nil }}, nil)
	m.t1, m.n2 = 30*time.Millisecond, 2
	local := ax25.Address{Callsign: "SP5ABC", CommandResponse: true}
	remote := ax25.Address{Callsign: "SP5ME"}
	m.Register(local, func(string, io.Reader, io.Writer) {})
	sabm := ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM, PollFinal: true}
	if !m.Handle("radio", sabm) {
		t.Fatal("SABM not handled")
	}
	_ = decodeSent(t, <-sent) // UA
	disc := decodeSent(t, <-sent)
	if disc.Type != ax25.TypeDISC || !disc.PollFinal || !isCommand(disc) {
		t.Fatalf("DISC=%+v", disc)
	}
	ua := ax25.Frame{Destination: ax25.Address{Callsign: "SP5ABC"}, Source: ax25.Address{Callsign: "SP5ME", CommandResponse: true}, Type: ax25.TypeUA, PollFinal: true}
	m.Handle("radio", ua)
	deadline := time.After(time.Second)
	for {
		m.mu.Lock()
		remaining := len(m.links)
		m.mu.Unlock()
		if remaining == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("link did not close after UA(F=1)")
		case <-time.After(time.Millisecond):
		}
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

func TestInboundRoutedServiceReceivesReturnPath(t *testing.T) {
	sent := make(chan []byte, 4)
	routes := make(chan InboundRoute, 1)
	m := NewInboundMux(map[string]Sender{"radio-fast": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil }}, nil)
	local, _ := ax25.ParseAddress("SP5ABC")
	local.CommandResponse = true
	remote, _ := ax25.ParseAddress("SQ9MDD")
	digi1, _ := ax25.ParseAddress("DIGI1-1")
	digi2, _ := ax25.ParseAddress("DIGI2-2")
	digi1.Repeated = true
	digi2.Repeated = true
	m.RegisterRouted(local, func(route InboundRoute, _ io.Reader, _ io.Writer) { routes <- route })

	if !m.Handle("radio-fast", ax25.Frame{Destination: local, Source: remote, Digipeaters: []ax25.Address{digi1, digi2}, Type: ax25.TypeSABM, PollFinal: true}) {
		t.Fatal("routed SABM not handled")
	}
	_ = <-sent // UA
	select {
	case route := <-routes:
		if route.Remote != "SQ9MDD" || route.Port != "radio-fast" {
			t.Fatalf("route=%+v", route)
		}
		assertReversePath(t, route.Digipeaters)
	case <-time.After(time.Second):
		t.Fatal("routed service was not called")
	}
}
