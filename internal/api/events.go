package api

import (
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// Broker never blocks a publisher. A subscriber that cannot keep up is
// disconnected; telemetry must never apply backpressure to radio processing.
type Broker struct {
	mu   sync.Mutex
	next uint64
	subs map[uint64]chan Event
}

func NewBroker() *Broker { return &Broker{subs: make(map[uint64]chan Event)} }

func (b *Broker) Subscribe(size int) (<-chan Event, func()) {
	if size < 1 {
		size = 64
	}
	b.mu.Lock()
	b.next++
	id := b.next
	ch := make(chan Event, size)
	b.subs[id] = ch
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			if c := b.subs[id]; c != nil {
				delete(b.subs, id)
				close(c)
			}
			b.mu.Unlock()
		})
	}
}

func (b *Broker) Publish(typ string, data any) {
	e := Event{Type: typ, Timestamp: time.Now().UTC(), Data: data}
	b.mu.Lock()
	for id, ch := range b.subs {
		select {
		case ch <- e:
		default:
			delete(b.subs, id)
			close(ch)
		}
	}
	b.mu.Unlock()
}
