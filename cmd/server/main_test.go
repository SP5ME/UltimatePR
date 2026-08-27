package main

import (
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/mheard"
)

func TestBeaconText(t *testing.T) {
	source, _ := ax25.ParseAddress("SP5ME-8")
	now := time.Now()
	entries := []mheard.Entry{
		{Callsign: "SQ5AAA", LastSeen: now},
		{Callsign: "sq5aaa", Port: "other", LastSeen: now.Add(-time.Second)},
		{Callsign: "SP5BBB"}, {Callsign: "SP5CCC"}, {Callsign: "SP5DDD"},
		{Callsign: "SP5EEE"}, {Callsign: "SP5FFF"},
	}
	want := "KO02MD\rDIGI\rUltimatePR\rSQ5AAA,SP5BBB,SP5CCC,SP5DDD,SP5EEE"
	if got := beaconText(source, " ko02md ", entries, source); got != want {
		t.Fatalf("beacon text:\n got %q\nwant %q", got, want)
	}
}

func TestBeaconTextOmitsMissingOptionalFields(t *testing.T) {
	source, _ := ax25.ParseAddress("SP5ME")
	want := "DIGI\rUltimatePR"
	if got := beaconText(source, "", nil, source); got != want {
		t.Fatalf("beacon text: got %q, want %q", got, want)
	}
}

func TestParseUltimatePRBeaconAndExcludeOwnCalls(t *testing.T) {
	text := "SP5ME-8\rKO02MD\rPruszkow\rDIGI\rUltimatePR\rSQ5AAA,SP5ME,SP5ME-8,bad call,SP5CCC"
	got := parseUltimatePRBeacon(text)
	station, _ := ax25.ParseAddress("SP5ME")
	bbs, _ := ax25.ParseAddress("SP5ME-8")
	got = withoutCallsigns(got, station, bbs)
	want := []string{"SQ5AAA", "SP5CCC"}
	if len(got) != len(want) {
		t.Fatalf("calls=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls=%v", got)
		}
	}
	if got := parseUltimatePRBeacon("ordinary foreign beacon\rSQ5AAA"); len(got) != 0 {
		t.Fatalf("foreign beacon parsed: %v", got)
	}
}

func TestDigipeaterOutputPortsUsesMHeardAndDiscoversUnknown(t *testing.T) {
	heard := mheard.New(10)
	heard.Heard("SQ9MDD", "tnc-x")
	heard.Heard("SR5DDD", "tnc-y")
	available := []string{"tnc-z", "tnc-x", "tnc-y"}

	if got := digipeaterOutputPorts("tnc-x", "SR5DDD", heard, available); len(got) != 1 || got[0] != "tnc-y" {
		t.Fatalf("known cross-port outputs=%v", got)
	}
	if got := digipeaterOutputPorts("tnc-y", "SR5DDD", heard, available); len(got) != 1 || got[0] != "tnc-y" {
		t.Fatalf("known same-port outputs=%v", got)
	}
	got := digipeaterOutputPorts("tnc-x", "UNKNOWN", heard, available)
	want := []string{"tnc-x", "tnc-y", "tnc-z"}
	if len(got) != len(want) {
		t.Fatalf("unknown destination outputs=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unknown destination outputs=%v", got)
		}
	}
	if got := digipeaterOutputPorts("tnc-x", "SR5DDD", heard, []string{"tnc-x", "tnc-z"}); len(got) != 2 || got[0] != "tnc-x" || got[1] != "tnc-z" {
		t.Fatalf("stale route fallback outputs=%v", got)
	}
}

func TestDirectlyHeardRejectsRepeatedPath(t *testing.T) {
	if !directlyHeard(ax25.Frame{}) {
		t.Fatal("frame without a VIA path should be direct")
	}
	via, _ := ax25.ParseAddress("SP5ME")
	if !directlyHeard(ax25.Frame{Digipeaters: []ax25.Address{via}}) {
		t.Fatal("frame awaiting its first digipeater should be direct on the input port")
	}
	via.Repeated = true
	if directlyHeard(ax25.Frame{Digipeaters: []ax25.Address{via}}) {
		t.Fatal("echoed frame with a repeated VIA must not overwrite the direct port route")
	}
}
