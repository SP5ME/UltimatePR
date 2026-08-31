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

func TestTAPRReverseForwarding(t *testing.T) {
	a, err := Open(filepath.Join(t.TempDir(), "a.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(filepath.Join(t.TempDir(), "b.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Send("B", "SP5AAA", "POL", "From A", "A to B"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send("B", "SP5BBB", "POL", "From B", "B to A"); err != nil {
		t.Fatal(err)
	}
	peerA := ForwardPeer{ID: "bbs-a", Callsign: "SP5AAA", Enabled: true, BulletinScopes: []string{"POL"}}
	peerB := ForwardPeer{ID: "bbs-b", Callsign: "SP5BBB", Enabled: true, Transport: "telnet", BulletinScopes: []string{"POL"}}
	if err := PrepareQueues(a, []ForwardPeer{peerB}, 10); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	remote := &Server{Node: "SP5BBB-8", Address: "SP5BBB.#PL.POL.EURO", Store: b, ForwardPeers: []ForwardPeer{peerA}, MaxForwardMessages: 10}
	go remote.runForwardListener(ctx, ln)
	peerB.Host = "127.0.0.1"
	peerB.Port = uint16(ln.Addr().(*net.TCPAddr).Port)
	f := &Forwarder{Store: a, Peers: []ForwardPeer{peerB}, LocalCall: "SP5AAA-8", LocalAddress: "SP5AAA.#PL.POL.EURO", MaxMessages: 10, ConnectTimeout: time.Second, SessionTimeout: time.Second}
	if err := f.forwardPeer(ctx, peerB); err != nil {
		t.Fatal(err)
	}
	if got := a.ForwardQueue(peerB.ID, 10); len(got) != 0 {
		t.Fatalf("A queue = %+v", got)
	}
	if got := b.ForwardQueue(peerA.ID, 10); len(got) != 0 {
		t.Fatalf("B queue = %+v", got)
	}
	if got := a.Messages(); len(got) != 2 || got[1].Subject != "From B" {
		t.Fatalf("A messages = %+v", got)
	}
	if got := b.Messages(); len(got) != 2 || got[1].Subject != "From A" {
		t.Fatalf("B messages = %+v", got)
	}
}
