package ax25core

import (
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/transport"
)

func encoded(t *testing.T, frame ax25.Frame) []byte {
	t.Helper()
	b, err := ax25.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func uiFrame(local, remote ax25.Address) ax25.Frame {
	pid := byte(0xF0)
	return ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeUI, PID: &pid}
}

func TestDispatchSeparatesUIAndConnectedFrames(t *testing.T) {
	d := New()
	connected, ui := make(chan FrameContext, 1), make(chan FrameContext, 1)
	d.RegisterConnected(func(ctx FrameContext) bool { connected <- ctx; return true })
	d.RegisterUI(func(ctx FrameContext) bool { ui <- ctx; return true })

	local := ax25.Address{Callsign: "LOCAL"}
	remote := ax25.Address{Callsign: "REMOTE"}
	if handled, err := d.Dispatch(transport.Packet{PortID: "vhf", Data: encoded(t, ax25.Frame{Destination: local, Source: remote, Type: ax25.TypeSABM})}); err != nil || !handled {
		t.Fatalf("connected dispatch handled=%v err=%v", handled, err)
	}
	if handled, err := d.Dispatch(transport.Packet{PortID: "uhf", Channel: 1, Data: encoded(t, uiFrame(local, remote))}); err != nil || !handled {
		t.Fatalf("UI dispatch handled=%v err=%v", handled, err)
	}
	select {
	case ctx := <-connected:
		if ctx.PortID != "vhf" || ctx.Frame.Type != ax25.TypeSABM {
			t.Fatalf("connected context=%+v", ctx)
		}
	case <-time.After(time.Second):
		t.Fatal("connected handler was not called")
	}
	select {
	case ctx := <-ui:
		if ctx.PortID != "uhf" || ctx.Channel != 1 || ctx.Frame.Type != ax25.TypeUI {
			t.Fatalf("UI context=%+v", ctx)
		}
	case <-time.After(time.Second):
		t.Fatal("UI handler was not called")
	}
}

func TestObserverDoesNotBlockDispatch(t *testing.T) {
	d := New()
	d.AddObserver(func(FrameContext) { <-make(chan struct{}) })
	d.RegisterUI(func(FrameContext) bool { return true })
	done := make(chan error, 1)
	go func() {
		_, err := d.Dispatch(transport.Packet{PortID: "vhf", Data: encoded(t, uiFrame(ax25.Address{Callsign: "LOCAL"}, ax25.Address{Callsign: "REMOTE"}))})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("observer blocked dispatch")
	}
}

func TestUnsupportedOrInvalidFrameIsSafe(t *testing.T) {
	d := New()
	called := false
	d.RegisterConnected(func(FrameContext) bool { called = true; return true })
	if handled, err := d.Dispatch(transport.Packet{PortID: "vhf", Data: []byte{0x01}}); err == nil || handled {
		t.Fatalf("invalid frame handled=%v err=%v", handled, err)
	}
	if called {
		t.Fatal("invalid frame reached a handler")
	}

	// A valid frame category without a registered handler is safely ignored.
	if handled, err := New().Dispatch(transport.Packet{PortID: "vhf", Data: encoded(t, uiFrame(ax25.Address{Callsign: "LOCAL"}, ax25.Address{Callsign: "REMOTE"}))}); err != nil || handled {
		t.Fatalf("unregistered frame handled=%v err=%v", handled, err)
	}
}
