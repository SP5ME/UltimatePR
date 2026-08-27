package tncproxy

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/transport/kiss"
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

	// The proxy owns the upstream connection and establishes it without waiting
	// for the first client transmission.
	upstream := waitForConnection(t, upConnCh)
	defer upstream.Close()

	client1 := dialEventually(t, proxyAddr)
	defer client1.Close()
	client2 := dialEventually(t, proxyAddr)
	defer client2.Close()
	time.Sleep(50 * time.Millisecond)

	want := kissFrame(t, kiss.Frame{Port: 0, Command: kiss.CommandData, Data: []byte("client-frame")})
	// Split a KISS frame across TCP writes. The upstream still receives one
	// complete, normalized KISS frame.
	if _, err := client1.Write(want[:3]); err != nil {
		t.Fatal(err)
	}
	if _, err := client1.Write(want[3:]); err != nil {
		t.Fatal(err)
	}
	if got := readKISSFrame(t, upstream); !bytes.Equal(got, want) {
		t.Fatalf("upstream data = %x, want %x", got, want)
	}
	if got := readKISSFrame(t, client2); !bytes.Equal(got, want) {
		t.Fatalf("peer data = %x, want %x", got, want)
	}
	if got := readOptional(client1, 100*time.Millisecond); len(got) != 0 {
		t.Fatalf("client transmission was echoed to its sender: %x", got)
	}

	want = kissFrame(t, kiss.Frame{Port: 0, Command: kiss.CommandData, Data: []byte("tnc-frame")})
	if _, err := upstream.Write(want); err != nil {
		t.Fatal(err)
	}
	if got := readKISSFrame(t, client1); !bytes.Equal(got, want) {
		t.Fatalf("client 1 data = %x, want %x", got, want)
	}
	if got := readKISSFrame(t, client2); !bytes.Equal(got, want) {
		t.Fatalf("client 2 data = %x, want %x", got, want)
	}
}

func TestProxyBlocksCommandsThatCanTakeOverSharedTNC(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proxyAddr := freeAddress(t)
	if err := Start(ctx, proxyAddr, upListener.Addr().String(), []string{"127.0.0.1"}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	upstream := waitForConnection(t, upConnCh)
	defer upstream.Close()
	client := dialEventually(t, proxyAddr)
	defer client.Close()

	for _, command := range []uint8{kiss.CommandSetHardware, kiss.CommandReturn} {
		if _, err := client.Write(kissFrame(t, kiss.Frame{Command: command})); err != nil {
			t.Fatal(err)
		}
	}
	if got := readOptional(upstream, 150*time.Millisecond); len(got) != 0 {
		t.Fatalf("unsafe KISS command reached upstream: %x", got)
	}
}

func TestAddressAllowedByHostname(t *testing.T) {
	address := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	if !addressAllowed(address, []string{"localhost"}) {
		t.Fatal("hostname address rejected")
	}
}

func TestAddressAllowedByIPv6WildcardWithZone(t *testing.T) {
	address := testAddr("[fe80::1234%eth0]:12345")
	if !addressAllowed(address, []string{"::"}) {
		t.Fatal("zoned IPv6 address rejected")
	}
}

type testAddr string

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return string(a) }

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

func waitForConnection(t *testing.T, connections <-chan net.Conn) net.Conn {
	t.Helper()
	select {
	case conn := <-connections:
		return conn
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not establish its upstream connection")
		return nil
	}
}

func kissFrame(t *testing.T, frame kiss.Frame) []byte {
	t.Helper()
	wire, err := kiss.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func readKISSFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	decoder := kiss.NewDecoder(maxKISSFrame)
	var wire []byte
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 64)
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			wire = append(wire, buffer[:n]...)
			frames, decodeErrs := decoder.Feed(buffer[:n])
			if len(decodeErrs) > 0 {
				t.Fatal(decodeErrs)
			}
			if len(frames) > 0 {
				return wire
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func readOptional(conn net.Conn, timeout time.Duration) []byte {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buffer := make([]byte, 64)
	n, _ := conn.Read(buffer)
	return buffer[:n]
}
