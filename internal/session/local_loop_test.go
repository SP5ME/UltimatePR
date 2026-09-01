package session

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/service"
	"github.com/packet-radio/ultimatepr/internal/transport"
	"github.com/packet-radio/ultimatepr/internal/transport/loopback"
)

func TestLocalLoopUsesConnectedAX25PathForGenericService(t *testing.T) {
	wire := make(chan transport.Packet, 128)
	loop := loopback.New(wire)
	var physicalCalls atomic.Int32
	physicalSend := func(context.Context, []byte) error {
		physicalCalls.Add(1)
		return nil
	}

	local := ax25.Address{Callsign: "LOCAL"}
	remote := ax25.Address{Callsign: "TEST", SSID: 10}
	registry := service.NewRegistry()
	serviceStarted := make(chan service.ServiceContext, 1)
	if err := registry.Register(service.ServiceRegistration{
		Service: service.Func{ServiceID: "echo", Handler: func(ctx service.ServiceContext) error {
			serviceStarted <- ctx
			buf := make([]byte, 5)
			if _, err := io.ReadFull(ctx.Reader, buf); err != nil {
				return err
			}
			if _, err := ctx.Writer.Write(buf); err != nil {
				return err
			}
			<-ctx.Context.Done()
			return nil
		}}, Callsign: remote, Enabled: true, NodeVisible: true,
	}); err != nil {
		t.Fatal(err)
	}

	localSend := LocalSender(func(portID string) Sender {
		return func(ctx context.Context, data []byte) error {
			return loop.Send(ctx, transport.Packet{PortID: portID, Data: append([]byte(nil), data...)})
		}
	})
	localPortSend := localSend("radio-2m")
	inbound := NewInboundMux(map[string]Sender{"radio-2m": localPortSend}, nil)
	inbound.SetRegistry(registry)
	hub := NewHub(local, map[string]Sender{"radio-2m": physicalSend})
	hub.SetLocalDelivery(func(address ax25.Address) bool { _, ok := registry.ByCallsign(address.String()); return ok }, localSend)
	manager, release := hub.NewSession()
	defer release()
	manager.Configure(200*time.Millisecond, 2, 64)
	events, unsubscribe := manager.Subscribe()
	defer unsubscribe()

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case pkt := <-wire:
				frame, err := ax25.Decode(pkt.Data)
				if err != nil {
					t.Errorf("loopback frame decode: %v", err)
					return
				}
				if manager.Handle(pkt.PortID, frame) {
					continue
				}
				inbound.Handle(pkt.PortID, frame)
			case <-stop:
				return
			}
		}
	}()

	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := manager.Connect(connectCtx, "radio-2m", remote.String()); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	select {
	case ctx := <-serviceStarted:
		if got := physicalCalls.Load(); got != 0 {
			t.Fatalf("physical sender calls=%d", got)
		}
		if ctx.PortID != "radio-2m" || ctx.EntryType != service.EntryAX25 || ctx.RemoteCall.String() != local.String() || ctx.LocalCall.String() != remote.String() {
			t.Fatalf("service context=%+v", ctx)
		}
	case <-time.After(time.Second):
		t.Fatal("service did not start through local AX.25")
	}

	if err := manager.Send(context.Background(), []byte("HELLO")); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		for event.Type != "data" {
			select {
			case event = <-events:
			case <-time.After(time.Second):
				t.Fatal("echo response not received")
			}
		}
		if string(event.Data) != "HELLO" {
			t.Fatalf("echo=%q", event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("echo response not received")
	}

	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer disconnectCancel()
	if err := manager.Disconnect(disconnectCtx); err != nil {
		t.Fatal(err)
	}
}

func TestLocalDeliveryDoesNotInterceptRemoteDestination(t *testing.T) {
	var physicalCalls atomic.Int32
	physicalSend := func(context.Context, []byte) error {
		physicalCalls.Add(1)
		return nil
	}
	hub := NewHub(ax25.Address{Callsign: "LOCAL"}, map[string]Sender{"radio-2m": physicalSend})
	hub.SetLocalDelivery(func(address ax25.Address) bool {
		return address.String() == "TEST-10"
	}, func(string) Sender {
		return func(context.Context, []byte) error {
			t.Fatal("local sender intercepted remote destination")
			return nil
		}
	})
	manager, release := hub.NewSession()
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := manager.Connect(ctx, "radio-2m", "REMOTE"); err == nil {
		t.Fatal("remote connection unexpectedly succeeded without a peer")
	}
	if got := physicalCalls.Load(); got == 0 {
		t.Fatal("physical sender was not used for remote destination")
	}
}
