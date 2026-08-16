package kiss

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncodeDecodeEscaping(t *testing.T) {
	want := Frame{Port: 2, Command: 0, Data: []byte{1, FEND, 2, FESC, 3}}
	b, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(64)
	got, errs := d.Feed(b)
	if len(errs) != 0 || len(got) != 1 {
		t.Fatalf("frames=%d errors=%v", len(got), errs)
	}
	if got[0].Port != want.Port || got[0].Command != 0 || !bytes.Equal(got[0].Data, want.Data) {
		t.Fatalf("got %#v", got[0])
	}
}
func TestSplitAndMultipleFrames(t *testing.T) {
	a, _ := Encode(Frame{Data: []byte("one")})
	b, _ := Encode(Frame{Data: []byte("two")})
	stream := append(a, b...)
	d := NewDecoder(64)
	var got []Frame
	for _, part := range [][]byte{stream[:2], stream[2:5], stream[5:]} {
		f, e := d.Feed(part)
		if len(e) > 0 {
			t.Fatal(e)
		}
		got = append(got, f...)
	}
	if len(got) != 2 || string(got[0].Data) != "one" || string(got[1].Data) != "two" {
		t.Fatalf("got %#v", got)
	}
}
func TestInvalidEscapeAndOversize(t *testing.T) {
	d := NewDecoder(2)
	_, errs := d.Feed([]byte{FEND, 0, FESC, 1, FEND})
	if !errors.Is(errs[0], ErrInvalidEscape) {
		t.Fatalf("%v", errs)
	}
	_, errs = d.Feed([]byte{FEND, 0, 1, 2, FEND})
	if len(errs) == 0 || !errors.Is(errs[0], ErrFrameTooLarge) {
		t.Fatalf("%v", errs)
	}
}

func TestAcceptFrame(t *testing.T) {
	if !acceptFrame(Frame{Port: 0, Command: 0}) {
		t.Fatal("expected KISS TCP frames from Direwolf channel 0 to be accepted")
	}
	if acceptFrame(Frame{Port: 1, Command: 0}) {
		t.Fatal("unexpected accept for non-zero channel")
	}
	if acceptFrame(Frame{Port: 0, Command: 1}) {
		t.Fatal("unexpected accept for non-data command")
	}
}

func FuzzDecoder(f *testing.F) {
	f.Add([]byte{FEND, 0, 1, FEND})
	f.Fuzz(func(t *testing.T, b []byte) { d := NewDecoder(4096); d.Feed(b) })
}
