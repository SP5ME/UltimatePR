package node

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/service"
)

func TestConnectUsesRouterPortNotServiceRegistry(t *testing.T) {
	registry := service.NewRegistry()
	called := false
	if err := registry.Register(service.ServiceRegistration{Service: service.Func{ServiceID: "remote", Handler: func(service.ServiceContext) error {
		called = true
		return nil
	}}, Aliases: []string{"REMOTE"}, Enabled: true, NodeVisible: true}); err != nil {
		t.Fatal(err)
	}
	router := New([]Neighbor{{ID: "vhf-node", Callsign: "REMOTE", Port: "vhf", Quality: 200}}, nil, nil)
	var gotPort string
	srv := &Server{Callsign: "LOCAL-7", Alias: "NODE", Router: router, Registry: registry, Connect: func(_ string, n Neighbor, _ Route, r io.Reader, w io.Writer) error {
		gotPort = n.Port
		data, _ := io.ReadAll(r)
		_, _ = w.Write(data)
		return nil
	}}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nC REMOTE\npayload"), &out)
	if called {
		t.Fatal("CONNECT invoked local registry service")
	}
	if gotPort != "vhf" || !strings.Contains(out.String(), "payload") {
		t.Fatalf("port=%q output=%q", gotPort, out.String())
	}
}

func TestConnectReportsInvalidLogicalPortWithoutPanic(t *testing.T) {
	router := New([]Neighbor{{ID: "bad", Callsign: "REMOTE", Port: "disabled-port", Quality: 100}}, nil, nil)
	srv := &Server{Callsign: "LOCAL-7", Alias: "NODE", Router: router, Connect: func(_ string, n Neighbor, _ Route, _ io.Reader, _ io.Writer) error {
		if n.Port != "disabled-port" {
			t.Fatalf("port=%q", n.Port)
		}
		return io.ErrClosedPipe
	}}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nC REMOTE\n"), &out)
	if !strings.Contains(out.String(), "CONNECT failed") {
		t.Fatalf("missing connect failure: %q", out.String())
	}
}
