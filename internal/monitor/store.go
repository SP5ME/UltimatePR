package monitor

import (
	"fmt"
	"strings"
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
	Via         string    `json:"via,omitempty"`
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
	s.items = append(s.items, Entry{Time: time.Now(), Direction: direction, Port: port, Source: f.Source.String(), Destination: f.Destination.String(), Via: formatVia(f.Digipeaters), Type: typeName(f.Type), Bytes: n, Content: string(f.Payload), Raw: raw})
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
	switch t {
	case ax25.TypeI:
		return "I"
	case ax25.TypeRR:
		return "RR"
	case ax25.TypeRNR:
		return "RNR"
	case ax25.TypeREJ:
		return "REJ"
	case ax25.TypeSREJ:
		return "SREJ"
	case ax25.TypeSABM:
		return "SABM"
	case ax25.TypeSABME:
		return "SABME"
	case ax25.TypeDISC:
		return "DISC"
	case ax25.TypeDM:
		return "DM"
	case ax25.TypeUA:
		return "UA"
	case ax25.TypeUI:
		return "UI"
	case ax25.TypeFRMR:
		return "FRMR"
	case ax25.TypeXID:
		return "XID"
	case ax25.TypeTEST:
		return "TEST"
	default:
		return "?"
	}
}

func formatVia(digis []ax25.Address) string {
	if len(digis) == 0 {
		return ""
	}
	out := make([]string, 0, len(digis))
	for _, digi := range digis {
		name := digi.String()
		if digi.Repeated {
			name += "*"
		}
		out = append(out, name)
	}
	return strings.Join(out, ", ")
}
