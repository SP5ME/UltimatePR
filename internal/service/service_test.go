package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

type testService string

func (s testService) ID() string               { return string(s) }
func (testService) Serve(ServiceContext) error { return nil }

func TestRegistryLookupAndList(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(ServiceRegistration{Service: testService("echo"), Callsign: ax25.Address{Callsign: "SP5ECH", SSID: 1}, Aliases: []string{"echo"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if got, ok := r.ByID(" ECHO "); !ok || got.Callsign.String() != "SP5ECH-1" {
		t.Fatalf("ByID=%+v, ok=%v", got, ok)
	}
	if got, ok := r.ByCallsign("sp5ech-1"); !ok || got.Service.ID() != "echo" {
		t.Fatalf("ByCallsign=%+v, ok=%v", got, ok)
	}
	if got, ok := r.ByAlias("ECHO"); !ok || got.Service.ID() != "echo" {
		t.Fatalf("ByAlias=%+v, ok=%v", got, ok)
	}
	if got := r.List(); len(got) != 1 || got[0].Service.ID() != "echo" {
		t.Fatalf("List=%+v", got)
	}
}

func TestRegistryRejectsActiveConflictsAndHidesDisabled(t *testing.T) {
	r := NewRegistry()
	base := ServiceRegistration{Service: testService("base"), Callsign: ax25.Address{Callsign: "SP5BAS"}, Aliases: []string{"BASE"}, Enabled: true}
	if err := r.Register(base); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(ServiceRegistration{Service: testService("call-conflict"), Callsign: base.Callsign, Enabled: true}); err == nil {
		t.Fatal("callsign conflict accepted")
	}
	if err := r.Register(ServiceRegistration{Service: testService("alias-conflict"), Callsign: ax25.Address{Callsign: "SP5OTH"}, Aliases: []string{"base"}, Enabled: true}); err == nil {
		t.Fatal("alias conflict accepted")
	}
	if err := r.Register(ServiceRegistration{Service: testService("disabled"), Callsign: base.Callsign, Aliases: []string{"base"}, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.ByID("disabled"); ok {
		t.Fatal("disabled service available by id")
	}
	if got := r.List(); len(got) != 1 {
		t.Fatalf("List contains disabled service: %+v", got)
	}
}

func TestRegistryTracksLifecycleAndCapabilities(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(ServiceRegistration{
		Service:      testService("chat"),
		Callsign:     ax25.Address{Callsign: "SP5CHA", SSID: 7},
		Aliases:      []string{"CHAT"},
		Capabilities: []string{"interactive-stream", "interactive-stream", "routing"},
		State:        StateStarting,
		Enabled:      true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.ByID("chat"); ok {
		t.Fatal("starting service should not be routable")
	}
	if !r.SetState("chat", StateAvailable) {
		t.Fatal("SetState returned false")
	}
	got, ok := r.Resolve("SP5CHA-7")
	if !ok || got.Service.ID() != "chat" || got.State != StateAvailable {
		t.Fatalf("Resolve=%+v ok=%v", got, ok)
	}
	if len(got.Capabilities) != 2 || got.Capabilities[0] != "interactive-stream" || got.Capabilities[1] != "routing" {
		t.Fatalf("Capabilities=%v", got.Capabilities)
	}
	if !r.Unregister("chat") {
		t.Fatal("Unregister returned false")
	}
	if _, ok := r.Resolve("chat"); ok {
		t.Fatal("unregistered service still resolvable")
	}
}

func TestServiceContextShape(t *testing.T) {
	ctx := ServiceContext{Context: context.Background(), Reader: io.Reader(nil), Writer: io.Writer(nil), EntryType: EntryLocal}
	if ctx.Context == nil || ctx.EntryType != EntryLocal {
		t.Fatalf("context=%+v", ctx)
	}
}

func TestRegistryServesLocalEntry(t *testing.T) {
	r := NewRegistry()
	called := false
	if err := r.Register(ServiceRegistration{Service: Func{ServiceID: "local", Handler: func(ctx ServiceContext) error {
		called = ctx.EntryType == EntryLocal
		return nil
	}}, Aliases: []string{"LOCAL"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.Serve("local", ServiceContext{EntryType: EntryLocal}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("local service was not invoked")
	}
	if err := r.Serve("missing", ServiceContext{}); !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("missing service error=%v", err)
	}
}

func TestRegistryHasCallsignSSIDAndAlias(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(ServiceRegistration{Service: testService("bbs-main"), Callsign: ax25.Address{Callsign: "SP5ME", SSID: 8}, Aliases: []string{"BBS"}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"SP5ME-8", "sp5me-8", "BBS"} {
		if !r.Has(key) {
			t.Fatalf("Has(%q)=false, want true", key)
		}
	}
	if r.Has("SP5ME-9") {
		t.Fatal("different SSID unexpectedly resolved")
	}
	if err := r.Register(ServiceRegistration{Service: testService("disabled"), Callsign: ax25.Address{Callsign: "SP5ME", SSID: 8}, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if !r.Has("disabled") {
		t.Fatal("Has should report configured disabled service")
	}
	if _, ok := r.Resolve("disabled"); ok {
		t.Fatal("disabled service unexpectedly resolvable")
	}
}
