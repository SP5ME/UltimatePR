package node

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/service"
)

func TestResolveBestStaticRoute(t *testing.T) {
	r := New([]Neighbor{{ID: "a", Callsign: "SP5AAA-7", Port: "p1", Quality: 100}, {ID: "b", Callsign: "SP5BBB-7", Port: "p2", Quality: 200}}, []Route{{Destination: "SR5DDD-7", Via: "a", Quality: 80}, {Destination: "SR5DDD-7", Via: "b", Quality: 190}}, nil)
	n, _, e := r.Resolve("sr5ddd-7")
	if e != nil {
		t.Fatal(e)
	}
	if n.ID != "b" {
		t.Fatalf("neighbor=%s", n.ID)
	}
}

func TestResolveSameCallsignChoosesBestPort(t *testing.T) {
	r := New([]Neighbor{
		{ID: "uhf", Callsign: "SQ5ABC", Port: "uhf", Quality: 80},
		{ID: "vhf", Callsign: "SQ5ABC", Port: "vhf", Quality: 200},
	}, nil, nil)
	n, route, err := r.Resolve("SQ5ABC")
	if err != nil {
		t.Fatal(err)
	}
	if n.Port != "vhf" || route.Via != "vhf" || route.Quality != 200 {
		t.Fatalf("neighbor=%+v route=%+v", n, route)
	}
}

func TestAdvertisedServicesAreSeparateFromLocalRegistry(t *testing.T) {
	r := New(nil, nil, []Service{{Name: "Remote BBS", Callsign: "REMOTE-8", Command: "BBS", Enabled: true}})
	if advertised, ok := r.Service("BBS"); !ok || advertised.Callsign != "REMOTE-8" {
		t.Fatalf("advertised service=%+v ok=%v", advertised, ok)
	}
	registry := service.NewRegistry()
	if _, ok := registry.ByAlias("BBS"); ok {
		t.Fatal("network advertisement was treated as local service")
	}
}
