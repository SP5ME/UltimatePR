package node

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/service"
)

func TestNodeRunsGenericVisibleEcho(t *testing.T) {
	registry := service.NewRegistry()
	if err := registry.Register(service.ServiceRegistration{
		Service: service.Func{ServiceID: "echo", Handler: func(ctx service.ServiceContext) error {
			data, _ := io.ReadAll(ctx.Reader)
			_, _ = ctx.Writer.Write(data)
			return nil
		}},
		Aliases: []string{"ECHO"}, Enabled: true, NodeVisible: true,
	}); err != nil {
		t.Fatal(err)
	}
	srv := &Server{Callsign: "LOCAL-7", Alias: "NODE", Registry: registry, Router: New(nil, nil, nil)}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nECHO\nabc"), &out)
	if !strings.Contains(out.String(), "abc") {
		t.Fatalf("echo output missing: %q", out.String())
	}
}

func TestNodeDoesNotRunHiddenOrDisabledServices(t *testing.T) {
	registry := service.NewRegistry()
	for _, registration := range []service.ServiceRegistration{
		{Service: service.Func{ServiceID: "hidden", Handler: func(service.ServiceContext) error { return nil }}, Aliases: []string{"HIDDEN"}, Enabled: true},
		{Service: service.Func{ServiceID: "disabled", Handler: func(service.ServiceContext) error { return nil }}, Aliases: []string{"DISABLED"}, Enabled: false, NodeVisible: true},
	} {
		if err := registry.Register(registration); err != nil {
			t.Fatal(err)
		}
	}
	srv := &Server{Callsign: "LOCAL-7", Alias: "NODE", Registry: registry, Router: New(nil, nil, nil)}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nSERVICES\nHIDDEN\nDISABLED\nBYE\n"), &out)
	text := out.String()
	if strings.Contains(text, "HIDDEN") || strings.Contains(text, "DISABLED") {
		t.Fatalf("hidden or disabled service exposed: %q", text)
	}
	if !strings.Contains(text, "unknown") && !strings.Contains(text, "unavailable") && !strings.Contains(text, "niedost") {
		t.Fatalf("missing unavailable service response: %q", text)
	}
}

func TestNodeBuiltInCommandWinsOverServiceAlias(t *testing.T) {
	registry := service.NewRegistry()
	called := false
	if err := registry.Register(service.ServiceRegistration{
		Service: service.Func{ServiceID: "connect-service", Handler: func(service.ServiceContext) error {
			called = true
			return nil
		}},
		Aliases: []string{"CONNECT"}, Enabled: true, NodeVisible: true,
	}); err != nil {
		t.Fatal(err)
	}
	srv := &Server{Callsign: "LOCAL-7", Alias: "NODE", Registry: registry, Router: New(nil, nil, nil)}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nCONNECT\nBYE\n"), &out)
	if called {
		t.Fatal("service alias replaced built-in CONNECT")
	}
	if !strings.Contains(out.String(), "Usage") && !strings.Contains(out.String(), "Uzycie") {
		t.Fatalf("built-in CONNECT response missing: %q", out.String())
	}
}

func TestNodePassesServiceContext(t *testing.T) {
	registry := service.NewRegistry()
	seen := make(chan service.ServiceContext, 1)
	if err := registry.Register(service.ServiceRegistration{
		Service: service.Func{ServiceID: "echo", Handler: func(ctx service.ServiceContext) error {
			seen <- ctx
			return nil
		}},
		Aliases: []string{"ECHO"}, Enabled: true, NodeVisible: true,
	}); err != nil {
		t.Fatal(err)
	}
	local, _ := ax25.ParseAddress("LOCAL-7")
	remote, _ := ax25.ParseAddress("SP5ME")
	srv := &Server{Callsign: local.String(), Alias: "NODE", Registry: registry, Router: New(nil, nil, nil)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	go srv.ServeContext(service.ServiceContext{Context: ctx, LocalCall: local, RemoteCall: remote, PortID: "UHF", Reader: strings.NewReader("ECHO\nBYE\n"), Writer: &out, EntryType: service.EntryAX25})
	select {
	case got := <-seen:
		if got.RemoteCall != remote || got.LocalCall != local || got.PortID != "UHF" || got.EntryType != service.EntryNode {
			t.Fatalf("context=%+v", got)
		}
	case <-ctx.Done():
		t.Fatal("node context canceled before service handoff")
	}
}
