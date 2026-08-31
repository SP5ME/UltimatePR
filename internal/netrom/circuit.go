package netrom

import (
	"errors"
	"fmt"
)

// CircuitState is the NET/ROM transport state for one virtual circuit.
type CircuitState uint8

const (
	CircuitIdle CircuitState = iota
	CircuitAwaitingConnect
	CircuitConnected
	CircuitAwaitingRelease
	CircuitClosed
)

// CircuitEvent contains packets that the caller must transmit and data that
// has arrived in order. The circuit deliberately does not perform radio I/O.
type CircuitEvent struct {
	Packets        []Packet
	Data           []byte
	ConnectRequest *ConnectRequest
	Connected      bool
	Closed         bool
	NAK            bool
}

// Circuit implements the modulo-256 NET/ROM transport sequence machine.
// Window sizes are limited to 127 so sequence comparisons remain unambiguous.
type Circuit struct {
	localIndex, localID uint8
	peerIndex, peerID   uint8
	localWindow         uint8
	peerWindow          uint8
	state               CircuitState
	vs, va, vr          uint8
	pending             map[uint8]Packet
	rxFragment          []byte
	peerBusy            bool
}

func NewCircuit(localIndex, localID, window uint8) (*Circuit, error) {
	if !validWindow(window) {
		return nil, fmt.Errorf("invalid NET/ROM window %d", window)
	}
	return &Circuit{localIndex: localIndex, localID: localID, localWindow: window, state: CircuitIdle, pending: map[uint8]Packet{}}, nil
}

func (c *Circuit) State() CircuitState { return c.state }

// Open starts an outgoing circuit. The returned packet is the complete
// transport payload for a NET/ROM CONNECT REQUEST.
func (c *Circuit) Open(req ConnectRequest) (Packet, error) {
	if c.state != CircuitIdle {
		return Packet{}, errors.New("NET/ROM circuit is already active")
	}
	if !validWindow(req.Window) {
		return Packet{}, fmt.Errorf("invalid NET/ROM proposed window %d", req.Window)
	}
	payload, err := req.Encode()
	if err != nil {
		return Packet{}, err
	}
	c.peerWindow = req.Window
	c.state = CircuitAwaitingConnect
	return Packet{CircuitIndex: c.localIndex, CircuitID: c.localID, Opcode: OpcodeConnectRequest, Payload: payload}, nil
}

// Accept completes an incoming CONNECT REQUEST and returns its ACK. The
// caller supplies the local receive window, which is negotiated down to the
// requester's proposal.
func (c *Circuit) Accept(request Packet, window uint8) (Packet, error) {
	if c.state != CircuitIdle {
		return Packet{}, errors.New("NET/ROM circuit is already active")
	}
	if request.Opcode != OpcodeConnectRequest || !validWindow(window) {
		return Packet{}, errors.New("invalid NET/ROM connect request")
	}
	req, err := DecodeConnectRequest(request.Payload)
	if err != nil || !validWindow(req.Window) {
		return Packet{}, errors.New("invalid NET/ROM connect request window")
	}
	c.peerIndex, c.peerID = request.CircuitIndex, request.CircuitID
	c.peerWindow = req.Window
	c.localWindow = minWindow(c.localWindow, req.Window)
	c.state = CircuitConnected
	return Packet{CircuitIndex: c.peerIndex, CircuitID: c.peerID, TXSequence: c.localIndex, RXSequence: c.localID, Opcode: OpcodeConnectAcknowledge, Payload: []byte{c.localWindow}}, nil
}

// Send queues one or more information packets when the negotiated window has
// room. The caller transmits the returned packets in order.
func (c *Circuit) Send(data []byte) ([]Packet, error) {
	if c.state != CircuitConnected {
		return nil, errors.New("NET/ROM circuit is not connected")
	}
	if c.peerBusy {
		return nil, errors.New("NET/ROM peer is choked")
	}
	if len(data) == 0 {
		return nil, nil
	}
	if outstanding(c.va, c.vs) >= int(c.peerWindow) {
		return nil, errors.New("NET/ROM transmit window is full")
	}
	packets := Fragment(data, c.peerIndex, c.peerID, c.vs, c.vr)
	available := int(c.peerWindow) - outstanding(c.va, c.vs)
	if len(packets) > available {
		return nil, errors.New("NET/ROM transmit window is too small for message")
	}
	for _, packet := range packets {
		c.pending[packet.TXSequence] = packet
		c.vs++
	}
	return packets, nil
}

