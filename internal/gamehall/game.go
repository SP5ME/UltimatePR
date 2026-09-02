// Package gamehall provides the shared lobby and session infrastructure for
// small, terminal-friendly games.
package gamehall

import (
	"errors"
	"strings"
	"time"
)

type GameType string
type JoinMode string
type StateVisibility string
type SessionState string
type RoomState string
type InvitationState string
type PlayerStatus string

const (
	TicTacToe GameType = "tic-tac-toe"

	JoinModeSolo   JoinMode = "solo"
	JoinModeInvite JoinMode = "invite"
	JoinModeRoom   JoinMode = "room"

	StatePublic       StateVisibility = "public"
	StateServerSecret  StateVisibility = "server_secret"

	SessionInvited      SessionState = "invited"
	SessionActive       SessionState = "active"
	SessionFinished     SessionState = "finished"
	SessionCancelled    SessionState = "cancelled"
	SessionDisconnected SessionState = "disconnected"

	RoomOpen   RoomState = "open"
	RoomClosed RoomState = "closed"

	InvitationPending InvitationState = "pending"
	InvitationDeclined InvitationState = "declined"
	InvitationExpired  InvitationState = "expired"
	InvitationAccepted InvitationState = "accepted"

	PlayerLobby   PlayerStatus = "lobby"
	PlayerWaiting PlayerStatus = "waiting"
	PlayerInRoom  PlayerStatus = "in_room"
	PlayerPlaying PlayerStatus = "playing"
)

var (
	ErrInvalidAction = errors.New("invalid action")
	ErrNotYourTurn   = errors.New("not your turn")
	ErrOccupied      = errors.New("field occupied")
	ErrFinished      = errors.New("game finished")
)

// Game is deliberately split into authoritative server state and a public
// player-specific view. Future games may keep words, hands or ship positions
// solely inside the implementation and omit them from View.
type Game interface {
	Type() GameType
	Apply(player, action string) error
	View(player string) PlayerView
	CurrentPlayer() string
	Finished() bool
	Winner() string
}

// PlayerView is the player-specific state a game may expose to a client.
// It should not include hidden server-secret information unless the game
// intentionally marks that part as public.
type PlayerView interface{}

// GameDefinition describes one lobby entry and how players can join it.
type GameDefinition struct {
	ID         GameType
	Name       string
	NameKey    string
	MinPlayers int
	MaxPlayers int
	JoinMode   JoinMode
	Visibility StateVisibility
	Prompt     string
}

type Factory func(players []string) (Game, error)

// GamePlayer is the shared player snapshot used by lobby and room listings.
type GamePlayer = Player

// GameSession is common to every game. GameData is authoritative server-only
// state and must never be serialized as a client response.
type GameSession struct {
	ID, CurrentPlayer  string
	GameType           GameType
	Players            []string
	State              SessionState
	CreatedAt          time.Time
	LastActivity       time.Time
	GameData           Game
	RematchRequestedBy string
}

// GameRoom is the shared room/staging model for ROOM-style games.
type GameRoom struct {
	ID           string
	GameType     GameType
	Host         string
	Players      []string
	State        RoomState
	CreatedAt    time.Time
	LastActivity time.Time
}

// Invitation is the shared invitation model for INVITE-style games.
type Invitation struct {
	ID           string
	GameType     GameType
	GameName     string
	From         string
	To           string
	State        InvitationState
	CreatedAt    time.Time
	LastActivity time.Time
	ExpiresAt    time.Time
}

// GameAction is the common command shape a future transport layer can reuse.
type GameAction struct {
	Player string
	Game   GameType
	Action string
	Data   string
}

func normalizeCall(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }
