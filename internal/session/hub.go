package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

// Hub owns independent outgoing AX.25 link state machines sharing one radio.
type Hub struct {
	mu           sync.RWMutex
	local        ax25.Address
	ports        map[string]Sender
	sessions     map[*Manager]hubSession
	next         uint64
	t1           time.Duration
	n2           int
	n1           int
	localResolve func(ax25.Address) bool
	localSend    LocalSender
}
type hubSession struct {
	id      string
	created time.Time
}
type Snapshot struct {
	ID      string
	State   State
	Created time.Time
}

func NewHub(local ax25.Address, ports map[string]Sender) *Hub {
	return &Hub{local: local, ports: ports, sessions: make(map[*Manager]hubSession), t1: defaultT1, n2: 10, n1: defaultN1}
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

// SetLocalDelivery configures transparent local AX.25 routing. The resolver
// is intentionally owned by the session boundary, not by the AX.25 state
// machine or service implementations.
func (h *Hub) SetLocalDelivery(resolve func(ax25.Address) bool, send LocalSender) {
	h.mu.Lock()
	h.localResolve = resolve
	h.localSend = send
	h.mu.Unlock()
}

func (h *Hub) NewSession() (*Manager, func()) {
	h.mu.Lock()
	m := New(h.local, h.ports)
	m.localResolve = h.localResolve
	m.localSend = h.localSend
	h.next++
	m.Configure(h.t1, h.n2, h.n1)
	h.sessions[m] = hubSession{id: fmt.Sprintf("session-%d", h.next), created: time.Now().UTC()}
	h.mu.Unlock()
	return m, func() {
		h.mu.Lock()
		delete(h.sessions, m)
		h.mu.Unlock()
	}
}

// Snapshot returns stable, intentionally small telemetry records for API/UI
// consumers without exposing Manager internals.
func (h *Hub) Snapshot() []Snapshot {
	h.mu.RLock()
	items := make([]struct {
		m *Manager
		s hubSession
	}, 0, len(h.sessions))
	for m, s := range h.sessions {
		items = append(items, struct {
			m *Manager
			s hubSession
		}{m, s})
	}
	h.mu.RUnlock()
	out := make([]Snapshot, 0, len(items))
	for _, item := range items {
		out = append(out, Snapshot{ID: item.s.id, State: item.m.State(), Created: item.s.created})
	}
	return out
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
