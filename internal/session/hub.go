package session

import (
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

// Hub owns independent outgoing AX.25 link state machines sharing one radio.
type Hub struct {
	mu       sync.RWMutex
	local    ax25.Address
	ports    map[string]Sender
	sessions map[*Manager]struct{}
	t1       time.Duration
	n2       int
	n1       int
}

func NewHub(local ax25.Address, ports map[string]Sender) *Hub {
	return &Hub{local: local, ports: ports, sessions: make(map[*Manager]struct{}), t1: defaultT1, n2: 10, n1: defaultN1}
}

func (h *Hub) Configure(t1 time.Duration, n2, n1 int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t1 > 0 {
		h.t1 = t1
	}
	if n2 > 0 {
		h.n2 = n2
	}
	if n1 > 0 {
		h.n1 = n1
	}
}

func (h *Hub) NewSession() (*Manager, func()) {
	m := New(h.local, h.ports)
	h.mu.Lock()
	m.Configure(h.t1, h.n2, h.n1)
	h.sessions[m] = struct{}{}
	h.mu.Unlock()
	return m, func() {
		h.mu.Lock()
		delete(h.sessions, m)
		h.mu.Unlock()
	}
}

func (h *Hub) Handle(port string, f ax25.Frame) bool {
	h.mu.RLock()
	all := make([]*Manager, 0, len(h.sessions))
	for m := range h.sessions {
		all = append(all, m)
	}
	h.mu.RUnlock()
	for _, m := range all {
		if m.Handle(port, f) {
			return true
		}
	}
	return false
}
