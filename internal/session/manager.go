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
	TimerRecovery      State = "timer_recovery"
)

type Event struct {
	Type    string
	State   State
	Data    []byte
	Message string
}
type Sender func(context.Context, []byte) error

const (
	// AX.25 v2.2 XID/default parameter set: T1=3000 ms and N2=10.
	// T1 may later be increased for the selected signalling rate/digi path.
	defaultT1 = 3 * time.Second
	// T3 is locally defined but must be greater than T1.
	defaultT3 = 5 * time.Minute
	// AX.25 v2.2 defines 256 octets as the default N1.
	defaultN1 = 256
)

type SendPacketProgress struct {
	Packet int
	Total  int
	Data   []byte
	State  string
	Error  string
}

type acknowledgement struct {
	type_    ax25.Type
	nr       uint8
	final    bool
	response bool
}

type controlEvent struct {
	type_    ax25.Type
	final    bool
	response bool
}

type Manager struct {
	mu          sync.Mutex
	operation   sync.Mutex
	local       ax25.Address
	state       State
	port        string
	remote      ax25.Address
	digipeaters []ax25.Address
	ports       map[string]Sender
	vs, va, vr  uint8
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
	return &Manager{local: local, state: Disconnected, ports: ports, control: make(chan controlEvent, 8), ack: make(chan acknowledgement, 8), subs: map[chan Event]struct{}{}, t1: defaultT1, n2: 10, paclen: defaultN1}
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
	m.operation.Lock()
	defer m.operation.Unlock()
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
	m.va = 0
	m.vr = 0
	m.peerBusy = false
	m.rejectSent = false
	m.drain()
	m.mu.Unlock()
	m.setState(AwaitingConnection, fmt.Sprintf("0/%d", m.n2))
	f := m.command(ax25.TypeSABM, true, nil, 0, 0)
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.sendFrame(ctx, send, f); err != nil {
			m.failLink(err.Error())
			return err
		}
		select {
		case event := <-m.control:
			// AX.25 v2.2 SDL state 1: UA/DM answering SABM(P=1) is valid
			// only as a response with F=1. UA(F=0) is error D and is ignored.
			if event.type_ == ax25.TypeUA && event.response && event.final {
				m.setState(Connected, "Sesja AX.25 polaczona")
				return nil
			}
			if event.type_ == ax25.TypeDM && event.response && event.final {
				m.failLink("Stacja odrzucila polaczenie")
				return errors.New("connection rejected (DM)")
			}
		case <-time.After(m.t1):
			m.setState(AwaitingConnection, fmt.Sprintf("%d/%d", attempt+1, m.n2))
		case <-ctx.Done():
			m.failLink("Polaczenie anulowane")
			return ctx.Err()
		}
	}
	m.failLink("Brak odpowiedzi UA")
	return errors.New("AX.25 connect timeout")
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.operation.Lock()
	defer m.operation.Unlock()
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
			if (event.type_ == ax25.TypeUA || event.type_ == ax25.TypeDM) && event.response && event.final {
				m.failLink("Rozlaczono")
				return nil
			}
		case <-time.After(m.t1):
		case <-ctx.Done():
			break
		}
	}
	m.failLink("Sesja zamknieta lokalnie")
	return nil
}

func (m *Manager) Send(ctx context.Context, data []byte) error {
	return m.SendWithProgress(ctx, data, nil)
}

