package netrom

import (
	"fmt"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

const NetworkHeaderSize = 15

type NetworkHeader struct {
	Origin      ax25.Address
	Destination ax25.Address
	TTL         uint8
}

// MaxInformationPayload is the NET/ROM user-data limit with a 256-byte AX.25
// information field: 15 bytes network header and 5 bytes transport header.
const MaxInformationPayload = 236

// DecrementTTL consumes one network hop. It returns false when the frame must
// be discarded instead of forwarded.
func (h *NetworkHeader) DecrementTTL() bool {
	if h == nil || h.TTL <= 1 {
		return false
	}
	h.TTL--
	return true
}

// Fragment splits one NET/ROM information message into wire-sized transport
// packets and marks every packet except the final one with MoreFollows.
func Fragment(data []byte, circuitIndex, circuitID, sequence, acknowledge uint8) []Packet {
	if len(data) == 0 {
		return nil
	}
	out := make([]Packet, 0, (len(data)+MaxInformationPayload-1)/MaxInformationPayload)
	for len(data) > 0 {
		n := len(data)
		if n > MaxInformationPayload {
			n = MaxInformationPayload
		}
		out = append(out, Packet{CircuitIndex: circuitIndex, CircuitID: circuitID, TXSequence: sequence, RXSequence: acknowledge, Opcode: OpcodeInformation, MoreFollows: n < len(data), Payload: append([]byte(nil), data[:n]...)})
		sequence++
		data = data[n:]
	}
	return out
}

type Frame struct {
	Network   NetworkHeader
	Transport Packet
}

// Forward prepares a frame for one network hop. The end-to-end addresses are
// preserved; only the hop lifetime is consumed. A frame addressed to the
// local node or with an expired TTL is rejected.
func (f Frame) Forward(local ax25.Address) (Frame, error) {
	if f.Network.Destination.String() == local.String() {
		return Frame{}, fmt.Errorf("NET/ROM frame is addressed to the local node")
	}
	if !f.Network.DecrementTTL() {
		return Frame{}, fmt.Errorf("NET/ROM frame TTL expired")
	}
	return f, nil
}

func (f Frame) Encode() ([]byte, error) {
	origin, err := packAddress(f.Network.Origin)
	if err != nil {
		return nil, fmt.Errorf("NET/ROM origin: %w", err)
	}
	destination, err := packAddress(f.Network.Destination)
	if err != nil {
		return nil, fmt.Errorf("NET/ROM destination: %w", err)
	}
	destination[6] |= 1 // end-of-address marker in the network header
	transport, err := f.Transport.Encode()
	if err != nil {
		return nil, err
	}
	out := append(origin, destination...)
	out = append(out, f.Network.TTL)
	return append(out, transport...), nil
}

func DecodeFrame(data []byte) (Frame, error) {
	if len(data) < NetworkHeaderSize+HeaderSize {
		return Frame{}, fmt.Errorf("NET/ROM frame too short")
	}
	origin, err := unpackAddress(data[:7])
	if err != nil {
		return Frame{}, fmt.Errorf("NET/ROM origin: %w", err)
	}
	destination, err := unpackAddress(data[7:14])
	if err != nil {
		return Frame{}, fmt.Errorf("NET/ROM destination: %w", err)
	}
	transport, err := DecodePacket(data[NetworkHeaderSize:])
	if err != nil {
		return Frame{}, err
	}
	return Frame{Network: NetworkHeader{Origin: origin, Destination: destination, TTL: data[14]}, Transport: transport}, nil
}
