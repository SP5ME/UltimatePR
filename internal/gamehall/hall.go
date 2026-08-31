package gamehall

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/packet-radio/ultimatepr/internal/language"
	"github.com/packet-radio/ultimatepr/internal/lineinput"
)

type PlayerStatus string

const (
	PlayerLobby   PlayerStatus = "lobby"
	PlayerWaiting PlayerStatus = "waiting"
	PlayerPlaying PlayerStatus = "playing"
)

type Player struct {
	Callsign  string
	Status    PlayerStatus
	SessionID string
}
type client struct {
	player  Player
	lang    string
	w       io.Writer
	writeMu sync.Mutex
}

func (c *client) write(format string, args ...any) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	fmt.Fprintf(c.w, format, args...)
}
func (c *client) text(value string) { c.write("%s", value) }

type Hall struct {
	mu            sync.RWMutex
	clients       map[string]*client
	sessions      map[string]*GameSession
	factories     map[GameType]Factory
	inviteTimeout time.Duration
	next          atomic.Uint64
}

func New(inviteTimeout time.Duration) *Hall {
	if inviteTimeout <= 0 {
		inviteTimeout = 2 * time.Minute
	}
	return &Hall{clients: map[string]*client{}, sessions: map[string]*GameSession{}, factories: map[GameType]Factory{TicTacToe: NewTicTacToe}, inviteTimeout: inviteTimeout}
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
	c := &client{player: Player{Callsign: call, Status: PlayerLobby}, lang: language.Normalize(lang), w: w}
	h.clients[call] = c
	h.mu.Unlock()
	return nil
}

func (h *Hall) Disconnect(call string) {
	h.leave(normalizeCall(call), SessionDisconnected)
	h.mu.Lock()
	delete(h.clients, normalizeCall(call))
	h.mu.Unlock()
}

