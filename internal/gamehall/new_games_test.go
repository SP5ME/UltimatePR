package gamehall

import (
	"strings"
	"testing"
)

func TestPhrasePackUTF8AndValidation(t *testing.T) {
	pack, err := LoadPhrasePack(strings.NewReader("# name=Polski\n# language=pl\n# version=1\n\nZwierzeta|BIAŁY NIEDŹWIEDŹ\n"))
	if err != nil || pack.Name != "Polski" || pack.Language != "pl" || pack.Version != "1" || len(pack.Entries) != 1 || pack.Entries[0].Category != "Zwierzeta" || !strings.Contains(pack.Entries[0].Phrase, "Ź") {
		t.Fatalf("pack=%+v err=%v", pack, err)
	}
	for _, bad := range []string{"CategoryOnly", "|phrase"} {
		if _, err := LoadPhrasePack(strings.NewReader(bad)); err == nil {
			t.Fatalf("invalid pack accepted: %q", bad)
		}
	}
}

func TestConnectFourRules(t *testing.T) {
	g := (&ConnectFourGame{players: [2]string{"A", "B"}})
	if err := g.Apply("A", "1"); err != nil || g.board[35] != 'X' {
		t.Fatalf("first move err=%v board=%q", err, g.board[35])
	}
	if err := g.Apply("B", "1"); err != nil || g.board[28] != 'O' {
		t.Fatalf("stack err=%v board=%q", err, g.board[28])
	}
	if g.Apply("A", "0") != ErrInvalidAction || g.Apply("A", "8") != ErrInvalidAction || g.Apply("B", "1") != ErrNotYourTurn {
		t.Fatal("column or turn validation failed")
	}
	for _, col := range []string{"2", "2", "3", "3", "4"} {
		_ = g.Apply(g.CurrentPlayer(), col)
	}
	if !g.Finished() || g.Winner() == "" {
		t.Fatal("horizontal win not detected")
	}
	if g.Apply("B", "5") != ErrFinished {
		t.Fatal("post-finish move accepted")
	}
	view := g.View("A").(ConnectFourView)
	if view.Board[35] != 'X' {
		t.Fatal("public view mismatch")
	}
}

func TestSecretGamesHidePhraseAndRotateTurns(t *testing.T) {
	h, _ := newHangman([]string{"A", "B"}, PhraseEntry{"Radio", "PACKET"})
	g := h.(*HangmanGame)
	if strings.Contains(g.View("A").(HangmanView).Mask, "PACKET") || g.phrase == "" {
		t.Fatal("secret leaked or missing")
	}
	if err := g.Apply("A", "Z"); err != nil || g.CurrentPlayer() != "B" {
		t.Fatalf("bad letter turn err=%v", err)
	}
	if err := g.Apply("B", "A"); err != nil || g.CurrentPlayer() != "B" {
		t.Fatalf("correct letter should keep turn")
	}
	if err := g.Apply("B", "PACKET"); err != nil || !g.Finished() {
		t.Fatalf("solve err=%v", err)
	}
	word := &WordGameState{phrase: "BANANA", players: []string{"A", "B"}, used: map[byte]bool{}, scores: map[string]int{}, pending: 300}
	if err := word.Apply("A", "A"); err != nil {
		t.Fatal(err)
	}
	if word.scores["A"] != 900 {
		t.Fatalf("unexpected score=%d", word.scores["A"])
	}
}

func TestNewGamesAreRegisteredWithExpectedModes(t *testing.T) {
	h := New(0)
	for id, limits := range map[GameType][2]int{ConnectFour: {2, 2}, Hangman: {1, 6}, WordGame: {1, 6}} {
		def, ok := h.Definition(id)
		if !ok || def.MinPlayers != limits[0] || def.MaxPlayers != limits[1] {
			t.Fatalf("definition %s=%+v ok=%v", id, def, ok)
		}
	}
	def, _ := h.Definition(Hangman)
	if !supportsJoinMode(def, JoinModeSolo) || !supportsJoinMode(def, JoinModeRoom) {
		t.Fatal("hangman modes incomplete")
	}
}

func TestSecretGameRenderersHaveTranslationsAndNoFormatErrors(t *testing.T) {
	word := WordGameView{Category: "Sprzet", Mask: "_ _ _", Scores: map[string]int{"SP5ME": 0}, CurrentPlayer: "SP5ME"}
	hangman := HangmanView{Category: "Radio", Mask: "_ A _", Used: "A", Errors: 1, CurrentPlayer: "SP5ME"}
	for _, lang := range []string{"pl", "en"} {
		wordOutput := renderWordGame(lang, word, true)
		hangmanOutput := renderHangman(lang, hangman, true)
		for name, output := range map[string]string{"word": wordOutput, "hangman": hangmanOutput} {
			if strings.Contains(output, "word_intro") || strings.Contains(output, "word_state") || strings.Contains(output, "hangman_intro") || strings.Contains(output, "hangman_state") || strings.Contains(output, "%!") || strings.Contains(output, "\r\r\n") {
				t.Fatalf("%s %s output=%q", lang, name, output)
			}
		}
		if !strings.Contains(wordOutput, "Sprzet") || !strings.Contains(wordOutput, "SP5ME") || !strings.Contains(wordOutput, "_ _ _") {
			t.Fatalf("word %s output=%q", lang, wordOutput)
		}
		if !strings.Contains(hangmanOutput, "Radio") || !strings.Contains(hangmanOutput, "_ A _") || !strings.Contains(hangmanOutput, "SP5ME") {
			t.Fatalf("hangman %s output=%q", lang, hangmanOutput)
		}
	}
}
