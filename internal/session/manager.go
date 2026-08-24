package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
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

type SendPacketProgress struct {
	Packet int
	Total  int
	Data   []byte
	State  string
	Error  string
}

type acknowledgement struct {
	type_ ax25.Type
	nr    uint8
}

type controlEvent struct {
	type_    ax25.Type
	final    bool
	response bool
}

type Manager struct {
	mu          sync.Mutex
	local       ax25.Address
	state       State
	port        string
	remote      ax25.Address
	digipeaters []ax25.Address
	ports       map[string]Sender
	vs, vr      uint8
	control     chan controlEvent
	ack         chan acknowledgement
	subs        map[chan Event]struct{}
	t1          time.Duration
	n2          int
	paclen      int
	peerBusy    bool
	rejectSent  bool
}

func New(local ax25.Address, ports map[string]Sender) *Manager {
	return &Manager{local: local, state: Disconnected, ports: ports, control: make(chan controlEvent, 8), ack: make(chan acknowledgement, 8), subs: map[chan Event]struct{}{}, t1: 10 * time.Second, n2: 3, paclen: 128}
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

func (m *Manager) Connect(ctx context.Context, port, target string, via ...string) error {
	send, ok := m.ports[port]
	if !ok {
		return fmt.Errorf("unknown port %q", port)
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("target station callsign is required")
	}
	remote, err := ax25.ParseAddress(target)
	if err != nil {
		return err
	}
	digis := make([]ax25.Address, 0, len(via))
	for _, call := range via {
		if call == "" {
			continue
		}
		digi, parseErr := ax25.ParseAddress(call)
		if parseErr != nil {
			return fmt.Errorf("invalid digipeater %q: %w", call, parseErr)
		}
		digis = append(digis, digi)
	}
	if len(digis) > 8 {
		return errors.New("AX.25 allows at most 8 digipeaters")
	}
	m.mu.Lock()
	if m.state != Disconnected {
		m.mu.Unlock()
		return errors.New("session already active")
	}
	m.port = port
	m.remote = remote
	m.digipeaters = digis
	m.vs = 0
	m.vr = 0
	m.peerBusy = false
	m.rejectSent = false
	m.drain()
	m.mu.Unlock()
	m.setState(AwaitingConnection, "0/3")
	f := m.command(ax25.TypeSABM, true, nil, 0, 0)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			m.setState(Disconnected, err.Error())
			return err
		}
		select {
		case event := <-m.control:
			if event.type_ == ax25.TypeUA && event.final && event.response {
				m.setState(Connected, "Sesja AX.25 polaczona")
				return nil
			}
			if event.type_ == ax25.TypeDM && event.final && event.response {
				m.setState(Disconnected, "Stacja odrzucila polaczenie")
				return errors.New("connection rejected (DM)")
			}
		case <-time.After(m.t1):
			m.setState(AwaitingConnection, fmt.Sprintf("%d/%d", attempt+1, m.n2))
		case <-ctx.Done():
			m.setState(Disconnected, "Polaczenie anulowane")
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
	m.setState(AwaitingRelease, "Rozlaczanie")
	f := m.command(ax25.TypeDISC, true, nil, 0, 0)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			break
		}
		select {
		case event := <-m.control:
			if (event.type_ == ax25.TypeUA || event.type_ == ax25.TypeDM) && event.final && event.response {
				m.setState(Disconnected, "Rozlaczono")
				return nil
			}
		case <-time.After(m.t1):
		case <-ctx.Done():
			break
		}
	}
	m.setState(Disconnected, "Sesja zamknieta lokalnie")
	return nil
}

func (m *Manager) Send(ctx context.Context, data []byte) error {
	return m.SendWithProgress(ctx, data, nil)
}

// SendWithProgress splits data according to paclen, preferring whitespace
// boundaries so words remain intact, and reports every AX.25 I frame.
func (m *Manager) SendWithProgress(ctx context.Context, data []byte, progress func(SendPacketProgress)) error {
	chunks := splitAX25Payload(data, m.paclen)
	for packet, chunk := range chunks {
		if progress != nil {
			progress(SendPacketProgress{Packet: packet + 1, Total: len(chunks), Data: chunk, State: "sending"})
		}
		if err := m.sendChunk(ctx, chunk); err != nil {
			if progress != nil {
				progress(SendPacketProgress{Packet: packet + 1, Total: len(chunks), Data: chunk, State: "error", Error: err.Error()})
			}
			return err
		}
		if progress != nil {
			progress(SendPacketProgress{Packet: packet + 1, Total: len(chunks), Data: chunk, State: "sent"})
		}
	}
	return nil
}

func splitAX25Payload(data []byte, paclen int) [][]byte {
	if len(data) == 0 {
		return nil
	}
	if paclen < 1 {
		paclen = 1
	}
	chunks := make([][]byte, 0, (len(data)+paclen-1)/paclen)
	for len(data) > 0 {
		n := len(data)
		if n > paclen {
			n = paclen
			for i := paclen - 1; i > 0; i-- {
				if data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n' {
					n = i + 1
					break
				}
			}
		}
		chunks = append(chunks, append([]byte(nil), data[:n]...))
		data = data[n:]
	}
	return chunks
}

