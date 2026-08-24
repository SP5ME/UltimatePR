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

func TestTypeNamesRemainStableWhenAX25TypesAreExtended(t *testing.T) {
	cases := map[ax25.Type]string{
		ax25.TypeI: "I", ax25.TypeRR: "RR", ax25.TypeRNR: "RNR",
		ax25.TypeREJ: "REJ", ax25.TypeSREJ: "SREJ", ax25.TypeSABM: "SABM",
		ax25.TypeSABME: "SABME", ax25.TypeDISC: "DISC", ax25.TypeDM: "DM",
		ax25.TypeUA: "UA", ax25.TypeUI: "UI", ax25.TypeFRMR: "FRMR",
		ax25.TypeXID: "XID", ax25.TypeTEST: "TEST", ax25.TypeUnknown: "?",
	}
	for typ, want := range cases {
		if got := typeName(typ); got != want {
			t.Errorf("typeName(%d)=%q, want %q", typ, got, want)
		}
	}
}

func TestClearRemovesBufferedEntriesAndAllowsNewFrames(t *testing.T) {
	s := New(4)
	f := ax25.Frame{Destination: ax25.Address{Callsign: "BEACON"}, Source: ax25.Address{Callsign: "SP5ME"}, Type: ax25.TypeUI, PID: bytePtr(0xF0)}
	s.Add("RX", "radio", f, 16)
	s.Clear()
	if got := s.List(); len(got) != 0 {
		t.Fatalf("entries after Clear=%d", len(got))
	}
	s.Add("TX", "radio", f, 16)
	if got := s.List(); len(got) != 1 || got[0].Direction != "TX" {
		t.Fatalf("entries after reuse=%+v", got)
	}
}

func bytePtr(v byte) *byte { return &v }
