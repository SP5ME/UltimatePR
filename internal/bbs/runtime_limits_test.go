package bbs

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoveExpiredKeepsUndeliveredMessages(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "bbs.json"))
	if err != nil {
		t.Fatal(err)
	}
	bulletin, _ := store.Send("B", "SP5AAA", "POL", "old bulletin", "body")
	private, _ := store.Send("P", "SP5AAA", "SP5BBB", "old private", "body")
	fresh, _ := store.Send("P", "SP5AAA", "SP5CCC", "fresh", "body")
	old := time.Now().Add(-48 * time.Hour)
	store.mu.Lock()
	for i := range store.data.Messages {
		if store.data.Messages[i].ID == bulletin.ID || store.data.Messages[i].ID == private.ID {
			store.data.Messages[i].CreatedAt = old
		}
	}
	store.data.Messages[0].Forward = map[string]ForwardState{"peer": {Status: "failed"}}
	store.mu.Unlock()
	removedB, removedP, err := store.RemoveExpired(time.Now(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if removedB != 0 || removedP != 1 {
		t.Fatalf("removed bulletins=%d personal=%d", removedB, removedP)
	}
	if len(store.Messages()) != 2 || !containsMessage(store.Messages(), bulletin.ID) || !containsMessage(store.Messages(), fresh.ID) {
		t.Fatalf("unexpected remaining messages: %+v", store.Messages())
	}
}

func TestRemoveExpiredDisabledForNonPositiveRetention(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	m, _ := store.Send("B", "SP5AAA", "POL", "bulletin", "body")
	store.mu.Lock()
	store.data.Messages[0].CreatedAt = time.Now().Add(-48 * time.Hour)
	store.mu.Unlock()
	if b, p, err := store.RemoveExpired(time.Now(), 0, 0); err != nil || b != 0 || p != 0 || !containsMessage(store.Messages(), m.ID) {
		t.Fatalf("disabled retention removed mail: b=%d p=%d err=%v", b, p, err)
	}
}

func TestBodyLimitAtStorageAndLocalSession(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	store.SetMaxBodyBytes(4)
	if _, err := store.Send("P", "SP5AAA", "SP5BBB", "ok", "1234"); err != nil {
		t.Fatalf("body at limit rejected: %v", err)
	}
	if _, err := store.Send("P", "SP5AAA", "SP5BBB", "too long", "12345"); err == nil {
		t.Fatal("body over limit was accepted")
	}
	server := &Server{Title: "BBS", Node: "SP5AAA-8", Language: "en", MaxBodyBytes: 4, Store: store}
	var output bytes.Buffer
	server.Serve(strings.NewReader("SP5CCC\nName\n\nQTH\nLOC\nS SP5BBB\nsubject\n12345\n/EX\nB\n"), &output)
	if !strings.Contains(output.String(), "message body exceeds max_body_bytes") {
		t.Fatalf("local session did not report body limit: %s", output.String())
	}
}

func TestForwardPayloadLimitRejectsOversizedStream(t *testing.T) {
	payload := append(bytes.Repeat([]byte{'x'}, 64*1024+16), 0x1a, '\r')
	if _, err := readUntilTAPREOM(bufio.NewReader(bytes.NewReader(payload)), 4); err == nil {
		t.Fatal("oversized forwarding payload was accepted")
	}
}

func TestPeerScheduleWindowsUseHostTime(t *testing.T) {
	peer := ForwardPeer{Schedule: []string{"08:00-18:00"}}
	if peerInSchedule(peer, time.Date(2026, 1, 1, 7, 59, 0, 0, time.Local)) || peerInSchedule(peer, time.Date(2026, 1, 1, 18, 0, 0, 0, time.Local)) {
		t.Fatal("single-day schedule boundary is incorrect")
	}
	peer.Schedule = []string{"22:00-06:00"}
	if !peerInSchedule(peer, time.Date(2026, 1, 1, 23, 0, 0, 0, time.Local)) || !peerInSchedule(peer, time.Date(2026, 1, 2, 5, 59, 0, 0, time.Local)) || peerInSchedule(peer, time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)) {
		t.Fatal("overnight schedule is incorrect")
	}
	peer.Schedule = nil
	if !peerInSchedule(peer, time.Now()) {
		t.Fatal("empty schedule should allow forwarding")
	}
}

func containsMessage(messages []Message, id int64) bool {
	for _, message := range messages {
		if message.ID == id {
			return true
		}
	}
	return false
}
