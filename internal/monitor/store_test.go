package monitor

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestFormatVia(t *testing.T) {
	digis := []ax25.Address{
		{Callsign: "SR5DDD", SSID: 1, Repeated: true},
		{Callsign: "SR5EEE", SSID: 2},
	}
	if got := formatVia(digis); got != "SR5DDD-1*, SR5EEE-2" {
		t.Fatalf("formatVia() = %q", got)
	}
}

func TestAddStoresVia(t *testing.T) {
	s := New(4)
	f := ax25.Frame{
		Destination: ax25.Address{Callsign: "BEACON"},
		Source:      ax25.Address{Callsign: "SP5ME"},
		Digipeaters: []ax25.Address{{Callsign: "SR5DDD", Repeated: true}},
		Type:        ax25.TypeUI,
	}
	s.Add("RX", "radio-2m", f, 42)
	items := s.List()
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	if items[0].Via != "SR5DDD*" {
		t.Fatalf("via=%q", items[0].Via)
	}
}
