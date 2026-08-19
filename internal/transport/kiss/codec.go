package kiss

import "errors"

const (
	FEND  byte = 0xC0
	FESC  byte = 0xDB
	TFEND byte = 0xDC
	TFESC byte = 0xDD

	CommandData        uint8 = 0x00
	CommandTXDelay     uint8 = 0x01
	CommandPersistence uint8 = 0x02
	CommandSlotTime    uint8 = 0x03
	CommandTXTail      uint8 = 0x04
	CommandFullDuplex  uint8 = 0x05
	CommandSetHardware uint8 = 0x06
	CommandReturn      uint8 = 0x0F
)

var ErrFrameTooLarge = errors.New("KISS frame too large")
var ErrInvalidEscape = errors.New("invalid KISS escape")

type Frame struct {
	Port    uint8
	Command uint8
	Data    []byte
}

func Encode(f Frame) ([]byte, error) {
	if f.Port > 15 || f.Command > 15 {
		return nil, errors.New("KISS port and command must be 0..15")
	}
	out := make([]byte, 0, len(f.Data)+3)
	out = append(out, FEND)
	appendEscaped := func(b byte) {
		switch b {
		case FEND:
			out = append(out, FESC, TFEND)
		case FESC:
			out = append(out, FESC, TFESC)
		default:
			out = append(out, b)
		}
	}
	appendEscaped((f.Port << 4) | f.Command)
	for _, b := range f.Data {
		appendEscaped(b)
	}
	out = append(out, FEND)
	return out, nil
}

type Decoder struct {
	max      int
	buf      []byte
	escaped  bool
	dropping bool
	synced   bool
}

func NewDecoder(max int) *Decoder { return &Decoder{max: max, buf: make([]byte, 0, max)} }

func (d *Decoder) Feed(input []byte) ([]Frame, []error) {
	var frames []Frame
	var errs []error
	for _, b := range input {
		if b == FEND {
			if !d.synced {
				d.synced = true
				d.reset()
				continue
			}
			if d.dropping {
				d.reset()
				continue
			}
			if d.escaped {
				errs = append(errs, ErrInvalidEscape)
				d.escaped = false
			}
			if len(d.buf) > 0 {
				cmd := d.buf[0]
				data := append([]byte(nil), d.buf[1:]...)
				frames = append(frames, Frame{Port: cmd >> 4, Command: cmd & 0x0F, Data: data})
			}
			d.reset()
			continue
		}
		if d.dropping {
			continue
		}
		if d.escaped {
			switch b {
			case TFEND:
				b = FEND
			case TFESC:
				b = FESC
			default:
				errs = append(errs, ErrInvalidEscape)
				d.escaped = false
				continue
			}
			d.escaped = false
		} else if b == FESC {
			d.escaped = true
			continue
		}
		if len(d.buf) >= d.max {
			errs = append(errs, ErrFrameTooLarge)
			d.dropping = true
			continue
		}
		d.buf = append(d.buf, b)
	}
	return frames, errs
}
func (d *Decoder) reset() { d.buf = d.buf[:0]; d.escaped = false; d.dropping = false }
