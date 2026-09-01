// Package ax25core contains the transport-independent AX.25 frame dispatch
// boundary. Connected-mode state machines remain owned by internal/session.
package ax25core

import (
	"fmt"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/transport"
)

type Category uint8

const (
	CategoryUnknown Category = iota
	CategoryConnected
	CategoryUI
)

type FrameContext struct {
	InterfaceID string
	PortID      string
	Channel     uint8
	Frame       ax25.Frame
	Raw         []byte
	ReceivedAt  time.Time
}

type Observer func(FrameContext)
type Handler func(FrameContext) bool

type Dispatcher struct {
	observers []Observer
	pre       []Handler
	connected []Handler
	ui        []Handler
	any       []Handler
}

// RegisterPre adds a handler that runs before category dispatch. Returning
// true consumes the frame, which preserves legacy digipeater short-circuiting.
func (d *Dispatcher) RegisterPre(handler Handler) {
	if handler != nil {
		d.pre = append(d.pre, handler)
	}
}

func New() *Dispatcher { return &Dispatcher{} }

func (d *Dispatcher) AddObserver(observer Observer) {
	if observer != nil {
		d.observers = append(d.observers, observer)
	}
}

func (d *Dispatcher) RegisterConnected(handler Handler) {
	if handler != nil {
		d.connected = append(d.connected, handler)
	}
}

func (d *Dispatcher) RegisterUI(handler Handler) {
	if handler != nil {
		d.ui = append(d.ui, handler)
	}
}

// RegisterAny runs after the category-specific handlers. It is intended for
// cross-cutting protocol actions such as digipeater decisions.
func (d *Dispatcher) RegisterAny(handler Handler) {
	if handler != nil {
		d.any = append(d.any, handler)
	}
}

// Dispatch decodes one transport packet and routes the resulting AX.25 frame.
// Observers are asynchronous by design: diagnostics must not hold up protocol
// handling. Handler execution remains ordered and synchronous.
func (d *Dispatcher) Dispatch(pkt transport.Packet) (bool, error) {
	frame, err := ax25.Decode(pkt.Data)
	if err != nil {
		return false, fmt.Errorf("decode AX.25 frame: %w", err)
	}
	ctx := FrameContext{
		InterfaceID: pkt.InterfaceID,
		PortID:      pkt.PortID,
		Channel:     pkt.Channel,
		Frame:       frame,
		Raw:         append([]byte(nil), pkt.Data...),
		ReceivedAt:  time.Now().UTC(),
	}
	for _, observer := range d.observers {
		go observer(ctx)
	}
	for _, handler := range d.pre {
		if handler(ctx) {
			return true, nil
		}
	}

	handled := false
	for _, handler := range handlersFor(category(frame), d) {
		if handler(ctx) {
			handled = true
			break
		}
	}
	for _, handler := range d.any {
		if handler(ctx) {
			handled = true
		}
	}
	return handled, nil
}

func category(frame ax25.Frame) Category {
	if frame.Type == ax25.TypeUI {
		return CategoryUI
	}
	// All other currently decoded AX.25 frame types are left for the existing
	// connected-mode handlers, which decide whether a frame belongs to a link.
	return CategoryConnected
}

func handlersFor(category Category, d *Dispatcher) []Handler {
	switch category {
	case CategoryUI:
		return d.ui
	case CategoryConnected:
		return d.connected
	default:
		return nil
	}
}
