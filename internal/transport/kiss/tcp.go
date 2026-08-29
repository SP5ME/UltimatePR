package kiss

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/packet-radio/ultimatepr/internal/transport"
)

type TCPConfig struct {
	ID, Address string
	MaxFrame    int
	Reconnect   time.Duration
	Queue       int
	Port        uint8
	TXDelay     *uint8
	Persistence *uint8
	SlotTime    *uint8
	TXTail      *uint8
	FullDuplex  *bool
}
type Stats struct{ RXFrames, TXFrames, RXBytes, TXBytes, DecodeErrors, DroppedTX atomic.Uint64 }
type TCPPort struct {
	cfg       TCPConfig
	tx        chan transport.Packet
	log       *slog.Logger
	stats     Stats
	connected atomic.Bool
}

func NewTCPPort(c TCPConfig, l *slog.Logger) *TCPPort {
	if c.Queue < 1 {
		c.Queue = 128
	}
	return &TCPPort{cfg: c, tx: make(chan transport.Packet, c.Queue), log: l}
}
func (p *TCPPort) ID() string { return p.cfg.ID }
func (p *TCPPort) Status() transport.Status {
	return transport.Status{ID: p.ID(), Type: "KISS TCP", Connected: p.connected.Load()}
}
func (p *TCPPort) Send(ctx context.Context, pkt transport.Packet) error {
	select {
	case p.tx <- pkt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		p.stats.DroppedTX.Add(1)
		return fmt.Errorf("KISS TX queue full")
	}
}
func (p *TCPPort) Run(ctx context.Context, out chan<- transport.Packet) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := p.runConn(ctx, out)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		p.log.Warn("port disconnected", "port", p.ID(), "error", err)
		select {
		case <-time.After(p.cfg.Reconnect):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (p *TCPPort) runConn(ctx context.Context, out chan<- transport.Packet) error {
	d := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	c, err := d.DialContext(ctx, "tcp", p.cfg.Address)
	if err != nil {
		return err
	}
	defer c.Close()
	p.connected.Store(true)
	defer p.connected.Store(false)
	p.log.Info("port connected", "port", p.ID(), "address", p.cfg.Address)
	errch := make(chan error, 2)
	go p.readLoop(ctx, c, out, errch)
	go p.writeLoop(ctx, c, errch)
	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *TCPPort) readLoop(ctx context.Context, c net.Conn, out chan<- transport.Packet, errch chan<- error) {
	d := NewDecoder(p.cfg.MaxFrame)
	b := make([]byte, 4096)
	for {
		n, err := c.Read(b)
		if n > 0 {
			p.stats.RXBytes.Add(uint64(n))
			fs, es := d.Feed(b[:n])
			p.stats.DecodeErrors.Add(uint64(len(es)))
			for _, f := range fs {
				if !acceptFrame(f, p.cfg.Port) {
					continue
				}
				p.stats.RXFrames.Add(1)
				pkt := transport.Packet{PortID: p.ID(), Channel: p.cfg.Port, Data: f.Data}
				select {
				case out <- pkt:
				case <-ctx.Done():
					errch <- ctx.Err()
					return
				}
			}
		}
		if err != nil {
			errch <- err
			return
		}
	}
}

func acceptFrame(f Frame, port uint8) bool {
	if f.Command != 0 {
		return false
	}
	return f.Port == port
}
func (p *TCPPort) writeLoop(ctx context.Context, c net.Conn, errch chan<- error) {
	if err := p.writeParameters(c); err != nil {
		errch <- err
		return
	}
	for {
		select {
		case pkt := <-p.tx:
			b, err := Encode(Frame{Port: p.cfg.Port, Command: CommandData, Data: pkt.Data})
			if err == nil {
				_, err = writeAll(c, b)
			}
			if err != nil {
				errch <- err
				return
			}
			p.stats.TXFrames.Add(1)
			p.stats.TXBytes.Add(uint64(len(b)))
		case <-ctx.Done():
			errch <- ctx.Err()
			return
		}
	}
}

func (p *TCPPort) writeParameters(w io.Writer) error {
	commands := []struct {
		command uint8
		value   *uint8
	}{
		{CommandTXDelay, p.cfg.TXDelay},
		{CommandPersistence, p.cfg.Persistence},
		{CommandSlotTime, p.cfg.SlotTime},
		{CommandTXTail, p.cfg.TXTail},
	}
	for _, item := range commands {
		if item.value == nil {
			continue
		}
		frame, err := Encode(Frame{Port: p.cfg.Port, Command: item.command, Data: []byte{*item.value}})
		if err != nil {
			return err
		}
		if _, err = writeAll(w, frame); err != nil {
			return err
		}
	}
	if p.cfg.FullDuplex != nil {
		value := byte(0)
		if *p.cfg.FullDuplex {
			value = 1
		}
		frame, err := Encode(Frame{Port: p.cfg.Port, Command: CommandFullDuplex, Data: []byte{value}})
		if err != nil {
			return err
		}
		_, err = writeAll(w, frame)
		return err
	}
	return nil
}
func writeAll(w io.Writer, b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		n, e := w.Write(b)
		total += n
		b = b[n:]
		if e != nil {
			return total, e
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}
