package gamehall

import (
	"bytes"
	"strings"
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

func TestTerminalLobbyScreens(t *testing.T) {
	h, out, _ := connectedHall(t)
	c := h.client("SP5AAA")
	h.writeLobby(c)
	lobby := out.String()
	if strings.Count(lobby, "SALON GIER") != 1 || strings.Contains(lobby, "1. ") || strings.Contains(lobby, "(INVITE)") {
		t.Fatalf("lobby=%q", lobby)
	}
	for _, want := range []string{"GAMES", "PLAYERS", "INVITES", "HELP", "QUIT"} {
		if !strings.Contains(lobby, want) {
			t.Fatalf("lobby missing %q: %q", want, lobby)
		}
	}
	out.Reset()
	c.mode = modeGames
	h.writeGames(c)
	games := out.String()
	if !strings.Contains(games, "1. Kolko i krzyzyk") || !strings.Contains(games, "2 graczy") || strings.Contains(games, "(INVITE)") {
		t.Fatalf("games=%q", games)
	}
	out.Reset()
	c.selected = TicTacToe
	c.mode = modeGame
	h.writeGameLobby(c)
	preGame := out.String()
	if strings.Contains(preGame, "1 2 3") || strings.Contains(preGame, "Ruch:") || strings.Count(preGame, "BACK") != 1 {
		t.Fatalf("pre-game=%q", preGame)
	}
}

func TestNumberedMenuRouting(t *testing.T) {
	h, _, _ := connectedHall(t)
	c := h.client("SP5AAA")

	if !h.handleLobbyCommand(c, "1", []string{"1"}) || c.mode != modeGames {
		t.Fatal("GAME> 1 did not open GAMES")
	}
	if !h.handleGamesCommand(c, "1") || c.mode != modeGame || c.selected != TicTacToe {
		t.Fatal("GAMES> 1 did not select Tic-Tac-Toe")
	}
	if !h.handleGameLobbyCommand(c, "1", []string{"1"}) || c.mode != modeInviteTarget {
		t.Fatal("TICTACTOE> 1 did not open callsign input")
	}
	if h.prompt(c) != "TICTACTOE" {
		t.Fatalf("invite prompt=%q", h.prompt(c))
	}
	if !h.handleInviteTarget(c, "SQ4BBB", "SQ4BBB") || len(h.Invitations("SQ4BBB")) != 1 {
		t.Fatal("callsign input did not use PLAY invite flow")
	}

	c.mode = modeLobby
	if !h.handleLobbyCommand(c, "2", []string{"2"}) || c.mode != modePlayers {
		t.Fatal("GAME> 2 did not open PLAYERS")
	}
	if !h.handlePlayersCommand(c, "3") || c.mode != modeLobby {
		t.Fatal("PLAYERS back number did not return to lobby")
	}
	if !h.handleLobbyCommand(c, "3", []string{"3"}) || c.mode != modeInvites {
		t.Fatal("GAME> 3 did not open INVITES")
	}
	if !h.handleInviteCommand(c, "BACK", []string{"BACK"}) || c.mode != modeLobby {
		t.Fatal("INVITES BACK did not return to lobby")
	}
	if !h.handleLobbyCommand(c, "4", []string{"4"}) {
		t.Fatal("GAME> 4 did not open help")
	}
	if !h.handleLobbyCommand(c, strings.ToUpper("games"), []string{"games"}) || c.mode != modeGames {
		t.Fatal("textual command was not preserved")
	}
	if !h.handleGamesCommand(c, "2") || c.mode != modeLobby {
		t.Fatal("GAMES back number did not return to lobby")
	}
}

func TestInvitationShortcuts(t *testing.T) {
	h, _, _ := connectedHall(t)
	c := h.client("SQ4BBB")
	_, err := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err != nil {
		t.Fatal(err)
	}
	if !h.handleInvitationShortcut(c, "A", []string{"A"}) {
		t.Fatal("single A was not handled")
	}
	if len(h.Invitations("SQ4BBB")) != 0 || len(h.sessions) != 1 {
		t.Fatalf("accept state invites=%v sessions=%d", h.Invitations("SQ4BBB"), len(h.sessions))
	}
	if h.handleInvitationShortcut(c, "D", []string{"D"}) {
		t.Fatal("D without an invitation should be unhandled")
	}

	// A second recipient is used to verify that a bare shortcut is not ambiguous.
	h2, _, _ := connectedHall(t)
	if err := h2.Connect("SQ4CCC", "pl", new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	first, _ := h2.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	second, _ := h2.Invite("SQ4CCC", "SQ4BBB", TicTacToe)
	recipient := h2.client("SQ4BBB")
	if !h2.handleInvitationShortcut(recipient, "A", []string{"A"}) || recipient.mode != modeInvites || len(h2.Invitations("SQ4BBB")) != 2 {
		t.Fatal("bare A selected an invitation with multiple pending invitations")
	}
	if !h2.handleInvitationShortcut(recipient, "D", []string{"D", "2"}) || len(h2.Invitations("SQ4BBB")) != 1 {
		t.Fatal("D 2 did not decline the selected invitation")
	}
	if h2.Invitations("SQ4BBB")[0].ID != first.ID || second.ID == first.ID {
		t.Fatal("wrong invitation remained after D 2")
	}
}

func TestAcceptRendersActiveBoardOnceSessionStarts(t *testing.T) {
	h, _, recipientOut := connectedHall(t)
	inv, err := h.Invite("SP5AAA", "SQ4BBB", TicTacToe)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Accept("SQ4BBB", inv.ID); err != nil {
		t.Fatal(err)
	}
	output := recipientOut.String()
	if !strings.Contains(output, "  1 2 3\r\nA . . .") || !strings.Contains(output, "X: SP5AAA") || !strings.Contains(output, "O: SQ4BBB") || !strings.Contains(output, "Turn: SP5AAA") {
		t.Fatalf("active output=%q", output)
	}
	if strings.Count(output, "  1 2 3\r\n") != 1 {
		t.Fatalf("board rendered more than once: %q", output)
	}
	if strings.Contains(output, "\r\r\n") {
		t.Fatalf("invalid CRLF: %q", output)
	}
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

func (roomTestGame) Type() GameType             { return GameType("room-test") }
func (roomTestGame) Apply(string, string) error { return nil }
func (roomTestGame) View(string) PlayerView     { return roomTestView{} }
func (roomTestGame) CurrentPlayer() string      { return "" }
func (roomTestGame) Finished() bool             { return false }
func (roomTestGame) Winner() string             { return "" }

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
