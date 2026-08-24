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
)

// Proxy shares one upstream KISS TCP connection with multiple clients.
type Proxy struct {
	listen   string
	upstream string
	allowed  []string
	log      *slog.Logger
	mu       sync.Mutex
	clients  map[net.Conn]*client
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
	p := &Proxy{listen: listen, upstream: upstream, allowed: append([]string(nil), allowed...), log: log, clients: make(map[net.Conn]*client)}
	log.Info("TNC proxy started", "listen", listen, "upstream", upstream)
	go p.run(ctx, l)
	return nil
}

func (p *Proxy) run(ctx context.Context, listener net.Listener) {
	defer listener.Close()
	go func() {
		<-ctx.Done()
		listener.Close()
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
	return netallow.Allowed(net.ParseIP(host), allowed)
}

func (p *Proxy) clientLoop(ctx context.Context, c *client) {
	defer p.removeClient(c)
	b := make([]byte, 4096)
	for {
		n, err := c.conn.Read(b)
		if n > 0 {
			data := append([]byte(nil), b[:n]...)
			if err := p.writeUpstream(ctx, data); err != nil && ctx.Err() == nil {
				p.log.Warn("TNC proxy upstream unavailable", "error", err)
			}
			p.broadcast(data, c)
		}
		if err != nil {
			return
		}
	}
}

func (p *Proxy) writeUpstream(ctx context.Context, data []byte) error {
	for {
		p.upMu.Lock()
		up := p.up
		if up != nil {
			_, err := up.Write(data)
			p.upMu.Unlock()
			if err != nil {
				p.closeUpstream()
			}
			return err
		}
		p.upMu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := p.connectUpstream(ctx); err != nil {
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
	}
}

func (p *Proxy) connectUpstream(ctx context.Context) error {
	p.upMu.Lock()
	defer p.upMu.Unlock()
	if p.up != nil {
		return nil
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", p.upstream)
	if err != nil {
		return err
	}
	p.up = conn
	p.log.Info("TNC proxy connected upstream", "address", p.upstream)
	go p.upstreamLoop(ctx, conn)
	return nil
}

func (p *Proxy) upstreamLoop(ctx context.Context, conn net.Conn) {
	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if n > 0 {
			p.broadcast(append([]byte(nil), b[:n]...), nil)
		}
		if err != nil {
			p.upMu.Lock()
			if p.up == conn {
				p.up = nil
			}
			p.upMu.Unlock()
			if ctx.Err() == nil && err != io.EOF {
				p.log.Warn("TNC proxy upstream disconnected", "error", err)
			}
			_ = conn.Close()
			return
		}
	}
}

func (p *Proxy) broadcast(data []byte, except *client) {
	p.mu.Lock()
	clients := make([]*client, 0, len(p.clients))
	for _, c := range p.clients {
		if c != except {
			clients = append(clients, c)
		}
	}
	p.mu.Unlock()
	for _, c := range clients {
		c.mu.Lock()
		_, err := c.conn.Write(data)
		c.mu.Unlock()
		if err != nil {
			p.removeClient(c)
		}
	}
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
