// Package gamehall provides the shared lobby and session infrastructure for
// small, terminal-friendly games.
package gamehall

import (
	"errors"
	"strings"
	"time"
)

type GameType string
type SessionState string

const (
	TicTacToe GameType = "tic-tac-toe"

	SessionInvited      SessionState = "invited"
	SessionActive       SessionState = "active"
	SessionFinished     SessionState = "finished"
	SessionCancelled    SessionState = "cancelled"
	SessionDisconnected SessionState = "disconnected"
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
	View(player string) any
	CurrentPlayer() string
	Finished() bool
	Winner() string
}

type Factory func(players []string) (Game, error)

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

func normalizeCall(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }
