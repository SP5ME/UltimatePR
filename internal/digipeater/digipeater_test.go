package digipeater

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func frame(t *testing.T, source string, vias ...ax25.Address) (ax25.Frame, []byte) {
	t.Helper()
	pid := byte(0xF0)
	src, _ := ax25.ParseAddress(source)
	dst, _ := ax25.ParseAddress("APRS")
	f := ax25.Frame{Destination: dst, Source: src, Digipeaters: vias, Type: ax25.TypeUI, PID: &pid, Payload: []byte("test")}
	b, err := ax25.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	return f, b
}

func TestRepeatsFirstPendingMatchingAlias(t *testing.T) {
	station, _ := ax25.ParseAddress("SP5ABC")
	bbs, _ := ax25.ParseAddress("SP5ABC-8")
	prior, _ := ax25.ParseAddress("OTHER-1")
	prior.Repeated = true
	f, raw := frame(t, "SQ5XYZ", prior, bbs)
	out, ok := New(station, bbs).Repeat(f, raw)
	if !ok {
		t.Fatal("frame not repeated")
	}
	got, err := ax25.Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Digipeaters[0].Repeated || !got.Digipeaters[1].Repeated {
		t.Fatalf("VIA path not marked correctly: %+v", got.Digipeaters)
	}
}

func TestRejectsWrongNextAliasDuplicateAndOwnFrame(t *testing.T) {
	station, _ := ax25.ParseAddress("SP5ABC")
	other, _ := ax25.ParseAddress("OTHER")
	d := New(station)
	f, raw := frame(t, "SQ5XYZ", station)
	if _, ok := d.Repeat(f, raw); !ok {
		t.Fatal("first frame rejected")
	}
	if _, ok := d.Repeat(f, raw); ok {
		t.Fatal("duplicate repeated")
	}
	wrong, wrongRaw := frame(t, "SQ5XYZ", other, station)
	if _, ok := d.Repeat(wrong, wrongRaw); ok {
		t.Fatal("non-first alias repeated")
	}
	own, ownRaw := frame(t, "SP5ABC", station)
	if _, ok := d.Repeat(own, ownRaw); ok {
		t.Fatal("own frame repeated")
	}
}

func TestConnectedModeRetriesAreNotSuppressed(t *testing.T) {
	station, _ := ax25.ParseAddress("SP5ABC")
	source, _ := ax25.ParseAddress("SQ5XYZ")
	destination, _ := ax25.ParseAddress("SR5DDD")
	original := ax25.Frame{Destination: destination, Source: source, Digipeaters: []ax25.Address{station}, Type: ax25.TypeSABM, PollFinal: true}
	raw, err := ax25.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	d := New(station)
	for attempt := 1; attempt <= 2; attempt++ {
		retry, err := ax25.Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := d.Repeat(retry, raw); !ok {
			t.Fatalf("connected-mode attempt %d was suppressed", attempt)
		}
	}
}
