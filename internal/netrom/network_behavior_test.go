package netrom

import (
	"bytes"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestFragmentUsesNETROMInformationLimit(t *testing.T) {
	data := bytes.Repeat([]byte{'x'}, MaxInformationPayload*2+1)
	packets := Fragment(data, 1, 2, 7, 3)
	if len(packets) != 3 || len(packets[0].Payload) != MaxInformationPayload || len(packets[1].Payload) != MaxInformationPayload || len(packets[2].Payload) != 1 {
		t.Fatalf("packets=%d lengths=%d,%d,%d", len(packets), len(packets[0].Payload), len(packets[1].Payload), len(packets[2].Payload))
	}
	if !packets[0].MoreFollows || !packets[1].MoreFollows || packets[2].MoreFollows || packets[0].TXSequence != 7 || packets[2].TXSequence != 9 {
		t.Fatalf("fragment metadata=%+v", packets)
	}
}

func TestForwardConsumesTTLAndPreservesEndpoints(t *testing.T) {
	origin, _ := ax25.ParseAddress("N0CALL")
	destination, _ := ax25.ParseAddress("W1XYZ")
	local, _ := ax25.ParseAddress("N0NODE")
	want := Frame{Network: NetworkHeader{Origin: origin, Destination: destination, TTL: 2}}
	got, err := want.Forward(local)
	if err != nil || got.Network.TTL != 1 || got.Network.Origin.String() != origin.String() || got.Network.Destination.String() != destination.String() {
		t.Fatalf("forwarded=%+v err=%v", got, err)
	}
	if _, err := got.Forward(local); err == nil {
		t.Fatal("expired NET/ROM frame forwarded")
	}
	if _, err := (Frame{Network: NetworkHeader{Destination: local, TTL: 2}}).Forward(local); err == nil {
		t.Fatal("locally addressed frame forwarded")
	}
}

func TestNetworkHeaderTTL(t *testing.T) {
	h := NetworkHeader{TTL: 2}
	if !h.DecrementTTL() || h.TTL != 1 || h.DecrementTTL() {
		t.Fatalf("TTL handling failed: %+v", h)
	}
}