// Handle consumes one transport packet and returns protocol responses and
// complete reassembled user data. Malformed or out-of-state packets are
// rejected rather than silently changing circuit state.
func (c *Circuit) Handle(packet Packet) (CircuitEvent, error) {
	var event CircuitEvent
	switch packet.Opcode {
	case OpcodeConnectAcknowledge:
		if c.state != CircuitAwaitingConnect || packet.CircuitIndex != c.localIndex || packet.CircuitID != c.localID || len(packet.Payload) != 1 || !validWindow(packet.Payload[0]) {
			return event, errors.New("invalid NET/ROM connect acknowledge")
		}
		c.peerIndex, c.peerID = packet.TXSequence, packet.RXSequence
		c.peerWindow = packet.Payload[0]
		c.state = CircuitConnected
		event.Connected = true
	case OpcodeInformation:
		if c.state != CircuitConnected || packet.CircuitIndex != c.localIndex || packet.CircuitID != c.localID {
			return event, errors.New("NET/ROM information for unknown circuit")
		}
		c.acknowledge(packet.RXSequence)
		if packet.Choke {
			c.peerBusy = true
		} else {
			c.peerBusy = false
		}
		if packet.TXSequence == c.vr {
			c.vr++
			c.rxFragment = append(c.rxFragment, packet.Payload...)
			if !packet.MoreFollows {
				event.Data = append([]byte(nil), c.rxFragment...)
				c.rxFragment = c.rxFragment[:0]
			}
		} else if packet.TXSequence != c.vr-1 {
			event.NAK = true
		}
		event.Packets = append(event.Packets, c.ackPacket(event.NAK))
	case OpcodeInformationAcknowledge:
		if c.state != CircuitConnected || packet.CircuitIndex != c.localIndex || packet.CircuitID != c.localID {
			return event, errors.New("NET/ROM acknowledgement for unknown circuit")
		}
		c.acknowledge(packet.RXSequence)
		c.peerBusy = packet.Choke
	case OpcodeDisconnectRequest:
		if c.state == CircuitClosed || packet.CircuitIndex != c.localIndex || packet.CircuitID != c.localID {
			return event, errors.New("NET/ROM disconnect for unknown circuit")
		}
		c.state = CircuitClosed
		event.Closed = true
		event.Packets = append(event.Packets, Packet{CircuitIndex: c.localIndex, CircuitID: c.localID, TXSequence: c.peerIndex, RXSequence: c.peerID, Opcode: OpcodeDisconnectAcknowledge})
	case OpcodeDisconnectAcknowledge:
		if c.state != CircuitAwaitingRelease || packet.CircuitIndex != c.localIndex || packet.CircuitID != c.localID {
			return event, errors.New("NET/ROM disconnect acknowledgement for unknown circuit")
		}
		c.state = CircuitClosed
		event.Closed = true
	default:
		return event, fmt.Errorf("unsupported NET/ROM opcode %d", packet.Opcode)
	}
	return event, nil
}

func (c *Circuit) Disconnect() (Packet, error) {
	if c.state != CircuitConnected && c.state != CircuitAwaitingConnect {
		return Packet{}, errors.New("NET/ROM circuit is not active")
	}
	c.state = CircuitAwaitingRelease
	return Packet{CircuitIndex: c.localIndex, CircuitID: c.localID, TXSequence: c.peerIndex, RXSequence: c.peerID, Opcode: OpcodeDisconnectRequest}, nil
}

func (c *Circuit) ackPacket(nak bool) Packet {
	return Packet{CircuitIndex: c.peerIndex, CircuitID: c.peerID, RXSequence: c.vr, Opcode: OpcodeInformationAcknowledge, NAK: nak, Choke: c.peerBusy}
}

func (c *Circuit) acknowledge(nr uint8) {
	if outstanding(c.va, nr) > outstanding(c.va, c.vs) {
		return
	}
	for c.va != nr {
		delete(c.pending, c.va)
		c.va++
		if c.va == c.vs {
			break
		}
	}
}

func validWindow(window uint8) bool { return window > 0 && window <= 127 }

func minWindow(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

func outstanding(acknowledged, sent uint8) int { return int(sent - acknowledged) }
