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
	m.Configure(30*time.Millisecond, 2, 256)
	return m, sent
}

func response(typ ax25.Type, nr uint8) ax25.Frame {
	return ax25.Frame{Destination: ax25.Address{Callsign: "LOCAL", SSID: 9}, Source: ax25.Address{Callsign: "REMOTE", SSID: 1, CommandResponse: true}, Type: typ, NR: nr, PollFinal: typ == ax25.TypeUA || typ == ax25.TypeDM}
}

func commandFromRemote(typ ax25.Type, nr uint8) ax25.Frame {
	return ax25.Frame{Destination: ax25.Address{Callsign: "LOCAL", SSID: 9, CommandResponse: true}, Source: ax25.Address{Callsign: "REMOTE", SSID: 1}, Type: typ, NR: nr}
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
	xid, err := ax25.Decode(<-sent)
	if err != nil || xid.Type != ax25.TypeXID || !xid.PollFinal || !isCommand(xid) {
		t.Fatalf("XID command=%#v err=%v", xid, err)
	}
	xidResponse := response(ax25.TypeXID, 0)
	xidResponse.PollFinal = true
	xidResponse.Payload = xid.Payload
	m.Handle("radio", xidResponse)
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
	incoming := commandFromRemote(ax25.TypeI, 1)
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

func TestConnectNegotiatesXIDResponseParameters(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	if sabm := decodeFrame(t, <-sent); sabm.Type != ax25.TypeSABM {
		t.Fatalf("first frame=%+v", sabm)
	}
	m.Handle("radio", response(ax25.TypeUA, 0))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	command := decodeFrame(t, <-sent)
	if command.Type != ax25.TypeXID || !command.PollFinal || !isCommand(command) {
		t.Fatalf("XID command=%+v", command)
	}
	peer := ax25.XIDLinkSettings{Modulo: ax25.Modulo8, ReceiveN1: 64, ReceiveWindow: 1, T1Milliseconds: 50, Retries: 4}
	payload, err := ax25.EncodeXID(ax25.XIDParameters(peer))
	if err != nil {
		t.Fatal(err)
	}
	xidResponse := response(ax25.TypeXID, 0)
	xidResponse.PollFinal, xidResponse.Payload = true, payload
	m.Handle("radio", xidResponse)
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.Lock()
		n1, t1, n2 := m.paclen, m.t1, m.n2
		m.mu.Unlock()
		if n1 == 64 && t1 == 50*time.Millisecond && n2 == 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("negotiated N1=%d T1=%s N2=%d", n1, t1, n2)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSendWithProgressReportsEveryPaclenFrame(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(30*time.Millisecond, 2, 4)
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
	m.Configure(30*time.Millisecond, 2, 12)
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
	m.Configure(80*time.Millisecond, 2, 256)
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
	m.Configure(time.Second, 2, 256)
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
	m.Configure(time.Second, 2, 256)
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

func TestConnectRejectsUAWithoutFinalBit(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	_ = <-sent
	ua := response(ax25.TypeUA, 0)
	ua.PollFinal = false
	m.Handle("radio", ua)
	_ = <-sent // retransmitted SABM after T1
	if err := <-done; err == nil {
		t.Fatal("UA without F=1 incorrectly completed connection")
	}
}

func TestConnectRejectsInvalidCommandResponseBits(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	_ = <-sent
	ua := response(ax25.TypeUA, 0)
	ua.Destination.CommandResponse = true // C/R bits equal: neither command nor response
	m.Handle("radio", ua)
	_ = <-sent
	if err := <-done; err == nil {
		t.Fatal("invalid C/R bits incorrectly completed connection")
	}
}

func TestUAWithFinalThenImmediateLinBPQBannerIsDelivered(t *testing.T) {
	m, sent := testManager(t)
	events, cancel := m.Subscribe()
	defer cancel()
	<-events
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	_ = <-sent
	ua := response(ax25.TypeUA, 0)
	ua.PollFinal = true
	m.Handle("radio", ua)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	pid := byte(0xF0)
	banner := commandFromRemote(ax25.TypeI, 0)
	banner.NS, banner.PID, banner.Payload = 0, &pid, []byte("=== LinBPQ ===\r")
	m.Handle("radio", banner)
	for {
		select {
		case event := <-events:
			if event.Type == "data" {
				if string(event.Data) != "=== LinBPQ ===\r" {
					t.Fatalf("banner=%q", event.Data)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("LinBPQ banner was not delivered")
		}
	}
}

func TestProtocolDefaults(t *testing.T) {
	m := New(ax25.Address{Callsign: "LOCAL"}, nil)
	if m.t1 != 10*time.Second || m.n2 != 10 || m.paclen != 256 {
		t.Fatalf("manager defaults: T1=%s N2=%d N1=%d", m.t1, m.n2, m.paclen)
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
	f := commandFromRemote(ax25.TypeI, 0)
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
	m.Configure(100*time.Millisecond, 2, 256)
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
	m.Configure(30*time.Millisecond, 4, 256)
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

func TestT1ExpiryPollsBeforeRetransmittingIFrame(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(20*time.Millisecond, 3, 256)
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("pending")) }()
	first := decodeFrame(t, <-sent)
	poll := decodeFrame(t, <-sent)
	if first.Type != ax25.TypeI || poll.Type != ax25.TypeRR || !poll.PollFinal {
		t.Fatalf("first=%+v poll=%+v", first, poll)
	}
	if m.State() != TimerRecovery {
		t.Fatalf("state=%s, want timer recovery", m.State())
	}
	answer := response(ax25.TypeRR, 0)
	answer.PollFinal = true
	m.Handle("radio", answer)
	retry := decodeFrame(t, <-sent)
	if retry.Type != ax25.TypeI || retry.NS != first.NS || string(retry.Payload) != "pending" {
		t.Fatalf("retry=%+v", retry)
	}
	m.Handle("radio", response(ax25.TypeRR, 1))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDISCUAMirrorsPollBit(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	disc := commandFromRemote(ax25.TypeDISC, 0)
	disc.PollFinal = false
	m.Handle("radio", disc)
	ua := decodeFrame(t, <-sent)
	if ua.Type != ax25.TypeUA || ua.PollFinal {
		t.Fatalf("UA=%+v, expected F=0 matching DISC P=0", ua)
	}
}

func TestT1N2ExhaustionDisconnectsLostLink(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(10*time.Millisecond, 2, 256)
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("lost")) }()
	if f := decodeFrame(t, <-sent); f.Type != ax25.TypeI {
		t.Fatalf("first=%+v", f)
	}
	for attempt := 0; attempt < m.n2; attempt++ {
		if poll := decodeFrame(t, <-sent); poll.Type != ax25.TypeRR || !poll.PollFinal {
			t.Fatalf("poll %d=%+v", attempt+1, poll)
		}
	}
	if err := <-done; err == nil {
		t.Fatal("missing timeout error")
	}
	if m.State() != Disconnected {
		t.Fatalf("state=%s", m.State())
	}
	if m.vs != 0 || m.vr != 0 || m.peerBusy || m.rejectSent {
		t.Fatalf("link state not reset: VS=%d VR=%d busy=%v reject=%v", m.vs, m.vr, m.peerBusy, m.rejectSent)
	}
}

func TestKeepAliveDisconnectsUnresponsiveStation(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(10*time.Millisecond, 2, 256)
	connectManager(t, m, sent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.mu.Lock()
	attempts := m.n2
	m.mu.Unlock()
	go m.KeepAlive(ctx, time.Millisecond)
	for attempt := 0; attempt < attempts; attempt++ {
		if poll := decodeFrame(t, <-sent); poll.Type != ax25.TypeRR || !poll.PollFinal {
			t.Fatalf("keepalive poll %d=%+v", attempt+1, poll)
		}
	}
	deadline := time.After(time.Second)
	for m.State() != Disconnected {
		select {
		case <-deadline:
			t.Fatal("unresponsive link remained connected")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestConnectedDMImmediatelyDropsLink(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	m.Handle("radio", response(ax25.TypeDM, 0))
	if m.State() != Disconnected {
		t.Fatalf("state=%s", m.State())
	}
}

func TestRemoteDMInterruptsOutstandingSend(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(time.Second, 2, 256)
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("pending")) }()
	_ = <-sent
	m.Handle("radio", response(ax25.TypeDM, 0))
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("send succeeded after DM")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("send was not interrupted by DM")
	}
	if m.State() != Disconnected {
		t.Fatalf("state=%s", m.State())
	}
}

func TestRemoteDMDuringTimerRecoveryDoesNotReconnect(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(15*time.Millisecond, 2, 256)
	connectManager(t, m, sent)
	done := make(chan error, 1)
	go func() { done <- m.Send(context.Background(), []byte("pending")) }()
	_ = decodeFrame(t, <-sent) // I
	_ = decodeFrame(t, <-sent) // RR(P=1)
	dm := response(ax25.TypeDM, 0)
	dm.PollFinal = true
	m.Handle("radio", dm)
	if err := <-done; err == nil {
		t.Fatal("send succeeded after DM during timer recovery")
	}
	if m.State() != Disconnected {
		t.Fatalf("state=%s", m.State())
	}
}

func TestSimultaneousSABMCompletesConnection(t *testing.T) {
	m, sent := testManager(t)
	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	_ = <-sent // local SABM
	remoteSABM := response(ax25.TypeSABM, 0)
	remoteSABM.PollFinal = true
	m.Handle("radio", remoteSABM)
	if ua := decodeFrame(t, <-sent); ua.Type != ax25.TypeUA || !ua.PollFinal {
		t.Fatalf("UA=%+v", ua)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if m.State() != Connected {
		t.Fatalf("state=%s", m.State())
	}
}

func TestRepeatedSABMResetsConnectedLink(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	m.mu.Lock()
	m.vs, m.vr, m.peerBusy, m.rejectSent = 4, 5, true, true
	m.mu.Unlock()
	f := response(ax25.TypeSABM, 0)
	f.PollFinal = true
	m.Handle("radio", f)
	ua := decodeFrame(t, <-sent)
	if ua.Type != ax25.TypeUA || !ua.PollFinal {
		t.Fatalf("UA=%+v", ua)
	}
	if m.State() != Connected || m.vs != 0 || m.vr != 0 || m.peerBusy || m.rejectSent {
		t.Fatalf("state=%s VS=%d VR=%d busy=%v reject=%v", m.State(), m.vs, m.vr, m.peerBusy, m.rejectSent)
	}
}

func TestDelayedDuplicateUAAfterRetriedSABMDoesNotDropLink(t *testing.T) {
	m, sent := testManager(t)
	connectManager(t, m, sent)
	duplicate := response(ax25.TypeUA, 0)
	duplicate.PollFinal = true
	if !m.Handle("radio", duplicate) {
		t.Fatal("duplicate UA was not claimed by active link")
	}
	if m.State() != Connected {
		t.Fatalf("state=%s after delayed duplicate UA", m.State())
	}
}

func TestHubClaimsOutgoingInformationBeforeInboundMux(t *testing.T) {
	sent := make(chan []byte, 8)
	hub := NewHub(ax25.Address{Callsign: "LOCAL", SSID: 9}, map[string]Sender{
		"radio": func(_ context.Context, b []byte) error { sent <- append([]byte(nil), b...); return nil },
	})
	m, release := hub.NewSession()
	defer release()
	m.Configure(30*time.Millisecond, 2, 256)
	connectManager(t, m, sent)
	pid := byte(0xF0)
	incoming := commandFromRemote(ax25.TypeI, 0)
	incoming.PID, incoming.Payload = &pid, []byte("banner")
	if !hub.Handle("radio", incoming) {
		t.Fatal("active outgoing information frame was not claimed")
	}
	if rr := decodeFrame(t, <-sent); rr.Type != ax25.TypeRR || rr.NR != 1 {
		t.Fatalf("RR=%+v", rr)
	}
}

func TestXIDUsesIndependentManagementRetryLimit(t *testing.T) {
	m, sent := testManager(t)
	m.Configure(30*time.Millisecond, 7, 256)
	m.mu.Lock()
	m.tm201 = 10 * time.Millisecond
	m.nm201 = 2
	m.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- m.Connect(context.Background(), "radio", "REMOTE-1") }()
	if sabm := decodeFrame(t, <-sent); sabm.Type != ax25.TypeSABM {
		t.Fatalf("first frame=%+v", sabm)
	}
	m.Handle("radio", response(ax25.TypeUA, 0))
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	for transmission := 1; transmission <= 3; transmission++ {
		select {
		case raw := <-sent:
			if xid := decodeFrame(t, raw); xid.Type != ax25.TypeXID || !xid.PollFinal || !isCommand(xid) {
				t.Fatalf("transmission %d=%+v", transmission, xid)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("missing XID transmission %d", transmission)
		}
	}
	select {
	case raw := <-sent:
		t.Fatalf("unexpected fourth XID=%+v", decodeFrame(t, raw))
	case <-time.After(25 * time.Millisecond):
	}

	m.mu.Lock()
	t1, n2 := m.t1, m.n2
	m.mu.Unlock()
	if t1 != 3*time.Second || n2 != 10 {
		t.Fatalf("legacy fallback T1=%s N2=%d", t1, n2)
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
