package netrom

import "testing"

func TestTransportPacketRoundTrip(t *testing.T) {
	want := Packet{CircuitIndex: 3, CircuitID: 9, TXSequence: 7, RXSequence: 2, Opcode: OpcodeInformation, MoreFollows: true, NAK: true, Choke: true, Payload: []byte("hello")}
	wire, err := want.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatal(err)
	}
	if got.CircuitIndex != want.CircuitIndex || got.CircuitID != want.CircuitID || got.TXSequence != want.TXSequence || got.RXSequence != want.RXSequence || got.Opcode != want.Opcode || !got.MoreFollows || !got.NAK || !got.Choke || string(got.Payload) != "hello" {
		t.Fatalf("decoded=%+v", got)
	}
}

func TestTransportRejectsInvalidOpcode(t *testing.T) {
	if _, err := DecodePacket([]byte{0, 0, 0, 0, 0x0F}); err == nil {
		t.Fatal("invalid opcode accepted")
	}
}
