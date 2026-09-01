package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/service"
)

// AX25Service handles one connected-mode byte stream. The remote callsign is
// authenticated by the AX.25 link header, not by text entered after connect.
type AX25Service func(remote string, r io.Reader, w io.Writer)

// InboundRoute describes the radio route of an accepted AX.25 connection.
// Digipeaters are stored in return order with their repeated bits cleared.
type InboundRoute struct {
	Remote      string
	Port        string
	Digipeaters []ax25.Address
}

type RoutedAX25Service func(route InboundRoute, r io.Reader, w io.Writer)

// AX25PacketService handles non-text I-frame payloads after the AX.25 link is
// established. The send function uses the supplied PID and preserves the
// link-level acknowledgement machinery.
type AX25PacketService func(route InboundRoute, pid byte, data []byte, send func(context.Context, byte, []byte) error)

type InboundMux struct {
	mu             sync.Mutex
	services       map[string]AX25Service
	routedServices map[string]RoutedAX25Service
	packetServices map[string]AX25PacketService
	registry       *service.Registry
	senders        map[string]Sender
	links          map[string]*inboundLink
	log            *slog.Logger
	t1             time.Duration
	t2             time.Duration
	n2             int
	paclen         int
}

type inboundLink struct {
	mu            sync.Mutex
	mux           *InboundMux
	port          string
	local, remote ax25.Address
	digipeaters   []ax25.Address
	linkCore
	in        chan []byte
	tx        chan []byte
	ack       chan acknowledgement
	control   chan controlEvent
	closed    chan struct{}
	closeOnce sync.Once
	t1        time.Duration
	t2        time.Duration
	n2        int
	paclen    int
	receiveN1 int
}

func NewInboundMux(senders map[string]Sender, log *slog.Logger) *InboundMux {
	return &InboundMux{services: map[string]AX25Service{}, routedServices: map[string]RoutedAX25Service{}, packetServices: map[string]AX25PacketService{}, senders: senders, links: map[string]*inboundLink{}, log: log, t1: defaultT1, t2: defaultT2, n2: 10, paclen: defaultN1}
}

func (m *InboundMux) SetRegistry(registry *service.Registry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry = registry
}

func (m *InboundMux) Configure(t1 time.Duration, n2, n1 int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t1 > 0 {
		m.t1 = t1
	}
	if n2 > 0 {
		m.n2 = n2
	}
	if n1 > 0 {
		m.paclen = n1
	}
}

func (m *InboundMux) Register(address ax25.Address, service AX25Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[address.String()] = service
}
func (m *InboundMux) RegisterRouted(address ax25.Address, service RoutedAX25Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routedServices[address.String()] = service
}

func (m *InboundMux) RegisterPacket(address ax25.Address, service AX25PacketService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packetServices[address.String()] = service
}

