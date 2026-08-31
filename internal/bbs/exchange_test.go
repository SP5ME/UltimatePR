package bbs

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestTwoBBSForwardingAndASCII(t *testing.T) {
	a, _ := Open(filepath.Join(t.TempDir(), "a.json"))
	b, _ := Open(filepath.Join(t.TempDir(), "b.json"))
	m, err := a.Send("P", "SP5AAA", "SP5ME@SP5BBB.#PL.POL.EURO", "Zażółć", "Wiadomość dla Łukasza")
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject != "Zazolc" || m.Body != "Wiadomosc dla Lukasza" {
		t.Fatalf("not ASCII: %+v", m)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	remote := &Server{Node: "SP5BBB-8", Address: "SP5BBB.#PL.POL.EURO", Store: b}
	go remote.runForwardListener(ctx, ln)
	peer := ForwardPeer{ID: "bbs-b", Enabled: true, Transport: "telnet", Host: "127.0.0.1", Port: uint16(ln.Addr().(*net.TCPAddr).Port), PrivateRoutes: []string{"SP5BBB.#PL.POL.EURO"}}
	if err = PrepareQueues(a, []ForwardPeer{peer}, 10); err != nil {
		t.Fatal(err)
	}
	f := &Forwarder{Store: a, LocalCall: "SP5AAA-8", LocalAddress: "SP5AAA.#PL.POL.EURO", MaxMessages: 10, ConnectTimeout: time.Second, SessionTimeout: time.Second}
	if err = f.forwardPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	got := b.List("SP5ME", false)
	if len(got) != 1 || got[0].MID != m.MID || got[0].BID != "" {
		t.Fatalf("destination messages: visible=%+v all=%+v", got, b.Messages())
	}
	if len(a.ForwardQueue(peer.ID, 10)) != 0 {
		t.Fatal("source message not acknowledged")
	}
	if err = f.forwardPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if len(b.List("SP5ME", false)) != 1 {
		t.Fatal("duplicate MID imported")
	}
}
