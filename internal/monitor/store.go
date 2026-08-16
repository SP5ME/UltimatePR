package monitor

import (
	"fmt"
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
	Content     string    `json:"content"`
	Raw         string    `json:"raw"`
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
	raw := ""
	if b, err := ax25.Encode(f); err == nil {
		raw = fmt.Sprintf("% X", b)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, Entry{Time: time.Now(), Direction: direction, Port: port, Source: f.Source.String(), Destination: f.Destination.String(), Type: typeName(f.Type), Bytes: n, Content: string(f.Payload), Raw: raw})
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
