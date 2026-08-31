package netrom

import "fmt"

type BridgeSide uint8

const (
	BridgeLeft BridgeSide = iota
	BridgeRight
)

// BridgeEvent separates responses for the receiving leg from packets that
// must be transmitted on the opposite leg.
type BridgeEvent struct {
	SameSide  []Packet
	OtherSide []Packet
	Data      []byte
	Closed    bool
}

// Bridge joins two already-established NET/ROM circuits. Circuit setup is
// intentionally left to the caller because each leg has different circuit
// identifiers and network addresses.
type Bridge struct {
	left, right *Circuit
}

func NewBridge(left, right *Circuit) (*Bridge, error) {
	if left == nil || right == nil || left.State() != CircuitConnected || right.State() != CircuitConnected {
		return nil, fmt.Errorf("NET/ROM bridge requires two connected circuits")
	}
	return &Bridge{left: left, right: right}, nil
}

func (b *Bridge) Handle(side BridgeSide, packet Packet) (BridgeEvent, error) {
	if b == nil {
		return BridgeEvent{}, fmt.Errorf("nil NET/ROM bridge")
	}
	from, to := b.left, b.right
	if side == BridgeRight {
		from, to = b.right, b.left
	} else if side != BridgeLeft {
		return BridgeEvent{}, fmt.Errorf("invalid NET/ROM bridge side %d", side)
	}
	event, err := from.Handle(packet)
	if err != nil {
		return BridgeEvent{}, err
	}
	out := BridgeEvent{SameSide: event.Packets, Data: event.Data, Closed: event.Closed}
	if len(event.Data) > 0 {
		out.OtherSide, err = to.Send(event.Data)
		if err != nil {
			return BridgeEvent{}, err
		}
	}
	return out, nil
}
