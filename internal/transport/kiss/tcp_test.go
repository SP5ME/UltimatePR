package kiss

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/transport"
)

func TestTCPReadMapsMultipleChannelsToLogicalPorts(t *testing.T) {
	peer, port := net.Pipe()
	defer peer.Close()
	defer port.Close()

	tcp := NewTCPPort(TCPConfig{
		ID:          "kiss-main",
		InterfaceID: "kiss-main",
		PortMap:     map[uint8]string{0: "vhf", 1: "uhf"},
		MaxFrame:    256,
	}, slog.Default())
	out := make(chan transport.Packet, 2)
	errch := make(chan error, 1)
	go tcp.readLoop(context.Background(), port, out, errch)

	first, _ := Encode(Frame{Port: 0, Data: []byte("vhf")})
	second, _ := Encode(Frame{Port: 1, Data: []byte("uhf")})
	if _, err := peer.Write(append(first, second...)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		port, data string
		channel    uint8
	}{
		{"vhf", "vhf", 0},
		{"uhf", "uhf", 1},
	} {
		select {
		case got := <-out:
			if got.InterfaceID != "kiss-main" || got.PortID != want.port || got.Channel != want.channel || string(got.Data) != want.data {
				t.Fatalf("packet=%+v, want port=%s channel=%d", got, want.port, want.channel)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for KISS packet")
		}
	}
	_ = peer.Close()
	select {
	case err := <-errch:
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			// The pipe close is only used to stop the read loop.
		}
	case <-time.After(time.Second):
	}
}

func TestTCPWriteUsesPacketChannel(t *testing.T) {
	peer, port := net.Pipe()
	defer peer.Close()
	defer port.Close()

	tcp := NewTCPPort(TCPConfig{ID: "kiss-main", Port: 0, PortMap: map[uint8]string{1: "uhf"}, MaxFrame: 256}, slog.Default())
	errch := make(chan error, 1)
	go tcp.writeLoop(context.Background(), port, errch)
	tcp.tx <- transport.Packet{PortID: "uhf", Channel: 1, Data: []byte("payload")}

	decoder := NewDecoder(256)
	buffer := make([]byte, 256)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	n, err := peer.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	frames, errs := decoder.Feed(buffer[:n])
	if len(errs) != 0 || len(frames) != 1 {
		t.Fatalf("frames=%+v errors=%v", frames, errs)
	}
	if frames[0].Port != 1 || string(frames[0].Data) != "payload" {
		t.Fatalf("frame=%+v", frames[0])
	}
}

func TestTCPWriteKeepsLegacyConfiguredChannel(t *testing.T) {
	peer, port := net.Pipe()
	defer peer.Close()
	defer port.Close()

	tcp := NewTCPPort(TCPConfig{ID: "kiss-main", Port: 7, MaxFrame: 256}, slog.Default())
	errch := make(chan error, 1)
	go tcp.writeLoop(context.Background(), port, errch)
	tcp.tx <- transport.Packet{PortID: "kiss-main", Data: []byte("legacy")}

	buffer := make([]byte, 256)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	n, err := peer.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	frames, errs := NewDecoder(256).Feed(buffer[:n])
	if len(errs) != 0 || len(frames) != 1 || frames[0].Port != 7 || string(frames[0].Data) != "legacy" {
		t.Fatalf("frames=%+v errors=%v", frames, errs)
	}
}
