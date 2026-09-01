package session

import (
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestLinkCoreSequenceAndAcknowledgement(t *testing.T) {
	var l linkCore
	for i := uint8(0); i < 8; i++ {
		ns, nr, expected := l.nextSend()
		if ns != i || nr != 0 || expected != (i+1)&7 {
			t.Fatalf("step %d: ns=%d nr=%d expected=%d", i, ns, nr, expected)
		}
		l.vs = expected
		if accepted, retransmit := l.acknowledge(expected, ax25.TypeRR); !accepted || retransmit {
			t.Fatalf("step %d: accepted=%v retransmit=%v", i, accepted, retransmit)
		}
		if l.va != expected {
			t.Fatalf("step %d: VA=%d, want %d", i, l.va, expected)
		}
	}
	if l.vs != 0 || l.va != 0 {
		t.Fatalf("modulo-8 state did not wrap: VS=%d VA=%d", l.vs, l.va)
	}
}

func TestLinkCoreReceiveRejectsUnexpectedSequenceOnce(t *testing.T) {
	var l linkCore
	if accepted, reject := l.receive(1); accepted || !reject {
		t.Fatalf("first unexpected frame: accepted=%v reject=%v", accepted, reject)
	}
	if accepted, reject := l.receive(1); accepted || reject {
		t.Fatalf("duplicate unexpected frame: accepted=%v reject=%v", accepted, reject)
	}
	if accepted, reject := l.receive(0); !accepted || reject || l.vr != 1 || l.rejectSent {
		t.Fatalf("expected sequence: accepted=%v reject=%v VR=%d rejected=%v", accepted, reject, l.vr, l.rejectSent)
	}
}

func TestLinkCoreREJRequestsRetransmissionWithoutAcknowledging(t *testing.T) {
	var l linkCore
	for i := uint8(0); i < 3; i++ {
		l.track(ax25.Frame{NS: i, Type: ax25.TypeI, Payload: []byte{byte(i)}})
	}
	if accepted, retransmit := l.acknowledge(1, ax25.TypeREJ); accepted || !retransmit {
		t.Fatalf("REJ result: accepted=%v retransmit=%v", accepted, retransmit)
	}
	frames := l.retransmitFrom(1)
	if len(frames) != 2 || frames[0].NS != 1 || frames[1].NS != 2 {
		t.Fatalf("REJ retransmission range=%+v", frames)
	}
}

func TestLinkCorePartialAndCompleteAcknowledgement(t *testing.T) {
	var l linkCore
	for i := uint8(0); i < 3; i++ {
		l.track(ax25.Frame{NS: i, Type: ax25.TypeI})
	}
	if accepted, retransmit := l.acknowledge(1, ax25.TypeRR); !accepted || retransmit || l.va != 1 {
		t.Fatalf("partial ACK: accepted=%v retransmit=%v VA=%d", accepted, retransmit, l.va)
	}
	if len(l.pending) != 2 || l.pending[0].sequence != 1 {
		t.Fatalf("partial ACK pending=%+v", l.pending)
	}
	if accepted, retransmit := l.acknowledge(3, ax25.TypeRR); !accepted || retransmit || l.va != 3 || len(l.pending) != 0 {
		t.Fatalf("complete ACK: accepted=%v retransmit=%v VA=%d pending=%d", accepted, retransmit, l.va, len(l.pending))
	}
}

func TestLinkCoreTimerAndRetryBudget(t *testing.T) {
	var l linkCore
	now := time.Unix(100, 0)
	l.startTimer(now, timerForData, 2)
	if l.timer.reason != timerForData || !l.timer.active || l.timerExpired(now.Add(time.Second), 2*time.Second) {
		t.Fatalf("timer state=%+v", l.timer)
	}
	if !l.timerExpired(now.Add(2*time.Second), 2*time.Second) || !l.retry() || !l.retry() || l.retry() {
		t.Fatalf("retry budget state=%+v", l.timer)
	}
	l.stopTimer()
	if l.timer.active {
		t.Fatal("timer remained active after stop")
	}
}

func TestLinkCoreDelayedAcknowledgementAndPiggyback(t *testing.T) {
	var l linkCore
	now := time.Unix(200, 0)
	serial := l.noteAcknowledgement(now)
	if !l.ackPending || serial == 0 || l.expireAcknowledgement(now.Add(500*time.Millisecond), serial, time.Second) {
		t.Fatalf("T2 became due too early: pending=%v serial=%d", l.ackPending, serial)
	}
	if !l.piggybackAcknowledgement() || l.ackPending {
		t.Fatalf("piggyback did not consume ACK pending: pending=%v", l.ackPending)
	}
	serial = l.noteAcknowledgement(now)
	if !l.expireAcknowledgement(now.Add(time.Second), serial, time.Second) || l.ackPending {
		t.Fatalf("T2 expiry: pending=%v serial=%d", l.ackPending, serial)
	}
}

func TestLinkCoreIdleAndRecoveryLifecycle(t *testing.T) {
	var l linkCore
	now := time.Unix(300, 0)
	l.touch(now)
	if l.idleExpired(now.Add(999*time.Millisecond), time.Second) {
		t.Fatal("T3 expired too early")
	}
	if !l.idleExpired(now.Add(time.Second), time.Second) {
		t.Fatal("T3 did not expire")
	}
	l.enterRecovery()
	if !l.recovery {
		t.Fatal("timer recovery was not entered")
	}
	l.exitRecovery()
	if l.recovery {
		t.Fatal("timer recovery was not exited")
	}
}

func TestLinkCoreResetAndPortAdapterIsolation(t *testing.T) {
	var vhf, uhf linkCore
	vhf.vs, vhf.vr, vhf.peerBusy = 3, 2, true
	uhf.vs, uhf.vr = 6, 5
	vhf.reset()
	if vhf.vs != 0 || vhf.vr != 0 || vhf.peerBusy {
		t.Fatalf("VHF core was not reset: %+v", vhf)
	}
	if uhf.vs != 6 || uhf.vr != 5 {
		t.Fatalf("UHF core changed with VHF reset: %+v", uhf)
	}
}
