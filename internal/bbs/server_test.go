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
	s := &Server{Title: "Test BBS", Node: "SP5ABC-7", Language: "en", Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	input := "SP5AAA\nAdam\n\nWarsaw\nKO02\nS SQ9XYZ\nHello\nFirst line\n/EX\nL\nR 1\nB\n"
	var output bytes.Buffer
	s.Serve(strings.NewReader(input), &output)
	text := output.String()
	for _, want := range []string{"Hello SP5AAA", "Message #1 saved", "Hello", "First line", "73!"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestSessionLanguageSwitch(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	s := &Server{Title: "BBS", Node: "SP5ABC-8", Language: "pl", Store: store}
	var out bytes.Buffer
	s.Serve(strings.NewReader("SP5AAA\nAdam\n\nWarsaw\nKO02\nLANG EN\nH\nB\n"), &out)
	text := out.String()
	if !strings.Contains(text, "Witaj SP5AAA") || !strings.Contains(text, "Language changed to English") || !strings.Contains(text, "RE <id> reply") {
		t.Fatalf("language switch failed:\n%s", text)
	}
}

func TestNewReadReplyAndSentCommands(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	_, _ = store.Send("P", "SQ5XYZ", "SP5AAA", "Question", "Hello")
	s := &Server{Title: "BBS", Node: "SP5ABC-8", Language: "en", Store: store}
	var out bytes.Buffer
	s.Serve(strings.NewReader("SP5AAA\nAdam\n\nWarsaw\nKO02\nN\nR 1\nN\nRE 1\nReply body\n/EX\nLS\nB\n"), &out)
	text := out.String()
	for _, want := range []string{"Question", "No new messages", "Re: Question", "Message #2 saved"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

func TestProfilePersistsAndHomeBBSChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bbs.json")
	store, _ := Open(path)
	s := &Server{Title: "BBS", Node: "SP5AAA-8", Address: "SP5AAA.#PL.POL.EU", Language: "en", Store: store}
	var out bytes.Buffer
	s.Serve(strings.NewReader("SP5ME\nMike\n\nWarsaw\nKO02MF\nHOMEBBS SR5DDD.#PL.POL.EU\nPROFILE\nB\n"), &out)
	p, ok := store.Profile("SP5ME")
	if !ok || !p.Completed || p.Name != "Mike" || p.HomeBBS != "SR5DDD.#PL.POL.EU" || p.Locator != "KO02MF" {
		t.Fatalf("bad profile: %+v", p)
	}
	var second bytes.Buffer
	s.Serve(strings.NewReader("SP5ME\nPROFILE\nB\n"), &second)
	if strings.Contains(second.String(), "First connection") || !strings.Contains(second.String(), "Home BBS: SR5DDD.#PL.POL.EU") {
		t.Fatalf("second login: %s", second.String())
	}
}

func TestHomeBBSExpandsRecipient(t *testing.T) {
	store, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	_ = store.SaveProfile(UserProfile{Callsign: "SQ9MDD", Name: "Tom", HomeBBS: "SR5DDD.#PL.POL.EU", Completed: true})
	s := &Server{Title: "BBS", Node: "SP5AAA-8", Address: "SP5AAA.#PL.POL.EU", Language: "en", Store: store}
	var out bytes.Buffer
	s.Serve(strings.NewReader("SP5ME\nMike\n\nWarsaw\nKO02\nS SQ9MDD\nTest\nBody\n/EX\nB\n"), &out)
	ms := store.Messages()
	if len(ms) != 1 || ms[0].To != "SQ9MDD" || ms[0].At != "SR5DDD.#PL.POL.EU" {
		t.Fatalf("address not expanded: %+v", ms)
	}
}
