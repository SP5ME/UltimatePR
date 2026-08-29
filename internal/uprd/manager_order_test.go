package uprd

import (
	"bytes"
	"context"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/mheard"
)

func testFrame(t *testing.T, source string, heard []string) ax25.Frame {
	t.Helper()
	a := ax25.Address{Callsign: source, SSID: 7}
	payload, err := EncodePayload(a, "KO02MD", heard, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	pid := byte(0xF0)
	return ax25.Frame{Source: a, Destination: ax25.Address{Callsign: "UPR"}, Type: ax25.TypeUI, PID: &pid, Payload: payload}
}

func TestReporterReplacementPreservesOrder(t *testing.T) {
	store := mheard.New(50)
	store.Heard("SP5AAA-7", "2m")
	m := &Manager{local: ax25.Address{Callsign: "LOCAL"}, locator: "KO02MD", heard: store, reports: map[string]reportState{}}
	m.apply(queuedFrame{port: "2m", order: 1, frame: testFrame(t, "SP5AAA", []string{"SQ5BBB", "SR5DDD", "SP7ABC"})})
	m.apply(queuedFrame{port: "2m", order: 2, frame: testFrame(t, "SP5AAA", []string{"SQ5BBB", "SP7ABC"})})
	s := m.Snapshot([]string{"2m"})
	var reportEdges []Edge
	for _, e := range s.Edges {
		if e.SourceType == "uprd" {
			reportEdges = append(reportEdges, e)
		}
		if e.To == "SR5DDD" {
			t.Fatal("stale relation remained")
		}
	}
	if len(reportEdges) != 2 || reportEdges[0].To != "SQ5BBB" || reportEdges[1].To != "SP7ABC" || reportEdges[0].ReportOrder != 0 || reportEdges[1].ReportOrder != 1 {
		t.Fatalf("replacement/order lost: %#v", s.Edges)
	}
	entries := store.ListByPort("2m")
	if len(entries) != 1 || entries[0].OperatorPresent == nil || !*entries[0].OperatorPresent {
		t.Fatalf("direct UPRD status was not stored: %+v", entries)
	}
}

func TestOlderReportCannotOverwriteNewer(t *testing.T) {
	m := &Manager{local: ax25.Address{Callsign: "LOCAL"}, locator: "KO02MD", heard: mheard.New(50), reports: map[string]reportState{}}
	m.apply(queuedFrame{port: "2m", order: 9, frame: testFrame(t, "SP5AAA", []string{"SP7NEW"})})
	m.apply(queuedFrame{port: "2m", order: 8, frame: testFrame(t, "SP5AAA", []string{"SP7OLD"})})
	if got := m.reports["2m\x00SP5AAA"].Heard[0]; got != "SP7NEW" {
		t.Fatalf("older report overwrote newer: %s", got)
	}
}

func TestParseFrameRejectsViaAndWrongDestination(t *testing.T) {
	f := testFrame(t, "SP5AAA", nil)
	f.Digipeaters = []ax25.Address{{Callsign: "WIDE1"}}
	if _, ok := ParseFrame(f); ok {
		t.Fatal("frame with VIA accepted")
	}
	f = testFrame(t, "SP5AAA", nil)
	f.Destination.Callsign = "APRS"
	if _, ok := ParseFrame(f); ok {
		t.Fatal("wrong destination accepted")
	}
}

func TestBuildFrameUsesBinaryStatusAndKeepsAX25Contract(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		present bool
		status  byte
	}{{true, 0x00}, {false, 0x01}} {
		m := New(ctx, ax25.Address{Callsign: "SP5ME"}, "KO02MD", mheard.New(10), Config{
			Enabled: true, MHeardLimit: 5, OperatorPresent: func() bool { return tc.present },
		}, nil)
		wire, ok, err := m.BuildFrame("")
		if err != nil || !ok {
			t.Fatalf("build present=%v: ok=%v err=%v", tc.present, ok, err)
		}
		frame, err := ax25.Decode(wire)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Destination.String() != "UPR" || frame.Type != ax25.TypeUI || frame.PID == nil || *frame.PID != 0xF0 {
			t.Fatalf("AX.25 contract changed: %+v", frame)
		}
		sep := bytes.IndexByte(frame.Payload, '|')
		if sep < 0 || sep+1 >= len(frame.Payload) || frame.Payload[sep+1] != tc.status {
			t.Fatalf("payload=% X status=%02X", frame.Payload, tc.status)
		}
		if parsed, ok := ParseFrame(frame); !ok || parsed.OperatorPresent != tc.present {
			t.Fatalf("parsed=%+v ok=%v", parsed, ok)
		}
	}
}