// Handle returns true when the frame belongs to a registered inbound service
// or to an established inbound session.
func (m *InboundMux) Handle(port string, f ax25.Frame) bool {
	key := linkKey(port, f.Destination, f.Source)
	m.mu.Lock()
	link := m.links[key]
	legacyService := m.services[f.Destination.String()]
	routedService := m.routedServices[f.Destination.String()]
	packetService := m.packetServices[f.Destination.String()]
	var registeredService service.Service
	if m.registry != nil {
		if registration, ok := m.registry.ByCallsign(f.Destination.String()); ok {
			registeredService = registration.Service
		} else if registration, ok := m.registry.ByAlias(f.Destination.String()); ok {
			registeredService = registration.Service
		}
	}
	registered := legacyService != nil || routedService != nil || packetService != nil || registeredService != nil
	m.mu.Unlock()
	if link != nil {
		link.mu.Lock()
		link.touch(time.Now())
		link.mu.Unlock()
	}

	if link == nil {
		if registered && f.Type == ax25.TypeXID && isCommand(f) {
			if f.PollFinal {
				_ = m.sendXIDResponse(port, f, nil)
			}
			return true
		}
		if registered && f.Type == ax25.TypeSABME && isCommand(f) {
			// This endpoint has not switched its receive decoder to modulo 128,
			// therefore AX.25 requires a negative mode-setting response.
			_ = m.sendDisconnectedResponse(port, f, ax25.TypeDM)
			return true
		}
		if registered && f.Type == ax25.TypeTEST && isCommand(f) {
			_ = m.sendUnnumberedResponse(port, f, ax25.TypeTEST, f.Payload)
			return true
		}
		if registered && f.Type == ax25.TypeUI && isCommand(f) {
			if f.PollFinal {
				_ = m.sendDisconnectedResponse(port, f, ax25.TypeDM)
			}
			return true
		}
		if registered && f.Type != ax25.TypeSABM && f.Type != ax25.TypeUI && isCommand(f) {
			_ = m.sendDisconnectedResponse(port, f, ax25.TypeDM)
			return true
		}
		if f.Type != ax25.TypeSABM || !registered || !isCommand(f) {
			return false
		}
		m.mu.Lock()
		linkT1, linkT2, linkN2, linkN1 := m.t1, m.t2, m.n2, m.paclen
		m.mu.Unlock()
		link = &inboundLink{mux: m, port: port, local: f.Destination, remote: f.Source, digipeaters: reverseDigipeaters(f.Digipeaters), in: make(chan []byte, 32), tx: make(chan []byte, 32), ack: make(chan acknowledgement, 8), control: make(chan controlEvent, 4), closed: make(chan struct{}), t1: linkT1, t2: linkT2, n2: linkN2, paclen: linkN1, receiveN1: linkN1}
		link.touch(time.Now())
		link.mu.Lock()
		sabmAction := link.handleEvent(linkEvent{kind: eventRemoteSABM, pf: f.PollFinal})
		link.touch(time.Now())
		link.mu.Unlock()
		created := false
		m.mu.Lock()
		if existing := m.links[key]; existing != nil {
			link = existing
		} else {
			m.links[key] = link
			created = true
		}
		m.mu.Unlock()
		_ = link.sendControl(sabmAction.send, sabmAction.pollFinal, sabmAction.nr)
		if created {
			go link.supervise()
			if registeredService != nil {
				go link.run(legacyService, routedService, registeredService)
			} else if legacyService != nil || routedService != nil {
				go link.run(legacyService, routedService, nil)
			}
		}
		if m.log != nil {
			m.log.Info("AX.25 inbound connected", "port", port, "remote", f.Source.String(), "service", f.Destination.String())
		}
		return true
	}
	link.mu.Lock()
	link.touch(time.Now())
	if isResponse(f) && f.PollFinal && (f.Type == ax25.TypeRR || f.Type == ax25.TypeRNR || f.Type == ax25.TypeREJ || f.Type == ax25.TypeSREJ) {
		link.exitRecovery()
	}
	link.mu.Unlock()

	switch f.Type {
	case ax25.TypeTEST:
		if isCommand(f) {
			_ = m.sendUnnumberedResponse(port, f, ax25.TypeTEST, f.Payload)
		}
	case ax25.TypeUI:
		if isCommand(f) && f.PollFinal {
			link.mu.Lock()
			nr := link.vr
			link.mu.Unlock()
			_ = link.sendControl(ax25.TypeRR, true, nr)
		}
	case ax25.TypeUA:
		if isResponse(f) {
			select {
			case link.control <- controlEvent{type_: f.Type, final: f.PollFinal, response: true}:
			default:
			}
		}
	case ax25.TypeDM:
		if isResponse(f) {
			link.mu.Lock()
			link.handleEvent(linkEvent{kind: eventRemoteDM})
			link.mu.Unlock()
			select {
			case link.control <- controlEvent{type_: f.Type, final: f.PollFinal, response: true}:
			default:
			}
			link.close()
		}
	case ax25.TypeXID:
		if isCommand(f) && f.PollFinal {
			_ = m.sendXIDResponse(port, f, link)
		}
	case ax25.TypeSABM:
		if !isCommand(f) {
			return true
		}
		link.mu.Lock()
		sabmAction := link.handleEvent(linkEvent{kind: eventRemoteSABM, pf: f.PollFinal})
		link.touch(time.Now())
		link.digipeaters = reverseDigipeaters(f.Digipeaters)
		link.mu.Unlock()
		_ = link.sendControl(sabmAction.send, sabmAction.pollFinal, sabmAction.nr)
	case ax25.TypeDISC:
		if !isCommand(f) {
			return true
		}
		link.mu.Lock()
		action := link.handleEvent(linkEvent{kind: eventRemoteDISC, pf: f.PollFinal})
		link.mu.Unlock()
		_ = link.sendControl(action.send, action.pollFinal, action.nr)
		link.close()
	case ax25.TypeRR, ax25.TypeREJ, ax25.TypeSREJ:
		link.mu.Lock()
		link.handleEvent(linkEvent{kind: eventRemoteReady})
		nr := link.vr
		link.mu.Unlock()
		select {
		case link.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
		default:
		}
		if f.PollFinal && isCommand(f) {
			_ = link.sendControl(ax25.TypeRR, true, nr)
		}
	case ax25.TypeRNR:
		link.mu.Lock()
		link.handleEvent(linkEvent{kind: eventRemoteBusy})
		nr := link.vr
		link.mu.Unlock()
		select {
		case link.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: isResponse(f)}:
		default:
		}
		if f.PollFinal && isCommand(f) {
			_ = link.sendControl(ax25.TypeRR, true, nr)
		}
	case ax25.TypeI:
		link.mu.Lock()
		receiveN1 := link.receiveN1
		link.mu.Unlock()
		if !isCommand(f) || f.PID == nil || len(f.Payload) > receiveN1 || (*f.PID != 0xF0 && packetService == nil) {
			return true
		}
		link.mu.Lock()
		accepted, sendReject := link.linkCore.receive(f.NS)
		ackSerial := uint64(0)
		if accepted {
			ackSerial = link.noteAcknowledgement(time.Now())
		}
		if accepted {
			link.mu.Unlock()
			data := append([]byte(nil), f.Payload...)
			if *f.PID == 0xF0 {
				select {
				case link.in <- data:
				case <-link.closed:
				}
			} else if packetService != nil {
				route := InboundRoute{Remote: link.remote.String(), Port: link.port, Digipeaters: append([]ax25.Address(nil), link.digipeaters...)}
				pid, sendData := *f.PID, data
				go packetService(route, pid, sendData, link.sendPacket)
			}
			if f.PollFinal {
				link.mu.Lock()
				link.piggybackAcknowledgement()
				nr := link.vr
				link.mu.Unlock()
				_ = link.sendControl(ax25.TypeRR, true, nr)
			} else {
				link.mu.Lock()
				t2 := link.t2
				link.mu.Unlock()
				link.scheduleAcknowledgement(ackSerial, t2)
			}
		} else {
			nr := link.vr
			link.mu.Unlock()
			if sendReject {
				_ = link.sendControl(ax25.TypeREJ, f.PollFinal, nr)
			}
			return true
		}
		select {
		case link.ack <- acknowledgement{type_: f.Type, nr: f.NR, final: f.PollFinal, response: false}:
		default:
		}
	}
	return true
}