// SendWithProgress splits data according to paclen, preferring whitespace
// boundaries so words remain intact, and reports every AX.25 I frame.
func (m *Manager) SendWithProgress(ctx context.Context, data []byte, progress func(SendPacketProgress)) error {
	m.operation.Lock()
	defer m.operation.Unlock()
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
		interval = defaultT3
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.operation.Lock()
			m.mu.Lock()
			if m.state != Connected {
				m.mu.Unlock()
				m.operation.Unlock()
				continue
			}
			send, nr := m.ports[m.port], m.vr
			m.mu.Unlock()
			if err := m.probeLink(ctx, send, nr); err != nil && ctx.Err() == nil {
				m.failLink("Utrata lacza AX.25")
			}
			m.operation.Unlock()
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
	// V(S) is advanced when the new I frame is sent, not when it is
	// acknowledged. V(A) remains the oldest unacknowledged sequence number.
	m.mu.Lock()
	m.vs = expected
	m.mu.Unlock()
	needSend := true
	pollOnly := false
	for attempt := 0; attempt < m.n2; attempt++ {
		if err := m.waitRemoteReady(ctx); err != nil {
			m.failLink("Zdalna stacja pozostaje zajeta")
			return err
		}
		if pollOnly {
			ack, err := m.pollAcknowledgement(ctx, send, nr)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}
			pollOnly = false
			if ack.type_ == ax25.TypeDM || ack.type_ == ax25.TypeDISC || ack.type_ == ax25.TypeSABM {
				return errors.New("AX.25 link terminated by remote station")
			}
			if ack.type_ == ax25.TypeRNR {
				m.mu.Lock()
				m.peerBusy = true
				m.mu.Unlock()
			}
			if m.validNR(ack.nr) && ack.nr == expected && ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ {
				m.mu.Lock()
				m.va = expected
				m.mu.Unlock()
				return nil
			}
			needSend = true
		}
		if needSend {
			if err := m.sendFrame(ctx, send, f); err != nil {
				m.failLink(err.Error())
				return err
			}
			needSend = false
		}
		timer := time.NewTimer(m.t1)
		cycleDone := false
		for {
			select {
			case ack := <-m.ack:
				if ack.type_ == ax25.TypeDM || ack.type_ == ax25.TypeDISC || ack.type_ == ax25.TypeSABM {
					stopTimer(timer)
					return errors.New("AX.25 link terminated by remote station")
				}
				if ack.type_ == ax25.TypeRNR {
					m.mu.Lock()
					m.peerBusy = true
					m.mu.Unlock()
					if ack.nr == expected {
						stopTimer(timer)
						m.mu.Lock()
						m.va = expected
						m.mu.Unlock()
						return nil
					}
					stopTimer(timer)
					needSend = true
					cycleDone = true
					break
				}
				if m.validNR(ack.nr) && ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ && ack.nr == expected {
					stopTimer(timer)
					m.mu.Lock()
					m.va = expected
					m.mu.Unlock()
					return nil
				}
				if (ack.type_ == ax25.TypeREJ || ack.type_ == ax25.TypeSREJ) && ack.nr == ns {
					stopTimer(timer)
					needSend = true
					cycleDone = true
					break
				}
				// A stale RR or piggybacked N(R) does not acknowledge this
				// frame and must not trigger an immediate retransmission.
			case <-timer.C:
				ack, err := m.pollAcknowledgement(ctx, send, nr)
				if err != nil {
					cycleDone = true
					needSend = false
					pollOnly = true
					break
				}
				if ack.type_ == ax25.TypeDM || ack.type_ == ax25.TypeDISC || ack.type_ == ax25.TypeSABM {
					return errors.New("AX.25 link terminated by remote station")
				}
				if ack.type_ == ax25.TypeRNR {
					m.mu.Lock()
					m.peerBusy = true
					m.mu.Unlock()
				}
				if m.validNR(ack.nr) && ack.nr == expected && ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ {
					m.mu.Lock()
					m.va = expected
					m.mu.Unlock()
					return nil
				}
				needSend = true
				cycleDone = true
				break
			case <-ctx.Done():
				stopTimer(timer)
				return ctx.Err()
			}
			if cycleDone {
				break
			}
		}
	}
	err := errors.New("AX.25 data acknowledgement timeout")
	m.failLink("Utrata lacza AX.25: brak potwierdzenia po N2 probach")
	return err
}

func (m *Manager) pollAcknowledgement(ctx context.Context, send Sender, nr uint8) (acknowledgement, error) {
	m.setState(TimerRecovery, "Odzyskiwanie lacza AX.25")
	if err := m.sendFrame(ctx, send, m.command(ax25.TypeRR, true, nil, 0, nr)); err != nil {
		return acknowledgement{}, err
	}
	timer := time.NewTimer(m.t1)
	defer stopTimer(timer)
	for {
		select {
		case ack := <-m.ack:
			// A command with P=1 is answered by Handle. Only the matching
			// response with F=1 completes the enquiry procedure (SDL state 4).
			if ack.response && ack.final {
				switch ack.type_ {
				case ax25.TypeRR, ax25.TypeRNR, ax25.TypeREJ, ax25.TypeSREJ:
					if !m.validNR(ack.nr) {
						continue
					}
					m.setState(Connected, "Sesja AX.25 polaczona")
					return ack, nil
				case ax25.TypeDM, ax25.TypeDISC, ax25.TypeSABM:
					return ack, nil
				}
			}
		case <-timer.C:
			return acknowledgement{}, errors.New("AX.25 supervisory poll timeout")
		case <-ctx.Done():
			return acknowledgement{}, ctx.Err()
		}
	}
}

