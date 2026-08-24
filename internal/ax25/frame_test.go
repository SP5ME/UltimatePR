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

func TestParseDigipeaters(t *testing.T) {
	got, err := ParseDigipeaters("sq5aaa-1, sp5bbb-2 ; sp5ccc-3")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SQ5AAA-1", "SP5BBB-2", "SP5CCC-3"}
	if len(got) != len(want) {
		t.Fatalf("digipeaters=%v", got)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Fatalf("digipeaters=%v", got)
		}
	}
	if got, err := ParseDigipeaters(" "); err != nil || got != nil {
		t.Fatalf("empty parse=%v err=%v", got, err)
	}
}

func TestFrameTypesRoundTrip(t *testing.T) {
	pid := byte(0xF0)
	types := []Type{TypeUI, TypeSABM, TypeSABME, TypeUA, TypeDISC, TypeDM, TypeI, TypeRR, TypeRNR, TypeREJ, TypeSREJ, TypeFRMR, TypeXID, TypeTEST}
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

func TestEncodeRejectsInformationOnControlFrame(t *testing.T) {
	_, err := Encode(Frame{Destination: Address{Callsign: "REMOTE"}, Source: Address{Callsign: "LOCAL"}, Type: TypeRR, Payload: []byte("invalid")})
	if err == nil {
		t.Fatal("expected control-frame information field error")
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
