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
	frames, errs := d.Feed([]byte{FEND, 0, FESC, 1, 2, FEND})
	if !errors.Is(errs[0], ErrInvalidEscape) {
		t.Fatalf("%v", errs)
	}
	if len(frames) != 1 || !bytes.Equal(frames[0].Data, []byte{2}) {
		t.Fatalf("invalid escape must skip only the bad escape sequence, got %#v", frames)
	}
	_, errs = d.Feed([]byte{FEND, 0, 1, 2, FEND})
	if len(errs) == 0 || !errors.Is(errs[0], ErrFrameTooLarge) {
		t.Fatalf("%v", errs)
	}
}

func TestDecoderDiscardsGarbageBeforeOpeningFEND(t *testing.T) {
	d := NewDecoder(64)
	frames, errs := d.Feed([]byte{0x00, 0x11, 0x22, FEND, 0x00, 'O', 'K', FEND})
	if len(errs) != 0 || len(frames) != 1 || string(frames[0].Data) != "OK" {
		t.Fatalf("frames=%#v errors=%v", frames, errs)
	}
}

func TestDecoderKeepsEscapeStateAcrossReads(t *testing.T) {
	d := NewDecoder(64)
	frames, errs := d.Feed([]byte{FEND, 0x00, FESC})
	if len(frames) != 0 || len(errs) != 0 {
		t.Fatalf("frames=%#v errors=%v", frames, errs)
	}
	frames, errs = d.Feed([]byte{TFEND, FEND, FEND})
	if len(errs) != 0 || len(frames) != 1 || !bytes.Equal(frames[0].Data, []byte{FEND}) {
		t.Fatalf("frames=%#v errors=%v", frames, errs)
	}
}

func TestAcceptFrame(t *testing.T) {
	if !acceptFrame(Frame{Port: 3, Command: 0}, 3) {
		t.Fatal("expected KISS TCP frames from Direwolf channel 0 to be accepted")
	}
	if acceptFrame(Frame{Port: 1, Command: 0}, 3) {
		t.Fatal("unexpected accept for non-zero channel")
	}
	if acceptFrame(Frame{Port: 3, Command: 1}, 3) {
		t.Fatal("unexpected accept for non-data command")
	}
}

func TestWriteParameters(t *testing.T) {
	txDelay, persistence, slotTime, txTail, fullDuplex := uint8(25), uint8(63), uint8(10), uint8(2), true
	p := NewTCPPort(TCPConfig{Port: 4, TXDelay: &txDelay, Persistence: &persistence, SlotTime: &slotTime, TXTail: &txTail, FullDuplex: &fullDuplex}, nil)
	var wire bytes.Buffer
	if err := p.writeParameters(&wire); err != nil {
		t.Fatal(err)
	}
	frames, errs := NewDecoder(64).Feed(wire.Bytes())
	if len(errs) != 0 || len(frames) != 5 {
		t.Fatalf("frames=%#v errors=%v", frames, errs)
	}
	wantCommands := []uint8{CommandTXDelay, CommandPersistence, CommandSlotTime, CommandTXTail, CommandFullDuplex}
	for i, frame := range frames {
		if frame.Port != 4 || frame.Command != wantCommands[i] || len(frame.Data) != 1 {
			t.Fatalf("frame[%d]=%#v", i, frame)
		}
	}
	if frames[4].Data[0] != 1 {
		t.Fatalf("full duplex value=%d", frames[4].Data[0])
	}
}

func FuzzDecoder(f *testing.F) {
	f.Add([]byte{FEND, 0, 1, FEND})
	f.Fuzz(func(t *testing.T, b []byte) { d := NewDecoder(4096); d.Feed(b) })
}