func (m *Manager) probeLink(ctx context.Context, send Sender, nr uint8) error {
	m.drainAck()
	for attempt := 0; attempt < m.n2; attempt++ {
		if _, err := m.pollAcknowledgement(ctx, send, nr); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return errors.New("AX.25 inactive link timeout")
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
				if ack.type_ == ax25.TypeDM || ack.type_ == ax25.TypeDISC {
					return errors.New("AX.25 link terminated by remote station")
				}
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
	case ax25.TypeTEST:
		if isCommand(f) {
			m.mu.Lock()
			send := m.ports[port]
			remote, local := m.remote, m.local
			digis := append([]ax25.Address(nil), m.digipeaters...)
			m.mu.Unlock()
			remote.CommandResponse, local.CommandResponse = false, true
			_ = m.sendFrame(context.Background(), send, ax25.Frame{Destination: remote, Source: local, Digipeaters: digis, Type: ax25.TypeTEST, PollFinal: f.PollFinal, Payload: append([]byte(nil), f.Payload...)})
		}
	case ax25.TypeUI:
		if isCommand(f) && f.PollFinal && (state == Connected || state == TimerRecovery) {
			m.mu.Lock()
			nr := m.vr
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeRR, true, nr)
		}
	case ax25.TypeXID:
		if isCommand(f) {
			payload, err := ax25.EncodeXID(ax25.BasicXIDParameters())
			if err == nil {
				m.mu.Lock()
				send := m.ports[port]
				remote, local := m.remote, m.local
				digis := append([]ax25.Address(nil), m.digipeaters...)
				m.mu.Unlock()
				remote.CommandResponse, local.CommandResponse = false, true
				_ = m.sendFrame(context.Background(), send, ax25.Frame{Destination: remote, Source: local, Digipeaters: digis, Type: ax25.TypeXID, PollFinal: f.PollFinal, Payload: payload})
			}
		}
	case ax25.TypeUA, ax25.TypeDM:
		response := isResponse(f)
		if (state == Connected || state == TimerRecovery) && f.Type == ax25.TypeDM {
			m.failLink("Zdalna stacja zakonczyla lacze (DM)")
			m.queueAck(acknowledgement{type_: ax25.TypeDM, final: f.PollFinal, response: response})
		} else if (state == Connected || state == TimerRecovery) && f.Type == ax25.TypeUA {
			// Error C. The SDL requires re-establishment rather than silently
			// continuing with potentially different sequence state.
			m.failLink("Nieoczekiwana odpowiedz UA - lacze wymaga ponownego zestawienia")
			m.queueAck(acknowledgement{type_: ax25.TypeDM})
		} else {
			select {
			case m.control <- controlEvent{type_: f.Type, final: f.PollFinal, response: response}:
			default:
			}
		}
	case ax25.TypeDISC:
		_ = m.sendResponse(context.Background(), port, ax25.TypeUA, f.PollFinal, 0)
		m.failLink("Zdalna stacja rozlaczyla sesje")
		m.queueAck(acknowledgement{type_: ax25.TypeDISC})
	case ax25.TypeSABM:
		if state == AwaitingConnection {
			_ = m.sendResponse(context.Background(), port, ax25.TypeUA, f.PollFinal, 0)
			select {
			case m.control <- controlEvent{type_: ax25.TypeUA, final: true, response: true}:
			default:
			}
		} else if state == Connected || state == TimerRecovery {
			m.mu.Lock()
			m.vs, m.va, m.vr = 0, 0, 0
			m.peerBusy = false
			m.rejectSent = false
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeUA, f.PollFinal, 0)
			m.queueAck(acknowledgement{type_: ax25.TypeSABM})
			m.emit(Event{Type: "notice", Message: "Zdalna stacja zresetowala lacze AX.25"})
		}
	case ax25.TypeRR:
		m.mu.Lock()
		m.peerBusy = false
		m.mu.Unlock()
		select {
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
		default:
		}
		if f.PollFinal && isCommand(f) {
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
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
		default:
		}
		if f.PollFinal && isCommand(f) {
			m.mu.Lock()
			nr := m.vr
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeRR, true, nr)
		}
		m.emit(Event{Type: "notice", Message: "Zdalna stacja chwilowo nie moze odbierac"})
	case ax25.TypeREJ, ax25.TypeSREJ:
		if f.PollFinal && isCommand(f) {
			m.mu.Lock()
			nr := m.vr
			m.mu.Unlock()
			_ = m.sendResponse(context.Background(), port, ax25.TypeRR, true, nr)
		}
		select {
		case m.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
		default:
		}
	case ax25.TypeI:
		if (state != Connected && state != TimerRecovery) || !isCommand(f) || len(f.Payload) > m.paclen {
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
			case m.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
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

func (m *Manager) queueAck(ack acknowledgement) {
	select {
	case m.ack <- ack:
	default:
	}
}

func (m *Manager) failLink(message string) {
	m.mu.Lock()
	m.state = Disconnected
	m.vs, m.va, m.vr = 0, 0, 0
	m.peerBusy = false
	m.rejectSent = false
	m.mu.Unlock()
	m.emit(Event{Type: "state", State: Disconnected, Message: message})
}

// validNR implements the modulo-8 acknowledgement range check from AX.25
// section 6.4.1. With the personal-station window k=1, N(R) may only name
// V(A) (no new acknowledgement) or V(S) (the outstanding frame accepted).
func (m *Manager) validNR(nr uint8) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nr == m.va || nr == m.vs
}

func isCommand(f ax25.Frame) bool {
	return f.Destination.CommandResponse && !f.Source.CommandResponse
}

func isResponse(f ax25.Frame) bool {
	return !f.Destination.CommandResponse && f.Source.CommandResponse
}

func stopTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
