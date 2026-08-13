package mheard

import (
	"sort"
	"sync"
	"time"
)

type Entry struct {
	Callsign string    `json:"callsign"`
	Port     string    `json:"port"`
	LastSeen time.Time `json:"last_seen"`
	Frames   uint64    `json:"frames"`
	Beacon   string    `json:"beacon,omitempty"`
	Indirect bool      `json:"indirect,omitempty"`
	Via      string    `json:"via,omitempty"`
}

// Beacon records the latest beacon text without changing the frame counter.
func (s *Store) Beacon(call, port, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.Beacon = call, port, text
	if e.LastSeen.IsZero() {
		e.LastSeen = time.Now()
	}
	s.entries[key] = e
}

type Store struct {
	mu      sync.Mutex
	entries map[string]Entry
	limit   int
}

func New(limit int) *Store {
	if limit < 1 {
		limit = 100
	}
	return &Store{entries: make(map[string]Entry), limit: limit}
}

func (s *Store) Heard(call, port string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.entries {
		if entry.Callsign == call && entry.Indirect {
			delete(s.entries, key)
		}
	}
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.LastSeen, e.Frames, e.Indirect, e.Via = call, port, time.Now(), e.Frames+1, false, ""
	s.entries[key] = e
	s.trim()
}

// Reported adds stations announced in an UltimatePR beacon. Directly heard
// entries always take precedence over these indirect observations.
func (s *Store) Reported(calls []string, via, port string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, call := range calls {
		direct := false
		for _, entry := range s.entries {
			if entry.Callsign == call && !entry.Indirect {
				direct = true
				break
			}
		}
		if direct {
			continue
		}
		key := "indirect\x00" + call
		e := s.entries[key]
		e.Callsign, e.Port, e.LastSeen, e.Indirect, e.Via = call, port, now, true, via
		s.entries[key] = e
	}
	s.trim()
}

func (s *Store) trim() {
	if len(s.entries) > s.limit {
		var oldestKey string
		var oldest time.Time
		for k, v := range s.entries {
			if oldestKey == "" || v.LastSeen.Before(oldest) {
				oldestKey, oldest = k, v.LastSeen
			}
		}
		delete(s.entries, oldestKey)
	}
}

func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}
