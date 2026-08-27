package uprd

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/mheard"
)

func testFrame(t *testing.T, source string, heard []string) ax25.Frame {
	t.Helper()
	a := ax25.Address{Callsign: source}
	payload, err := EncodePayload(a, "KO02MD", heard, 10)
	if err != nil {
		t.Fatal(err)
	}
	pid := byte(0xF0)
	return ax25.Frame{Source: a, Destination: ax25.Address{Callsign: "UPR"}, Type: ax25.TypeUI, PID: &pid, Payload: []byte(payload)}
}

func TestReporterReplacementPreservesOrder(t *testing.T) {
	m := &Manager{local: ax25.Address{Callsign: "LOCAL"}, locator: "KO02MD", heard: mheard.New(50), reports: map[string]reportState{}}
	m.apply(queuedFrame{port: "2m", order: 1, frame: testFrame(t, "SP5AAA", []string{"SQ5BBB", "SR5DDD", "SP7ABC"})})
	m.apply(queuedFrame{port: "2m", order: 2, frame: testFrame(t, "SP5AAA", []string{"SQ5BBB", "SP7ABC"})})
	s := m.Snapshot([]string{"2m"})
	if len(s.Edges) != 2 || s.Edges[0].To != "SQ5BBB" || s.Edges[1].To != "SP7ABC" || s.Edges[0].ReportOrder != 0 || s.Edges[1].ReportOrder != 1 {
		t.Fatalf("replacement/order lost: %#v", s.Edges)
	}
	for _, e := range s.Edges {
		if e.To == "SR5DDD" {
			t.Fatal("stale relation remained")
		}
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
