package gamehall

import "fmt"

const ConnectFour GameType = "connect-four"

type ConnectFourView struct {
	Board         [42]byte
	CurrentPlayer string
	Winner        string
	Finished      bool
}

type ConnectFourGame struct {
	players [2]string
	board   [42]byte
	turn    int
	winner  string
	done    bool
}

func NewConnectFour(players []string) (Game, error) {
	if len(players) != 2 || normalizeCall(players[0]) == normalizeCall(players[1]) {
		return nil, errorsNewPlayers()
	}
	return &ConnectFourGame{players: [2]string{normalizeCall(players[0]), normalizeCall(players[1])}}, nil
}
func (g *ConnectFourGame) Type() GameType { return ConnectFour }
func (g *ConnectFourGame) CurrentPlayer() string {
	if g.done {
		return ""
	}
	return g.players[g.turn]
}
func (g *ConnectFourGame) Finished() bool { return g.done }
func (g *ConnectFourGame) Winner() string { return g.winner }
func (g *ConnectFourGame) View(string) PlayerView {
	return ConnectFourView{g.board, g.CurrentPlayer(), g.winner, g.done}
}
func (g *ConnectFourGame) Apply(player, action string) error {
	if g.done {
		return ErrFinished
	}
	if normalizeCall(player) != g.CurrentPlayer() {
		return ErrNotYourTurn
	}
	col := 0
	if _, err := fmt.Sscanf(action, "%d", &col); err != nil || col < 1 || col > 7 {
		return ErrInvalidAction
	}
	col--
	row := -1
	for r := 5; r >= 0; r-- {
		if g.board[r*7+col] == 0 {
			row = r
			break
		}
	}
	if row < 0 {
		return ErrOccupied
	}
	mark := byte('X')
	if g.turn == 1 {
		mark = 'O'
	}
	g.board[row*7+col] = mark
	if g.hasLine(row, col, mark) {
		g.done = true
		g.winner = g.players[g.turn]
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
func (g *ConnectFourGame) hasLine(row, col int, mark byte) bool {
	dirs := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		n := 1
		for _, sign := range []int{-1, 1} {
			r, c := row+d[0]*sign, col+d[1]*sign
			for r >= 0 && r < 6 && c >= 0 && c < 7 && g.board[r*7+c] == mark {
				n++
				r += d[0] * sign
				c += d[1] * sign
			}
		}
		if n >= 4 {
			return true
		}
	}
	return false
}

func RenderConnectFour(v ConnectFourView) string {
	s := "  1 2 3 4 5 6 7\r\n"
	for r := 0; r < 6; r++ {
		s += "|"
		for c := 0; c < 7; c++ {
			cell := v.Board[r*7+c]
			if cell == 0 {
				cell = '.'
			}
			s += fmt.Sprintf("%c|", cell)
		}
		s += "\r\n"
	}
	return s
}
