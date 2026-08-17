package session

import (
	"context"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func testManager(t *testing.T) (*Manager, chan []byte) {
	t.Helper()
	sent := make(chan []byte, 16)
	m := New(ax25.Address{Callsign: "LOCAL", SSID: 9}, map[string]Sender{
		"radio": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil },
	})
	m.t1 = 30 * time.Millisecond
	m.n2 = 2
	return m, sent
}

func response(typ ax25.Type, nr uint8) ax25.Frame {
	return ax25.Frame{Destination: ax25.Address{Callsign: "LOCAL", SSID: 9}, Source: ax25.Address{Callsign: "REMOTE", SSID: 1}, Type: typ, NR: nr}
}

func connectManager(t *testing.T, m *Manager, sent chan []byte) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	f, err := ax25.Decode(<-sent)
	if err != nil || f.Type != ax25.TypeSABM {
		t.Fatalf("connect frame=%#v err=%v", f, err)
	}
	m.Handle("radio", response(ax25.TypeUA, 0))
	if err = <-done; err != nil || m.State() != Connected {
		t.Fatalf("connect err=%v state=%s", err, m.State())
	}
}

func TestConnectSendReceiveDisconnect(t *testing.T) {
	m, sent := testManager(t)
	events, cancel := m.Subscribe()
	defer cancel()
	<-events
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("hello")) }()
	f, err := ax25.Decode(<-sent)
	if err != nil || f.Type != ax25.TypeI || string(f.Payload) != "hello" || f.NS != 0 {
		t.Fatalf("I frame=%#v err=%v", f, err)
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err = <-done; err != nil {
		t.Fatal(err)
	}

	pid := byte(0xF0)
	incoming := response(ax25.TypeI, 1)
	incoming.NS = 0
	incoming.PID = &pid
	incoming.Payload = []byte("world")
	m.Handle("radio", incoming)
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == "data" {
				if string(ev.Data) != "world" {
					t.Fatalf("data=%q", ev.Data)
				}
				goto received
			}
		case <-deadline:
			t.Fatal("no data event")
		}
	}
received:
	rr, err := ax25.Decode(<-sent)
	if err != nil || rr.Type != ax25.TypeRR || rr.NR != 1 {
		t.Fatalf("RR=%#v err=%v", rr, err)
	}

	go func() { done <- m.Disconnect(context.Background()) }()
	disc, err := ax25.Decode(<-sent)
	if err != nil || disc.Type != ax25.TypeDISC {
		t.Fatalf("DISC=%#v err=%v", disc, err)
	}
	m.Handle("radio", response(ax25.TypeUA, 0))
	if err = <-done; err != nil || m.State() != Disconnected {
		t.Fatalf("disconnect err=%v state=%s", err, m.State())
	}
}

func TestSendWithProgressReportsEveryPaclenFrame(t *testing.T) {
	m, sent := testManager(t)
	m.paclen = 4
	connectManager(t, m, sent)

	var progress []SendPacketProgress
	done := make(chan error, 1)
	go func() {
		done <- m.SendWithProgress(context.Background(), []byte("abcdefghij"), func(p SendPacketProgress) {
			progress = append(progress, p)
		})
	}()

	for packet, want := range []string{"abcd", "efgh", "ij"} {
		f, err := ax25.Decode(<-sent)
		if err != nil || string(f.Payload) != want {
			t.Fatalf("packet %d payload=%q err=%v", packet+1, f.Payload, err)
		}
		m.Handle("radio", response(ax25.TypeRR, uint8(packet+1)&7))
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(progress) != 6 {
		t.Fatalf("progress events=%d, want 6", len(progress))
	}
	for packet := 1; packet <= 3; packet++ {
		sending, sentState := progress[(packet-1)*2], progress[(packet-1)*2+1]
		if sending.Packet != packet || sending.Total != 3 || sending.State != "sending" {
			t.Fatalf("sending progress=%+v", sending)
		}
		if sentState.Packet != packet || sentState.Total != 3 || sentState.State != "sent" {
			t.Fatalf("sent progress=%+v", sentState)
		}
	}
}

func TestConnectTimeout(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	<-sent
	<-sent
	if err := <-done; err == nil || m.State() != Disconnected {
		t.Fatalf("err=%v state=%s", err, m.State())
	}
}

func TestConnectIncludesDigipeaterPath(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1", "DIGI1-2", "DIGI2-3") }()
	f, err := ax25.Decode(<-sent)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Digipeaters) != 2 || f.Digipeaters[0].String() != "DIGI1-2" || f.Digipeaters[1].String() != "DIGI2-3" {
		t.Fatalf("digipeaters=%v", f.Digipeaters)
	}
	m.Handle("radio", response(ax25.TypeUA, 0))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestKeepAliveUsesSupervisoryFrame(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.KeepAlive(ctx, 5*time.Millisecond)
	f, err := ax25.Decode(<-sent)
	if err != nil || f.Type != ax25.TypeRR || !f.PollFinal || len(f.Payload) != 0 {
		t.Fatalf("keepalive=%#v err=%v", f, err)
	}
}
