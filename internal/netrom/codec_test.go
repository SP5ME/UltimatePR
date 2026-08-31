package netrom

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestRoutingBroadcastRoundTrip(t *testing.T) {
	call, _ := ax25.ParseAddress("N0CALL-7")
	via, _ := ax25.ParseAddress("W1XYZ-1")
	wire, err := (RoutingBroadcast{Sender: "LOCAL", Destinations: []Destination{{Callsign: call, Mnemonic: "LOCAL", Neighbor: via, Quality: 192}}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRouting(wire)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sender != "LOCAL" || len(got.Destinations) != 1 || got.Destinations[0].Callsign.String() != "N0CALL-7" || got.Destinations[0].Quality != 192 {
		t.Fatalf("decoded=%+v", got)
	}
}

func TestDecodeRejectsMalformedRoutingBroadcast(t *testing.T) {
	if _, err := DecodeRouting([]byte{RoutingMarker, 'N', 'O', 'D', 'E', ' ', ' '}); err == nil {
		t.Fatal("malformed broadcast accepted")
	}
}

func TestRoutingBroadcastRejectsMoreThanElevenDestinations(t *testing.T) {
	call, _ := ax25.ParseAddress("N0CALL")
	via, _ := ax25.ParseAddress("W1XYZ")
	destinations := make([]Destination, MaxRoutingDestinations+1)
	for i := range destinations {
		destinations[i] = Destination{Callsign: call, Mnemonic: "NODE", Neighbor: via, Quality: 1}
	}
	if _, err := (RoutingBroadcast{Sender: "NODE", Destinations: destinations}).Encode(); err == nil {
		t.Fatal("oversized routing broadcast accepted")
	}
}
