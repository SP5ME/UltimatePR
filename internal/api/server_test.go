package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	"github.com/packet-radio/ultimatepr/internal/transport"
)

func testServer(scopes ...string) (*Server, string) {
	plain := "test-secret"
	h := sha256.Sum256([]byte(plain))
	return New(Config{Callsign: "N0CALL", Version: "test", Tokens: []Token{{Name: "test", Hash: hex.EncodeToString(h[:]), Scopes: scopes}}, Ports: func() []transport.Status {
		return []transport.Status{{ID: "vhf", Type: "kiss-tcp", Enabled: true, Connected: true}}
	}, MHeard: mheard.New(10), Monitor: monitor.New(10), Broker: NewBroker()}, nilLogger()), plain
}

func nilLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func request(t *testing.T, h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHealthAndAuthentication(t *testing.T) {
	s, token := testServer("status.read")
	h := s.Handler()
	if w := request(t, h, "GET", "/api/v1/health", ""); w.Code != 200 {
		t.Fatalf("health=%d", w.Code)
	}
	if w := request(t, h, "GET", "/api/v1/status", ""); w.Code != 401 {
		t.Fatalf("missing token=%d", w.Code)
	}
	if w := request(t, h, "GET", "/api/v1/status", "wrong"); w.Code != 401 {
		t.Fatalf("bad token=%d", w.Code)
	}
	w := request(t, h, "GET", "/api/v1/status", token)
	if w.Code != 200 {
		t.Fatalf("status=%d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body["callsign"] != "N0CALL" {
		t.Fatalf("bad JSON: %s", w.Body.String())
	}
}

func TestScopePortAndDisabledServices(t *testing.T) {
	s, token := testServer("ports.read", "node.read", "bbs.read", "digipeater.read")
	h := s.Handler()
	if w := request(t, h, "GET", "/api/v1/sessions", token); w.Code != 403 {
		t.Fatalf("scope=%d", w.Code)
	}
	if w := request(t, h, "GET", "/api/v1/ports/missing", token); w.Code != 404 {
		t.Fatalf("port=%d", w.Code)
	}
	for _, path := range []string{"/api/v1/node/status", "/api/v1/bbs/status", "/api/v1/digipeater/status"} {
		if w := request(t, h, "GET", path, token); w.Code != 200 {
			t.Fatalf("%s=%d", path, w.Code)
		}
	}
}

func TestMHeardFormatAndMonitorLimit(t *testing.T) {
	s, token := testServer("mheard.read", "monitor.read")
	s.cfg.MHeard.Heard("SP5ME", "vhf")
	w := request(t, s.Handler(), "GET", "/api/v1/mheard", token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"callsign":"SP5ME"`) {
		t.Fatalf("mheard: %s", w.Body.String())
	}
	for i := 0; i < 3; i++ {
		s.cfg.Monitor.Add("RX", "vhf", testFrame(), 10)
	}
	w = request(t, s.Handler(), "GET", "/api/v1/monitor?limit=2", token)
	var out struct {
		Items []monitor.Entry `json:"items"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Items) != 2 {
		t.Fatalf("monitor: %s", w.Body.String())
	}
}

func TestWebSocketReceivesEvent(t *testing.T) {
	s, token := testServer("events.read")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	h := http.Header{"Authorization": []string{"Bearer " + token}}
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/events", h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s.cfg.Broker.Publish("frame.rx", map[string]string{"port": "vhf"})
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	var e Event
	if err = c.ReadJSON(&e); err != nil {
		t.Fatal(err)
	}
	if e.Type != "frame.rx" {
		t.Fatalf("event=%q", e.Type)
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	b := NewBroker()
	_, cancel := b.Subscribe(1)
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.Publish("test", i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked")
	}
}

func testFrame() ax25.Frame {
	pid := byte(0xf0)
	return ax25.Frame{Source: ax25.Address{Callsign: "SP5ME"}, Destination: ax25.Address{Callsign: "APRS"}, Type: ax25.TypeUI, PID: &pid, Payload: []byte("test")}
}
