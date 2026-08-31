package netrom

import (
	"fmt"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

const HeaderSize = 5

type Opcode uint8

const (
	OpcodeExtension Opcode = iota
	OpcodeConnectRequest
	OpcodeConnectAcknowledge
	OpcodeDisconnectRequest
	OpcodeDisconnectAcknowledge
	OpcodeInformation
	OpcodeInformationAcknowledge
	OpcodeReset
)

type Packet struct {
	CircuitIndex uint8
	CircuitID    uint8
	// TXSequence and RXSequence are sequence numbers for INFO and
	// INFO_ACK. For CONNECT messages the same two octets carry the peer
	// circuit index and ID, respectively.
	TXSequence  uint8
	RXSequence  uint8
	Opcode      Opcode
	MoreFollows bool
	NAK         bool
	Choke       bool
	Payload     []byte
}

type ConnectRequest struct {
	OriginUser ax25.Address
	OriginNode ax25.Address
	Window     uint8
}

func (r ConnectRequest) Encode() ([]byte, error) {
	user, err := packAddress(r.OriginUser)
	if err != nil {
		return nil, err
	}
	node, err := packAddress(r.OriginNode)
	if err != nil {
		return nil, err
	}
	return append(append(user, node...), r.Window), nil
}

func DecodeConnectRequest(data []byte) (ConnectRequest, error) {
	if len(data) != 15 {
		return ConnectRequest{}, fmt.Errorf("invalid NET/ROM connect request length")
	}
	user, err := unpackAddress(data[:7])
	if err != nil {
		return ConnectRequest{}, err
	}
	node, err := unpackAddress(data[7:14])
	if err != nil {
		return ConnectRequest{}, err
	}
	return ConnectRequest{OriginUser: user, OriginNode: node, Window: data[14]}, nil
}

type ConnectAcknowledge struct {
	CircuitIndex uint8
	CircuitID    uint8
	Window       uint8
}

func (a ConnectAcknowledge) Encode() []byte {
	return []byte{a.CircuitIndex, a.CircuitID, a.Window}
}

func DecodeConnectAcknowledge(data []byte) (ConnectAcknowledge, error) {
	if len(data) != 3 {
		return ConnectAcknowledge{}, fmt.Errorf("invalid NET/ROM connect acknowledge length")
	}
	return ConnectAcknowledge{CircuitIndex: data[0], CircuitID: data[1], Window: data[2]}, nil
}

func (p Packet) Encode() ([]byte, error) {
	if p.Opcode > OpcodeReset {
		return nil, fmt.Errorf("invalid NET/ROM opcode %d", p.Opcode)
	}
	flags := uint8(p.Opcode)
	if p.MoreFollows {
		flags |= 1 << 5
	}
	if p.NAK {
		flags |= 1 << 6
	}
	if p.Choke {
		flags |= 1 << 7
	}
	out := []byte{p.CircuitIndex, p.CircuitID, p.TXSequence, p.RXSequence, flags}
	return append(out, p.Payload...), nil
}

func DecodePacket(data []byte) (Packet, error) {
	if len(data) < HeaderSize {
		return Packet{}, fmt.Errorf("NET/ROM packet too short")
	}
	flags := data[4]
	opcode := Opcode(flags & 0x0F)
	if opcode > OpcodeReset {
		return Packet{}, fmt.Errorf("invalid NET/ROM opcode %d", opcode)
	}
	return Packet{CircuitIndex: data[0], CircuitID: data[1], TXSequence: data[2], RXSequence: data[3], Opcode: opcode, MoreFollows: flags&0x20 != 0, NAK: flags&0x40 != 0, Choke: flags&0x80 != 0, Payload: append([]byte(nil), data[5:]...)}, nil
}
