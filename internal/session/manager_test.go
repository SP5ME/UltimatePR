package session

import (
	"bytes"
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
	return ax25.Frame{Destination: ax25.Address{Callsign: "LOCAL", SSID: 9}, Source: ax25.Address{Callsign: "REMOTE", SSID: 1, CommandResponse: true}, Type: typ, NR: nr, PollFinal: typ == ax25.TypeUA || typ == ax25.TypeDM}
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

func TestSendSplitsAtSpacesAndPreservesOriginalText(t *testing.T) {
	m, sent := testManager(t)
	m.paclen = 12
	connectManager(t, m, sent)
	text := "test dlugiej wiadomosci"
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte(text)) }()

	want := []string{"test ", "dlugiej ", "wiadomosci"}
	var joined []byte
	for i, expected := range want {
		f, err := ax25.Decode(<-sent)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(f.Payload); got != expected {
			t.Fatalf("chunk %d = %q, want %q", i+1, got, expected)
		}
		joined = append(joined, f.Payload...)
		m.Handle("radio", response(ax25.TypeRR, uint8(i+1)&7))
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if string(joined) != text {
		t.Fatalf("joined payload = %q, want %q", joined, text)
	}
}

func TestPayloadWithoutWhitespaceUsesHardPaclenLimit(t *testing.T) {
	chunks := splitAX25Payload([]byte("abcdefghijkl"), 5)
	if len(chunks) != 3 || string(chunks[0]) != "abcde" || string(chunks[1]) != "fghij" || string(chunks[2]) != "kl" {
		t.Fatalf("hard-split chunks=%q", chunks)
	}
}

func TestSendIgnoresStaleAcknowledgementWithoutImmediateRetry(t *testing.T) {
	m, sent := testManager(t)
	m.t1 = 80 * time.Millisecond
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("hello")) }()

	first := <-sent
	m.Handle("radio", response(ax25.TypeRR, 0))
	select {
	case duplicate := <-sent:
		t.Fatalf("stale RR caused immediate retry: first=% X duplicate=% X", first, duplicate)
	case <-time.After(30 * time.Millisecond):
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSendRetriesImmediatelyOnRejectForCurrentSequence(t *testing.T) {
	m, sent := testManager(t)
	m.t1 = time.Second
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("hello")) }()

	first := <-sent
	m.Handle("radio", response(ax25.TypeREJ, 0))
	select {
	case duplicate := <-sent:
		if !bytes.Equal(first, duplicate) {
			t.Fatalf("REJ retry differs from original: first=% X duplicate=% X", first, duplicate)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("REJ did not trigger immediate retry")
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSendRetriesImmediatelyOnSelectiveReject(t *testing.T) {
	m, sent := testManager(t)
	m.t1 = time.Second
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("hello")) }()
	first := <-sent
	m.Handle("radio", response(ax25.TypeSREJ, 0))
	select {
	case duplicate := <-sent:
		if !bytes.Equal(first, duplicate) {
			t.Fatal("SREJ retry differs from original")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SREJ did not trigger retry")
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
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

func TestConnectAcceptsLegacyUAWithoutFinalBit(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	_ = <-sent
	ua := response(ax25.TypeUA, 0)
	ua.PollFinal = false
	m.Handle("radio", ua)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestProtocolDefaults(t *testing.T) {
	m := New(ax25.Address{Callsign: "LOCAL"}, nil)
	if m.t1 != 10*time.Second || m.paclen != 256 {
		t.Fatalf("manager defaults: T1=%s N1=%d", m.t1, m.paclen)
	}
	inbound := NewInboundMux(nil, nil)
	if inbound.t1 != 10*time.Second || inbound.paclen != 256 {
		t.Fatalf("inbound defaults: T1=%s N1=%d", inbound.t1, inbound.paclen)
	}
}

func TestConnectRequiresTargetCallsign(t *testing.T) {
	m, sent := testManager(t)
	if err := m.Connect(context.Background(), "radio", "   "); err == nil {
		t.Fatal("expected error for empty target")
	}
	select {
	case frame := <-sent:
		t.Fatalf("unexpected frame sent: %#v", frame)
	default:
	}
	if m.State() != Disconnected {
		t.Fatalf("state=%s", m.State())
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

func TestOutOfSequenceIFrameSendsSingleREJ(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	pid := byte(0xF0)
	f := response(ax25.TypeI, 0)
	f.NS, f.PID, f.Payload = 3, &pid, []byte("late")
	m.Handle("radio", f)
	rej := decodeFrame(t, <-sent)
	if rej.Type != ax25.TypeREJ || rej.NR != 0 {
		t.Fatalf("REJ=%+v", rej)
	}
	m.Handle("radio", f)
	select {
	case extra := <-sent:
		t.Fatalf("duplicate out-of-sequence frame caused response: % X", extra)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRNRPausesNextIFrameUntilRR(t *testing.T) {
	m, sent := testManager(t)
	m.t1 = 100 * time.Millisecond
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("first")) }()
	_ = decodeFrame(t, <-sent)
	m.Handle("radio", response(ax25.TypeRNR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	go func() { done <- m.Send(context.Background(), []byte("second")) }()
	poll := decodeFrame(t, <-sent)
	if poll.Type != ax25.TypeRR || !poll.PollFinal {
		t.Fatalf("poll=%+v", poll)
	}
	select {
	case frame := <-sent:
		t.Fatalf("I frame sent while peer busy: % X", frame)
	case <-time.After(20 * time.Millisecond):
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	second := decodeFrame(t, <-sent)
	if second.Type != ax25.TypeI || string(second.Payload) != "second" {
		t.Fatalf("second=%+v", second)
	}
	m.Handle("radio", response(ax25.TypeRR, 2))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRNRDoesNotRetransmitUnacknowledgedIWhileBusy(t *testing.T) {
	m, sent := testManager(t)
	m.t1 = 30 * time.Millisecond
	m.n2 = 4
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("pending")) }()
	first := decodeFrame(t, <-sent)
	m.Handle("radio", response(ax25.TypeRNR, 0))
	poll := decodeFrame(t, <-sent)
	if poll.Type != ax25.TypeRR || !poll.PollFinal {
		t.Fatalf("busy recovery sent %+v instead of RR poll; first=%+v", poll, first)
	}
	m.Handle("radio", response(ax25.TypeRR, 0))
	retry := decodeFrame(t, <-sent)
	if retry.Type != ax25.TypeI || retry.NS != first.NS || string(retry.Payload) != "pending" {
		t.Fatalf("retry=%+v", retry)
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func decodeFrame(t *testing.T, b []byte) ax25.Frame {
	t.Helper()
	f, err := ax25.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
