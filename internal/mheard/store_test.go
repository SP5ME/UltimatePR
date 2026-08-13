package mheard

import "testing"

func TestBeaconKeepsFrameCountAndUpdatesText(t *testing.T) {
	s := New(10)
	s.Heard("SP5ABC", "radio")
	s.Beacon("SP5ABC", "radio", "SP5ABC\rDIGI\rUltimatePR")
	s.Beacon("SP5ABC", "radio", "new beacon")
	entries := s.List()
	if len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
	if entries[0].Frames != 1 {
		t.Fatalf("frames=%d", entries[0].Frames)
	}
	if entries[0].Beacon != "new beacon" {
		t.Fatalf("beacon=%q", entries[0].Beacon)
	}
}

func TestDirectEntryReplacesIndirectReport(t *testing.T) {
	s := New(10)
	s.Reported([]string{"SP5ABC"}, "SQ5VIA", "radio")
	entries := s.List()
	if len(entries) != 1 || !entries[0].Indirect || entries[0].Via != "SQ5VIA" {
		t.Fatalf("indirect entry=%+v", entries)
	}
	s.Heard("SP5ABC", "radio")
	entries = s.List()
	if len(entries) != 1 || entries[0].Indirect || entries[0].Frames != 1 {
		t.Fatalf("direct entry=%+v", entries)
	}
}
