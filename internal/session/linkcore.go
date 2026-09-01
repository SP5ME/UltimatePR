package session

import (
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

type linkTimerReason string

const (
	timerForConnect linkTimerReason = "connect"
	timerForData    linkTimerReason = "data"
	timerForPoll    linkTimerReason = "poll"
	timerForRelease linkTimerReason = "release"
	timerForBusy    linkTimerReason = "peer-busy"
)

type pendingFrame struct {
	sequence uint8
	frame    ax25.Frame
}

type linkTimerState struct {
	active    bool
	startedAt time.Time
	reason    linkTimerReason
	retries   int
	limit     int
}

type linkEventKind uint8

const (
	eventConnectRequested linkEventKind = iota
	eventRemoteSABM
	eventDisconnectRequested
	eventRemoteDISC
	eventRemoteDM
	eventT1Expired
	eventT2Expired
	eventT3Expired
	eventTimerRecoveryResponse
	eventRemoteBusy
	eventRemoteReady
	eventServiceClosed
)

type linkEvent struct {
	kind  linkEventKind
	final bool
	pf    bool
}

type linkAction struct {
	send        ax25.Type
	pollFinal   bool
	nr          uint8
	state       State
	retry       bool
	terminate   bool
	recover     bool
	acknowledge bool
}

// linkCore is the shared modulo-8 connected-link state. Adapters own the
// mutex and call these methods while holding it, so their existing channel and
// lifecycle synchronization remains unchanged.
type linkCore struct {
	vs, va, vr uint8
	peerBusy   bool
	rejectSent bool
	pending    []pendingFrame
	timer      linkTimerState
	ackPending bool
	ackSince   time.Time
	ackSerial  uint64
	lastActive time.Time
	recovery   bool
	state      State
}

func (l *linkCore) reset() {
	l.vs, l.va, l.vr = 0, 0, 0
	l.peerBusy = false
	l.rejectSent = false
	l.pending = nil
	l.timer = linkTimerState{}
	l.ackPending = false
	l.ackSince = time.Time{}
	l.ackSerial = 0
	l.lastActive = time.Time{}
	l.recovery = false
	l.state = Disconnected
}

func (l linkCore) nextSend() (ns, nr, expected uint8) {
	return l.vs, l.vr, (l.vs + 1) & 7
}

func (l linkCore) validNR(nr uint8) bool {
	return (nr-l.va)&7 <= (l.vs-l.va)&7
}

// acknowledge applies the common acknowledgement rule. REJ/SREJ request a
// retransmission of the frame named by N(R); RR/RNR acknowledge V(A)..V(S).
func (l *linkCore) acknowledge(nr uint8, typ ax25.Type) (accepted, retransmit bool) {
	if typ == ax25.TypeREJ || typ == ax25.TypeSREJ {
		return false, l.validNR(nr) && len(l.pending) > 0
	}
	if !l.validNR(nr) {
		return false, false
	}
	oldVA := l.va
	l.va = nr
	kept := l.pending[:0]
	for _, frame := range l.pending {
		if (frame.sequence-oldVA)&7 >= (nr-oldVA)&7 {
			kept = append(kept, frame)
		}
	}
	l.pending = kept
	return true, false
}

func (l *linkCore) track(frame ax25.Frame) {
	l.pending = append(l.pending, pendingFrame{sequence: frame.NS, frame: cloneFrame(frame)})
	l.vs = (l.vs + 1) & 7
}

func (l *linkCore) noteAcknowledgement(now time.Time) uint64 {
	l.ackPending = true
	l.ackSince = now
	l.ackSerial++
	return l.ackSerial
}

func (l *linkCore) piggybackAcknowledgement() bool {
	if !l.ackPending {
		return false
	}
	l.ackPending = false
	l.ackSince = time.Time{}
	return true
}

func (l *linkCore) expireAcknowledgement(now time.Time, serial uint64, t2 time.Duration) bool {
	if !l.ackPending || serial != l.ackSerial || now.Sub(l.ackSince) < t2 {
		return false
	}
	l.ackPending = false
	l.ackSince = time.Time{}
	return true
}

func (l *linkCore) touch(now time.Time) {
	l.lastActive = now
}

func (l linkCore) idleExpired(now time.Time, t3 time.Duration) bool {
	return !l.lastActive.IsZero() && now.Sub(l.lastActive) >= t3
}

func (l *linkCore) enterRecovery() {
	l.recovery = true
}

func (l *linkCore) exitRecovery() {
	l.recovery = false
}

// handleEvent is the protocol decision boundary. Adapters translate its
// action into actual frame I/O and physical timer operations.
func (l *linkCore) handleEvent(event linkEvent) linkAction {
	switch event.kind {
	case eventConnectRequested:
		l.reset()
		l.state = AwaitingConnection
		return linkAction{send: ax25.TypeSABM, pollFinal: true, state: l.state}
	case eventRemoteSABM:
		l.reset()
		l.state = Connected
		return linkAction{send: ax25.TypeUA, pollFinal: event.pf, state: l.state}
	case eventDisconnectRequested, eventServiceClosed:
		if l.state == Disconnected {
			return linkAction{state: Disconnected}
		}
		l.state = AwaitingRelease
		return linkAction{send: ax25.TypeDISC, pollFinal: true, state: l.state}
	case eventRemoteDISC:
		l.state = Disconnected
		l.stopTimer()
		return linkAction{send: ax25.TypeUA, pollFinal: event.pf, state: l.state, terminate: true}
	case eventRemoteDM:
		l.state = Disconnected
		l.stopTimer()
		return linkAction{state: l.state, terminate: true}
	case eventRemoteBusy:
		l.peerBusy = true
		return linkAction{state: l.state}
	case eventRemoteReady:
		l.peerBusy = false
		return linkAction{state: l.state}
	case eventTimerRecoveryResponse:
		if event.final {
			l.exitRecovery()
			l.state = Connected
			return linkAction{state: l.state, acknowledge: true}
		}
	case eventT2Expired:
		return linkAction{send: ax25.TypeRR, nr: l.vr, state: l.state, acknowledge: true}
	case eventT3Expired:
		l.enterRecovery()
		return linkAction{send: ax25.TypeRR, pollFinal: true, nr: l.vr, state: TimerRecovery, recover: true}
	case eventT1Expired:
		if !l.retry() {
			l.state = Disconnected
			return linkAction{state: l.state, terminate: true}
		}
		switch l.state {
		case AwaitingConnection:
			return linkAction{send: ax25.TypeSABM, pollFinal: true, state: l.state, retry: true}
		case AwaitingRelease:
			return linkAction{send: ax25.TypeDISC, pollFinal: true, state: l.state, retry: true}
		case Connected, TimerRecovery:
			l.enterRecovery()
			l.state = TimerRecovery
			return linkAction{send: ax25.TypeRR, pollFinal: true, nr: l.vr, state: l.state, retry: true, recover: true}
		}
	}
	return linkAction{state: l.state}
}

func (l linkCore) retransmitFrom(nr uint8) []ax25.Frame {
	frames := make([]ax25.Frame, 0, len(l.pending))
	for _, pending := range l.pending {
		if (pending.sequence-nr)&7 < (l.vs-nr)&7 {
			frames = append(frames, cloneFrame(pending.frame))
		}
	}
	return frames
}

func (l *linkCore) startTimer(now time.Time, reason linkTimerReason, limit int) {
	l.timer = linkTimerState{active: true, startedAt: now, reason: reason, limit: limit}
}

func (l *linkCore) stopTimer() {
	l.timer.active = false
}

func (l *linkCore) timerExpired(now time.Time, t1 time.Duration) bool {
	return l.timer.active && now.Sub(l.timer.startedAt) >= t1
}

func (l *linkCore) retry() bool {
	if l.timer.retries >= l.timer.limit {
		return false
	}
	l.timer.retries++
	return true
}

func (l *linkCore) receive(ns uint8) (accepted, sendReject bool) {
	if ns == l.vr {
		l.vr = (l.vr + 1) & 7
		l.rejectSent = false
		return true, false
	}
	if l.rejectSent {
		return false, false
	}
	l.rejectSent = true
	return false, true
}

func cloneFrame(frame ax25.Frame) ax25.Frame {
	frame.Payload = append([]byte(nil), frame.Payload...)
	if frame.PID != nil {
		pid := *frame.PID
		frame.PID = &pid
	}
	frame.Digipeaters = append([]ax25.Address(nil), frame.Digipeaters...)
	return frame
}
