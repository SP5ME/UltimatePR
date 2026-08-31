package bbs

import (
	"bufio"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestTAPRForwardingGoldenTranscriptAndDuplicateBID(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "bbs.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Node: "SP5BBB-8", Address: "SP5BBB.#PL.POL.EURO", Store: store}
	client, peer := net.Pipe()
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	done := make(chan error, 1)
	go func() {
		done <- server.serveTAPRForward(peer)
		_ = peer.Close()
	}()
	r := bufio.NewReader(client)
	if sid, err := readTAPRLine(r); err != nil || !isTAPRSID(sid) {
		t.Fatalf("SID = %q, %v", sid, err)
	}
	if err := writeTAPRLine(client, "[TEST-1.0-H$]"); err != nil {
		t.Fatal(err)
	}
	if err := expectTAPRPrompt(r); err != nil {
		t.Fatal(err)
	}
	proposal := "SB ALL @ POL < SP5AAA $123_SP5AAA"
	if err := writeTAPRLine(client, proposal); err != nil {
		t.Fatal(err)
	}
	if response, err := readTAPRLine(r); err != nil || response != "OK" {
		t.Fatalf("response = %q, %v", response, err)
	}
	if _, err := client.Write([]byte("TAPR bulletin\rR:260831/0900Z 42@SP5SRC.#PL.POL.EURO\r\rBody\r\x1a\r")); err != nil {
		t.Fatal(err)
	}
	if err := expectTAPRPrompt(r); err != nil {
		t.Fatal(err)
	}
	if err := writeTAPRLine(client, proposal); err != nil {
		t.Fatal(err)
	}
	if response, err := readTAPRLine(r); err != nil || response != "NO" {
		t.Fatalf("duplicate response = %q, %v", response, err)
	}
	if err := expectTAPRPrompt(r); err != nil {
		t.Fatal(err)
	}
	if err := writeTAPRLine(client, "F>"); err != nil {
		t.Fatal(err)
	}
	if err := expectTAPRReverse(r); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	messages := store.Messages()
	if len(messages) != 1 {
		t.Fatalf("messages = %+v", messages)
	}
	m := messages[0]
	if m.Type != "B" || m.To != "ALL" || m.Distribution != "POL" || m.BID != "123_SP5AAA" || m.MID != "123_SP5AAA" || m.Body != "Body" {
		t.Fatalf("message = %+v", m)
	}
	if len(m.Routing) != 1 || m.Routing[0] != "R:260831/0900Z 42@SP5SRC.#PL.POL.EURO" {
		t.Fatalf("routing = %#v", m.Routing)
	}
}

func TestTAPRSendGrammar(t *testing.T) {
	private, err := parseTAPRSend("SP SP5ME @ SP5BBB.#PL.POL.EURO < SP5AAA")
	if err != nil || private.Kind != "P" || private.To != "SP5ME" || private.At != "SP5BBB.#PL.POL.EURO" {
		t.Fatalf("private = %+v, %v", private, err)
	}
	bulletin, err := parseTAPRSend("SB ALL @ POL < SP5AAA $123_SP5AAA")
	if err != nil || bulletin.Kind != "B" || bulletin.At != "POL" || bulletin.BID != "123_SP5AAA" {
		t.Fatalf("bulletin = %+v, %v", bulletin, err)
	}
	traffic, err := parseTAPRSend("ST SP5ME @ SP5BBB.#PL.POL.EURO < SP5AAA")
	if err != nil || traffic.Kind != "T" || traffic.To != "SP5ME" {
		t.Fatalf("traffic = %+v, %v", traffic, err)
	}
	for _, invalid := range []string{
		"SB ALL @ POL < SP5AAA",
		"SP SP5ME @ SP5BBB.#PL.POL.EU < SP5AAA",
		"SB ALL @ POL < SP5AAA $IDENTIFIER_TOO_LONG",
		"ST SP5ME @ SP5BBB.#PL.POL.EURO < SP5AAA $123_SP5AAA",
	} {
		if _, err := parseTAPRSend(invalid); err == nil {
			t.Fatalf("accepted invalid TAPR command %q", invalid)
		}
	}
}
