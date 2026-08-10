package web

import (
	"net"
	"sync"
)

const (
	iac  = 255
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250
	se   = 240
)

type telnetFilter struct {
	conn    net.Conn
	state   byte
	command byte
	mu      sync.Mutex
}

func newTelnetFilter(c net.Conn) *telnetFilter { return &telnetFilter{conn: c} }
func (f *telnetFilter) Feed(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for _, b := range in {
		switch f.state {
		case 0:
			if b == iac {
				f.state = 1
			} else {
				out = append(out, b)
			}
		case 1:
			switch b {
			case iac:
				out = append(out, iac)
				f.state = 0
			case will, wont, do, dont:
				f.command = b
				f.state = 2
			case sb:
				f.state = 3
			default:
				f.state = 0
			}
		case 2:
			f.reject(f.command, b)
			f.state = 0
		case 3:
			if b == iac {
				f.state = 4
			}
		case 4:
			if b == se {
				f.state = 0
			} else if b != iac {
				f.state = 3
			}
		}
	}
	return out
}
func (f *telnetFilter) reject(cmd, opt byte) {
	reply := byte(dont)
	if cmd == do || cmd == dont {
		reply = wont
	}
	f.mu.Lock()
	_, _ = f.conn.Write([]byte{iac, reply, opt})
	f.mu.Unlock()
}