func (h *Hall) Invite(from, to string, gameType GameType) (*GameSession, error) {
	from, to = normalizeCall(from), normalizeCall(to)
	h.mu.Lock()
	inviter, ok1 := h.clients[from]
	invited, ok2 := h.clients[to]
	factory := h.factories[gameType]
	if !ok1 || !ok2 || factory == nil || from == to || inviter.player.Status != PlayerLobby || invited.player.Status != PlayerLobby {
		h.mu.Unlock()
		return nil, ErrInvalidAction
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("G%06d", h.next.Add(1))
	s := &GameSession{ID: id, GameType: gameType, Players: []string{from, to}, State: SessionInvited, CreatedAt: now, LastActivity: now}
	h.sessions[id] = s
	inviter.player.Status, inviter.player.SessionID = PlayerWaiting, id
	invited.player.Status, invited.player.SessionID = PlayerWaiting, id
	h.mu.Unlock()
	invited.write(language.T(invited.lang, "game_invited"), from, id, id, id)
	time.AfterFunc(h.inviteTimeout, func() { h.expire(id) })
	return s, nil
}

func (h *Hall) Accept(call, id string) error {
	call = normalizeCall(call)
	h.mu.Lock()
	s := h.sessions[id]
	if s == nil || s.State != SessionInvited || len(s.Players) != 2 || s.Players[1] != call {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	g, err := h.factories[s.GameType](s.Players)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	s.GameData, s.State, s.CurrentPlayer, s.LastActivity = g, SessionActive, g.CurrentPlayer(), time.Now().UTC()
	clients := []*client{h.clients[s.Players[0]], h.clients[s.Players[1]]}
	for _, c := range clients {
		if c != nil {
			c.player.Status = PlayerPlaying
		}
	}
	h.mu.Unlock()
	for _, c := range clients {
		if c != nil {
			c.write(language.T(c.lang, "game_started"), id)
			h.writeState(c, s)
		}
	}
	return nil
}

func (h *Hall) Decline(call, id string) error {
	return h.finishInvite(normalizeCall(call), id, SessionCancelled, "game_declined")
}
func (h *Hall) Close(call string) { h.leave(normalizeCall(call), SessionCancelled) }

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
	h.mu.Unlock()
	for _, x := range clients {
		h.writeState(x, s)
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
			other.write(language.T(other.lang, "game_rematch_request"), call)
		}
		return nil
	}
	if s.RematchRequestedBy == call {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	// Swap starting order on each accepted rematch.
	s.Players[0], s.Players[1] = s.Players[1], s.Players[0]
	g, err := h.factories[s.GameType](s.Players)
	if err != nil {
		h.mu.Unlock()
		return err
	}
	s.GameData, s.State, s.CurrentPlayer, s.RematchRequestedBy, s.LastActivity = g, SessionActive, g.CurrentPlayer(), "", time.Now().UTC()
	clients := h.sessionClientsLocked(s)
	h.mu.Unlock()
	for _, x := range clients {
		x.text(language.T(x.lang, "game_rematch_started"))
		h.writeState(x, s)
	}
	return nil
}

func (h *Hall) Serve(call, lang string, in *bufio.Scanner, w io.Writer) {
	call = normalizeCall(call)
	if err := h.Connect(call, lang, w); err != nil {
		fmt.Fprint(w, language.T(lang, "game_connect_error"))
		return
	}
	defer h.Disconnect(call)
	c := h.client(call)
	c.text(language.T(c.lang, "game_welcome"))
	h.writeLobby(c)
	for {
		c.write("GAME> ")
		if !in.Scan() {
			return
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cmd := strings.ToUpper(fields[0])
		current := h.client(call)
		if current == nil {
			return
		}
		playing := current.player.Status == PlayerPlaying
		if playing {
			switch cmd {
			case "HELP", "H", "?":
				current.text(language.T(current.lang, "ttt_help"))
			case "BOARD":
				h.writeCurrentState(current)
			case "REMATCH":
				h.writeError(current, h.Rematch(call))
			case "QUIT", "Q":
				h.Close(call)
				current.text(language.T(current.lang, "game_back_lobby"))
				h.writeLobby(current)
			default:
				h.writeError(current, h.Action(call, line))
			}
			continue
		}
		switch cmd {
		case "1", "GAMES":
			current.text(language.T(current.lang, "game_games"))
		case "2", "PLAYERS":
			h.writePlayers(current)
		case "3", "INVITES":
			h.writeInvites(current)
		case "4", "HELP", "H", "?":
			current.text(language.T(current.lang, "game_help"))
		case "PLAY", "TTT":
			if len(fields) < 2 {
				current.text(language.T(current.lang, "game_play_usage"))
				continue
			}
			s, err := h.Invite(call, fields[1], TicTacToe)
			if err != nil {
				h.writeError(current, err)
			} else {
				current.write(language.T(current.lang, "game_invite_sent"), s.ID, normalizeCall(fields[1]))
			}
		case "ACCEPT":
			if len(fields) < 2 {
				current.text(language.T(current.lang, "game_accept_usage"))
				continue
			}
			h.writeError(current, h.Accept(call, strings.ToUpper(fields[1])))
		case "DECLINE":
			if len(fields) < 2 {
				current.text(language.T(current.lang, "game_decline_usage"))
				continue
			}
			h.writeError(current, h.Decline(call, strings.ToUpper(fields[1])))
		case "5", "QUIT", "Q", "BYE":
			current.text(language.T(current.lang, "game_goodbye"))
			return
		default:
			current.text(language.T(current.lang, "game_unknown"))
		}
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
func (h *Hall) writeLobby(c *client) { c.text(language.T(c.lang, "game_lobby")) }
func (h *Hall) writePlayers(c *client) {
	players := h.Players()
	c.text(language.T(c.lang, "game_players_header"))
	for _, p := range players {
		c.write("%-10s %s\r\n", p.Callsign, language.T(c.lang, "game_status_"+string(p.Status)))
	}
}
func (h *Hall) writeInvites(c *client) {
	h.mu.RLock()
	var ids []string
	for id, s := range h.sessions {
		if s.State == SessionInvited && len(s.Players) == 2 && s.Players[1] == c.player.Callsign {
			ids = append(ids, id+" "+s.Players[0])
		}
	}
	h.mu.RUnlock()
	sort.Strings(ids)
	if len(ids) == 0 {
		c.text(language.T(c.lang, "game_no_invites"))
		return
	}
	for _, v := range ids {
		c.write("%s TIC-TAC-TOE\r\n", v)
	}
}
func (h *Hall) writeCurrentState(c *client) {
	h.mu.RLock()
	s := h.sessions[c.player.SessionID]
	h.mu.RUnlock()
	if s != nil {
		h.writeState(c, s)
	}
}
func (h *Hall) writeState(c *client, s *GameSession) {
	h.mu.RLock()
	view, typ, state := s.GameData.View(c.player.Callsign), s.GameType, s.State
	h.mu.RUnlock()
	if typ == TicTacToe {
		v := view.(TicTacToeView)
		c.write("%s", RenderTicTacToe(v))
		if state == SessionFinished {
			if v.Winner == "" {
				c.text(language.T(c.lang, "game_draw"))
			} else {
				c.write(language.T(c.lang, "game_winner"), v.Winner)
			}
			c.text(language.T(c.lang, "game_finished_help"))
		} else {
			c.write(language.T(c.lang, "game_turn"), v.CurrentPlayer)
		}
	}
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
func (h *Hall) finishInvite(call, id string, state SessionState, key string) error {
	h.mu.Lock()
	s := h.sessions[id]
	if s == nil || s.State != SessionInvited || len(s.Players) != 2 || s.Players[1] != call {
		h.mu.Unlock()
		return ErrInvalidAction
	}
	s.State = state
	s.LastActivity = time.Now().UTC()
	clients := h.sessionClientsLocked(s)
	for _, c := range clients {
		c.player.Status = PlayerLobby
		c.player.SessionID = ""
	}
	h.mu.Unlock()
	for _, c := range clients {
		c.write(language.T(c.lang, key), id)
	}
	return nil
}
func (h *Hall) expire(id string) {
	h.mu.Lock()
	s := h.sessions[id]
	if s == nil || s.State != SessionInvited {
		h.mu.Unlock()
		return
	}
	s.State = SessionCancelled
	s.LastActivity = time.Now().UTC()
	clients := h.sessionClientsLocked(s)
	for _, c := range clients {
		c.player.Status = PlayerLobby
		c.player.SessionID = ""
	}
	h.mu.Unlock()
	for _, c := range clients {
		c.write(language.T(c.lang, "game_invite_expired"), id)
	}
}
func (h *Hall) leave(call string, state SessionState) {
	h.mu.Lock()
	c := h.clients[call]
	if c == nil || c.player.SessionID == "" {
		h.mu.Unlock()
		return
	}
	s := h.sessions[c.player.SessionID]
	if s == nil {
		c.player.Status = PlayerLobby
		c.player.SessionID = ""
		h.mu.Unlock()
		return
	}
	s.State = state
	s.LastActivity = time.Now().UTC()
	other := h.otherClientLocked(s, call)
	for _, x := range h.sessionClientsLocked(s) {
		x.player.Status = PlayerLobby
		x.player.SessionID = ""
	}
	h.mu.Unlock()
	if other != nil {
		key := "game_left"
		if state == SessionDisconnected {
			key = "game_disconnected"
		}
		other.write(language.T(other.lang, key), call)
	}
}
