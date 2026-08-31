package netrom

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestNetworkFrameRoundTrip(t *testing.T) {
	origin, _ := ax25.ParseAddress("N0CALL-7")
	destination, _ := ax25.ParseAddress("W1XYZ-1")
	want := Frame{Network: NetworkHeader{Origin: origin, Destination: destination, TTL: 32}, Transport: Packet{CircuitIndex: 1, CircuitID: 2, Opcode: OpcodeInformation, Payload: []byte("data")}}
	wire, err := want.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) != NetworkHeaderSize+HeaderSize+4 {
		t.Fatalf("wire length=%d", len(wire))
	}
	got, err := DecodeFrame(wire)
	if err != nil {
		t.Fatal(err)
	}
	if got.Network.Origin.String() != origin.String() || got.Network.Destination.String() != destination.String() || got.Network.TTL != 32 || string(got.Transport.Payload) != "data" {
		t.Fatalf("decoded=%+v", got)
	}
}
