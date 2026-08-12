package bbs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	wl2k "github.com/la5nta/wl2k-go/fbb"
	"github.com/packet-radio/modernbbs/internal/language"
)

// This is the classic, uncompressed FBB forwarding protocol used by packet
// BBS software. B1F/B2F compression can be negotiated later without changing
// the queue and message store.
const fbbSID = "[ModernBBS-0.3.0-BF$]"

type fbbProposal struct {
	Kind, From, At, To, BID string
	Size                    int
}

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
	mailbox := newB2FMailbox(s.Store, "", nil)
	session := wl2k.NewSession(stripSSID(s.Node), "REMOTE", "", mailbox)
	session.IsMaster(true)
	configureB2FSession(session)
	_, _ = session.Exchange(c)
}

type Forwarder struct {
	Store                                    *Store
	Peers                                    []ForwardPeer
	Interval, ConnectTimeout, SessionTimeout time.Duration
	MaxMessages                              int
	LocalCall                                string
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
			if p.Enabled && p.Transport == "telnet" {
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
	mailbox := newB2FMailbox(f.Store, p.ID, q)
	session := wl2k.NewSession(stripSSID(f.LocalCall), stripSSID(p.Callsign), "", mailbox)
	configureB2FSession(session)
	_, err = session.Exchange(c)
	mailbox.flushResults()
	return err
}

func formatFBBProposal(m Message) string {
	at := m.At
	if at == "" {
		at = m.To
	}
	return fmt.Sprintf("FB %s %s %s %s %s %d", m.Type, stripSSID(m.From), at, stripSSID(m.To), wireBID(m), len(language.ASCII(m.Body)))
}

func parseFBBProposal(line string) (fbbProposal, bool) {
	f := strings.Fields(line)
	if len(f) != 7 || f[0] != "FB" || (f[1] != "P" && f[1] != "B") || len(f[5]) > 12 {
		return fbbProposal{}, false
	}
	sz, err := strconv.Atoi(f[6])
	if err != nil || sz < 0 {
		return fbbProposal{}, false
	}
	return fbbProposal{Kind: f[1], From: f[2], At: f[3], To: f[4], BID: f[5], Size: sz}, true
}

func sendFBBMessage(w io.Writer, m Message, local string) error {
	if err := writeFBBLine(w, language.ASCII(m.Subject)); err != nil {
		return err
	}
	now := time.Now().UTC()
	rline := fmt.Sprintf("R:%sZ %d@%s ModernBBS", now.Format("060102/1504"), m.ID, stripSSID(local))
	body := strings.ReplaceAll(language.ASCII(m.Body), "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	_, err := io.WriteString(w, rline+"\r\n\r\n"+body+"\r\n\x1a\r")
	return err
}

func readUntilCtrlZ(r *bufio.Reader) ([]byte, error) {
	b, err := r.ReadBytes(0x1a)
	if err != nil {
		return nil, err
	}
	b = b[:len(b)-1]
	next, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if next != '\r' {
		return nil, errors.New("FBB message is not terminated by CR")
	}
	if r.Buffered() > 0 {
		n, _ := r.Peek(1)
		if len(n) > 0 && n[0] == '\n' {
			_, _ = r.ReadByte()
		}
	}
	return b, nil
}

func readFBBLine(r *bufio.Reader) (string, error) {
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

func writeFBBLine(w io.Writer, line string) error {
	_, err := io.WriteString(w, line+"\r")
	return err
}

func checksumLine(line string) byte {
	var sum byte
	for _, b := range []byte(line + "\r") {
		sum += b
	}
	return sum
}

func isSID(s string) bool {
	return strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && strings.Contains(s, "F")
}
func durationOr(v, fallback time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return fallback
}
func minPositive(v, ceiling int) int {
	if v < 1 || v > ceiling {
		return ceiling
	}
	return v
}
func stripSSID(s string) string { return strings.Split(strings.ToUpper(strings.TrimSpace(s)), "-")[0] }
func wireBID(m Message) string {
	bid := strings.ToUpper(strings.TrimSpace(m.BID))
	if len(bid) > 0 && len(bid) <= 12 && !strings.ContainsAny(bid, " \r\n") {
		return bid
	}
	bid = fmt.Sprintf("%d_%s", m.ID, stripSSID(m.From))
	if len(bid) > 12 {
		bid = bid[:12]
	}
	return bid
}
