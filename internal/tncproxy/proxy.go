package tncproxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/netallow"
	"github.com/packet-radio/ultimatepr/internal/transport/kiss"
)

const (
	maxKISSFrame  = 65535
	upstreamQueue = 256
)

// Proxy shares one upstream KISS TCP connection with multiple clients.
// It proxies complete KISS frames, not arbitrary TCP read fragments.
type Proxy struct {
	listen   string
	upstream string
	allowed  []string
	log      *slog.Logger
	mu       sync.Mutex
	clients  map[net.Conn]*client
	tx       chan []byte
	upMu     sync.Mutex
	up       net.Conn
}

type client struct {
	conn net.Conn
	mu   sync.Mutex
}

func Start(ctx context.Context, listen, upstream string, allowed []string, log *slog.Logger) error {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listen, err)
	}
	p := &Proxy{
		listen: listen, upstream: upstream, allowed: append([]string(nil), allowed...),
		log: log, clients: make(map[net.Conn]*client), tx: make(chan []byte, upstreamQueue),
	}
	log.Info("TNC proxy started", "listen", listen, "upstream", upstream)
	go p.run(ctx, l)
	return nil
}

func (p *Proxy) run(ctx context.Context, listener net.Listener) {
	defer listener.Close()
	go p.upstreamManager(ctx)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		p.closeUpstream()
		p.closeClients()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() == nil {
				p.log.Warn("TNC proxy listener stopped", "listen", p.listen, "error", err)
			}
			return
		}
		if !addressAllowed(conn.RemoteAddr(), p.allowed) {
			p.log.Warn("TNC proxy client rejected", "remote", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}
		c := &client{conn: conn}
		p.mu.Lock()
		p.clients[conn] = c
		p.mu.Unlock()
		p.log.Info("TNC proxy client connected", "remote", conn.RemoteAddr())
		go p.clientLoop(ctx, c)
	}
}

func addressAllowed(address net.Addr, allowed []string) bool {
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return false
	}
	return netallow.Allowed(netallow.ParseIP(host), allowed)
}

func (p *Proxy) clientLoop(ctx context.Context, c *client) {
	defer p.removeClient(c)
	decoder := kiss.NewDecoder(maxKISSFrame)
	b := make([]byte, 4096)
	for {
		n, err := c.conn.Read(b)
		if n > 0 {
			frames, decodeErrs := decoder.Feed(b[:n])
			for _, decodeErr := range decodeErrs {
				p.log.Warn("TNC proxy rejected malformed client KISS data", "remote", c.conn.RemoteAddr(), "error", decodeErr)
			}
			for _, frame := range frames {
				if !clientCommandAllowed(frame.Command) {
					p.log.Warn("TNC proxy blocked client KISS command", "remote", c.conn.RemoteAddr(), "command", frame.Command)
					continue
				}
				wire, encodeErr := kiss.Encode(frame)
				if encodeErr != nil {
					continue
				}
				select {
				case p.tx <- wire:
				case <-ctx.Done():
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func clientCommandAllowed(command uint8) bool {
	// DATA and the portable KISS link parameters are safe to forward.
	// SET HARDWARE is device-specific and RETURN would terminate KISS for every client.
	return command <= kiss.CommandFullDuplex
}

func (p *Proxy) upstreamManager(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, "tcp", p.upstream)
		if err != nil {
			p.log.Warn("TNC proxy upstream unavailable", "address", p.upstream, "error", err)
			if !waitFor(ctx, time.Second) {
				return
			}
			continue
		}
		p.setUpstream(conn)
		p.log.Info("TNC proxy connected upstream", "address", p.upstream)
		err = p.serveUpstream(ctx, conn)
		p.clearUpstream(conn)
		_ = conn.Close()
		if ctx.Err() == nil {
			p.log.Warn("TNC proxy upstream disconnected", "address", p.upstream, "error", err)
			if !waitFor(ctx, time.Second) {
				return
			}
		}
	}
}

func (p *Proxy) serveUpstream(ctx context.Context, conn net.Conn) error {
	readErr := make(chan error, 1)
	go p.readUpstream(ctx, conn, readErr)
	for {
		select {
		case wire := <-p.tx:
			if _, err := writeAll(conn, wire); err != nil {
				return err
			}
		case err := <-readErr:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Proxy) readUpstream(ctx context.Context, conn net.Conn, errch chan<- error) {
	decoder := kiss.NewDecoder(maxKISSFrame)
	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if n > 0 {
			frames, decodeErrs := decoder.Feed(b[:n])
			for _, decodeErr := range decodeErrs {
				p.log.Warn("TNC proxy rejected malformed upstream KISS data", "error", decodeErr)
			}
			for _, frame := range frames {
				wire, encodeErr := kiss.Encode(frame)
				if encodeErr == nil {
					p.broadcast(wire)
				}
			}
		}
		if err != nil {
			select {
			case errch <- err:
			case <-ctx.Done():
			}
			return
		}
	}
}

func (p *Proxy) broadcast(data []byte) {
	p.mu.Lock()
	clients := make([]*client, 0, len(p.clients))
	for _, c := range p.clients {
		clients = append(clients, c)
	}
	p.mu.Unlock()
	for _, c := range clients {
		c.mu.Lock()
		_, err := writeAll(c.conn, data)
		c.mu.Unlock()
		if err != nil {
			p.removeClient(c)
		}
	}
}

func writeAll(w io.Writer, b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		n, err := w.Write(b)
		total += n
		b = b[n:]
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	select {
	case <-time.After(delay):
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Proxy) setUpstream(conn net.Conn) {
	p.upMu.Lock()
	p.up = conn
	p.upMu.Unlock()
}

func (p *Proxy) clearUpstream(conn net.Conn) {
	p.upMu.Lock()
	if p.up == conn {
		p.up = nil
	}
	p.upMu.Unlock()
}

func (p *Proxy) removeClient(c *client) {
	p.mu.Lock()
	if _, ok := p.clients[c.conn]; ok {
		delete(p.clients, c.conn)
		_ = c.conn.Close()
		p.log.Info("TNC proxy client disconnected", "remote", c.conn.RemoteAddr())
	}
	p.mu.Unlock()
}

func (p *Proxy) closeUpstream() {
	p.upMu.Lock()
	defer p.upMu.Unlock()
	if p.up != nil {
		_ = p.up.Close()
		p.up = nil
	}
}

func (p *Proxy) closeClients() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for conn := range p.clients {
		_ = conn.Close()
		delete(p.clients, conn)
	}
}
