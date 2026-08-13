package ax25

import (
	"fmt"
	"strings"
)

type Address struct {
	Callsign        string
	SSID            uint8
	Repeated        bool
	CommandResponse bool
}

func ParseAddress(s string) (Address, error) {
	p := strings.Split(strings.ToUpper(strings.TrimSpace(s)), "-")
	if len(p) > 2 {
		return Address{}, fmt.Errorf("invalid address")
	}
	a := Address{Callsign: p[0]}
	if len(p) == 2 {
		var n int
		if _, e := fmt.Sscanf(p[1], "%d", &n); e != nil || n < 0 || n > 15 {
			return Address{}, fmt.Errorf("invalid SSID")
		}
		a.SSID = uint8(n)
	}
	if err := a.Validate(); err != nil {
		return Address{}, err
	}
	return a, nil
}

// ParseDigipeaters parses a comma-separated digipeater path.
// Empty input returns nil, nil.
func ParseDigipeaters(s string) ([]Address, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	out := make([]Address, 0, len(parts))
	for _, part := range parts {
		a, err := ParseAddress(part)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if len(out) > 8 {
		return nil, fmt.Errorf("too many digipeaters")
	}
	return out, nil
}
func (a Address) Validate() error {
	if len(a.Callsign) < 1 || len(a.Callsign) > 6 || a.SSID > 15 {
		return fmt.Errorf("invalid AX.25 address")
	}
	for _, r := range a.Callsign {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return fmt.Errorf("invalid callsign")
		}
	}
	return nil
}
func (a Address) String() string {
	if a.SSID == 0 {
		return a.Callsign
	}
	return fmt.Sprintf("%s-%d", a.Callsign, a.SSID)
}
func encodeAddress(a Address, last bool) ([7]byte, error) {
	var out [7]byte
	if err := a.Validate(); err != nil {
		return out, err
	}
	cs := fmt.Sprintf("%-6s", strings.ToUpper(a.Callsign))
	for i := 0; i < 6; i++ {
		out[i] = cs[i] << 1
	}
	out[6] = 0x60 | (a.SSID << 1)
	if a.Repeated || a.CommandResponse {
		out[6] |= 0x80
	}
	if last {
		out[6] |= 1
	}
	return out, nil
}
func decodeAddress(b []byte) (Address, bool, error) {
	if len(b) < 7 {
		return Address{}, false, fmt.Errorf("short AX.25 address")
	}
	raw := make([]byte, 6)
	for i := 0; i < 6; i++ {
		if b[i]&1 != 0 {
			return Address{}, false, fmt.Errorf("invalid shifted callsign")
		}
		raw[i] = b[i] >> 1
	}
	a := Address{Callsign: strings.TrimSpace(string(raw)), SSID: (b[6] >> 1) & 15, CommandResponse: b[6]&0x80 != 0, Repeated: b[6]&0x80 != 0}
	if err := a.Validate(); err != nil {
		return Address{}, false, err
	}
	return a, b[6]&1 != 0, nil
}