// KeepAlive sends AX.25 supervisory polls while an established link is idle.
// It does not inject any text into the remote terminal.
func (m *Manager) KeepAlive(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.state != Connected {
				m.mu.Unlock()
				continue
			}
			send, nr := m.ports[m.port], m.vr
			m.mu.Unlock()
			_ = m.sendFrame(ctx, send, m.command(ax25.TypeRR, true, nil, 0, nr))
		}
	}
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
		if err := m.waitRemoteReady(ctx); err != nil {
			return err
		}
		if err := m.sendFrame(ctx, send, f); err != nil {
			return err
		}
		timer := time.NewTimer(m.t1)
	waitForAck:
		for {
			select {
			case ack := <-m.ack:
				if ack.type_ == ax25.TypeRNR {
					m.mu.Lock()
					m.peerBusy = true
					m.mu.Unlock()
					if ack.nr == expected {
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						m.mu.Lock()
						m.vs = expected
						m.mu.Unlock()
						return nil
					}
					continue
				}
				if ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ && ack.nr == expected {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					m.mu.Lock()
					m.vs = expected
					m.mu.Unlock()
					return nil
				}
				if (ack.type_ == ax25.TypeREJ || ack.type_ == ax25.TypeSREJ) && ack.nr == ns {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					break waitForAck
				}
				// A stale RR or piggybacked N(R) does not acknowledge this
				// frame and must not trigger an immediate retransmission.
			case <-timer.C:
				break waitForAck
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			}
		}
	}
	return errors.New("AX.25 data acknowledgement timeout")
}

func (m *Manager) waitRemoteReady(ctx context.Context) error {
	for attempt := 0; attempt < m.n2; attempt++ {
		m.mu.Lock()
		busy := m.peerBusy
		send, nr := m.ports[m.port], m.vr
		m.mu.Unlock()
		if !busy {
			return nil
		}
		m.drainAck()
		if err := m.sendFrame(ctx, send, m.command(ax25.TypeRR, true, nil, 0, nr)); err != nil {
			return err
		}
		timer := time.NewTimer(m.t1)
		for {
			select {
			case ack := <-m.ack:
				if ack.type_ == ax25.TypeRR {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					m.mu.Lock()
					m.peerBusy = false
					m.mu.Unlock()
					return nil
				}
			case <-timer.C:
				goto retry
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			}
		}
	retry:
	}
	return errors.New("AX.25 remote receiver remains busy")
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
		response := (!f.Destination.CommandResponse && f.Source.CommandResponse) || f.Destination.CommandResponse == f.Source.CommandResponse
		select {
		case m.control <- controlEvent{type_: f.Type, final: f.PollFinal, response: response}:
		default:
		}
	case ax25.TypeDISC:
		_ = m.sendResponse(context.Background(), port, ax25.TypeUA, true, 0)
		m.setState(Disconnected, "Zdalna stacja rozlaczyla sesje")
	case ax25.TypeRR:
		m.mu.Lock()
		m.peerBusy = false
		m.mu.Unlock()
		select {
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR}:
		default:
		}
		if f.PollFinal && f.Destination.CommandResponse {
			m.mu.Lock()
			nr := m.vr
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeRR, true, nr)
		}
	case ax25.TypeRNR:
		m.mu.Lock()
		m.peerBusy = true
		m.mu.Unlock()
		select {
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR}:
		default:
		}
		if f.PollFinal && f.Destination.CommandResponse {
			m.mu.Lock()
			nr := m.vr
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeRR, true, nr)
		}
		m.emit(Event{Type: "notice", Message: "Zdalna stacja chwilowo nie moze odbierac"})
	case ax25.TypeREJ, ax25.TypeSREJ:
		select {
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR}:
		default:
		}
	case ax25.TypeI:
		if state != Connected {
			return
		}
		m.mu.Lock()
		if f.NS == m.vr {
			m.vr = (m.vr + 1) & 7
			m.rejectSent = false
			m.mu.Unlock()
			m.emit(Event{Type: "data", Data: append([]byte(nil), f.Payload...)})
		} else {
			alreadyRejected := m.rejectSent
			m.rejectSent = true
			m.mu.Unlock()
			if !alreadyRejected {
				m.mu.Lock()
				nr := m.vr
				m.mu.Unlock()
				_ = m.sendResponse(context.Background(), port, ax25.TypeREJ, f.PollFinal, nr)
			}
			return
		}
		if f.NR <= 7 {
			select {
			case m.ack <- acknowledgement{type_: f.Type, nr: f.NR}:
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
	m.mu.Lock()
	digis := append([]ax25.Address(nil), m.digipeaters...)
	m.mu.Unlock()
	return ax25.Frame{Destination: remote, Source: local, Digipeaters: digis, Type: t, PollFinal: pf, PID: pid, NS: ns, NR: nr}
}
func (m *Manager) sendResponse(ctx context.Context, port string, t ax25.Type, pf bool, nr uint8) error {
	m.mu.Lock()
	send := m.ports[port]
	remote, local := m.remote, m.local
	digis := append([]ax25.Address(nil), m.digipeaters...)
	m.mu.Unlock()
	remote.CommandResponse = false
	local.CommandResponse = true
	return m.sendFrame(ctx, send, ax25.Frame{Destination: remote, Source: local, Digipeaters: digis, Type: t, PollFinal: pf, NR: nr})
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
