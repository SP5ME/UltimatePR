package session

import (
	"testing"

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
	l.vs, l.va = 1, 0
	if accepted, retransmit := l.acknowledge(0, ax25.TypeREJ); accepted || !retransmit {
		t.Fatalf("REJ result: accepted=%v retransmit=%v", accepted, retransmit)
	}
	if l.va != 0 {
		t.Fatalf("REJ changed VA to %d", l.va)
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
