package gamehall

import (
	"bytes"
	"testing"
	"time"
)

func connectedHall(t *testing.T) (*Hall, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	h := New(time.Minute)
	a, b := new(bytes.Buffer), new(bytes.Buffer)
	if err := h.Connect("SP5AAA", "pl", a); err != nil {
		t.Fatal(err)
	}
	if err := h.Connect("SQ4BBB", "en", b); err != nil {
		t.Fatal(err)
	}
	return h, a, b
}

func TestInvitationAcceptCreatesActiveSession(t *testing.T) {
	h, _, _ := connectedHall(t)
	s, err := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != SessionInvited {
		t.Fatalf("state=%s", s.State)
	}
	if err = h.Accept("SQ4BBB", s.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := h.Session(s.ID)
	if got.State != SessionActive || got.CurrentPlayer != "SP5AAA" || got.GameData == nil {
		t.Fatalf("session=%+v", got)
	}
}
func TestInvitationDeclineReturnsPlayersToLobby(t *testing.T) {
	h, _, _ := connectedHall(t)
	s, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err := h.Decline("SQ4BBB", s.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := h.Session(s.ID)
	if got.State != SessionCancelled {
		t.Fatalf("state=%s", got.State)
	}
	for _, p := range h.Players() {
		if p.Status != PlayerLobby || p.SessionID != "" {
			t.Fatalf("player=%+v", p)
		}
	}
}
func TestCloseActiveSession(t *testing.T) {
	h, _, _ := connectedHall(t)
	s, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	_ = h.Accept("SQ4BBB", s.ID)
	h.Close("SP5AAA")
	got, _ := h.Session(s.ID)
	if got.State != SessionCancelled {
		t.Fatalf("state=%s", got.State)
	}
}
func TestDisconnectMarksSessionAndNotifiesPeer(t *testing.T) {
	h, _, out := connectedHall(t)
	s, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	_ = h.Accept("SQ4BBB", s.ID)
	h.Disconnect("SP5AAA")
	got, _ := h.Session(s.ID)
	if got.State != SessionDisconnected {
		t.Fatalf("state=%s", got.State)
	}
	if !bytes.Contains(out.Bytes(), []byte("disconnected")) {
		t.Fatalf("notification=%q", out.String())
	}
}
func TestInvitationExpires(t *testing.T) {
	h := New(10 * time.Millisecond) // New enforces a practical default; call expiry directly to keep the test deterministic.
	a, b := new(bytes.Buffer), new(bytes.Buffer)
	_ = h.Connect("SP5AAA", "pl", a)
	_ = h.Connect("SQ4BBB", "pl", b)
	s, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	h.expire(s.ID)
	got, _ := h.Session(s.ID)
	if got.State != SessionCancelled {
		t.Fatalf("state=%s", got.State)
	}
}