func (l *inboundLink) scheduleAcknowledgement(serial uint64, t2 time.Duration) {
	time.AfterFunc(t2, func() {
		l.mu.Lock()
		if !l.expireAcknowledgement(time.Now(), serial, t2) {
			l.mu.Unlock()
			return
		}
		action := l.handleEvent(linkEvent{kind: eventT2Expired})
		l.mu.Unlock()
		select {
		case <-l.closed:
			return
		default:
		}
		_ = l.sendControl(action.send, action.pollFinal, action.nr)
	})
}

func (l *inboundLink) run(legacyService AX25Service, routedService RoutedAX25Service, registeredService service.Service) {
	r, pw := io.Pipe()
	pr, w := io.Pipe()
	txDone := make(chan struct{})
	go func() {
		defer pw.Close()
		for {
			select {
			case b := <-l.in:
				if _, err := pw.Write(b); err != nil {
					return
				}
			case <-l.closed:
				return
			}
		}
	}()
	go func() {
		defer pr.Close()
		defer close(l.tx)
		buf := make([]byte, 1024)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				b := append([]byte(nil), buf[:n]...)
				select {
				case l.tx <- b:
				case <-l.closed:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { l.transmit(); close(txDone) }()
	if registeredService != nil {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-l.closed:
				cancel()
			case <-ctx.Done():
			}
		}()
		_ = registeredService.Serve(service.ServiceContext{
			Context: ctx, LocalCall: l.local, RemoteCall: l.remote, PortID: l.port,
			Digipeaters: append([]ax25.Address(nil), l.digipeaters...), Reader: r, Writer: w,
			EntryType: service.EntryAX25, Cancel: cancel, Disconnect: l.disconnect,
		})
		cancel()
	} else if routedService != nil {
		route := InboundRoute{Remote: l.remote.String(), Port: l.port, Digipeaters: append([]ax25.Address(nil), l.digipeaters...)}
		routedService(route, r, w)
	} else {
		legacyService(l.remote.String(), r, w)
	}
	_ = w.Close()
	select {
	case <-txDone:
	case <-l.closed:
		return
	}
	_ = l.disconnect()
	l.close()
}

