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
