package monitor

import (
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

type Entry struct {
	Time        time.Time `json:"time"`
	Direction   string    `json:"direction"`
	Port        string    `json:"port"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Type        string    `json:"type"`
	Bytes       int       `json:"bytes"`
}
type Store struct {
	mu    sync.Mutex
	items []Entry
	limit int
}

func New(n int) *Store {
	if n < 1 {
		n = 200
	}
	return &Store{limit: n}
}
func (s *Store) Add(direction, port string, f ax25.Frame, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, Entry{time.Now(), direction, port, f.Source.String(), f.Destination.String(), typeName(f.Type), n})
	if len(s.items) > s.limit {
		s.items = s.items[len(s.items)-s.limit:]
	}
}
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.items))
	for i := range s.items {
		out[len(out)-1-i] = s.items[i]
	}
	return out
}
func typeName(t ax25.Type) string {
	names := []string{"I", "RR", "RNR", "REJ", "SABM", "DISC", "DM", "UA", "UI"}
	if int(t) < len(names) {
		return names[t]
	}
	return "?"
}
