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

func activeSessionID(h *Hall, call string) string {
	for _, p := range h.Players() {
		if p.Callsign == normalizeCall(call) {
			return p.SessionID
		}
	}
	return ""
}

func TestInvitationLifecycle(t *testing.T) {
	h, _, _ := connectedHall(t)
	inv, err := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err != nil {
		t.Fatal(err)
	}
	if inv.State != InvitationPending {
		t.Fatalf("state=%s", inv.State)
	}
	if got := h.Invitations("SQ4BBB"); len(got) != 1 || got[0].ID != inv.ID {
		t.Fatalf("invites=%+v", got)
	}
	if err := h.Accept("SQ4BBB", inv.ID); err != nil {
		t.Fatal(err)
	}
	sid := activeSessionID(h, "SP5AAA")
	if sid == "" {
		t.Fatal("no active session after accept")
	}
	got, _ := h.Session(sid)
	if got.State != SessionActive || got.CurrentPlayer != "SP5AAA" || got.GameData == nil {
		t.Fatalf("session=%+v", got)
	}
}

func TestInvitationDeclineReturnsPlayersToLobby(t *testing.T) {
	h, _, _ := connectedHall(t)
	inv, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err := h.Decline("SQ4BBB", inv.ID); err != nil {
		t.Fatal(err)
	}
	if got := h.Invitations("SQ4BBB"); len(got) != 0 {
		t.Fatalf("invites=%+v", got)
	}
	for _, p := range h.Players() {
		if p.Status != PlayerLobby || p.SessionID != "" || p.RoomID != "" {
			t.Fatalf("player=%+v", p)
		}
	}
}

func TestCloseActiveSession(t *testing.T) {
	h, _, _ := connectedHall(t)
	inv, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	_ = h.Accept("SQ4BBB", inv.ID)
	sid := activeSessionID(h, "SP5AAA")
	if sid == "" {
		t.Fatal("no active session")
	}
	h.Close("SP5AAA")
	got, _ := h.Session(sid)
	if got.State != SessionCancelled {
		t.Fatalf("state=%s", got.State)
	}
}

func TestDisconnectMarksSessionAndNotifiesPeer(t *testing.T) {
	h, _, out := connectedHall(t)
	inv, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	_ = h.Accept("SQ4BBB", inv.ID)
	sid := activeSessionID(h, "SP5AAA")
	if sid == "" {
		t.Fatal("no active session")
	}
	h.Disconnect("SP5AAA")
	got, _ := h.Session(sid)
	if got.State != SessionDisconnected {
		t.Fatalf("state=%s", got.State)
	}
	if !bytes.Contains(out.Bytes(), []byte("disconnected")) {
		t.Fatalf("notification=%q", out.String())
	}
}

func TestInvitationExpires(t *testing.T) {
	h, _, _ := connectedHall(t)
	inv, _ := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	h.expire(inv.ID)
	if got := h.Invitations("SQ4BBB"); len(got) != 0 {
		t.Fatalf("invites=%+v", got)
	}
}

type roomTestGame struct{}

func (roomTestGame) Type() GameType { return GameType("room-test") }
func (roomTestGame) Apply(string, string) error { return nil }
func (roomTestGame) View(string) PlayerView { return roomTestView{} }
func (roomTestGame) CurrentPlayer() string { return "" }
func (roomTestGame) Finished() bool { return false }
func (roomTestGame) Winner() string { return "" }

type roomTestView struct{}

func registerRoomGame(t *testing.T, h *Hall) GameType {
	t.Helper()
	gameType := GameType("room-test")
	if err := h.RegisterGame(GameDefinition{
		ID:         gameType,
		Name:       "Room Test",
		NameKey:    "game_room_test_name",
		MinPlayers: 2,
		MaxPlayers: 2,
		JoinMode:   JoinModeRoom,
		Visibility: StatePublic,
		Prompt:     "ROOMTEST",
	}, func(players []string) (Game, error) {
		return roomTestGame{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	return gameType
}

func TestRoomStartAndCapacity(t *testing.T) {
	h, _, _ := connectedHall(t)
	registerRoomGame(t, h)
	if err := h.Connect("SQ4CCC", "pl", new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	room, err := h.CreateRoom("SP5AAA", GameType("room-test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.StartRoom("SP5AAA", room.ID); err == nil {
		t.Fatal("start before min_players should fail")
	}
	if _, err := h.JoinRoom("SQ4BBB", room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.JoinRoom("SQ4CCC", room.ID); err == nil {
		t.Fatal("join beyond max_players should fail")
	}
	sid := ""
	if sess, err := h.StartRoom("SP5AAA", room.ID); err != nil {
		t.Fatal(err)
	} else {
		sid = sess.ID
	}
	got, _ := h.Session(sid)
	if got.State != SessionActive || len(got.Players) != 2 {
		t.Fatalf("session=%+v", got)
	}
}

func TestRoomHostCloseReturnsPlayersToLobby(t *testing.T) {
	h, _, _ := connectedHall(t)
	registerRoomGame(t, h)
	room, err := h.CreateRoom("SP5AAA", GameType("room-test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.JoinRoom("SQ4BBB", room.ID); err != nil {
		t.Fatal(err)
	}
	h.Close("SP5AAA")
	if _, ok := h.Room(room.ID); ok {
		t.Fatal("room still exists after host close")
	}
	for _, p := range h.Players() {
		if p.Status != PlayerLobby || p.RoomID != "" {
			t.Fatalf("player=%+v", p)
		}
	}
}
