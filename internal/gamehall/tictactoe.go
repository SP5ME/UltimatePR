package gamehall

import (
	"fmt"
	"strings"
)

type TicTacToeGame struct {
	players [2]string
	board   [9]byte
	turn    int
	winner  string
	done    bool
}

type TicTacToeView struct {
	Board         [9]byte
	CurrentPlayer string
	Winner        string
	Finished      bool
}

func NewTicTacToe(players []string) (Game, error) {
	if len(players) != 2 || normalizeCall(players[0]) == "" || normalizeCall(players[1]) == "" || normalizeCall(players[0]) == normalizeCall(players[1]) {
		return nil, errorsNewPlayers()
	}
	return &TicTacToeGame{players: [2]string{normalizeCall(players[0]), normalizeCall(players[1])}}, nil
}

func errorsNewPlayers() error           { return fmt.Errorf("tic-tac-toe requires two different players") }
func (g *TicTacToeGame) Type() GameType { return TicTacToe }
func (g *TicTacToeGame) CurrentPlayer() string {
	if g.done {
		return ""
	}
	return g.players[g.turn]
}
func (g *TicTacToeGame) Finished() bool { return g.done }
func (g *TicTacToeGame) Winner() string { return g.winner }
func (g *TicTacToeGame) View(_ string) any {
	return TicTacToeView{Board: g.board, CurrentPlayer: g.CurrentPlayer(), Winner: g.winner, Finished: g.done}
}

func (g *TicTacToeGame) Apply(player, action string) error {
	if g.done {
		return ErrFinished
	}
	if normalizeCall(player) != g.players[g.turn] {
		return ErrNotYourTurn
	}
	pos, ok := parsePosition(action)
	if !ok {
		return ErrInvalidAction
	}
	if g.board[pos] != 0 {
		return ErrOccupied
	}
	mark := byte('X')
	if g.turn == 1 {
		mark = 'O'
	}
	g.board[pos] = mark
	if hasLine(g.board, mark) {
		g.done, g.winner = true, g.players[g.turn]
		return nil
	}
	full := true
	for _, cell := range g.board {
		if cell == 0 {
			full = false
			break
		}
	}
	if full {
		g.done = true
		return nil
	}
	g.turn = 1 - g.turn
	return nil
}

func parsePosition(v string) (int, bool) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if len(v) != 2 || v[0] < 'A' || v[0] > 'C' || v[1] < '1' || v[1] > '3' {
		return 0, false
	}
	return int(v[0]-'A')*3 + int(v[1]-'1'), true
}

func hasLine(b [9]byte, mark byte) bool {
	lines := [8][3]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {0, 3, 6}, {1, 4, 7}, {2, 5, 8}, {0, 4, 8}, {2, 4, 6}}
	for _, line := range lines {
		if b[line[0]] == mark && b[line[1]] == mark && b[line[2]] == mark {
			return true
		}
	}
	return false
}

func RenderTicTacToe(view TicTacToeView) string {
	cell := func(i int) byte {
		if view.Board[i] == 0 {
			return '.'
		}
		return view.Board[i]
	}
	return fmt.Sprintf("  1 2 3\r\nA %c %c %c\r\nB %c %c %c\r\nC %c %c %c\r\n", cell(0), cell(1), cell(2), cell(3), cell(4), cell(5), cell(6), cell(7), cell(8))
}
