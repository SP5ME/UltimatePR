package mheard

import (
	"testing"
	"time"
)

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

func TestBeaconExpiresAndUpdates(t *testing.T) {
	now := time.Unix(1000, 0)
	s := New(10)
	s.now = func() time.Time { return now }
	s.Heard("SP5ABC", "radio")
	s.Beacon("SP5ABC", "radio", "first beacon")
	now = now.Add(20 * time.Minute)
	s.Beacon("SP5ABC", "radio", "second beacon")
	entries := s.List()
	if len(entries) != 1 || entries[0].Beacon != "second beacon" {
		t.Fatalf("unexpected beacon update: %+v", entries)
	}
	now = now.Add(46 * time.Minute)
	entries = s.List()
	if len(entries) != 0 {
		t.Fatalf("expired entry still present: %+v", entries)
	}
}

func TestOldEntryDisappearsFromList(t *testing.T) {
	now := time.Unix(2000, 0)
	s := New(10)
	s.now = func() time.Time { return now }
	s.Heard("SP5ABC", "radio")
	if entries := s.List(); len(entries) != 1 {
		t.Fatalf("entries=%d", len(entries))
	}
	now = now.Add(46 * time.Minute)
	if entries := s.List(); len(entries) != 0 {
		t.Fatalf("stale entry still visible: %+v", entries)
	}
}
