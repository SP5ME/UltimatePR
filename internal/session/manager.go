package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/packet-radio/modernbbs/internal/ax25"
)

type State string

const (
	Disconnected       State = "disconnected"
	AwaitingConnection State = "awaiting_connection"
	Connected          State = "connected"
	AwaitingRelease    State = "awaiting_release"
)

type Event struct {
	Type    string
	State   State
	Data    []byte
	Message string
}
type Sender func(context.Context, []byte) error

type Manager struct {
	mu      sync.Mutex
	local   ax25.Address
	state   State
	port    string
	remote  ax25.Address
	ports   map[string]Sender
	vs, vr  uint8
	control chan ax25.Type
	ack     chan uint8
	subs    map[chan Event]struct{}
	t1      time.Duration
	n2      int
	paclen  int
}

func New(local ax25.Address, ports map[string]Sender) *Manager {
	return &Manager{local: local, state: Disconnected, ports: ports, control: make(chan ax25.Type, 8), ack: make(chan uint8, 8), subs: map[chan Event]struct{}{}, t1: 3 * time.Second, n2: 5, paclen: 128}
}

func (m *Manager) Subscribe() (<-chan Event, func()) {
	c := make(chan Event, 64)
	m.mu.Lock()
	m.subs[c] = struct{}{}
	state := m.state
	m.mu.Unlock()
	c <- Event{Type: "state", State: state}
	return c, func() {
		m.mu.Lock()
		if _, ok := m.subs[c]; ok {
			delete(m.subs, c)
			close(c)
		}
		m.mu.Unlock()
	}
}
func (m *Manager) emit(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for c := range m.subs {
		select {
		case c <- e:
		default:
		}
	}
}
func (m *Manager) setState(s State, msg string) {
	m.mu.Lock()
	m.state = s
	m.mu.Unlock()
	m.emit(Event{Type: "state", State: s, Message: msg})
}
func (m *Manager) State() State { m.mu.Lock(); defer m.mu.Unlock(); return m.state }

func (m *Manager) Connect(ctx context.Context, port, target string) error {
	send, ok := m.ports[port]
	if !ok {
		return fmt.Errorf("unknown port %q", port)
	}
	remote, err := ax25.ParseAddress(target)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.state != Disconnected {
		m.mu.Unlock()
		return errors.New("session already active")
	}
	m.port = port
	m.remote = remote
	m.vs = 0
	m.vr = 0
	m.drain()
	m.mu.Unlock()
	m.setState(AwaitingConnection, "Wysyłanie SABM")
	f := m.command(ax25.TypeSABM, true, nil, 0, 0)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			m.setState(Disconnected, err.Error())
			return err
		}
		select {
		case typ := <-m.control:
			if typ == ax25.TypeUA {
				m.setState(Connected, "Sesja AX.25 połączona")
				return nil
			}
			if typ == ax25.TypeDM {
				m.setState(Disconnected, "Stacja odrzuciła połączenie")
				return errors.New("connection rejected (DM)")
			}
		case <-time.After(m.t1):
		case <-ctx.Done():
			m.setState(Disconnected, "Połączenie anulowane")
			return ctx.Err()
		}
	}
	m.setState(Disconnected, "Brak odpowiedzi UA")
	return errors.New("AX.25 connect timeout")
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	if m.state == Disconnected {
		m.mu.Unlock()
		return nil
	}
	send := m.ports[m.port]
	m.drain()
	m.mu.Unlock()
	m.setState(AwaitingRelease, "Rozłączanie")
	f := m.command(ax25.TypeDISC, true, nil, 0, 0)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			break
		}
		select {
		case typ := <-m.control:
			if typ == ax25.TypeUA || typ == ax25.TypeDM {
				m.setState(Disconnected, "Rozłączono")
				return nil
			}
		case <-time.After(m.t1):
		case <-ctx.Done():
			break
		}
	}
	m.setState(Disconnected, "Sesja zamknięta lokalnie")
	return nil
}

