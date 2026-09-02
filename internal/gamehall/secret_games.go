package gamehall

import (
	"fmt"
	"strings"

	"github.com/packet-radio/ultimatepr/internal/language"
)

const (
	Hangman  GameType = "hangman"
	WordGame GameType = "word-game"
)

type HangmanView struct {
	Category, Mask, Used  string
	Errors                int
	CurrentPlayer, Winner string
	Finished              bool
}
type HangmanGame struct {
	phrase, category string
	players          []string
	used             map[byte]bool
	errors, turn     int
	done             bool
	winner           string
}

func NewHangman(players []string) (Game, error) {
	return newHangman(players, defaultPhrasePack.Entries[0])
}
func newHangman(players []string, entry PhraseEntry) (Game, error) {
	if len(players) < 1 || len(players) > 6 {
		return nil, ErrInvalidAction
	}
	return &HangmanGame{phrase: strings.ToUpper(entry.Phrase), category: entry.Category, players: append([]string(nil), players...), used: map[byte]bool{}}, nil
}
func (g *HangmanGame) Type() GameType { return Hangman }
func (g *HangmanGame) CurrentPlayer() string {
	if g.done {
		return ""
	}
	return g.players[g.turn]
}
func (g *HangmanGame) Finished() bool { return g.done }
func (g *HangmanGame) Winner() string { return g.winner }
func (g *HangmanGame) View(_ string) PlayerView {
	mask := ""
	for _, r := range g.phrase {
		if r == ' ' {
			mask += "  "
		} else if g.used[byte(r)] {
			mask += string(r) + " "
		} else {
			mask += "_ "
		}
	}
	used := ""
	for r := range g.used {
		used += string(r)
	}
	if used == "" {
		used = "-"
	}
	return HangmanView{g.category, strings.TrimSpace(mask), used, g.errors, g.CurrentPlayer(), g.winner, g.done}
}
func (g *HangmanGame) Apply(player, action string) error {
	if g.done {
		return ErrFinished
	}
	if normalizeCall(player) != g.CurrentPlayer() {
		return ErrNotYourTurn
	}
	a := strings.TrimSpace(strings.ToUpper(action))
	if len(a) == 1 && a[0] >= 'A' && a[0] <= 'Z' {
		if g.used[a[0]] {
			return ErrInvalidAction
		}
		g.used[a[0]] = true
		if !strings.ContainsRune(g.phrase, rune(a[0])) {
			g.errors++
			g.turn = (g.turn + 1) % len(g.players)
		} else if g.solved() {
			g.done = true
			g.winner = player
		}
		if g.errors >= 6 {
			g.done = true
		}
		return nil
	}
	if strings.HasPrefix(a, "H ") || strings.HasPrefix(a, "SOLVE ") {
		guess := strings.TrimSpace(a[2:])
		if strings.HasPrefix(a, "SOLVE ") {
			guess = strings.TrimSpace(a[6:])
		}
		if guess == g.phrase {
			g.done = true
			g.winner = player
			return nil
		}
		g.turn = (g.turn + 1) % len(g.players)
		return ErrInvalidAction
	}
	if strings.TrimSpace(a) == g.phrase {
		g.done = true
		g.winner = player
		return nil
	}
	g.turn = (g.turn + 1) % len(g.players)
	return ErrInvalidAction
}
func (g *HangmanGame) solved() bool {
	for _, r := range g.phrase {
		if r != ' ' && !g.used[byte(r)] {
			return false
		}
	}
	return true
}

type WordGameView struct {
	Mask, Category, Used  string
	Scores                map[string]int
	CurrentPlayer, Winner string
	Finished              bool
}
type WordGameState struct {
	phrase, category string
	players          []string
	used             map[byte]bool
	scores           map[string]int
	turn             int
	pending          int
	done             bool
	winner           string
}

