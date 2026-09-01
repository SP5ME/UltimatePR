package session

import "github.com/packet-radio/ultimatepr/internal/ax25"

// linkCore is the shared modulo-8 connected-link state. Adapters own the
// mutex and call these methods while holding it, so their existing channel and
// lifecycle synchronization remains unchanged.
type linkCore struct {
	vs, va, vr uint8
	peerBusy   bool
	rejectSent bool
}

func (l *linkCore) reset() {
	l.vs, l.va, l.vr = 0, 0, 0
	l.peerBusy = false
	l.rejectSent = false
}

func (l linkCore) nextSend() (ns, nr, expected uint8) {
	return l.vs, l.vr, (l.vs + 1) & 7
}

func (l linkCore) validNR(nr uint8) bool {
	return nr == l.va || nr == l.vs
}

// acknowledge applies the common acknowledgement rule. REJ/SREJ request a
// retransmission of the frame named by N(R); RR/RNR acknowledge V(A)..V(S).
func (l *linkCore) acknowledge(nr uint8, typ ax25.Type) (accepted, retransmit bool) {
	if typ == ax25.TypeREJ || typ == ax25.TypeSREJ {
		return false, nr == l.va
	}
	if !l.validNR(nr) || nr != l.vs {
		return false, false
	}
	l.va = nr
	return true, false
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
