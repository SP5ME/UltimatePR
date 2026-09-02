package gamehall

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/packet-radio/ultimatepr/internal/language"
	"github.com/packet-radio/ultimatepr/internal/lineinput"
)

type clientMode string

const (
	modeLobby        clientMode = "lobby"
	modeGames        clientMode = "games"
	modeGame         clientMode = "game"
	modeInvites      clientMode = "invites"
	modePlayers      clientMode = "players"
	modeRoom         clientMode = "room"
	modePlay         clientMode = "play"
	modeInviteTarget clientMode = "invite_target"
)

type Player struct {
	Callsign  string
	Status    PlayerStatus
	GameType  GameType
	SessionID string
	RoomID    string
}

type client struct {
	player      Player
	lang        string
	w           io.Writer
	writeMu     sync.Mutex
	mode        clientMode
	selected    GameType
	roomID      string
	sessionID   string
	pendingGame string
	returnMode  clientMode
}

func (c *client) write(format string, args ...any) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	fmt.Fprintf(c.w, format, args...)
}

func (c *client) text(value string) { c.write("%s", terminalBlock(value)) }

type Hall struct {
	mu            sync.RWMutex
	clients       map[string]*client
	sessions      map[string]*GameSession
	rooms         map[string]*GameRoom
	invitations   map[string]*Invitation
	definitions   map[GameType]GameDefinition
	factories     map[GameType]Factory
	order         []GameType
	inviteTimeout time.Duration
	next          atomic.Uint64
	nextRoom      atomic.Uint64
}

func New(inviteTimeout time.Duration) *Hall {
	if inviteTimeout <= 0 {
		inviteTimeout = 2 * time.Minute
	}
	h := &Hall{
		clients:       map[string]*client{},
		sessions:      map[string]*GameSession{},
		rooms:         map[string]*GameRoom{},
		invitations:   map[string]*Invitation{},
		definitions:   map[GameType]GameDefinition{},
		factories:     map[GameType]Factory{},
		inviteTimeout: inviteTimeout,
	}
	_ = h.RegisterGame(GameDefinition{
		ID:         TicTacToe,
		Name:       "Tic-Tac-Toe",
		NameKey:    "game_tictactoe_name",
		MinPlayers: 2,
		MaxPlayers: 2,
		JoinMode:   JoinModeInvite,
		Visibility: StatePublic,
		Prompt:     "TICTACTOE",
	}, NewTicTacToe)
	_ = h.RegisterGame(GameDefinition{ID: ConnectFour, Name: "Connect Four", MinPlayers: 2, MaxPlayers: 2, JoinMode: JoinModeInvite, Visibility: StatePublic, Prompt: "CONNECT4"}, NewConnectFour)
	_ = h.RegisterGame(GameDefinition{ID: Hangman, NameKey: "game_hangman_name", MinPlayers: 1, MaxPlayers: 6, JoinMode: JoinModeRoom, JoinModes: []JoinMode{JoinModeSolo, JoinModeRoom}, Visibility: StateServerSecret, Prompt: "HANGMAN"}, NewHangman)
	_ = h.RegisterGame(GameDefinition{ID: WordGame, NameKey: "game_word_name", MinPlayers: 1, MaxPlayers: 6, JoinMode: JoinModeRoom, JoinModes: []JoinMode{JoinModeSolo, JoinModeRoom}, Visibility: StateServerSecret, Prompt: "WORDGAME"}, NewWordGame)
	return h
}

func supportsJoinMode(def GameDefinition, mode JoinMode) bool {
	if def.JoinMode == mode {
		return true
	}
	for _, candidate := range def.JoinModes {
		if candidate == mode {
			return true
		}
	}
	return false
}

func (h *Hall) RegisterGame(def GameDefinition, factory Factory) error {
	if h == nil {
		return ErrInvalidAction
	}
	if factory == nil || strings.TrimSpace(string(def.ID)) == "" || def.MinPlayers < 1 || def.MaxPlayers < def.MinPlayers || def.JoinMode == "" {
		return ErrInvalidAction
	}
	if def.Visibility == "" {
		def.Visibility = StatePublic
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.definitions[def.ID]; !exists {
		h.order = append(h.order, def.ID)
	}
	h.definitions[def.ID] = def
	h.factories[def.ID] = factory
	return nil
}

func (h *Hall) Definitions() []GameDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]GameDefinition, 0, len(h.order))
	for _, id := range h.order {
		if def, ok := h.definitions[id]; ok {
			out = append(out, def)
		}
	}
	return out
}

func (h *Hall) Definition(id GameType) (GameDefinition, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	def, ok := h.definitions[id]
	return def, ok
}

func (h *Hall) Players() []Player {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Player, 0, len(h.clients))
	for _, c := range h.clients {
		out = append(out, c.player)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Callsign < out[j].Callsign })
	return out
}

func (h *Hall) Session(id string) (GameSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.sessions[id]
	if !ok {
		return GameSession{}, false
	}
	copy := *s
	copy.Players = append([]string(nil), s.Players...)
	return copy, true
}

func (h *Hall) Room(id string) (GameRoom, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room, ok := h.rooms[id]
	if !ok {
		return GameRoom{}, false
	}
	copy := *room
	copy.Players = append([]string(nil), room.Players...)
	return copy, true
}

