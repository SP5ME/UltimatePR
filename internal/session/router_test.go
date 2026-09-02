package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/node"
	"github.com/packet-radio/ultimatepr/internal/service"
)

func TestRouterUsesHubReturnPathForBidirectionalStream(t *testing.T) {
	frames := make(chan ax25.Frame, 16)
	local := ax25.Address{Callsign: "LOCAL"}
	remote := ax25.Address{Callsign: "REMOTE"}
	// Capture the encoded sender at the frame boundary used by Manager.
	hub := NewHub(local, map[string]Sender{"vhf": func(_ context.Context, data []byte) error {
		frame, err := ax25.Decode(data)
		if err != nil {
			return err
		}
		frames <- frame
		return nil
	}})
	router := &Router{Hub: hub, Node: node.New([]node.Neighbor{{ID: "n1", Callsign: "REMOTE", Port: "vhf", Quality: 100}}, nil, nil), NodeEnabled: true}
	s, err := router.DialSession(context.Background(), service.SessionRequest{Target: "REMOTE", Transport: "node"})
	if err != nil {
		t.Fatal(err)
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- s.Connect(context.Background(), "REMOTE") }()
	select {
	case frame := <-frames:
		if frame.Type != ax25.TypeSABM {
			t.Fatalf("first frame = %v, want SABM", frame.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("did not observe outbound SABM")
	}
	remoteResponse := remote
	remoteResponse.CommandResponse = true
	localCommand := local
	localCommand.CommandResponse = true
	if !hub.Handle("vhf", ax25.Frame{Destination: local, Source: remoteResponse, Type: ax25.TypeUA, PollFinal: true}) {
		t.Fatal("hub did not claim UA")
	}
	select {
	case err := <-connectDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("session did not connect")
	}
	writeDone := make(chan error, 1)
	go func() { _, err := s.Write([]byte("outbound")); writeDone <- err }()
	deadline := time.After(time.Second)
	for {
		select {
		case frame := <-frames:
			if frame.Type != ax25.TypeI {
				continue
			}
			if string(frame.Payload) != "outbound" {
				t.Fatalf("outbound frame = %+v", frame)
			}
			if !hub.Handle("vhf", ax25.Frame{Destination: local, Source: remoteResponse, Type: ax25.TypeRR, NR: 1, PollFinal: true}) {
				t.Fatal("hub did not claim outbound acknowledgement")
			}
			goto outboundObserved
		case <-deadline:
			t.Fatal("did not observe outbound data")
		}
	}
outboundObserved:
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if !hub.Handle("vhf", ax25.Frame{Destination: localCommand, Source: remote, Type: ax25.TypeI, NS: 0, PID: bytePtr(0xf0), Payload: []byte("inbound")}) {
		t.Fatal("hub did not claim inbound data")
	}
	buf := make([]byte, 32)
	n, err := s.Read(buf)
	if err != nil || string(buf[:n]) != "inbound" {
		t.Fatalf("read n=%d err=%v data=%q", n, err, buf[:n])
	}
	closed := make(chan struct{})
	go func() { _ = s.Close(); close(closed) }()
	select {
	case frame := <-frames:
		if frame.Type != ax25.TypeDISC {
			t.Fatalf("close frame = %v, want DISC", frame.Type)
		}
		if !hub.Handle("vhf", ax25.Frame{Destination: local, Source: remoteResponse, Type: ax25.TypeUA, PollFinal: true}) {
			t.Fatal("hub did not claim close acknowledgement")
		}
	case <-time.After(time.Second):
		t.Fatal("did not observe close DISC")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("session did not close")
	}
}

func bytePtr(v byte) *byte { return &v }

func TestRouterNeverFallsBackToRFForUnavailableLocalService(t *testing.T) {
	router := &Router{Hub: NewHub(ax25.Address{Callsign: "LOCAL"}, nil), Node: node.New([]node.Neighbor{{ID: "rf", Callsign: "REMOTE", Port: "vhf", Quality: 100}}, nil, nil), NodeEnabled: true, UnavailableTargets: []string{"BBS-8"}}
	_, err := router.DialSession(context.Background(), service.SessionRequest{Target: "BBS-8", Transport: "node"})
	if !errors.Is(err, service.ErrServiceUnavailable) {
		t.Fatalf("error = %v, want service unavailable", err)
	}
}

func TestRouterReportsMissingRouteBeforeSessionCreation(t *testing.T) {
	router := &Router{Hub: NewHub(ax25.Address{Callsign: "LOCAL"}, nil), Node: node.New(nil, nil, nil), NodeEnabled: true}
	_, err := router.DialSession(context.Background(), service.SessionRequest{Target: "UNKNOWN-8", Transport: "node"})
	if !errors.Is(err, service.ErrRouteNotFound) {
		t.Fatalf("error = %v, want route not found", err)
	}
}

func TestRouterRequiresExplicitRFSignalForFallback(t *testing.T) {
	router := &Router{Hub: NewHub(ax25.Address{Callsign: "LOCAL"}, nil), NodeEnabled: false}
	if _, err := router.DialSession(context.Background(), service.SessionRequest{Target: "REMOTE", Transport: "node"}); !errors.Is(err, service.ErrServiceUnavailable) {
		t.Fatalf("without fallback error = %v", err)
	}
	if _, err := router.DialSession(context.Background(), service.SessionRequest{Target: "REMOTE", Transport: "node", FallbackToRF: true, RFPort: "vhf"}); err != nil {
		t.Fatalf("explicit fallback error = %v", err)
	}
}

func TestRouterExplainRouteDoesNotConnect(t *testing.T) {
	registry := service.NewRegistry()
	if err := registry.Register(service.ServiceRegistration{Service: service.Func{ServiceID: "bbs-main"}, Callsign: ax25.Address{Callsign: "LOCAL", SSID: 8}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	router := &Router{Hub: NewHub(ax25.Address{Callsign: "LOCAL"}, nil), Registry: registry, NodeEnabled: true, Node: node.New(nil, nil, nil)}
	result := router.ExplainRoute("LOCAL-8")
	if result.Resolution != "local service" || result.ServiceID != "bbs-main" || result.State != string(service.StateAvailable) {
		t.Fatalf("unexpected local explanation: %+v", result)
	}
	result = router.ExplainRoute("UNKNOWN-8")
	if result.Resolution != "route not found" || result.Error != service.ErrRouteNotFound.Error() {
		t.Fatalf("unexpected missing explanation: %+v", result)
	}
}
