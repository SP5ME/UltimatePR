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

func TestModulo128NumberedFramesRoundTrip(t *testing.T) {
	pid := byte(0xF0)
	for _, typ := range []Type{TypeI, TypeRR, TypeRNR, TypeREJ, TypeSREJ} {
		f := Frame{Destination: Address{Callsign: "REMOTE"}, Source: Address{Callsign: "LOCAL"}, Type: typ, NS: 93, NR: 117, PollFinal: true, PID: &pid}
		if typ == TypeI {
			f.Payload = []byte("extended")
		}
		encoded, err := EncodeWithModulo(f, Modulo128)
		if err != nil {
			t.Fatalf("type %d encode: %v", typ, err)
		}
		got, err := DecodeWithModulo(encoded, Modulo128)
		if err != nil || got.Type != typ || got.NR != 117 || !got.PollFinal {
			t.Fatalf("type %d got=%+v err=%v", typ, got, err)
		}
		if typ == TypeI && (got.NS != 93 || !bytes.Equal(got.Payload, f.Payload)) {
			t.Fatalf("I frame got=%+v", got)
		}
	}
}

func TestXIDRoundTripAndValidation(t *testing.T) {
	parameters := []XIDParameter{
		{Identifier: 2, Value: []byte{0x00, 0x20}},
		{Identifier: 3, Value: []byte{0x86, 0xA8, 0x02}},
		{Identifier: 6, Value: []byte{0x08, 0x00}},
		{Identifier: 8, Value: []byte{7}},
		{Identifier: 9, Value: []byte{0x27, 0x10}},
		{Identifier: 10, Value: []byte{10}},
	}
	encoded, err := EncodeXID(parameters)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeXID(encoded)
	if err != nil || len(got) != len(parameters) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for i := range parameters {
		if got[i].Identifier != parameters[i].Identifier || !bytes.Equal(got[i].Value, parameters[i].Value) {
			t.Fatalf("parameter %d got=%v want=%v", i, got[i], parameters[i])
		}
	}
	if _, err := DecodeXID([]byte{0x82, 0x80, 0, 3, 2, 2, 0}); err == nil {
		t.Fatal("truncated XID accepted")
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
