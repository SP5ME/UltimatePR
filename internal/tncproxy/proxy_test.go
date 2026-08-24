package tncproxy

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestProxySharesUpstreamWithMultipleClients(t *testing.T) {
	upListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upListener.Close()
	upConnCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := upListener.Accept()
		if acceptErr == nil {
			upConnCh <- conn
		}
	}()

	proxyAddr := freeAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Start(ctx, proxyAddr, upListener.Addr().String(), []string{"127.0.0.1"}, slog.Default()); err != nil {
		t.Fatal(err)
	}

	client1 := dialEventually(t, proxyAddr)
	defer client1.Close()
	client2 := dialEventually(t, proxyAddr)
	defer client2.Close()
	time.Sleep(50 * time.Millisecond)

	// Register the second client before testing the shared frame path.
	if _, err := client2.Write([]byte("ready")); err != nil {
		t.Fatal(err)
	}
	if got := readWithDeadline(t, client1); string(got) != "ready" {
		t.Fatalf("client registration data = %q, want %q", got, "ready")
	}
	upstream := <-upConnCh
	defer upstream.Close()
	if got := readWithDeadline(t, upstream); string(got) != "ready" {
		t.Fatalf("upstream registration data = %q, want %q", got, "ready")
	}

	want := []byte("client-frame")
	if _, err := client1.Write(want); err != nil {
		t.Fatal(err)
	}
	got := readWithDeadline(t, upstream)
	if string(got) != string(want) {
		t.Fatalf("upstream data = %q, want %q", got, want)
	}
	got = readWithDeadline(t, client2)
	if string(got) != string(want) {
		t.Fatalf("peer data = %q, want %q", got, want)
	}

	want = []byte("tnc-frame")
	if _, err := upstream.Write(want); err != nil {
		t.Fatal(err)
	}
	if got := readWithDeadline(t, client1); string(got) != string(want) {
		t.Fatalf("client 1 data = %q, want %q", got, want)
	}
	if got := readWithDeadline(t, client2); string(got) != string(want) {
		t.Fatalf("client 2 data = %q, want %q", got, want)
	}
}

func TestAddressAllowedByHostname(t *testing.T) {
	address := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	if !addressAllowed(address, []string{"localhost"}) {
		t.Fatal("hostname address rejected")
	}
}

func freeAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	_ = l.Close()
	return address
}

func dialEventually(t *testing.T, address string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", address)
		if err == nil {
			return conn
		}
		time.Sleep(time.Millisecond * 10)
	}
	t.Fatalf("could not connect to proxy %s", address)
	return nil
}

func readWithDeadline(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 64)
	n, err := conn.Read(buffer)
	if err != nil && n == 0 {
		t.Fatal(err)
	}
	return buffer[:n]
}
