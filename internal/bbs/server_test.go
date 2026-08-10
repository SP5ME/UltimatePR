package bbs

import (
	"bytes"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalConversation(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "bbs.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Title: "Test BBS", Node: "SP5ABC-7", Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	input := "SP5AAA\nS SQ9XYZ\nHello\nFirst line\n/EX\nL\nR 1\nB\n"
	var output bytes.Buffer
	s.Serve(strings.NewReader(input), &output)
	text := output.String()
	for _, want := range []string{"Hello SP5AAA", "Message #1 saved", "Hello", "First line", "73!"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