func (l *inboundLink) supervise() {
	ticker := time.NewTicker(defaultT3)
	defer ticker.Stop()
	for {
		select {
		case <-l.closed:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			if !l.idleExpired(now, defaultT3) {
				l.mu.Unlock()
				continue
			}
			action := l.handleEvent(linkEvent{kind: eventT3Expired})
			l.mu.Unlock()
			_ = l.sendControl(action.send, action.pollFinal, action.nr)
		}
	}
}

func (l *inboundLink) disconnect() error {
	l.mu.Lock()
	t1, n2 := l.t1, l.n2
	l.mu.Unlock()
	for {
		select {
		case <-l.control:
		default:
			goto drained
		}
	}
drained:
	l.mu.Lock()
	action := l.handleEvent(linkEvent{kind: eventServiceClosed})
	l.startTimer(time.Now(), timerForRelease, n2)
	l.mu.Unlock()
	if action.state == Disconnected {
		return nil
	}
	for attempt := 0; attempt < n2; attempt++ {
		if err := l.sendCommand(action.send, action.pollFinal, action.nr); err != nil {
			return err
		}
		timer := time.NewTimer(t1)
		select {
		case event := <-l.control:
			stopTimer(timer)
			if event.response && event.final && (event.type_ == ax25.TypeUA || event.type_ == ax25.TypeDM) {
				l.mu.Lock()
				l.stopTimer()
				l.mu.Unlock()
				return nil
			}
		case <-timer.C:
			l.mu.Lock()
			l.timerExpired(time.Now(), t1)
			action := l.handleEvent(linkEvent{kind: eventT1Expired})
			l.mu.Unlock()
			if action.terminate {
				return errors.New("AX.25 disconnect timeout")
			}
		case <-l.closed:
			stopTimer(timer)
			return io.ErrClosedPipe
		}
	}
	return errors.New("AX.25 disconnect timeout")
}

func (l *inboundLink) transmit() {
	for {
		select {
		case data, ok := <-l.tx:
			if !ok {
				return
			}
			for len(data) > 0 {
				l.mu.Lock()
				paclen := l.paclen
				l.mu.Unlock()
				n := len(data)
				if n > paclen {
					n = paclen
				}
				if l.sendChunk(data[:n]) != nil {
					l.close()
					return
				}
				data = data[n:]
			}
		case <-l.closed:
			return
		}
	}
}

func (l *inboundLink) sendChunk(data []byte) error {
	return l.sendChunkWithPID(0xF0, data)
}

func (l *inboundLink) sendPacket(ctx context.Context, pid byte, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return l.sendChunkWithPID(pid, data)
}

