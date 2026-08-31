package node

import (
	"bytes"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/bbs"
)

func TestNodeEntersBBSAndReturns(t *testing.T) {
	store, err := bbs.Open(filepath.Join(t.TempDir(), "bbs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveProfile(bbs.UserProfile{Callsign: "SP5AAA", Name: "Test", HomeBBS: "SP5ABC.#PL.POL.EURO", Language: "en", Completed: true}); err != nil {
		t.Fatal(err)
	}
	mail := &bbs.Server{Title: "Test BBS", Node: "SP5ABC-8", Language: "en", Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router := New(nil, nil, []Service{{Name: "BBS", Callsign: "SP5ABC-8", Command: "BBS", Enabled: true}})
	srv := &Server{Callsign: "SP5ABC-7", Alias: "SP5ND", Language: "en", Router: router, BBS: mail, Ports: []string{"radio-2m"}}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5AAA\nSERVICES\nC BBS\nL\nB\nPORTS\nBYE\n"), &out)
	text := out.String()
	for _, want := range []string{"SP5ND:SP5ABC-7 NODE", "SP5ABC-8", "Hello SP5AAA", "Returned to node", "radio-2m", "73!"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

func TestNodeConnectHandsRemainingStreamToConnector(t *testing.T) {
	router := New([]Neighbor{{ID: "n1", Callsign: "REMOTE-7", Port: "radio", Quality: 100}}, nil, nil)
	var gotTarget string
	var got []byte
	srv := &Server{
		Callsign: "LOCAL-7",
		Alias:    "NODE",
		Language: "en",
		Router:   router,
		Connect: func(target string, _ Neighbor, _ Route, r io.Reader, w io.Writer) error {
			gotTarget = target
			got, _ = io.ReadAll(r)
			_, _ = io.WriteString(w, "BRIDGED\r\n")
			return nil
		},
	}
	var out bytes.Buffer
	srv.Serve(strings.NewReader("SP5ME\nC REMOTE-7\npayload"), &out)
	if gotTarget != "REMOTE-7" {
		t.Fatalf("target=%q", gotTarget)
	}
	if string(got) != "payload" {
		t.Fatalf("remaining stream=%q", got)
	}
	if !strings.Contains(out.String(), "BRIDGED") {
		t.Fatalf("bridge output missing: %q", out.String())
	}
}
