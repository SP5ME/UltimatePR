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
	key := port + "\x00" + call
	e := s.entries[key]
	e.Callsign, e.Port, e.LastSeen, e.Frames = call, port, time.Now(), e.Frames+1
	s.entries[key] = e
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