func (l *inboundLink) sendChunkWithPID(pid byte, data []byte) error {
	l.mu.Lock()
	ns, nr, expected := l.nextSend()
	t1, n2 := l.t1, l.n2
	l.mu.Unlock()
	f := l.command(ax25.TypeI, false, nr)
	f.NS, f.PID, f.Payload = ns, &pid, append([]byte(nil), data...)
	l.mu.Lock()
	l.track(f)
	l.startTimer(time.Now(), timerForData, n2)
	l.mu.Unlock()
	needSend := true
	for attempt := 0; attempt < n2; attempt++ {
		if err := l.waitRemoteReady(); err != nil {
			return err
		}
		if needSend {
			if err := l.send(f); err != nil {
				return err
			}
			needSend = false
		}
		select {
		case ack := <-l.ack:
			if l.validNR(ack.nr) && ack.nr == expected && ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ {
				l.mu.Lock()
				l.stopTimer()
				l.linkCore.acknowledge(expected, ack.type_)
				l.mu.Unlock()
				return nil
			}
			if (ack.type_ == ax25.TypeREJ || ack.type_ == ax25.TypeSREJ) && ack.nr == ns {
				l.mu.Lock()
				frames := l.retransmitFrom(ack.nr)
				l.mu.Unlock()
				if len(frames) > 0 {
					f = frames[0]
				}
				needSend = true
			}
		case <-time.After(t1):
			l.mu.Lock()
			l.timerExpired(time.Now(), t1)
			action := l.handleEvent(linkEvent{kind: eventT1Expired})
			l.mu.Unlock()
			if action.terminate {
				return errors.New("AX.25 inbound data acknowledgement timeout")
			}
			if err := l.sendControl(action.send, action.pollFinal, action.nr); err != nil {
				return err
			}
			pollTimer := time.NewTimer(t1)
			pollDone := false
			for !pollDone {
				select {
				case ack := <-l.ack:
					if !ack.response || !ack.final {
						continue
					}
					stopTimer(pollTimer)
					if l.validNR(ack.nr) && ack.nr == expected && ack.type_ != ax25.TypeREJ && ack.type_ != ax25.TypeSREJ {
						l.mu.Lock()
						l.stopTimer()
						l.linkCore.acknowledge(expected, ack.type_)
						l.mu.Unlock()
						return nil
					}
					needSend = true
					pollDone = true
				case <-pollTimer.C:
					l.mu.Lock()
					l.timerExpired(time.Now(), t1)
					action := l.handleEvent(linkEvent{kind: eventT1Expired})
					l.mu.Unlock()
					if action.terminate {
						return errors.New("AX.25 inbound data acknowledgement timeout")
					}
					pollDone = true
				case <-l.closed:
					stopTimer(pollTimer)
					return io.ErrClosedPipe
				}
			}
		case <-l.closed:
			return io.ErrClosedPipe
		}
	}
	return errors.New("AX.25 inbound data acknowledgement timeout")
}

func (l *inboundLink) waitRemoteReady() error {
	l.mu.Lock()
	t1, n2 := l.t1, l.n2
	l.mu.Unlock()
	for attempt := 0; attempt < n2; attempt++ {
		l.mu.Lock()
		busy, nr := l.peerBusy, l.vr
		l.mu.Unlock()
		if !busy {
			return nil
		}
		for {
			select {
			case <-l.ack:
			default:
				goto drained
			}
		}
	drained:
		if err := l.sendControl(ax25.TypeRR, true, nr); err != nil {
			return err
		}
		select {
		case ack := <-l.ack:
			l.mu.Lock()
			if ack.response && ack.final && ack.type_ == ax25.TypeRR && !l.peerBusy {
				l.mu.Unlock()
				return nil
			}
			l.mu.Unlock()
		case <-time.After(t1):
			l.mu.Lock()
			l.timerExpired(time.Now(), t1)
			action := l.handleEvent(linkEvent{kind: eventT1Expired})
			l.mu.Unlock()
			if action.terminate {
				return errors.New("AX.25 remote receiver remains busy")
			}
		case <-l.closed:
			return io.ErrClosedPipe
		}
	}
	return errors.New("AX.25 remote receiver remains busy")
}

func (m *InboundMux) sendDisconnectedResponse(port string, received ax25.Frame, typ ax25.Type) error {
	d, s := received.Source, received.Destination
	d.CommandResponse, s.CommandResponse = false, true
	f := ax25.Frame{Destination: d, Source: s, Digipeaters: reverseDigipeaters(received.Digipeaters), Type: typ, PollFinal: true}
	b, err := ax25.Encode(f)
	if err != nil {
		return err
	}
	send := m.senders[port]
	if send == nil {
		return errors.New("AX.25 port unavailable")
	}
	return send(context.Background(), b)
}

