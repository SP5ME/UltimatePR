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

const (
	upstreamQueue = 256
	clientQueue   = 64
	writeTimeout  = 5 * time.Second
)

// Proxy shares one upstream KISS TCP connection with multiple clients.
// It is deliberately transparent: KISS framing and commands belong to clients
// and the upstream TNC, while the proxy only distributes the TCP byte stream.
type Proxy struct {
	listen   string
	upstream string
	allowed  []string
	log      *slog.Logger
	mu       sync.Mutex
	clients  map[net.Conn]*client
	tx       chan upstreamItem
	upMu     sync.Mutex
	up       net.Conn
}

type client struct {
	conn net.Conn
	tx   chan []byte
}

type upstreamItem struct {
	data   []byte
	source *client
}

func Start(ctx context.Context, listen, upstream string, allowed []string, log *slog.Logger) error {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listen, err)
	}
	p := &Proxy{
		listen: listen, upstream: upstream, allowed: append([]string(nil), allowed...),
		log: log, clients: make(map[net.Conn]*client), tx: make(chan upstreamItem, upstreamQueue),
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
		c := &client{conn: conn, tx: make(chan []byte, clientQueue)}
		p.mu.Lock()
		p.clients[conn] = c
		p.mu.Unlock()
		p.log.Info("TNC proxy client connected", "remote", conn.RemoteAddr())
		go p.clientWriter(ctx, c)
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
	b := make([]byte, 4096)
	for {
		n, err := c.conn.Read(b)
		if n > 0 {
			data := append([]byte(nil), b[:n]...)
			// Queue untouched bytes for the live upstream connection. The
			// upstream writer publishes them to peers after a successful write.
			if !p.enqueueUpstream(data, c) {
				p.log.Warn("TNC proxy dropped client data", "remote", c.conn.RemoteAddr(), "reason", "upstream unavailable or busy")
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *Proxy) clientWriter(ctx context.Context, c *client) {
	for {
		select {
		case data := <-c.tx:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := writeAll(c.conn, data); err != nil {
				p.removeClient(c)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Proxy) enqueueUpstream(data []byte, source *client) bool {
	p.upMu.Lock()
	connected := p.up != nil
	p.upMu.Unlock()
	if !connected {
		return false
	}
	select {
	case p.tx <- upstreamItem{data: data, source: source}:
		return true
	default:
		return false
	}
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
		p.discardQueuedUpstream()
		if ctx.Err() == nil {
			p.log.Warn("TNC proxy upstream disconnected", "address", p.upstream, "error", err)
			if !waitFor(ctx, time.Second) {
				return
			}
		}
	}
}

func (p *Proxy) discardQueuedUpstream() {
	for {
		select {
		case <-p.tx:
		default:
			return
		}
	}
}

func (p *Proxy) serveUpstream(ctx context.Context, conn net.Conn) error {
	readErr := make(chan error, 1)
	go p.readUpstream(ctx, conn, readErr)
	for {
		select {
		case item := <-p.tx:
			_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := writeAll(conn, item.data); err != nil {
				return err
			}
			p.broadcastExcept(item.data, item.source)
		case err := <-readErr:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Proxy) readUpstream(ctx context.Context, conn net.Conn, errch chan<- error) {
	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if n > 0 {
			p.broadcast(b[:n])
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
	p.broadcastExcept(data, nil)
}

func (p *Proxy) broadcastExcept(data []byte, excluded *client) {
	p.mu.Lock()
	clients := make([]*client, 0, len(p.clients))
	for _, c := range p.clients {
		if c == excluded {
			continue
		}
		clients = append(clients, c)
	}
	p.mu.Unlock()
	for _, c := range clients {
		frame := append([]byte(nil), data...)
		select {
		case c.tx <- frame:
		default:
			p.log.Warn("TNC proxy disconnected slow client", "remote", c.conn.RemoteAddr())
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
