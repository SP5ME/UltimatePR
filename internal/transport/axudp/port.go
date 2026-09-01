package axudp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/packet-radio/ultimatepr/internal/transport"
)

type Config struct {
	ID, InterfaceID, Listen, RemoteHost string
	RemotePort                          uint16
	FCS                                 bool
	AllowFrom                           []string
	MaxFrame, Queue                     int
}
type Stats struct{ RXFrames, TXFrames, RXBytes, TXBytes, BadFCS, Dropped, Rejected atomic.Uint64 }
type Port struct {
	cfg     Config
	tx      chan transport.Packet
	log     *slog.Logger
	stats   Stats
	running atomic.Bool
}

func New(c Config, l *slog.Logger) *Port {
	if c.Queue < 1 {
		c.Queue = 256
	}
	if c.MaxFrame < 256 {
		c.MaxFrame = 4096
	}
	if c.InterfaceID == "" {
		c.InterfaceID = c.ID
	}
	return &Port{cfg: c, tx: make(chan transport.Packet, c.Queue), log: l}
}
func (p *Port) ID() string { return p.cfg.ID }
func (p *Port) Status() transport.Status {
	return transport.Status{ID: p.ID(), Type: "AXUDP", Connected: p.running.Load()}
}
func (p *Port) Send(ctx context.Context, pkt transport.Packet) error {
	select {
	case p.tx <- pkt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		p.stats.Dropped.Add(1)
		return errors.New("AXUDP TX queue full")
	}
}
func (p *Port) Run(ctx context.Context, out chan<- transport.Packet) error {
	local, err := net.ResolveUDPAddr("udp", p.cfg.Listen)
	if err != nil {
		return fmt.Errorf("resolve listen: %w", err)
	}
	remote, err := net.ResolveUDPAddr("udp", net.JoinHostPort(p.cfg.RemoteHost, fmt.Sprint(p.cfg.RemotePort)))
	if err != nil {
		return fmt.Errorf("resolve remote: %w", err)
	}
	c, err := net.ListenUDP("udp", local)
	if err != nil {
		return err
	}
	defer c.Close()
	p.running.Store(true)
	defer p.running.Store(false)
	p.log.Info("AXUDP listening", "port", p.ID(), "listen", c.LocalAddr(), "remote", remote)
	go func() { <-ctx.Done(); _ = c.Close() }()
	errch := make(chan error, 2)
	go p.read(ctx, c, out, errch)
	go p.write(ctx, c, remote, errch)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	}
}
func (p *Port) read(ctx context.Context, c *net.UDPConn, out chan<- transport.Packet, errch chan<- error) {
	b := make([]byte, p.cfg.MaxFrame+2)
	for {
		n, from, err := c.ReadFromUDP(b)
		if err != nil {
			errch <- err
			return
		}
		if !p.allowed(from.IP) {
			p.stats.Rejected.Add(1)
			continue
		}
		p.stats.RXBytes.Add(uint64(n))
		frame := append([]byte(nil), b[:n]...)
		if p.cfg.FCS {
			if len(frame) < 3 || !validFCS(frame) {
				p.stats.BadFCS.Add(1)
				continue
			}
			frame = frame[:len(frame)-2]
		}
		if len(frame) > p.cfg.MaxFrame {
			p.stats.Dropped.Add(1)
			continue
		}
		p.stats.RXFrames.Add(1)
		select {
		case out <- transport.Packet{InterfaceID: p.cfg.InterfaceID, PortID: p.ID(), Data: frame}:
		case <-ctx.Done():
			return
		}
	}
}
func (p *Port) write(ctx context.Context, c *net.UDPConn, remote *net.UDPAddr, errch chan<- error) {
	for {
		select {
		case pkt := <-p.tx:
			b := append([]byte(nil), pkt.Data...)
			if p.cfg.FCS {
				b = appendFCS(b)
			}
			n, err := c.WriteToUDP(b, remote)
			if err != nil {
				errch <- err
				return
			}
			p.stats.TXFrames.Add(1)
			p.stats.TXBytes.Add(uint64(n))
		case <-ctx.Done():
			return
		}
	}
}
func (p *Port) allowed(ip net.IP) bool {
	if len(p.cfg.AllowFrom) == 0 {
		return true
	}
	for _, v := range p.cfg.AllowFrom {
		if x := net.ParseIP(v); x != nil && x.Equal(ip) {
			return true
		}
		if _, n, e := net.ParseCIDR(v); e == nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
func appendFCS(b []byte) []byte { f := crc(b); return binary.LittleEndian.AppendUint16(b, f) }
func validFCS(b []byte) bool {
	want := binary.LittleEndian.Uint16(b[len(b)-2:])
	return crc(b[:len(b)-2]) == want
}
func crc(b []byte) uint16 {
	v := uint16(0xffff)
	for _, x := range b {
		v ^= uint16(x)
		for i := 0; i < 8; i++ {
			if v&1 != 0 {
				v = (v >> 1) ^ 0x8408
			} else {
				v >>= 1
			}
		}
	}
	return ^v
}
