package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

// AX25Service handles one connected-mode byte stream. The remote callsign is
// authenticated by the AX.25 link header, not by text entered after connect.
type AX25Service func(remote string, r io.Reader, w io.Writer)

type InboundMux struct {
	mu       sync.Mutex
	services map[string]AX25Service
	senders  map[string]Sender
	links    map[string]*inboundLink
	log      *slog.Logger
	t1       time.Duration
	n2       int
	paclen   int
}

type inboundLink struct {
	mu            sync.Mutex
	mux           *InboundMux
	port          string
	local, remote ax25.Address
	vr, vs        uint8
	in            chan []byte
	tx            chan []byte
	ack           chan uint8
	closed        chan struct{}
	closeOnce     sync.Once
}

func NewInboundMux(senders map[string]Sender, log *slog.Logger) *InboundMux {
	return &InboundMux{services: map[string]AX25Service{}, senders: senders, links: map[string]*inboundLink{}, log: log, t1: 3 * time.Second, n2: 5, paclen: 128}
}

func (m *InboundMux) Register(address ax25.Address, service AX25Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[address.String()] = service
}

// Handle returns true when the frame belongs to a registered inbound service
// or to an established inbound session.
func (m *InboundMux) Handle(port string, f ax25.Frame) bool {
	key := linkKey(port, f.Destination, f.Source)
	m.mu.Lock()
	link := m.links[key]
	service := m.services[f.Destination.String()]
	m.mu.Unlock()

	if link == nil {
		if f.Type != ax25.TypeSABM || service == nil {
			return false
		}
		link = &inboundLink{mux: m, port: port, local: f.Destination, remote: f.Source, in: make(chan []byte, 32), tx: make(chan []byte, 32), ack: make(chan uint8, 8), closed: make(chan struct{})}
		created := false
		m.mu.Lock()
		if existing := m.links[key]; existing != nil {
			link = existing
		} else {
			m.links[key] = link
			created = true
		}
		m.mu.Unlock()
		_ = link.sendControl(ax25.TypeUA, f.PollFinal, 0)
		if created {
			go link.run(service)
		}
		if m.log != nil {
			m.log.Info("AX.25 inbound connected", "port", port, "remote", f.Source.String(), "service", f.Destination.String())
		}
		return true
	}

	switch f.Type {
	case ax25.TypeSABM:
		link.mu.Lock()
		link.vr, link.vs = 0, 0
		link.mu.Unlock()
		_ = link.sendControl(ax25.TypeUA, f.PollFinal, 0)
	case ax25.TypeDISC:
		_ = link.sendControl(ax25.TypeUA, f.PollFinal, 0)
		link.close()
	case ax25.TypeRR, ax25.TypeREJ:
		select {
		case link.ack <- f.NR:
		default:
		}
	case ax25.TypeI:
		if f.PID == nil || *f.PID != 0xF0 {
			return true
		}
		link.mu.Lock()
		if f.NS == link.vr {
			link.vr = (link.vr + 1) & 7
			link.mu.Unlock()
			data := append([]byte(nil), f.Payload...)
			select {
			case link.in <- data:
			case <-link.closed:
			}
		} else {
			link.mu.Unlock()
		}
		select {
		case link.ack <- f.NR:
		default:
		}
		link.mu.Lock()
		nr := link.vr
		link.mu.Unlock()
		_ = link.sendControl(ax25.TypeRR, f.PollFinal, nr)
	}
	return true
}

func (l *inboundLink) run(service AX25Service) {
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
	service(l.remote.String(), r, w)
	_ = w.Close()
	select {
	case <-txDone:
	case <-l.closed:
		return
	}
	_ = l.sendControl(ax25.TypeDISC, true, 0)
	l.close()
}

func (l *inboundLink) transmit() {
	for {
		select {
		case data, ok := <-l.tx:
			if !ok {
				return
			}
			for len(data) > 0 {
				n := len(data)
				if n > l.mux.paclen {
					n = l.mux.paclen
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
	l.mu.Lock()
	ns, nr := l.vs, l.vr
	expected := (ns + 1) & 7
	l.mu.Unlock()
	pid := byte(0xF0)
	f := l.response(ax25.TypeI, false, nr)
	f.NS, f.PID, f.Payload = ns, &pid, append([]byte(nil), data...)
	for attempt := 0; attempt < l.mux.n2; attempt++ {
		if err := l.send(f); err != nil {
			return err
		}
		select {
		case nr := <-l.ack:
			if nr == expected {
				l.mu.Lock()
				l.vs = expected
				l.mu.Unlock()
				return nil
			}
		case <-time.After(l.mux.t1):
		case <-l.closed:
			return io.ErrClosedPipe
		}
	}
	return errors.New("AX.25 inbound data acknowledgement timeout")
}

func (l *inboundLink) sendControl(t ax25.Type, pf bool, nr uint8) error {
	return l.send(l.response(t, pf, nr))
}
func (l *inboundLink) response(t ax25.Type, pf bool, nr uint8) ax25.Frame {
	d, s := l.remote, l.local
	d.CommandResponse, s.CommandResponse = false, true
	return ax25.Frame{Destination: d, Source: s, Type: t, PollFinal: pf, NR: nr}
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
