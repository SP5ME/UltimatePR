package bbs

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

type ForwardPeer struct {
	ID, Callsign, Transport, Host, ViaNode string
	Port                                   uint16
	PrivateRoutes, BulletinScopes          []string
	ToCalls, AtCalls, HierarchicalRoutes   []string
	Enabled, Send, Receive                 bool
	SendConfigured, ReceiveConfigured      bool
}

func (p ForwardPeer) CanSend() bool    { return !p.SendConfigured || p.Send }
func (p ForwardPeer) CanReceive() bool { return !p.ReceiveConfigured || p.Receive }

type ForwardItem struct {
	PeerID  string
	Message Message
}

// PlanForwarding selects messages for each peer. Transmission and TAPR protocol
// negotiation stay separate from routing and persistence.
func PlanForwarding(messages []Message, peers []ForwardPeer, maxPerPeer int) []ForwardItem {
	if maxPerPeer < 1 {
		maxPerPeer = 50
	}
	var out []ForwardItem
	for _, p := range peers {
		if !p.Enabled || !p.CanSend() {
			continue
		}
		count := 0
		for _, m := range messages {
			if count >= maxPerPeer {
				break
			}
			if _, exists := m.Forward[p.ID]; !exists && matchesPeer(m, p) {
				out = append(out, ForwardItem{PeerID: p.ID, Message: m})
				count++
			}
		}
	}
	return out
}

func PrepareQueues(store *Store, peers []ForwardPeer, maxPerPeer int) error {
	items := PlanForwarding(store.Messages(), peers, maxPerPeer)
	byPeer := map[string][]int64{}
	for _, item := range items {
		byPeer[item.PeerID] = append(byPeer[item.PeerID], item.Message.ID)
	}
	for peer, ids := range byPeer {
		if err := store.QueueForward(peer, ids); err != nil {
			return err
		}
	}
	return nil
}

type QueuePlanner struct {
	Store      *Store
	Peers      []ForwardPeer
	Interval   time.Duration
	MaxPerPeer int
	Log        *slog.Logger
}

func (q *QueuePlanner) Run(ctx context.Context) {
	if q.Interval < 10*time.Second {
		q.Interval = 5 * time.Minute
	}
	run := func() {
		if err := PrepareQueues(q.Store, q.Peers, q.MaxPerPeer); err != nil && q.Log != nil {
			q.Log.Warn("forward queue planning failed", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(q.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			run()
		case <-ctx.Done():
			return
		}
	}
}
func matchesPeer(m Message, p ForwardPeer) bool {
	if m.Type == "P" || m.Type == "T" {
		if matches(strings.ToUpper(m.To), p.ToCalls) {
			return true
		}
		dest := strings.ToUpper(m.At)
		if dest == "" {
			dest = strings.ToUpper(m.To)
		}
		atCall := strings.Split(dest, ".")[0]
		return matches(atCall, p.AtCalls) || matches(dest, p.HierarchicalRoutes) || matches(dest, p.PrivateRoutes)
	}
	if m.Type == "B" {
		return matches(strings.ToUpper(m.Distribution), p.BulletinScopes)
	}
	return false
}
func matches(value string, patterns []string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	for _, raw := range patterns {
		p := strings.ToUpper(strings.TrimSpace(raw))
		if p == "*" || value == p || strings.HasPrefix(value, p+"-") || strings.HasSuffix(value, p) {
			return true
		}
	}
	return false
}