func (h *Hall) Rooms(gameType GameType) []GameRoom {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]GameRoom, 0, len(h.rooms))
	for _, room := range h.rooms {
		if room.State != RoomOpen {
			continue
		}
		if gameType != "" && room.GameType != gameType {
			continue
		}
		copy := *room
		copy.Players = append([]string(nil), room.Players...)
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (h *Hall) Invitations(call string) []Invitation {
	call = normalizeCall(call)
	h.mu.RLock()
	defer h.mu.RUnlock()
	now := time.Now().UTC()
	out := make([]Invitation, 0)
	for _, inv := range h.invitations {
		if inv.State != InvitationPending || inv.To != call || !inv.ExpiresAt.After(now) {
			continue
		}
		copy := *inv
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (h *Hall) Connect(call, lang string, w io.Writer) error {
	call = normalizeCall(call)
	if call == "" {
		return fmt.Errorf("callsign required")
	}
	h.mu.Lock()
	if _, exists := h.clients[call]; exists {
		h.mu.Unlock()
		return fmt.Errorf("player already connected")
	}
	c := &client{player: Player{Callsign: call, Status: PlayerLobby}, lang: language.Normalize(lang), w: w, mode: modeLobby}
	h.clients[call] = c
	h.mu.Unlock()
	return nil
}

func (h *Hall) Disconnect(call string) {
	call = normalizeCall(call)
	h.leaveSession(call, SessionDisconnected)
	h.leaveRoom(call, true)
	h.cancelInvitationsFor(call, true)
	h.mu.Lock()
	delete(h.clients, call)
	h.mu.Unlock()
}

func (h *Hall) Invite(from, to string, gameType GameType) (*Invitation, error) {
	from, to = normalizeCall(from), normalizeCall(to)
	h.mu.Lock()
	defer h.mu.Unlock()
	inviter, ok1 := h.clients[from]
	invited, ok2 := h.clients[to]
	def, ok3 := h.definitions[gameType]
	factory := h.factories[gameType]
	if !ok1 || !ok2 || !ok3 || factory == nil || def.JoinMode != JoinModeInvite || from == to {
		return nil, ErrInvalidAction
	}
	if inviter.player.Status != PlayerLobby || (invited.player.Status != PlayerLobby && invited.player.Status != PlayerWaiting) {
		return nil, ErrInvalidAction
	}
	now := time.Now().UTC()
	id := h.newIDLocked("I")
	inv := &Invitation{
		ID:           id,
		GameType:     gameType,
		GameName:     definitionName(def, invited.lang),
		From:         from,
		To:           to,
		State:        InvitationPending,
		CreatedAt:    now,
		LastActivity: now,
		ExpiresAt:    now.Add(h.inviteTimeout),
	}
	h.invitations[id] = inv
	h.refreshPlayerLocked(from)
	h.refreshPlayerLocked(to)
	go h.expireInvitation(id, inv.ExpiresAt)
	go inviter.text(fmt.Sprintf(language.T(inviter.lang, "game_invite_sent"), inv.GameName, to))
	go invited.text(fmt.Sprintf(language.T(invited.lang, "game_invited"), from, inv.GameName))
	return inv, nil
}

func (h *Hall) Accept(call, id string) error {
	call = normalizeCall(call)
	h.mu.Lock()
	inv := h.invitations[id]
	if inv == nil || inv.State != InvitationPending || inv.To != call || inv.ExpiresAt.Before(time.Now().UTC()) {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	_, ok := h.definitions[inv.GameType]
	factory := h.factories[inv.GameType]
	from := h.clients[inv.From]
	to := h.clients[inv.To]
	if !ok || factory == nil || from == nil || to == nil {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	session, err := h.startSessionLocked(inv.GameType, []string{inv.From, inv.To})
	if err != nil {
		h.mu.Unlock()
		return err
	}
	inv.State = InvitationAccepted
	inv.LastActivity = time.Now().UTC()
	delete(h.invitations, id)
	h.refreshPlayerLocked(inv.From)
	h.refreshPlayerLocked(inv.To)
	h.mu.Unlock()
	h.writeSessionState(from, session, true)
	h.writeSessionState(to, session, true)
	return nil
}

func (h *Hall) Decline(call, id string) error {
	call = normalizeCall(call)
	h.mu.Lock()
	inv := h.invitations[id]
	if inv == nil || inv.State != InvitationPending || inv.To != call {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	from := h.clients[inv.From]
	to := h.clients[inv.To]
	inv.State = InvitationDeclined
	inv.LastActivity = time.Now().UTC()
	delete(h.invitations, id)
	h.refreshPlayerLocked(inv.From)
	h.refreshPlayerLocked(inv.To)
	h.mu.Unlock()
	if from != nil {
		from.text(fmt.Sprintf(language.T(from.lang, "game_declined"), inv.GameName))
	}
	if to != nil {
		to.text(fmt.Sprintf(language.T(to.lang, "game_declined_self"), inv.GameName))
	}
	return nil
}

func (h *Hall) Close(call string) {
	call = normalizeCall(call)
	if h.leaveSession(call, SessionCancelled) {
		return
	}
	if h.leaveRoom(call, true) {
		return
	}
}

func (h *Hall) Action(call, action string) error {
	call = normalizeCall(call)
	h.mu.Lock()
	c := h.clients[call]
	if c == nil {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	s := h.sessions[c.player.SessionID]
	if s == nil || s.State != SessionActive {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	if err := s.GameData.Apply(call, action); err != nil {
		h.mu.Unlock()
		return err
	}
	s.CurrentPlayer, s.LastActivity = s.GameData.CurrentPlayer(), time.Now().UTC()
	if s.GameData.Finished() {
		s.State = SessionFinished
	}
	clients := h.sessionClientsLocked(s)
	viewState := s.State
	h.mu.Unlock()
	for _, x := range clients {
		h.writeSessionState(x, s, viewState == SessionActive)
	}
	return nil
}

func (h *Hall) Rematch(call string) error {
	call = normalizeCall(call)
	h.mu.Lock()
	c := h.clients[call]
	if c == nil {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	s := h.sessions[c.player.SessionID]
	if s == nil || s.State != SessionFinished {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	if s.RematchRequestedBy == "" {
		s.RematchRequestedBy = call
		other := h.otherClientLocked(s, call)
		h.mu.Unlock()
		if other != nil {
			other.text(fmt.Sprintf(language.T(other.lang, "game_rematch_request"), call))
		}
		return nil
	}
	if s.RematchRequestedBy == call {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	s.Players[0], s.Players[1] = s.Players[1], s.Players[0]
	factory := h.factories[s.GameType]
	if factory == nil {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	g, err := factory(s.Players)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	s.GameData, s.State, s.CurrentPlayer, s.RematchRequestedBy, s.LastActivity = g, SessionActive, g.CurrentPlayer(), "", time.Now().UTC()
	clients := h.sessionClientsLocked(s)
	h.mu.Unlock()
	for _, x := range clients {
		x.text(language.T(x.lang, "game_rematch_started"))
		h.writeSessionState(x, s, true)
	}
	return nil
}

func (h *Hall) Serve(call, lang string, in *bufio.Scanner, w io.Writer) {
	call = normalizeCall(call)
	if err := h.Connect(call, lang, w); err != nil {
		fmt.Fprint(w, terminalBlock(language.T(lang, "game_connect_error")))
		return
	}
	defer h.Disconnect(call)
	c := h.client(call)
	h.writeLobby(c)
	for {
		c.write("%s> ", h.prompt(c))
		if !in.Scan() {
			return
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cmd := strings.ToUpper(fields[0])
		h.refreshClientLocked(c)
		if c.mode == modeLobby && (cmd == "QUIT" || cmd == "Q" || cmd == "BYE" || cmd == "5") {
			c.text(language.T(c.lang, "game_goodbye"))
			return
		}
		if h.handleInvitationShortcut(c, cmd, fields) {
			continue
		}
		if c.mode == modeInviteTarget && h.handleInviteTarget(c, cmd, line) {
			continue
		}
		switch c.player.Status {
		case PlayerPlaying:
			if h.handleSessionCommand(c, cmd, fields, line) {
				continue
			}
		case PlayerInRoom:
			if h.handleRoomCommand(c, cmd, fields) {
				continue
			}
		}
		switch c.mode {
		case modeGames:
			if h.handleGamesCommand(c, cmd) {
				continue
			}
		case modeGame:
			if h.handleGameLobbyCommand(c, cmd, fields) {
				continue
			}
		case modeInvites:
			if h.handleInviteCommand(c, cmd, fields) {
				continue
			}
		case modePlayers:
			if h.handlePlayersCommand(c, cmd) {
				continue
			}
		case modeRoom:
			if h.handleRoomCommand(c, cmd, fields) {
				continue
			}
		}
		if h.handleLobbyCommand(c, cmd, fields) {
			continue
		}
		c.text(language.T(c.lang, "game_unknown"))
	}
}

func (h *Hall) ServeAX25(call, lang string, r io.Reader, w io.Writer) {
	scanner := lineinput.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	h.Serve(call, lang, scanner, w)
}

func (h *Hall) client(call string) *client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[call]
}

func (h *Hall) prompt(c *client) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch c.player.Status {
	case PlayerPlaying:
		if s := h.sessions[c.player.SessionID]; s != nil {
			return h.sessionPromptLocked(s)
		}
	case PlayerInRoom:
		if room := h.rooms[c.player.RoomID]; room != nil {
			return "ROOM#" + room.ID
		}
	}
	switch c.mode {
	case modeGame:
		if def, ok := h.definitions[c.selected]; ok {
			return definitionPrompt(def)
		}
		return "GAME"
	case modeInviteTarget:
		if def, ok := h.definitions[c.selected]; ok {
			return definitionPrompt(def)
		}
		return "GAME"
	case modeGames:
		return "GAMES"
	case modeInvites:
		return "INVITES"
	case modePlayers:
		return "PLAYERS"
	case modeRoom:
		if c.roomID != "" {
			return "ROOM#" + c.roomID
		}
		return "ROOM"
	default:
		return "GAME"
	}
}

func (h *Hall) writeLobby(c *client) {
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_menu_header"))
	b.WriteString(language.T(c.lang, "game_menu_footer"))
	c.text(b.String())
}

func (h *Hall) writeGames(c *client) {
	h.mu.RLock()
	defs := make([]GameDefinition, 0, len(h.order))
	for _, id := range h.order {
		if def, ok := h.definitions[id]; ok {
			defs = append(defs, def)
		}
	}
	h.mu.RUnlock()
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_games_header"))
	for i, def := range defs {
		b.WriteString(fmt.Sprintf("%d. %s\r\n", i+1, definitionName(def, c.lang)))
		b.WriteString(fmt.Sprintf("   %s\r\n", playerCountText(c.lang, def.MinPlayers, def.MaxPlayers)))
	}
	b.WriteString(language.T(c.lang, "game_games_footer"))
	b.WriteString(fmt.Sprintf(language.T(c.lang, "game_back_option"), len(defs)+1))
	c.text(b.String())
}

func (h *Hall) writePlayers(c *client) {
	players := h.Players()
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_players_header"))
	for _, p := range players {
		b.WriteString(fmt.Sprintf("%-10s %s\r\n", p.Callsign, language.T(c.lang, "game_status_"+string(p.Status))))
	}
	if len(players) == 0 {
		b.WriteString(language.T(c.lang, "game_no_players"))
	}
	b.WriteString(fmt.Sprintf(language.T(c.lang, "game_back_option"), len(players)+1))
	c.text(b.String())
}

func (h *Hall) writeInvites(c *client) {
	invites := h.Invitations(c.player.Callsign)
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_invites_header"))
	if len(invites) == 0 {
		b.WriteString(language.T(c.lang, "game_no_invites"))
	} else {
		for i, inv := range invites {
			b.WriteString(fmt.Sprintf("%d. %s - %s\r\n", i+1, inv.From, inv.GameName))
		}
		b.WriteString(language.T(c.lang, "game_invites_footer"))
	}
	b.WriteString(fmt.Sprintf(language.T(c.lang, "game_back_option"), len(invites)+1))
	c.text(b.String())
}

func (h *Hall) writeRooms(c *client, gameType GameType) {
	rooms := h.Rooms(gameType)
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_rooms_header"))
	if len(rooms) == 0 {
		b.WriteString(language.T(c.lang, "game_no_rooms"))
		c.text(b.String())
		return
	}
	b.WriteString(language.T(c.lang, "game_rooms_columns"))
	for _, room := range rooms {
		b.WriteString(fmt.Sprintf("%-4s %-8s %d/%d\r\n", room.ID, room.Host, len(room.Players), h.roomLimit(room.GameType)))
	}
	b.WriteString(language.T(c.lang, "game_rooms_footer"))
	c.text(b.String())
}

func (h *Hall) writeGameLobby(c *client) {
	h.mu.RLock()
	def, ok := h.definitions[c.selected]
	h.mu.RUnlock()
	if !ok {
		h.writeLobby(c)
		return
	}
	var b strings.Builder
	heading := definitionName(def, c.lang)
	if def.ID == TicTacToe {
		heading = language.T(c.lang, "ttt_heading")
	}
	b.WriteString(heading + "\r\n\r\n")
	if supportsJoinMode(def, JoinModeSolo) && supportsJoinMode(def, JoinModeRoom) {
		b.WriteString(language.T(c.lang, "game_secret_intro"))
		b.WriteString(language.T(c.lang, "game_secret_help"))
		c.text(b.String())
		return
	}
	switch def.JoinMode {
	case JoinModeInvite:
		b.WriteString(language.T(c.lang, "ttt_goal"))
		b.WriteString(language.T(c.lang, "game_game_invite_help"))
		c.text(b.String())
	case JoinModeRoom:
		b.WriteString(language.T(c.lang, "game_room_intro"))
		c.text(b.String())
		h.writeRooms(c, def.ID)
	default:
		b.WriteString(language.T(c.lang, "game_game_solo_help"))
		c.text(b.String())
	}
}

func (h *Hall) handleLobbyCommand(c *client, cmd string, fields []string) bool {
	switch cmd {
	case "GAMES", "MENU", "1":
		c.mode = modeGames
		h.writeGames(c)
		return true
	case "PLAYERS", "2":
		c.returnMode = modeLobby
		c.mode = modePlayers
		h.writePlayers(c)
		return true
	case "INVITES", "3":
		c.returnMode = modeLobby
		c.mode = modeInvites
		h.writeInvites(c)
		return true
	case "HELP", "H", "?", "4":
		c.text(language.T(c.lang, "game_help"))
		return true
	case "QUIT", "Q", "BYE", "5":
		c.text(language.T(c.lang, "game_goodbye"))
		return true
	case "BACK":
		h.writeLobby(c)
		return true
	}
	return false
}

func (h *Hall) handleGamesCommand(c *client, cmd string) bool {
	if idx, ok := parseMenuIndex(cmd); ok {
		defs := h.Definitions()
		if idx == len(defs)+1 {
			c.mode = modeLobby
			h.writeLobby(c)
			return true
		}
		if h.selectGame(c, idx-1) {
			return true
		}
	}
	if h.selectGameByToken(c, cmd) {
		return true
	}
	switch cmd {
	case "BACK", "Q", "QUIT":
		c.mode = modeLobby
		h.writeLobby(c)
		return true
	case "HELP", "H", "?":
		c.text(language.T(c.lang, "game_games_help"))
		return true
	}
	return false
}

func (h *Hall) handleGameLobbyCommand(c *client, cmd string, fields []string) bool {
	def, ok := h.definition(c.selected)
	if !ok {
		c.mode = modeLobby
		h.writeLobby(c)
		return true
	}
	if supportsJoinMode(def, JoinModeSolo) && supportsJoinMode(def, JoinModeRoom) {
		switch cmd {
		case "1", "START", "PLAY":
			session, err := h.startSoloSession(c.player.Callsign, def.ID)
			if err != nil {
				h.writeError(c, err)
				return true
			}
			c.mode, c.sessionID = modePlay, session.ID
			h.writeSessionState(c, session, true)
			return true
		case "2", "CREATE":
			room, err := h.CreateRoom(c.player.Callsign, def.ID)
			if err != nil {
				h.writeError(c, err)
				return true
			}
			c.mode, c.roomID = modeRoom, room.ID
			h.writeRoomState(c, room)
			return true
		case "3", "ROOMS", "OPEN":
			h.writeRooms(c, def.ID)
			return true
		case "4", "BACK", "Q", "QUIT":
			c.mode, c.selected = modeLobby, ""
			h.writeLobby(c)
			return true
		}
	}
	switch def.JoinMode {
	case JoinModeInvite:
		switch cmd {
		case "1":
			c.mode = modeInviteTarget
			c.text(language.T(c.lang, "game_invite_target"))
			return true
		case "PLAYERS", "2":
			c.returnMode = modeGame
			c.mode = modePlayers
			h.writePlayers(c)
			return true
		case "INVITES", "3":
			c.returnMode = modeGame
			c.mode = modeInvites
			h.writeInvites(c)
			return true
		case "PLAY":
			if len(fields) < 2 {
				c.text(language.T(c.lang, "game_play_usage"))
				return true
			}
			_, err := h.Invite(c.player.Callsign, fields[1], def.ID)
			if err != nil {
				h.writeError(c, err)
			}
			return true
		case "HELP", "H", "?":
			c.text(language.T(c.lang, "game_game_invite_help"))
			return true
		case "BACK", "Q", "QUIT":
			c.mode = modeLobby
			c.selected = ""
			h.writeLobby(c)
			return true
		}
		if cmd == "4" {
			c.mode = modeLobby
			c.selected = ""
			h.writeLobby(c)
			return true
		}
	case JoinModeRoom:
		switch cmd {
		case "CREATE":
			room, err := h.CreateRoom(c.player.Callsign, def.ID)
			if err != nil {
				h.writeError(c, err)
				return true
			}
			c.mode = modeRoom
			c.roomID = room.ID
			c.text(fmt.Sprintf(language.T(c.lang, "game_room_created"), room.ID))
			h.writeRoomState(c, room)
			return true
		case "JOIN":
			if len(fields) < 2 {
				c.text(language.T(c.lang, "game_room_join_usage"))
				return true
			}
			room, err := h.JoinRoom(c.player.Callsign, fields[1])
			if err != nil {
				h.writeError(c, err)
				return true
			}
			c.mode = modeRoom
			c.roomID = room.ID
			h.writeRoomState(c, room)
			return true
		case "HELP", "H", "?":
			c.text(language.T(c.lang, "game_room_help"))
			return true
		case "BACK", "Q", "QUIT":
			c.mode = modeLobby
			c.selected = ""
			h.writeLobby(c)
			return true
		}
	case JoinModeSolo:
		switch cmd {
		case "START", "PLAY":
			session, err := h.startSoloSession(c.player.Callsign, def.ID)
			if err != nil {
				h.writeError(c, err)
				return true
			}
			c.mode = modePlay
			c.sessionID = session.ID
			h.writeSessionState(c, session, true)
			return true
		case "BACK", "Q", "QUIT":
			c.mode = modeLobby
			c.selected = ""
			h.writeLobby(c)
			return true
		}
	}
	if cmd == "PLAYERS" {
		h.writePlayers(c)
		return true
	}
	if cmd == "INVITES" {
		c.mode = modeInvites
		h.writeInvites(c)
		return true
	}
	if cmd == "BACK" {
		c.mode = modeLobby
		c.selected = ""
		h.writeLobby(c)
		return true
	}
	if cmd == "HELP" || cmd == "H" || cmd == "?" {
		c.text(language.T(c.lang, "game_help"))
		return true
	}
	return false
}

func (h *Hall) handleInviteCommand(c *client, cmd string, fields []string) bool {
	if h.handleInvitationShortcut(c, cmd, fields) {
		return true
	}
	switch cmd {
	case "ACCEPT", "DECLINE":
		invites := h.Invitations(c.player.Callsign)
		if id, ok := resolveInvitationID(invites, fields); ok {
			if cmd == "ACCEPT" {
				h.writeError(c, h.Accept(c.player.Callsign, id))
			} else {
				h.writeError(c, h.Decline(c.player.Callsign, id))
			}
			return true
		}
		c.text(language.T(c.lang, "game_accept_usage"))
		return true
	case "BACK", "Q", "QUIT":
		h.returnFromSubmenu(c)
		return true
	case "HELP", "H", "?":
		c.text(language.T(c.lang, "game_invites_help"))
		return true
	case "PLAYERS":
		h.writePlayers(c)
		return true
	case "INVITES":
		h.writeInvites(c)
		return true
	}
	return false
}

func (h *Hall) handlePlayersCommand(c *client, cmd string) bool {
	if cmd == "BACK" || cmd == "Q" || cmd == "QUIT" {
		h.returnFromSubmenu(c)
		return true
	}
	if idx, ok := parseMenuIndex(cmd); ok && idx == len(h.Players())+1 {
		h.returnFromSubmenu(c)
		return true
	}
	return false
}

func (h *Hall) handleInviteTarget(c *client, cmd, line string) bool {
	if cmd == "BACK" || cmd == "Q" || cmd == "QUIT" {
		c.mode = modeGame
		h.writeGameLobby(c)
		return true
	}
	if _, err := h.Invite(c.player.Callsign, line, c.selected); err != nil {
		h.writeError(c, err)
	} else {
		c.mode = modeGame
		h.writeGameLobby(c)
	}
	return true
}

func (h *Hall) returnFromSubmenu(c *client) {
	if c.returnMode == modeGame {
		c.mode = modeGame
		h.writeGameLobby(c)
		return
	}
	c.mode = modeLobby
	h.writeLobby(c)
}

func (h *Hall) handleInvitationShortcut(c *client, cmd string, fields []string) bool {
	if cmd != "A" && cmd != "D" {
		return false
	}
	invites := h.Invitations(c.player.Callsign)
	if len(invites) == 0 {
		return false
	}
	if len(fields) == 1 && len(invites) > 1 {
		c.mode = modeInvites
		h.writeInvites(c)
		return true
	}
	id, ok := resolveInvitationID(invites, fields)
	if !ok {
		c.mode = modeInvites
		h.writeInvites(c)
		return true
	}
	if cmd == "A" {
		h.writeError(c, h.Accept(c.player.Callsign, id))
	} else {
		h.writeError(c, h.Decline(c.player.Callsign, id))
	}
	return true
}

func (h *Hall) handleRoomCommand(c *client, cmd string, fields []string) bool {
	room, ok := h.Room(c.roomID)
	if !ok {
		c.mode = modeLobby
		c.roomID = ""
		h.writeLobby(c)
		return true
	}
	switch cmd {
	case "START":
		if c.player.Callsign != room.Host {
			h.writeError(c, ErrInvalidAction)
			return true
		}
		session, err := h.StartRoom(c.player.Callsign, room.ID)
		if err != nil {
			h.writeError(c, err)
			return true
		}
		c.mode = modePlay
		c.sessionID = session.ID
		c.roomID = ""
		h.writeSessionState(c, session, true)
		return true
	case "LEAVE", "Q", "QUIT", "BACK":
		h.leaveRoom(c.player.Callsign, true)
		c.mode = modeLobby
		c.roomID = ""
		c.selected = ""
		h.writeLobby(c)
		return true
	case "HELP", "H", "?":
		c.text(language.T(c.lang, "game_room_active_help"))
		return true
	}
	return false
}

func (h *Hall) handleSessionCommand(c *client, cmd string, fields []string, line string) bool {
	if !strings.HasPrefix(strings.TrimSpace(line), "/") {
		h.writeError(c, h.Action(c.player.Callsign, line))
		return true
	}
	commandLine := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "/"))
	commandFields := strings.Fields(commandLine)
	if len(commandFields) == 0 {
		return true
	}
	cmd = strings.ToUpper(commandFields[0])
	switch cmd {
	case "HELP", "H":
		if s := h.sessionByClient(c.player.Callsign); s != nil && s.GameType == TicTacToe {
			c.text(language.T(c.lang, "ttt_help"))
		} else if s != nil && s.GameType == Hangman {
			c.text(language.T(c.lang, "hangman_help"))
		} else if s != nil && s.GameType == WordGame {
			c.text(language.T(c.lang, "word_help"))
		} else {
			c.text(language.T(c.lang, "game_session_help"))
		}
		return true
	case "BOARD", "B":
		if s := h.sessionByClient(c.player.Callsign); s != nil {
			h.writeSessionState(c, s, false)
		}
		return true
	case "REMATCH", "R":
		h.writeError(c, h.Rematch(c.player.Callsign))
		return true
	case "HASLO", "ANSWER", "SOLVE":
		guess := strings.TrimSpace(strings.TrimPrefix(commandLine, commandFields[0]))
		h.writeError(c, h.Action(c.player.Callsign, "H "+guess))
		return true
	case "QUIT", "Q", "BACK":
		h.Close(c.player.Callsign)
		c.mode = modeLobby
		c.selected = ""
		c.sessionID = ""
		h.writeLobby(c)
		return true
	default:
		h.writeError(c, h.Action(c.player.Callsign, line))
		return true
	}
}

func (h *Hall) selectGame(c *client, index int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if index < 0 || index >= len(h.order) {
		return false
	}
	id := h.order[index]
	if _, ok := h.definitions[id]; !ok {
		return false
	}
	c.selected = id
	c.mode = modeGame
	h.writeGameLobby(c)
	return true
}

func (h *Hall) selectGameByToken(c *client, token string) bool {
	token = strings.ToUpper(strings.TrimSpace(token))
	if token == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, id := range h.order {
		def, ok := h.definitions[id]
		if !ok {
			continue
		}
		if strings.EqualFold(string(def.ID), token) || strings.EqualFold(def.Prompt, token) {
			c.selected = def.ID
			c.mode = modeGame
			h.writeGameLobby(c)
			return true
		}
	}
	return false
}

func (h *Hall) sessionByClient(call string) *GameSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionByClientLocked(call)
}

func (h *Hall) roomByClient(call string) *GameRoom {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.roomByClientLocked(call)
}

func (h *Hall) sessionByClientLocked(call string) *GameSession {
	call = normalizeCall(call)
	for _, s := range h.sessions {
		for _, p := range s.Players {
			if p == call {
				return s
			}
		}
	}
	return nil
}

func (h *Hall) roomByClientLocked(call string) *GameRoom {
	call = normalizeCall(call)
	for _, room := range h.rooms {
		for _, p := range room.Players {
			if p == call {
				return room
			}
		}
	}
	return nil
}

func (h *Hall) refreshClientLocked(c *client) {
	if c == nil {
		return
	}
	if s := h.sessionByClientLocked(c.player.Callsign); s != nil && (s.State == SessionActive || s.State == SessionFinished) {
		c.player.Status = PlayerPlaying
		c.player.SessionID = s.ID
		c.player.RoomID = ""
		c.player.GameType = s.GameType
		c.mode = modePlay
		c.sessionID = s.ID
		c.roomID = ""
		return
	}
	if room := h.roomByClientLocked(c.player.Callsign); room != nil {
		c.player.Status = PlayerInRoom
		c.player.RoomID = room.ID
		c.player.SessionID = ""
		c.player.GameType = room.GameType
		c.mode = modeRoom
		c.roomID = room.ID
		c.sessionID = ""
		return
	}
	if h.hasPendingInvitationLocked(c.player.Callsign) {
		c.player.Status = PlayerWaiting
		c.player.SessionID = ""
		c.player.RoomID = ""
		c.player.GameType = ""
		if c.mode == "" {
			c.mode = modeLobby
		}
		return
	}
	c.player.Status = PlayerLobby
	c.player.SessionID = ""
	c.player.RoomID = ""
	c.player.GameType = ""
	if c.mode == "" || c.mode == modePlay || c.mode == modeRoom || c.mode == modeInvites {
		c.mode = modeLobby
		c.selected = ""
		c.sessionID = ""
		c.roomID = ""
	}
}

func (h *Hall) hasPendingInvitationLocked(call string) bool {
	call = normalizeCall(call)
	now := time.Now().UTC()
	for _, inv := range h.invitations {
		if inv.State == InvitationPending && inv.To == call && inv.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}

func (h *Hall) refreshPlayerLocked(call string) {
	if c := h.clients[normalizeCall(call)]; c != nil {
		h.refreshClientLocked(c)
	}
}

func (h *Hall) startSoloSession(call string, gameType GameType) (*GameSession, error) {
	return h.startSessionLocked(gameType, []string{normalizeCall(call)})
}

func (h *Hall) startSessionLocked(gameType GameType, players []string) (*GameSession, error) {
	def, ok := h.definitions[gameType]
	factory := h.factories[gameType]
	if !ok || factory == nil {
		return nil, ErrInvalidAction
	}
	if len(players) < def.MinPlayers || len(players) > def.MaxPlayers {
		return nil, ErrInvalidAction
	}
	now := time.Now().UTC()
	id := h.newIDLocked("S")
	g, err := factory(players)
	if err != nil {
		return nil, err
	}
	s := &GameSession{ID: id, GameType: gameType, Players: append([]string(nil), players...), State: SessionActive, CreatedAt: now, LastActivity: now, GameData: g, CurrentPlayer: g.CurrentPlayer()}
	h.sessions[id] = s
	for _, p := range players {
		if c := h.clients[p]; c != nil {
			c.player.Status = PlayerPlaying
			c.player.SessionID = id
			c.player.RoomID = ""
			c.player.GameType = gameType
			c.mode = modePlay
			c.sessionID = id
			c.roomID = ""
		}
	}
	return s, nil
}

func (h *Hall) CreateRoom(host string, gameType GameType) (*GameRoom, error) {
	host = normalizeCall(host)
	h.mu.Lock()
	defer h.mu.Unlock()
	def, ok := h.definitions[gameType]
	if !ok || !supportsJoinMode(def, JoinModeRoom) {
		return nil, ErrInvalidAction
	}
	c := h.clients[host]
	if c == nil || c.player.Status != PlayerLobby {
		return nil, ErrInvalidAction
	}
	now := time.Now().UTC()
	id := h.newRoomIDLocked()
	room := &GameRoom{ID: id, GameType: gameType, Host: host, Players: []string{host}, State: RoomOpen, CreatedAt: now, LastActivity: now}
	h.rooms[id] = room
	c.player.Status = PlayerInRoom
	c.player.RoomID = id
	c.player.GameType = gameType
	c.player.SessionID = ""
	c.mode = modeRoom
	c.roomID = id
	return room, nil
}

func (h *Hall) JoinRoom(call, id string) (*GameRoom, error) {
	call = normalizeCall(call)
	id = strings.TrimSpace(id)
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[id]
	if room == nil || room.State != RoomOpen {
		return nil, ErrInvalidAction
	}
	def, ok := h.definitions[room.GameType]
	if !ok || len(room.Players) >= def.MaxPlayers {
		return nil, ErrInvalidAction
	}
	c := h.clients[call]
	if c == nil || c.player.Status != PlayerLobby || call == room.Host {
		return nil, ErrInvalidAction
	}
	for _, p := range room.Players {
		if p == call {
			return nil, ErrInvalidAction
		}
	}
	room.Players = append(room.Players, call)
	room.LastActivity = time.Now().UTC()
	for _, p := range room.Players {
		if x := h.clients[p]; x != nil {
			x.player.Status = PlayerInRoom
			x.player.RoomID = room.ID
			x.player.GameType = room.GameType
			x.player.SessionID = ""
			x.mode = modeRoom
			x.roomID = room.ID
		}
	}
	c.mode = modeRoom
	c.roomID = room.ID
	return cloneRoom(room), nil
}

func (h *Hall) StartRoom(call, id string) (*GameSession, error) {
	call = normalizeCall(call)
	id = strings.TrimSpace(id)
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[id]
	if room == nil || room.State != RoomOpen || room.Host != call {
		return nil, ErrInvalidAction
	}
	def, ok := h.definitions[room.GameType]
	if !ok || len(room.Players) < def.MinPlayers {
		return nil, ErrInvalidAction
	}
	players := append([]string(nil), room.Players...)
	delete(h.rooms, id)
	session, err := h.startSessionLocked(room.GameType, players)
	if err != nil {
		return nil, err
	}
	for _, p := range players {
		if c := h.clients[p]; c != nil {
			c.roomID = ""
		}
	}
	return session, nil
}

func (h *Hall) leaveRoom(call string, force bool) bool {
	call = normalizeCall(call)
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.roomByClientLocked(call)
	if room == nil {
		return false
	}
	if room.Host == call || force {
		delete(h.rooms, room.ID)
		for _, p := range room.Players {
			if c := h.clients[p]; c != nil {
				if c.player.Status == PlayerInRoom {
					c.player.Status = PlayerLobby
					c.player.RoomID = ""
					c.player.GameType = ""
					c.player.SessionID = ""
					c.mode = modeLobby
					c.roomID = ""
				}
			}
		}
		return true
	}
	players := room.Players[:0]
	for _, p := range room.Players {
		if p != call {
			players = append(players, p)
		}
	}
	room.Players = append([]string(nil), players...)
	room.LastActivity = time.Now().UTC()
	if c := h.clients[call]; c != nil {
		c.player.Status = PlayerLobby
		c.player.RoomID = ""
		c.player.GameType = ""
		c.player.SessionID = ""
		c.mode = modeLobby
		c.roomID = ""
	}
	return true
}

func (h *Hall) leaveSession(call string, state SessionState) bool {
	call = normalizeCall(call)
	h.mu.Lock()
	defer h.mu.Unlock()
	c := h.clients[call]
	if c == nil || c.player.SessionID == "" {
		return false
	}
	s := h.sessions[c.player.SessionID]
	if s == nil {
		c.player.Status = PlayerLobby
		c.player.SessionID = ""
		c.player.GameType = ""
		c.mode = modeLobby
		return true
	}
	s.State = state
	s.LastActivity = time.Now().UTC()
	other := h.otherClientLocked(s, call)
	for _, p := range s.Players {
		if x := h.clients[p]; x != nil {
			x.player.Status = PlayerLobby
			x.player.SessionID = ""
			x.player.GameType = ""
			x.mode = modeLobby
			x.sessionID = ""
		}
	}
	if other != nil {
		key := "game_left"
		if state == SessionDisconnected {
			key = "game_disconnected"
		}
		other.text(fmt.Sprintf(language.T(other.lang, key), call))
	}
	return true
}

func (h *Hall) cancelInvitationsFor(call string, notify bool) {
	call = normalizeCall(call)
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, inv := range h.invitations {
		if inv.To != call && inv.From != call {
			continue
		}
		delete(h.invitations, id)
		if notify {
			if from := h.clients[inv.From]; from != nil && inv.From != call {
				from.text(fmt.Sprintf(language.T(from.lang, "game_invite_cancelled"), inv.GameName, call))
			}
			if to := h.clients[inv.To]; to != nil && inv.To != call {
				to.text(fmt.Sprintf(language.T(to.lang, "game_invite_cancelled"), inv.GameName, call))
			}
		}
	}
}

func (h *Hall) expireInvitation(id string, expiresAt time.Time) {
	timer := time.NewTimer(time.Until(expiresAt))
	defer timer.Stop()
	<-timer.C
	h.mu.Lock()
	inv := h.invitations[id]
	if inv == nil || inv.State != InvitationPending {
		h.mu.Unlock()
		return
	}
	if time.Now().UTC().Before(inv.ExpiresAt) {
		h.mu.Unlock()
		return
	}
	inv.State = InvitationExpired
	delete(h.invitations, id)
	from := h.clients[inv.From]
	to := h.clients[inv.To]
	h.refreshPlayerLocked(inv.From)
	h.refreshPlayerLocked(inv.To)
	h.mu.Unlock()
	if from != nil {
		from.text(fmt.Sprintf(language.T(from.lang, "game_invite_expired"), inv.GameName))
	}
	if to != nil {
		to.text(fmt.Sprintf(language.T(to.lang, "game_invite_expired"), inv.GameName))
	}
}

func (h *Hall) expire(id string) {
	h.mu.Lock()
	inv := h.invitations[id]
	if inv == nil || inv.State != InvitationPending {
		h.mu.Unlock()
		return
	}
	inv.State = InvitationExpired
	delete(h.invitations, id)
	from := h.clients[inv.From]
	to := h.clients[inv.To]
	h.refreshPlayerLocked(inv.From)
	h.refreshPlayerLocked(inv.To)
	h.mu.Unlock()
	if from != nil {
		from.text(fmt.Sprintf(language.T(from.lang, "game_invite_expired"), inv.GameName))
	}
	if to != nil {
		to.text(fmt.Sprintf(language.T(to.lang, "game_invite_expired"), inv.GameName))
	}
}

func (h *Hall) writeSessionState(c *client, s *GameSession, includeIntro bool) {
	if c == nil || s == nil || s.GameData == nil {
		return
	}
	switch s.GameType {
	case TicTacToe:
		view, ok := s.GameData.View(c.player.Callsign).(TicTacToeView)
		if !ok {
			return
		}
		var b strings.Builder
		if includeIntro {
			def := h.definitionMust(s.GameType)
			b.WriteString(fmt.Sprintf(language.T(c.lang, "game_started"), definitionName(def, c.lang)))
			b.WriteString(language.T(c.lang, "ttt_goal"))
			b.WriteString(language.T(c.lang, "ttt_move_help"))
			b.WriteString(language.T(c.lang, "ttt_active_actions"))
		}
		b.WriteString(RenderTicTacToe(view))
		if view.XPlayer != "" {
			b.WriteString(fmt.Sprintf("X: %s\r\n", view.XPlayer))
		}
		if view.OPlayer != "" {
			b.WriteString(fmt.Sprintf("O: %s\r\n", view.OPlayer))
		}
		if s.State == SessionFinished {
			if view.Winner == "" {
				b.WriteString(language.T(c.lang, "game_draw"))
			} else {
				b.WriteString(fmt.Sprintf(language.T(c.lang, "game_winner"), view.Winner))
			}
			b.WriteString(language.T(c.lang, "game_finished_help"))
		} else {
			b.WriteString(fmt.Sprintf(language.T(c.lang, "game_turn"), view.CurrentPlayer))
		}
		c.text(b.String())
	case ConnectFour:
		view, ok := s.GameData.View(c.player.Callsign).(ConnectFourView)
		if !ok {
			return
		}
		var b strings.Builder
		if includeIntro {
			b.WriteString(language.T(c.lang, "connect4_intro"))
		}
		b.WriteString(RenderConnectFour(view))
		if view.Finished {
			if view.Winner == "" {
				b.WriteString(language.T(c.lang, "game_draw"))
			} else {
				b.WriteString(fmt.Sprintf(language.T(c.lang, "game_winner"), view.Winner))
			}
		} else {
			b.WriteString(fmt.Sprintf(language.T(c.lang, "game_turn"), view.CurrentPlayer))
		}
		c.text(b.String())
	case Hangman:
		view, ok := s.GameData.View(c.player.Callsign).(HangmanView)
		if !ok {
			return
		}
		c.text(renderHangman(c.lang, view, includeIntro))
	case WordGame:
		view, ok := s.GameData.View(c.player.Callsign).(WordGameView)
		if !ok {
			return
		}
		c.text(renderWordGame(c.lang, view, includeIntro))
	default:
		c.text(fmt.Sprint(s.GameData.View(c.player.Callsign)))
	}
}

func (h *Hall) writeRoomState(c *client, room *GameRoom) {
	if c == nil || room == nil {
		return
	}
	def := h.definitionMust(room.GameType)
	var b strings.Builder
	b.WriteString(language.T(c.lang, "game_room_state_header"))
	b.WriteString(fmt.Sprintf("%s #%s\r\n", definitionName(def, c.lang), room.ID))
	b.WriteString(fmt.Sprintf("%s: %s\r\n", language.T(c.lang, "game_room_host"), room.Host))
	b.WriteString(fmt.Sprintf("%s: %d/%d\r\n", language.T(c.lang, "game_room_players"), len(room.Players), h.roomLimit(room.GameType)))
	for i, p := range room.Players {
		b.WriteString(fmt.Sprintf("%d. %s\r\n", i+1, p))
	}
	b.WriteString(language.T(c.lang, "game_room_active_help"))
	c.text(b.String())
}

func (h *Hall) writeError(c *client, err error) {
	if err == nil {
		return
	}
	key := "game_invalid"
	switch err {
	case ErrNotYourTurn:
		key = "game_not_turn"
	case ErrOccupied:
		key = "game_occupied"
	case ErrFinished:
		key = "game_already_finished"
	}
	c.text(language.T(c.lang, key))
}

func (h *Hall) sessionClientsLocked(s *GameSession) []*client {
	var out []*client
	for _, p := range s.Players {
		if c := h.clients[p]; c != nil {
			out = append(out, c)
		}
	}
	return out
}

func (h *Hall) otherClientLocked(s *GameSession, call string) *client {
	for _, p := range s.Players {
		if p != call {
			return h.clients[p]
		}
	}
	return nil
}

func (h *Hall) definition(id GameType) (GameDefinition, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	def, ok := h.definitions[id]
	return def, ok
}

func (h *Hall) definitionMust(id GameType) GameDefinition {
	def, _ := h.definition(id)
	return def
}

func (h *Hall) sessionPromptLocked(s *GameSession) string {
	if def, ok := h.definitions[s.GameType]; ok {
		if strings.TrimSpace(def.Prompt) != "" {
			return def.Prompt
		}
	}
	return strings.ToUpper(strings.ReplaceAll(string(s.GameType), "-", ""))
}

func (h *Hall) roomLimit(gameType GameType) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if def, ok := h.definitions[gameType]; ok {
		return def.MaxPlayers
	}
	return 0
}

func (h *Hall) newIDLocked(prefix string) string {
	return fmt.Sprintf("%s%06d", prefix, h.next.Add(1))
}

func (h *Hall) newRoomIDLocked() string {
	return fmt.Sprintf("%02d", h.nextRoom.Add(1))
}

func (h *Hall) handlePendingInvitationByNumber(invites []Invitation, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if idx, err := strconv.Atoi(token); err == nil && idx >= 1 && idx <= len(invites) {
		return invites[idx-1].ID
	}
	for _, inv := range invites {
		if strings.EqualFold(inv.ID, token) {
			return inv.ID
		}
	}
	return ""
}

func (h *Hall) handleInviteSelection(c *client, cmd string, fields []string, invites []Invitation) bool {
	if len(fields) < 2 {
		return false
	}
	id := h.handlePendingInvitationByNumber(invites, fields[1])
	if id == "" {
		return false
	}
	switch cmd {
	case "A":
		h.writeError(c, h.Accept(c.player.Callsign, id))
	case "D":
		h.writeError(c, h.Decline(c.player.Callsign, id))
	}
	return true
}

func parseMenuIndex(cmd string) (int, bool) {
	idx, err := strconv.Atoi(strings.TrimSpace(cmd))
	if err != nil {
		return 0, false
	}
	return idx, true
}

func resolveInvitationID(invites []Invitation, fields []string) (string, bool) {
	if len(fields) < 2 {
		if len(invites) == 1 {
			return invites[0].ID, true
		}
		return "", false
	}
	token := strings.TrimSpace(fields[1])
	if token == "" {
		return "", false
	}
	if idx, err := strconv.Atoi(token); err == nil && idx >= 1 && idx <= len(invites) {
		return invites[idx-1].ID, true
	}
	for _, inv := range invites {
		if strings.EqualFold(inv.ID, token) {
			return inv.ID, true
		}
	}
	return "", false
}

func invitationToken(invites []Invitation, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		if len(invites) == 1 {
			return invites[0].ID
		}
		return ""
	}
	if idx, err := strconv.Atoi(token); err == nil && idx >= 1 && idx <= len(invites) {
		return invites[idx-1].ID
	}
	for _, inv := range invites {
		if strings.EqualFold(inv.ID, token) {
			return inv.ID
		}
	}
	return ""
}

func cloneRoom(room *GameRoom) *GameRoom {
	if room == nil {
		return nil
	}
	copy := *room
	copy.Players = append([]string(nil), room.Players...)
	return &copy
}

func definitionName(def GameDefinition, lang string) string {
	if def.NameKey != "" {
		if v := language.T(lang, def.NameKey); strings.TrimSpace(v) != "" && v != def.NameKey {
			return v
		}
	}
	if strings.TrimSpace(def.Name) != "" {
		return def.Name
	}
	return strings.ToUpper(strings.ReplaceAll(string(def.ID), "-", ""))
}

func definitionPrompt(def GameDefinition) string {
	if strings.TrimSpace(def.Prompt) != "" {
		return def.Prompt
	}
	return strings.ToUpper(strings.ReplaceAll(string(def.ID), "-", ""))
}

func playerCountText(lang string, min, max int) string {
	if min == max {
		return fmt.Sprintf(language.T(lang, "game_player_count"), min)
	}
	return fmt.Sprintf(language.T(lang, "game_player_range"), min, max)
}

func terminalBlock(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", "\r\n")
	if !strings.HasPrefix(text, "\r\n") {
		text = "\r\n" + text
	}
	if !strings.HasSuffix(text, "\r\n") {
		text += "\r\n"
	}
	return text
}