func (m *InboundMux) sendXIDResponse(port string, received ax25.Frame, link *inboundLink) error {
	m.mu.Lock()
	n1, n2, t1 := m.paclen, m.n2, m.t1
	m.mu.Unlock()
	command, err := ax25.DecodeXID(received.Payload)
	if err != nil {
		return err
	}
	selected, offered, err := ax25.NegotiateXID(command, xidSettings(n1, t1, n2))
	if err != nil {
		return err
	}
	if link != nil {
		link.mu.Lock()
		link.paclen = min(n1, offered.ReceiveN1)
		link.t1 = time.Duration(selected.T1Milliseconds) * time.Millisecond
		link.n2 = selected.Retries
		link.mu.Unlock()
	}
	payload, err := ax25.EncodeXID(ax25.XIDParameters(selected))
	if err != nil {
		return err
	}
	d, s := received.Source, received.Destination
	d.CommandResponse, s.CommandResponse = false, true
	f := ax25.Frame{Destination: d, Source: s, Digipeaters: reverseDigipeaters(received.Digipeaters), Type: ax25.TypeXID, PollFinal: received.PollFinal, Payload: payload}
	b, err := ax25.Encode(f)
	if err != nil {
		return err
	}
	if send := m.senders[port]; send != nil {
		return send(context.Background(), b)
	}
	return errors.New("AX.25 port unavailable")
}

func (m *InboundMux) sendUnnumberedResponse(port string, received ax25.Frame, typ ax25.Type, payload []byte) error {
	d, s := received.Source, received.Destination
	d.CommandResponse, s.CommandResponse = false, true
	f := ax25.Frame{Destination: d, Source: s, Digipeaters: reverseDigipeaters(received.Digipeaters), Type: typ, PollFinal: received.PollFinal, Payload: append([]byte(nil), payload...)}
	b, err := ax25.Encode(f)
	if err != nil {
		return err
	}
	if send := m.senders[port]; send != nil {
		return send(context.Background(), b)
	}
	return errors.New("AX.25 port unavailable")
}

func (l *inboundLink) sendControl(t ax25.Type, pf bool, nr uint8) error {
	return l.send(l.response(t, pf, nr))
}
func (l *inboundLink) sendCommand(t ax25.Type, pf bool, nr uint8) error {
	return l.send(l.command(t, pf, nr))
}
func (l *inboundLink) response(t ax25.Type, pf bool, nr uint8) ax25.Frame {
	l.mu.Lock()
	d, s := l.remote, l.local
	digis := append([]ax25.Address(nil), l.digipeaters...)
	l.mu.Unlock()
	d.CommandResponse, s.CommandResponse = false, true
	return ax25.Frame{Destination: d, Source: s, Digipeaters: digis, Type: t, PollFinal: pf, NR: nr}
}

func (l *inboundLink) command(t ax25.Type, pf bool, nr uint8) ax25.Frame {
	l.mu.Lock()
	d, s := l.remote, l.local
	digis := append([]ax25.Address(nil), l.digipeaters...)
	l.mu.Unlock()
	d.CommandResponse, s.CommandResponse = true, false
	return ax25.Frame{Destination: d, Source: s, Digipeaters: digis, Type: t, PollFinal: pf, NR: nr}
}

func (l *inboundLink) validNR(nr uint8) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.linkCore.validNR(nr)
}

func reverseDigipeaters(path []ax25.Address) []ax25.Address {
	if len(path) == 0 {
		return nil
	}
	reversed := make([]ax25.Address, len(path))
	for i := range path {
		digi := path[len(path)-1-i]
		digi.Repeated = false
		digi.CommandResponse = false
		reversed[i] = digi
	}
	return reversed
}
func (l *inboundLink) send(f ax25.Frame) error {
	b, err := ax25.Encode(f)
	if err != nil {
		return err
	}
	s := l.mux.senders[l.port]
	if s == nil {
		return errors.New("AX.25 port unavailable")
	}
	return s(context.Background(), b)
}
func (l *inboundLink) close() {
	l.closeOnce.Do(func() {
		close(l.closed)
		l.mux.mu.Lock()
		delete(l.mux.links, linkKey(l.port, l.local, l.remote))
		l.mux.mu.Unlock()
		if l.mux.log != nil {
			l.mux.log.Info("AX.25 inbound disconnected", "port", l.port, "remote", l.remote.String())
		}
	})
}
func linkKey(port string, local, remote ax25.Address) string {
	return port + "|" + local.String() + "|" + remote.String()
}
