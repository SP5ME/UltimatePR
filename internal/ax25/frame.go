package ax25

import "fmt"

type Type uint8

const (
	TypeI Type = iota
	TypeRR
	TypeRNR
	TypeREJ
	TypeSABM
	TypeDISC
	TypeDM
	TypeUA
	TypeUI
	TypeUnknown
)

type Frame struct {
	Destination, Source Address
	Digipeaters         []Address
	Type                Type
	PollFinal           bool
	NS, NR              uint8
	PID                 *byte
	Payload             []byte
}

func control(f Frame) (byte, error) {
	pf := byte(0)
	if f.PollFinal {
		pf = 0x10
	}
	switch f.Type {
	case TypeI:
		return (f.NS&7)<<1 | pf | (f.NR&7)<<5, nil
	case TypeRR:
		return 0x01 | pf | (f.NR&7)<<5, nil
	case TypeRNR:
		return 0x05 | pf | (f.NR&7)<<5, nil
	case TypeREJ:
		return 0x09 | pf | (f.NR&7)<<5, nil
	case TypeSABM:
		return 0x2F | pf, nil
	case TypeDISC:
		return 0x43 | pf, nil
	case TypeDM:
		return 0x0F | pf, nil
	case TypeUA:
		return 0x63 | pf, nil
	case TypeUI:
		return 0x03 | pf, nil
	default:
		return 0, fmt.Errorf("unsupported AX.25 type")
	}
}
func Encode(f Frame) ([]byte, error) {
	if len(f.Digipeaters) > 8 {
		return nil, fmt.Errorf("too many digipeaters")
	}
	addrs := append([]Address{f.Destination, f.Source}, f.Digipeaters...)
	out := make([]byte, 0, len(addrs)*7+2+len(f.Payload))
	for i, a := range addrs {
		e, err := encodeAddress(a, i == len(addrs)-1)
		if err != nil {
			return nil, err
		}
		out = append(out, e[:]...)
	}
	c, err := control(f)
	if err != nil {
		return nil, err
	}
	out = append(out, c)
	if f.Type == TypeI || f.Type == TypeUI {
		if f.PID == nil {
			return nil, fmt.Errorf("PID required")
		}
		out = append(out, *f.PID)
	}
	out = append(out, f.Payload...)
	return out, nil
}
func Decode(b []byte) (Frame, error) {
	if len(b) < 15 {
		return Frame{}, fmt.Errorf("AX.25 frame too short")
	}
	var as []Address
	pos := 0
	for {
		if pos+7 > len(b) || len(as) >= 10 {
			return Frame{}, fmt.Errorf("invalid address field")
		}
		a, last, err := decodeAddress(b[pos : pos+7])
		if err != nil {
			return Frame{}, err
		}
		as = append(as, a)
		pos += 7
		if last {
			break
		}
	}
	if len(as) < 2 || pos >= len(b) {
		return Frame{}, fmt.Errorf("missing source or control")
	}
	f := Frame{Destination: as[0], Source: as[1]}
	if len(as) > 2 {
		f.Digipeaters = as[2:]
	}
	c := b[pos]
	pos++
	f.PollFinal = c&0x10 != 0
	// Bit 7 is C/R for destination/source and H (has-been-repeated) for digis.
	f.Destination.Repeated = false
	f.Source.Repeated = false
	for i := range f.Digipeaters {
		f.Digipeaters[i].CommandResponse = false
	}
	if c&1 == 0 {
		f.Type = TypeI
		f.NS = (c >> 1) & 7
		f.NR = (c >> 5) & 7
	} else if c&3 == 1 {
		f.NR = (c >> 5) & 7
		switch c & 0x0F {
		case 1:
			f.Type = TypeRR
		case 5:
			f.Type = TypeRNR
		case 9:
			f.Type = TypeREJ
		default:
			f.Type = TypeUnknown
		}
	} else {
		switch c & 0xEF {
		case 0x2F:
			f.Type = TypeSABM
		case 0x43:
			f.Type = TypeDISC
		case 0x0F:
			f.Type = TypeDM
		case 0x63:
			f.Type = TypeUA
		case 0x03:
			f.Type = TypeUI
		default:
			f.Type = TypeUnknown
		}
	}
	if f.Type == TypeI || f.Type == TypeUI {
		if pos >= len(b) {
			return Frame{}, fmt.Errorf("missing PID")
		}
		pid := b[pos]
		f.PID = &pid
		pos++
	}
	f.Payload = append([]byte(nil), b[pos:]...)
	return f, nil
}
