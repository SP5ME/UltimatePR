package bbs

import (
	"errors"
	"io"
	"log"
	"os"
	"strings"

	wl2k "github.com/la5nta/wl2k-go/fbb"
)

// b2fMailbox adapts UltimatePR persistence and forwarding state to the open
// FBB B2 forwarding implementation. Protocol data never becomes the database
// format; this boundary also makes classical FBB fallbacks possible.
type b2fMailbox struct {
	store   *Store
	peerID  string
	queue   []Message
	byMID   map[string]Message
	results map[string]bool
}

func newB2FMailbox(store *Store, peerID string, queue []Message) *b2fMailbox {
	m := &b2fMailbox{store: store, peerID: peerID, queue: queue, byMID: make(map[string]Message), results: make(map[string]bool)}
	for _, msg := range queue {
		m.byMID[wireBID(msg)] = msg
	}
	return m
}

func (m *b2fMailbox) Prepare() error {
	if m.store == nil {
		return errors.New("BBS store is unavailable")
	}
	return nil
}

func (m *b2fMailbox) GetOutbound(_ ...wl2k.Address) (out []*wl2k.Message) {
	for _, src := range m.queue {
		if _, handled := m.results[wireBID(src)]; handled {
			continue
		}
		kind := wl2k.Private
		if src.Type == "B" {
			kind = wl2k.MsgType("Bulletin")
		}
		msg := wl2k.NewMessage(kind, stripSSID(src.From))
		msg.Header.Set(wl2k.HEADER_MID, wireBID(src))
		msg.Header.Set(wl2k.HEADER_MBO, stripSSID(src.From))
		msg.SetDate(src.CreatedAt)
		msg.SetSubject(src.Subject)
		msg.AddTo(src.To)
		if err := msg.SetBody(src.Body); err != nil {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func (m *b2fMailbox) SetSent(mid string, _ bool) {
	m.results[mid] = true
}

func (m *b2fMailbox) SetDeferred(mid string) {
	m.results[mid] = false
}

// flushResults writes session outcomes only after the protocol has exchanged
// FF/FQ. Slow disks must never stall the on-air protocol state machine.
func (m *b2fMailbox) flushResults() {
	for mid, delivered := range m.results {
		if src, ok := m.byMID[mid]; ok && m.peerID != "" {
			reason := ""
			if !delivered {
				reason = "deferred by remote BBS"
			}
			_ = m.store.RecordForward(m.peerID, src.ID, delivered, reason)
		}
	}
}

func (m *b2fMailbox) GetInboundAnswer(p wl2k.Proposal) wl2k.ProposalAnswer {
	if m.store.HasBID(p.MID()) {
		return wl2k.Reject
	}
	return wl2k.Accept
}

func (m *b2fMailbox) ProcessInbound(messages ...*wl2k.Message) error {
	for _, src := range messages {
		body, err := src.Body()
		if err != nil {
			return err
		}
		to := src.To()
		if len(to) == 0 {
			return errors.New("B2F message has no recipient")
		}
		kind := "P"
		if strings.EqualFold(string(src.Type()), "Bulletin") {
			kind = "B"
		}
		_, _, err = m.store.Import(Message{
			Type:      kind,
			From:      src.From().Addr,
			To:        to[0].Addr,
			BID:       src.MID(),
			Subject:   src.Subject(),
			Body:      body,
			CreatedAt: src.Date(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func configureB2FSession(s *wl2k.Session) {
	s.SetUserAgent(wl2k.UserAgent{Name: "UltimatePR", Version: BuildVersion})
	if os.Getenv("ULTIMATEPR_FBB_TRACE") == "1" || os.Getenv("MODERNBBS_FBB_TRACE") == "1" {
		s.SetLogger(log.New(os.Stderr, "FBB ", 0))
	} else {
		s.SetLogger(log.New(io.Discard, "", 0))
	}
}