func (m *Manager) Send(ctx context.Context, data []byte) error {
	for len(data) > 0 {
		n := len(data)
		if n > m.paclen {
			n = m.paclen
		}
		if err := m.sendChunk(ctx, data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
func (m *Manager) sendChunk(ctx context.Context, data []byte) error {
	m.mu.Lock()
	if m.state != Connected {
		m.mu.Unlock()
		return errors.New("AX.25 session is not connected")
	}
	send := m.ports[m.port]
	ns, nr := m.vs, m.vr
	expected := (m.vs + 1) & 7
	m.drainAck()
	m.mu.Unlock()
	pid := byte(0xF0)
	f := m.command(ax25.TypeI, false, &pid, ns, nr)
	f.Payload = append([]byte(nil), data...)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			return err
		}
		select {
		case nr := <-m.ack:
			if nr == expected {
				m.mu.Lock()
				m.vs = expected
				m.mu.Unlock()
				return nil
			}
		case <-time.After(m.t1):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("AX.25 data acknowledgement timeout")
}

func (m *Manager) Handle(port string, f ax25.Frame) {
	m.mu.Lock()
	active := port == m.port && f.Source.String() == m.remote.String() && f.Destination.String() == m.local.String()
	state := m.state
	m.mu.Unlock()
	if !active {
		return
	}
	switch f.Type {
	case ax25.TypeUA, ax25.TypeDM:
		select {
		case m.control <- f.Type:
		default:
		}
	case ax25.TypeDISC:
		_ = m.sendResponse(context.Background(), port, ax25.TypeUA, true, 0)
		m.setState(Disconnected, "Zdalna stacja rozłączyła sesję")
	case ax25.TypeRR:
		select {
		case m.ack <- f.NR:
		default:
		}
	case ax25.TypeRNR:
		m.emit(Event{Type: "notice", Message: "Zdalna stacja chwilowo nie może odbierać"})
	case ax25.TypeREJ:
		select {
		case m.ack <- f.NR:
		default:
		}
	case ax25.TypeI:
		if state != Connected {
			return
		}
		m.mu.Lock()
		if f.NS == m.vr {
			m.vr = (m.vr + 1) & 7
			m.mu.Unlock()
			m.emit(Event{Type: "data", Data: append([]byte(nil), f.Payload...)})
		} else {
			m.mu.Unlock()
		}
		if f.NR <= 7 {
			select {
			case m.ack <- f.NR:
			default:
			}
		}
		m.mu.Lock()
		nr := m.vr
		m.mu.Unlock()
		_ = m.sendResponse(context.Background(), port, ax25.TypeRR, f.PollFinal, nr)
	}
}

func (m *Manager) command(t ax25.Type, pf bool, pid *byte, ns, nr uint8) ax25.Frame {
	m.mu.Lock()
	remote, local := m.remote, m.local
	m.mu.Unlock()
	remote.CommandResponse = true
	local.CommandResponse = false
	return ax25.Frame{Destination: remote, Source: local, Type: t, PollFinal: pf, PID: pid, NS: ns, NR: nr}
}
func (m *Manager) sendResponse(ctx context.Context, port string, t ax25.Type, pf bool, nr uint8) error {
	m.mu.Lock()
	send := m.ports[port]
	remote, local := m.remote, m.local
	m.mu.Unlock()
	remote.CommandResponse = false
	local.CommandResponse = true
	return m.sendFrame(ctx, send, ax25.Frame{Destination: remote, Source: local, Type: t, PollFinal: pf, NR: nr})
}
func (m *Manager) sendFrame(ctx context.Context, send Sender, f ax25.Frame) error {
	if send == nil {
		return errors.New("port unavailable")
	}
	b, err := ax25.Encode(f)
	if err != nil {
		return err
	}
	return send(ctx, b)
}
func (m *Manager) drain() {
	for {
		select {
		case <-m.control:
		default:
			return
		}
	}
}
func (m *Manager) drainAck() {
	for {
		select {
		case <-m.ack:
		default:
			return
		}
	}
}
