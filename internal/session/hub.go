package session

import (
	"sync"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

// Hub owns independent outgoing AX.25 link state machines sharing one radio.
type Hub struct {
	mu       sync.RWMutex
	local    ax25.Address
	ports    map[string]Sender
	sessions map[*Manager]struct{}
}

func NewHub(local ax25.Address, ports map[string]Sender) *Hub {
	return &Hub{local: local, ports: ports, sessions: make(map[*Manager]struct{})}
}

func (h *Hub) NewSession() (*Manager, func()) {
	m := New(h.local, h.ports)
	h.mu.Lock()
	h.sessions[m] = struct{}{}
	h.mu.Unlock()
	return m, func() {
		h.mu.Lock()
		delete(h.sessions, m)
		h.mu.Unlock()
	}
}

func (h *Hub) Handle(port string, f ax25.Frame) {
	h.mu.RLock()
	all := make([]*Manager, 0, len(h.sessions))
	for m := range h.sessions {
		all = append(all, m)
	}
	h.mu.RUnlock()
	for _, m := range all {
		m.Handle(port, f)
	}
}
