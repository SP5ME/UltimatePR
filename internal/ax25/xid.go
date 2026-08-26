package ax25

import "fmt"

const (
	xidFormatIdentifier = 0x82
	xidGroupIdentifier  = 0x80
)

// XIDParameter is one ISO 8885 PI/PL/PV parameter used by AX.25 v2.2.
// Value is stored in network byte order and unknown identifiers are retained.
type XIDParameter struct {
	Identifier byte
	Value      []byte
}

// BasicXIDParameters advertises exactly the capabilities implemented by the
// interoperable connected-mode engine: half duplex, implicit REJ, modulo 8,
// N1=256 octets, receive window k=1, T1=10 seconds and N2=10.
func BasicXIDParameters() []XIDParameter {
	return LinkXIDParameters(256, 1, 10000, 10)
}

// LinkXIDParameters builds the conservative modulo-8 profile implemented by
// UltimatePR using the configured link limits.
func LinkXIDParameters(n1, window, t1Milliseconds, n2 int) []XIDParameter {
	if n1 < 1 {
		n1 = 256
	}
	if window < 1 {
		window = 1
	}
	if t1Milliseconds < 1 {
		t1Milliseconds = 10000
	}
	if n2 < 1 {
		n2 = 10
	}
	// These fields have fixed widths in the AX.25 XID parameter encoding.
	// Clamp defensively so callers can never advertise wrapped values.
	if n1 > 8191 {
		n1 = 8191
	}
	if window > 255 {
		window = 255
	}
	if t1Milliseconds > 65535 {
		t1Milliseconds = 65535
	}
	if n2 > 255 {
		n2 = 255
	}
	n1Bits := n1 * 8
	return []XIDParameter{
		{Identifier: 2, Value: []byte{0x00, 0x20}},
		{Identifier: 3, Value: []byte{0x81, 0xA4, 0x02}},
		{Identifier: 6, Value: []byte{byte(n1Bits >> 8), byte(n1Bits)}},
		{Identifier: 8, Value: []byte{byte(window)}},
		{Identifier: 9, Value: []byte{byte(t1Milliseconds >> 8), byte(t1Milliseconds)}},
		{Identifier: 10, Value: []byte{byte(n2)}},
	}
}

func EncodeXID(parameters []XIDParameter) ([]byte, error) {
	length := 0
	previous := -1
	for _, p := range parameters {
		if int(p.Identifier) <= previous {
			return nil, fmt.Errorf("XID parameters must be unique and ascending")
		}
		if len(p.Value) > 255 {
			return nil, fmt.Errorf("XID parameter %d is too long", p.Identifier)
		}
		previous = int(p.Identifier)
		length += 2 + len(p.Value)
	}
	if length > 0xFFFF {
		return nil, fmt.Errorf("XID parameter group is too long")
	}
	out := []byte{xidFormatIdentifier, xidGroupIdentifier, byte(length >> 8), byte(length)}
	for _, p := range parameters {
		out = append(out, p.Identifier, byte(len(p.Value)))
		out = append(out, p.Value...)
	}
	return out, nil
}

func DecodeXID(in []byte) ([]XIDParameter, error) {
	if len(in) < 4 || in[0] != xidFormatIdentifier || in[1] != xidGroupIdentifier {
		return nil, fmt.Errorf("invalid AX.25 XID header")
	}
	length := int(in[2])<<8 | int(in[3])
	if length != len(in)-4 {
		return nil, fmt.Errorf("invalid AX.25 XID group length")
	}
	var out []XIDParameter
	previous := -1
	for pos := 4; pos < len(in); {
		if pos+2 > len(in) {
			return nil, fmt.Errorf("truncated AX.25 XID parameter")
		}
		id, size := in[pos], int(in[pos+1])
		pos += 2
		if int(id) <= previous || pos+size > len(in) {
			return nil, fmt.Errorf("invalid AX.25 XID parameter list")
		}
		out = append(out, XIDParameter{Identifier: id, Value: append([]byte(nil), in[pos:pos+size]...)})
		previous = int(id)
		pos += size
	}
	return out, nil
}
