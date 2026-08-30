package mheard

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

const entryTTL = 45 * time.Minute

type Entry struct {
	Callsign        string    `json:"callsign"`
	Port            string    `json:"port"`
	LastSeen        time.Time `json:"last_seen"`
	Frames          uint64    `json:"frames"`
	Beacon          string    `json:"beacon,omitempty"`
	Indirect        bool      `json:"indirect,omitempty"`
	Via             string    `json:"via,omitempty"`
	SourceType      string    `json:"source_type,omitempty"`
	OperatorPresent *bool     `json:"operator_present,omitempty"`
}

// Beacon records the latest beacon text without changing the frame counter.
func (s *Store) Beacon(call, port, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.SourceType = call, port, "beacon"
	if e.LastSeen.IsZero() {
		e.LastSeen = now
	}
	s.entries[key] = e
	if s.beacons == nil {
		s.beacons = make(map[string]beaconRecord)
	}
	s.beacons[key] = beaconRecord{text: text, seen: now}
	s.pruneLocked(now)
}

type Store struct {
	mu      sync.Mutex
	entries map[string]Entry
	beacons map[string]beaconRecord
	limit   int
	now     func() time.Time
}

type beaconRecord struct {
	text string
	seen time.Time
}

func New(limit int) *Store {
	if limit < 1 {
		limit = 100
	}
	return &Store{entries: make(map[string]Entry), beacons: make(map[string]beaconRecord), limit: limit, now: time.Now}
}

// SaveSnapshot persists the current MHEARD view across a planned process
// restart. It is deliberately separate from normal MHEARD operation: an
// unplanned start without a fresh snapshot still begins with an empty list.
func (s *Store) SaveSnapshot(path string) error {
	entries := s.List()
	b, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	// Windows does not replace an existing destination with os.Rename. A stale
	// snapshot is expendable because the freshly written temporary file is the
	// authoritative state for this planned restart.
	_ = os.Remove(path)
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// LoadSnapshot restores non-expired entries that still refer to configured
// ports. The caller removes the one-shot file after a successful load.
func (s *Store) LoadSnapshot(path string, ports map[string]bool) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []Entry
	if err = json.Unmarshal(b, &entries); err != nil {
		return err
	}
	now := s.nowTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		if e.Callsign == "" || e.Port == "" || !ports[e.Port] || e.LastSeen.IsZero() || now.Sub(e.LastSeen) > entryTTL || e.LastSeen.After(now.Add(time.Minute)) {
			continue
		}
		key := e.Port + "\x00" + e.Callsign
		beacon := e.Beacon
		e.Beacon = ""
		s.entries[key] = e
		if beacon != "" {
			s.beacons[key] = beaconRecord{text: beacon, seen: e.LastSeen}
		}
	}
	s.pruneLocked(now)
	return nil
}

func (s *Store) Heard(call, port string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.LastSeen, e.Frames, e.Indirect, e.Via, e.SourceType = call, port, now, e.Frames+1, false, "", "direct"
	s.entries[key] = e
	s.pruneLocked(now)
}

// SetOperatorPresent associates the latest directly heard UPRD status with a station.
func (s *Store) SetOperatorPresent(call, port string, present bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := port + "\x00" + call
	e, ok := s.entries[key]
	if !ok || e.Indirect {
		return
	}
	e.OperatorPresent = &present
	s.entries[key] = e
}

// HeardVia records a frame received through digipeaters. The via argument is
// the already reversed return path. A directly heard entry always takes
// precedence, so an echoed digipeated frame cannot replace a direct route.
func (s *Store) HeardVia(call, port, via string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.LastSeen, e.Frames, e.Indirect, e.Via, e.SourceType = call, port, now, e.Frames+1, true, via, "via"
	s.entries[key] = e
	s.pruneLocked(now)
}

// Reported adds stations announced in an UltimatePR beacon. Directly heard
// entries always take precedence over these indirect observations.
func (s *Store) Reported(calls []string, via, port string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	for _, call := range calls {
		key := port + "\x00" + call
		if existing, ok := s.entries[key]; ok && existing.SourceType != "reported" {
			continue
		}
		e := s.entries[key]
		e.Callsign, e.Port, e.LastSeen, e.Indirect, e.Via, e.SourceType = call, port, now, true, via, "reported"
		s.entries[key] = e
	}
	s.pruneLocked(now)
}

func (s *Store) nowTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Store) pruneLocked(now time.Time) {
	if len(s.beacons) > 0 {
		for key, b := range s.beacons {
			if now.Sub(b.seen) > entryTTL {
				delete(s.beacons, key)
			}
		}
	}
	for key, e := range s.entries {
		if now.Sub(e.LastSeen) > entryTTL {
			delete(s.entries, key)
			delete(s.beacons, key)
		}
	}
	if len(s.entries) > s.limit {
		var oldestKey string
		var oldest time.Time
		for k, v := range s.entries {
			if oldestKey == "" || v.LastSeen.Before(oldest) {
				oldestKey, oldest = k, v.LastSeen
			}
		}
		delete(s.entries, oldestKey)
		delete(s.beacons, oldestKey)
	}
}

// DirectPort returns the port on which call was heard directly most recently.
// Indirect beacon reports are never used for cross-port AX.25 digipeating.
func (s *Store) DirectPort(call string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	s.pruneLocked(now)
	var newest Entry
	found := false
	for _, entry := range s.entries {
		if entry.Callsign != call || entry.Indirect || entry.Port == "" {
			continue
		}
		if !found || entry.LastSeen.After(newest.LastSeen) {
			newest, found = entry, true
		}
	}
	return newest.Port, found
}
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	s.pruneLocked(now)
	out := make([]Entry, 0, len(s.entries))
	for key, e := range s.entries {
		if b, ok := s.beacons[key]; ok && now.Sub(b.seen) <= entryTTL {
			e.Beacon = b.text
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

func (s *Store) ListByPort(port string) []Entry {
	return s.ListByPortFilter(func(e Entry) bool { return e.Port == port })
}

func (s *Store) ListByPortFilter(match func(Entry) bool) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowTime()
	s.pruneLocked(now)
	out := make([]Entry, 0, len(s.entries))
	for key, e := range s.entries {
		if match != nil && !match(e) {
			continue
		}
		if b, ok := s.beacons[key]; ok && now.Sub(b.seen) <= entryTTL {
			e.Beacon = b.text
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}
