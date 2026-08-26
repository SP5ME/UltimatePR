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

// XIDLinkSettings contains the AX.25 v2.2 parameters used by UltimatePR.
// ReceiveN1 and ReceiveWindow are notifications of local receive limits;
// T1Milliseconds and Retries are negotiated values.
type XIDLinkSettings struct {
	FullDuplex      bool
	SelectiveReject bool
	Modulo          Modulo
	ReceiveN1       int
	ReceiveWindow   int
	T1Milliseconds  int
	Retries         int
}

func DefaultXIDLinkSettings() XIDLinkSettings {
	return XIDLinkSettings{SelectiveReject: true, Modulo: Modulo8, ReceiveN1: 256, ReceiveWindow: 7, T1Milliseconds: 3000, Retries: 10}
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
	return XIDParameters(XIDLinkSettings{Modulo: Modulo8, ReceiveN1: n1, ReceiveWindow: window, T1Milliseconds: t1Milliseconds, Retries: n2})
}

// ParseXIDLinkSettings applies recognized parameters over current settings.
// Unknown PIs are ignored as required by AX.25 v2.2 section 4.3.3.7.
func ParseXIDLinkSettings(parameters []XIDParameter, current XIDLinkSettings) (XIDLinkSettings, error) {
	defaults := DefaultXIDLinkSettings()
	for _, p := range parameters {
		if len(p.Value) == 0 {
			switch p.Identifier {
			case 2:
				current.FullDuplex = defaults.FullDuplex
			case 3:
				current.SelectiveReject, current.Modulo = defaults.SelectiveReject, defaults.Modulo
			case 6:
				current.ReceiveN1 = defaults.ReceiveN1
			case 8:
				current.ReceiveWindow = defaults.ReceiveWindow
			case 9:
				current.T1Milliseconds = defaults.T1Milliseconds
			case 10:
				current.Retries = defaults.Retries
			}
			continue
		}
		switch p.Identifier {
		case 2:
			if len(p.Value) != 2 || (p.Value[1]&0x60) == 0 || (p.Value[1]&0x60) == 0x60 {
				return current, fmt.Errorf("invalid XID classes of procedures")
			}
			current.FullDuplex = p.Value[1]&0x40 != 0
		case 3:
			if len(p.Value) != 3 {
				return current, fmt.Errorf("invalid XID optional functions")
			}
			reject := p.Value[0] & 0x06
			if reject == 0 {
				return current, fmt.Errorf("invalid XID reject function")
			}
			current.SelectiveReject = reject&0x04 != 0
			modulo := p.Value[1] & 0x0C
			if modulo != 0x04 && modulo != 0x08 {
				return current, fmt.Errorf("invalid XID modulo selection")
			}
			if modulo == 0x08 {
				current.Modulo = Modulo128
			} else {
				current.Modulo = Modulo8
			}
		case 6:
			value, err := xidNumber(p.Value)
			if err != nil || value < 8 {
				return current, fmt.Errorf("invalid XID receive I-field length")
			}
			current.ReceiveN1 = value / 8
		case 8:
			value, err := xidNumber(p.Value)
			if err != nil || value < 1 || value > 127 {
				return current, fmt.Errorf("invalid XID receive window")
			}
			current.ReceiveWindow = value
		case 9:
			value, err := xidNumber(p.Value)
			if err != nil || value < 1 {
				return current, fmt.Errorf("invalid XID acknowledge timer")
			}
			current.T1Milliseconds = value
		case 10:
			value, err := xidNumber(p.Value)
			if err != nil || value < 1 {
				return current, fmt.Errorf("invalid XID retry count")
			}
			current.Retries = value
		}
	}
	return current, nil
}

func xidNumber(value []byte) (int, error) {
	if len(value) == 0 || len(value) > 4 {
		return 0, fmt.Errorf("invalid XID numeric length")
	}
	n := 0
	for _, b := range value {
		n = n<<8 | int(b)
	}
	return n, nil
}

func xidEncodedNumber(value int) []byte {
	if value < 1 {
		return []byte{0}
	}
	out := []byte{byte(value)}
	for value >>= 8; value > 0; value >>= 8 {
		out = append([]byte{byte(value)}, out...)
	}
	return out
}

// NegotiateXID selects the mutually supported negotiable parameters. The
// returned response retains the local N1/window receive notifications.
func NegotiateXID(command []XIDParameter, local XIDLinkSettings) (XIDLinkSettings, XIDLinkSettings, error) {
	offered, err := ParseXIDLinkSettings(command, local)
	if err != nil {
		return local, offered, err
	}
	selected := local
	selected.FullDuplex = local.FullDuplex && offered.FullDuplex
	selected.SelectiveReject = local.SelectiveReject && offered.SelectiveReject
	if local.Modulo != Modulo128 || offered.Modulo != Modulo128 {
		selected.Modulo = Modulo8
	}
	if offered.T1Milliseconds > selected.T1Milliseconds {
		selected.T1Milliseconds = offered.T1Milliseconds
	}
	if offered.Retries > selected.Retries {
		selected.Retries = offered.Retries
	}
	return selected, offered, nil
}

func XIDParameters(settings XIDLinkSettings) []XIDParameter {
	if settings.ReceiveN1 < 1 {
		settings.ReceiveN1 = 256
	}
	if settings.ReceiveWindow < 1 || settings.ReceiveWindow > 127 {
		settings.ReceiveWindow = 1
	}
	if settings.T1Milliseconds < 1 {
		settings.T1Milliseconds = 3000
	}
	if settings.Retries < 1 {
		settings.Retries = 10
	}
	parameters := []XIDParameter{
		{Identifier: 2, Value: []byte{0x00, 0x20}},
		{Identifier: 3},
		{Identifier: 6, Value: xidEncodedNumber(settings.ReceiveN1 * 8)},
		{Identifier: 8, Value: xidEncodedNumber(settings.ReceiveWindow)},
		{Identifier: 9, Value: xidEncodedNumber(settings.T1Milliseconds)},
		{Identifier: 10, Value: xidEncodedNumber(settings.Retries)},
	}
	if settings.FullDuplex {
		parameters[0].Value = []byte{0x00, 0x40}
	}
	optional := []byte{0x82, 0xA4, 0x02}
	if settings.SelectiveReject {
		optional[0] = 0x84
	}
	if settings.Modulo == Modulo128 {
		optional[1] = optional[1]&^0x04 | 0x08
	}
	parameters[1].Value = optional
	return parameters
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
