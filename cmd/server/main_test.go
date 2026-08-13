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
	want := "SP5ME-8\rKO02MD\rPruszkow\rDIGI\rUltimatePR\rSQ5AAA,SP5BBB,SP5CCC,SP5DDD,SP5EEE"
	if got := beaconText(source, " ko02md ", " Pruszkow ", entries, source); got != want {
		t.Fatalf("beacon text:\n got %q\nwant %q", got, want)
	}
}

func TestBeaconTextOmitsMissingOptionalFields(t *testing.T) {
	source, _ := ax25.ParseAddress("SP5ME")
	want := "SP5ME\rDIGI\rUltimatePR"
	if got := beaconText(source, "", "", nil, source); got != want {
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
