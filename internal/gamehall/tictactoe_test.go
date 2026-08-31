package gamehall

import "testing"

func newTestGame(t *testing.T) *TicTacToeGame {
	t.Helper()
	game, err := NewTicTacToe([]string{"SP5AAA", "SQ4BBB"})
	if err != nil {
		t.Fatal(err)
	}
	return game.(*TicTacToeGame)
}

func TestTicTacToeMoveValidation(t *testing.T) {
	g := newTestGame(t)
	if err := g.Apply("SP5AAA", "A1"); err != nil {
		t.Fatalf("valid move: %v", err)
	}
	if err := g.Apply("SP5AAA", "A2"); err != ErrNotYourTurn {
		t.Fatalf("turn error=%v", err)
	}
	if err := g.Apply("SQ4BBB", "D1"); err != ErrInvalidAction {
		t.Fatalf("bounds error=%v", err)
	}
	if err := g.Apply("SQ4BBB", "A1"); err != ErrOccupied {
		t.Fatalf("occupied error=%v", err)
	}
}

func TestTicTacToeWins(t *testing.T) {
	tests := []struct {
		name  string
		moves []string
	}{
		{"horizontal", []string{"A1", "B1", "A2", "B2", "A3"}},
		{"vertical", []string{"A1", "A2", "B1", "B2", "C1"}},
		{"diagonal", []string{"A1", "A2", "B2", "A3", "C3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newTestGame(t)
			for i, move := range tt.moves {
				if err := g.Apply(g.players[i%2], move); err != nil {
					t.Fatalf("move %s: %v", move, err)
				}
			}
			if !g.Finished() || g.Winner() != "SP5AAA" {
				t.Fatalf("finished=%v winner=%q", g.Finished(), g.Winner())
			}
			if err := g.Apply("SQ4BBB", "C2"); err != ErrFinished {
				t.Fatalf("post-finish error=%v", err)
			}
		})
	}
}

func TestTicTacToeDraw(t *testing.T) {
	g := newTestGame(t)
	moves := []string{"A1", "A2", "A3", "B1", "B3", "B2", "C1", "C3", "C2"}
	for i, move := range moves {
		if err := g.Apply(g.players[i%2], move); err != nil {
			t.Fatalf("move %s: %v", move, err)
		}
	}
	if !g.Finished() || g.Winner() != "" {
		t.Fatalf("finished=%v winner=%q", g.Finished(), g.Winner())
	}
}

func TestTicTacToePublicViewIsACopy(t *testing.T) {
	g := newTestGame(t)
	view := g.View("SP5AAA").(TicTacToeView)
	view.Board[0] = 'X'
	if g.board[0] != 0 {
		t.Fatal("public view mutated authoritative state")
	}
}
