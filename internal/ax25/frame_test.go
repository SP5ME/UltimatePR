package ax25

import (
	"bytes"
	"testing"
)

func TestAddressRoundTrip(t *testing.T) {
	a, e := ParseAddress("sp5abc-9")
	if e != nil || a.String() != "SP5ABC-9" {
		t.Fatalf("%v %v", a, e)
	}
	b, _ := encodeAddress(a, true)
	got, last, e := decodeAddress(b[:])
	if e != nil || !last || got.String() != a.String() {
		t.Fatalf("%v %v", got, e)
	}
}
func TestFrameTypesRoundTrip(t *testing.T) {
	pid := byte(0xF0)
	types := []Type{TypeUI, TypeSABM, TypeUA, TypeDISC, TypeDM, TypeI, TypeRR, TypeRNR, TypeREJ}
	for _, typ := range types {
		f := Frame{Destination: Address{Callsign: "REMOTE", SSID: 1}, Source: Address{Callsign: "LOCAL", SSID: 9}, Type: typ, NS: 2, NR: 4, PID: &pid}
		if typ == TypeI || typ == TypeUI {
			f.Payload = []byte("hello")
		}
		b, e := Encode(f)
		if e != nil {
			t.Fatalf("type %d: %v", typ, e)
		}
		got, e := Decode(b)
		if e != nil || got.Type != typ || got.NS != f.NS && typ == TypeI || got.NR != f.NR && (typ == TypeI || typ == TypeRR || typ == TypeRNR || typ == TypeREJ) || !bytes.Equal(got.Payload, f.Payload) {
			t.Fatalf("type %d got=%#v err=%v", typ, got, e)
		}
	}
}
func TestDecodeRejectsShort(t *testing.T) {
	if _, e := Decode([]byte{1, 2}); e == nil {
		t.Fatal("expected error")
	}
}
func FuzzDecode(f *testing.F) {
	f.Add([]byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, b []byte) { Decode(b) })
}
