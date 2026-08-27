package digipeater

import (
	"crypto/sha256"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

// Digipeater implements traditional AX.25 same-port digipeating. It repeats a
// frame only when one of its aliases is the first not-yet-repeated VIA entry.
type Digipeater struct {
	mu      sync.Mutex
	aliases map[string]struct{}
	seen    map[[32]byte]time.Time
	window  time.Duration
	now     func() time.Time
}

func New(aliases ...ax25.Address) *Digipeater {
	d := &Digipeater{aliases: make(map[string]struct{}), seen: make(map[[32]byte]time.Time), window: 30 * time.Second, now: time.Now}
	for _, alias := range aliases {
		if alias.Validate() == nil {
			d.aliases[alias.String()] = struct{}{}
		}
	}
	return d
}

// Repeat returns the encoded frame with the matching VIA H bit set. A false
// result means the frame is not addressed to this digipeater or is a duplicate UI frame.
func (d *Digipeater) Repeat(frame ax25.Frame, raw []byte) ([]byte, bool) {
	if len(frame.Digipeaters) == 0 {
		return nil, false
	}
	if _, own := d.aliases[frame.Source.String()]; own {
		return nil, false
	}
	next := -1
	for i, via := range frame.Digipeaters {
		if !via.Repeated {
			next = i
			break
		}
	}
	if next < 0 {
		return nil, false
	}
	if _, ours := d.aliases[frame.Digipeaters[next].String()]; !ours {
		return nil, false
	}
	// UI frames have no protocol acknowledgement, so suppress duplicate copies.
	// Connected-mode frames must pass every time: an identical SABM, I, RR or
	// DISC frame can be a required AX.25 retry after a lost response.
	if frame.Type == ax25.TypeUI {
		key := sha256.Sum256(raw)
		now := d.now()
		d.mu.Lock()
		for k, expires := range d.seen {
			if !now.Before(expires) {
				delete(d.seen, k)
			}
		}
		if expires, duplicate := d.seen[key]; duplicate && now.Before(expires) {
			d.mu.Unlock()
			return nil, false
		}
		d.seen[key] = now.Add(d.window)
		d.mu.Unlock()
	}
	frame.Digipeaters[next].Repeated = true
	out, err := ax25.Encode(frame)
	return out, err == nil
}
