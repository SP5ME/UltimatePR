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

// linkCore is the shared modulo-8 connected-link state. Adapters own the
// mutex and call these methods while holding it, so their existing channel and
// lifecycle synchronization remains unchanged.
type linkCore struct {
	vs, va, vr uint8
	peerBusy   bool
	rejectSent bool
	pending    []pendingFrame
	timer      linkTimerState
}

func (l *linkCore) reset() {
	l.vs, l.va, l.vr = 0, 0, 0
	l.peerBusy = false
	l.rejectSent = false
	l.pending = nil
	l.timer = linkTimerState{}
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
