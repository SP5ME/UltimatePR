package bbs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

func (s *Server) RunForward(ctx context.Context, listen string) error {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	return s.runForwardListener(ctx, ln)
}

func (s *Server) runForwardListener(ctx context.Context, ln net.Listener) error {
	defer ln.Close()
	go func() { <-ctx.Done(); _ = ln.Close() }()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() { defer wg.Done(); defer c.Close(); s.handleForward(c) }()
	}
}

func (s *Server) handleForward(c net.Conn) {
	_ = c.SetDeadline(time.Now().Add(2 * time.Minute))
	if err := s.serveTAPRForward(c); err != nil && s.Log != nil {
		s.Log.Warn("TAPR BBS forwarding session failed", "error", err)
	}
}

type Forwarder struct {
	Store                                    *Store
	Peers                                    []ForwardPeer
	Interval, ConnectTimeout, SessionTimeout time.Duration
	MaxMessages                              int
	MaxBodyBytes                             int
	LocalCall                                string
	LocalAddress                             string
	Log                                      *slog.Logger
	trigger                                  chan struct{}
	triggerOnce                              sync.Once
}

func (f *Forwarder) Trigger() {
	f.triggerOnce.Do(func() { f.trigger = make(chan struct{}, 1) })
	select {
	case f.trigger <- struct{}{}:
	default:
	}
}

func (f *Forwarder) Run(ctx context.Context) {
	f.triggerOnce.Do(func() { f.trigger = make(chan struct{}, 1) })
	if f.Interval < 10*time.Second {
		f.Interval = 5 * time.Minute
	}
	run := func() {
		_ = PrepareQueues(f.Store, f.Peers, f.MaxMessages)
		for _, p := range f.Peers {
			if p.Enabled && p.CanSend() && p.Transport == "telnet" {
				if err := f.forwardPeer(ctx, p); err != nil && f.Log != nil {
					f.Log.Warn("BBS forwarding failed", "peer", p.ID, "error", err)
				}
			}
		}
	}
	run()
	t := time.NewTicker(f.Interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			run()
		case <-f.trigger:
			run()
		case <-ctx.Done():
			return
		}
	}
}

func (f *Forwarder) forwardPeer(ctx context.Context, p ForwardPeer) error {
	if !peerInSchedule(p, time.Now()) {
		if f.Log != nil {
			f.Log.Info("BBS forwarding peer outside schedule window", "peer", p.ID)
		}
		return nil
	}
	if !p.CanSend() {
		return nil
	}
	q := f.Store.ForwardQueue(p.ID, f.MaxMessages)
	if len(q) == 0 {
		return nil
	}
	d := net.Dialer{Timeout: durationOr(f.ConnectTimeout, 10*time.Second)}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(p.Host, fmt.Sprint(p.Port)))
	if err != nil {
		return err
	}
	defer c.Close()
	sessionTimeout := durationOr(f.SessionTimeout, time.Minute)
	if sessionTimeout < 2*time.Minute {
		sessionTimeout = 2 * time.Minute
	}
	_ = c.SetDeadline(time.Now().Add(sessionTimeout))
	return f.exchangeTAPRMaster(c, p.ID, q)
}

func readUntilTAPREOM(r *bufio.Reader, maxBodyBytes int) ([]byte, error) {
	maxPayload := maxBodyBytes + 64*1024
	if maxPayload < 64*1024 {
		maxPayload = 64 * 1024
	}
	b := make([]byte, 0, minInt(maxPayload, 4096))
	for {
		value, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if value == 0x1a {
			break
		}
		if len(b) >= maxPayload {
			return nil, fmt.Errorf("message payload exceeds max_body_bytes (%d)", maxBodyBytes)
		}
		b = append(b, value)
	}
	next, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if next != '\r' {
		return nil, errors.New("TAPR message is not terminated by CR")
	}
	if r.Buffered() > 0 {
		n, _ := r.Peek(1)
		if len(n) > 0 && n[0] == '\n' {
			_, _ = r.ReadByte()
		}
	}
	return b, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (f *Forwarder) bodyLimitBytes() int {
	if f.MaxBodyBytes > 0 {
		return f.MaxBodyBytes
	}
	if f.Store != nil {
		return f.Store.bodyLimitBytes()
	}
	return 131072
}

func readTAPRLine(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\r')
	if err != nil {
		return "", err
	}
	if r.Buffered() > 0 {
		if next, _ := r.Peek(1); len(next) == 1 && next[0] == '\n' {
			_, _ = r.ReadByte()
		}
	}
	return strings.TrimSuffix(s, "\r"), nil
}

func writeTAPRLine(w io.Writer, line string) error {
	_, err := io.WriteString(w, line+"\r")
	return err
}

func durationOr(v, fallback time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return fallback
}
func stripSSID(s string) string { return strings.Split(strings.ToUpper(strings.TrimSpace(s)), "-")[0] }