func NewWordGame(players []string) (Game, error) {
	if len(players) < 1 || len(players) > 6 {
		return nil, ErrInvalidAction
	}
	e := defaultPhrasePack.Entries[1]
	return &WordGameState{phrase: strings.ToUpper(e.Phrase), category: e.Category, players: append([]string(nil), players...), used: map[byte]bool{}, scores: map[string]int{}}, nil
}
func (g *WordGameState) Type() GameType { return WordGame }
func (g *WordGameState) CurrentPlayer() string {
	if g.done {
		return ""
	}
	return g.players[g.turn]
}
func (g *WordGameState) Finished() bool { return g.done }
func (g *WordGameState) Winner() string { return g.winner }
func (g *WordGameState) View(_ string) PlayerView {
	mask := ""
	for _, r := range g.phrase {
		if r == ' ' {
			mask += "  "
		} else if g.used[byte(r)] {
			mask += string(r) + " "
		} else {
			mask += "_ "
		}
	}
	u := ""
	for r := range g.used {
		u += string(r)
	}
	if u == "" {
		u = "-"
	}
	return WordGameView{strings.TrimSpace(mask), g.category, u, g.scores, g.CurrentPlayer(), g.winner, g.done}
}
func (g *WordGameState) Apply(player, action string) error {
	if g.done {
		return ErrFinished
	}
	if normalizeCall(player) != g.CurrentPlayer() {
		return ErrNotYourTurn
	}
	a := strings.TrimSpace(strings.ToUpper(action))
	if strings.HasPrefix(a, "H ") || strings.HasPrefix(a, "SOLVE ") {
		if strings.HasPrefix(a, "H ") {
			a = strings.TrimSpace(a[2:])
		} else {
			a = strings.TrimSpace(a[6:])
		}
		if a == g.phrase {
			g.done = true
			g.winner = player
			return nil
		}
		g.turn = (g.turn + 1) % len(g.players)
		return ErrInvalidAction
	}
	if strings.HasPrefix(a, "L ") {
		a = strings.TrimSpace(a[2:])
	}
	if strings.HasPrefix(a, "LETTER ") {
		a = strings.TrimSpace(a[7:])
	}
	if len(a) != 1 || a[0] < 'A' || a[0] > 'Z' {
		return ErrInvalidAction
	}
	if g.used[a[0]] {
		return ErrInvalidAction
	}
	g.used[a[0]] = true
	count := strings.Count(g.phrase, string(a[0]))
	if count == 0 {
		g.turn = (g.turn + 1) % len(g.players)
		return nil
	}
	g.scores[player] += g.pending * count
	if g.solved() {
		g.done = true
		g.winner = player
	}
	return nil
}
func (g *WordGameState) solved() bool {
	for _, r := range g.phrase {
		if r != ' ' && !g.used[byte(r)] {
			return false
		}
	}
	return true
}
func (g *WordGameState) String() string { return fmt.Sprint(g.View("")) }

func renderHangman(lang string, v HangmanView, intro bool) string {
	var b strings.Builder
	if intro {
		b.WriteString(language.T(lang, "hangman_intro"))
		b.WriteString(language.T(lang, "hangman_actions"))
	}
	b.WriteString(fmt.Sprintf(language.T(lang, "hangman_state"), v.Category, v.Mask, v.Used, v.Errors, v.CurrentPlayer))
	if v.Finished {
		b.WriteString(fmt.Sprintf(language.T(lang, "game_winner"), v.Winner))
	}
	return b.String()
}

func renderWordGame(lang string, v WordGameView, intro bool) string {
	var b strings.Builder
	if intro {
		b.WriteString(language.T(lang, "word_intro"))
		b.WriteString(language.T(lang, "word_actions"))
	}
	b.WriteString(fmt.Sprintf(language.T(lang, "word_state"), v.Category, v.Mask, v.Used, v.CurrentPlayer))
	for call, score := range v.Scores {
		b.WriteString(fmt.Sprintf("%s %d\r\n", call, score))
	}
	if v.Finished {
		b.WriteString(fmt.Sprintf(language.T(lang, "game_winner"), v.Winner))
	}
	return b.String()
}
