package netrom

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestCircuitConnectNegotiatesWindowAndExchangesData(t *testing.T) {
	user, _ := ax25.ParseAddress("SP5ABC-1")
	node, _ := ax25.ParseAddress("SP5ND-7")
	caller, err := NewCircuit(2, 9, 4)
	if err != nil {
		t.Fatal(err)
	}
	request, err := caller.Open(ConnectRequest{OriginUser: user, OriginNode: node, Window: 3})
	if err != nil {
		t.Fatal(err)
	}
	callee, err := NewCircuit(7, 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := callee.Accept(request, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ack.CircuitIndex != request.CircuitIndex || ack.CircuitID != request.CircuitID || ack.TXSequence != 7 || ack.RXSequence != 12 || ack.Payload[0] != 2 {
		t.Fatalf("ack=%+v", ack)
	}
	event, err := caller.Handle(ack)
	if err != nil || !event.Connected || caller.State() != CircuitConnected {
		t.Fatalf("connect event=%+v err=%v state=%v", event, err, caller.State())
	}
	packets, err := caller.Send([]byte("hello"))
	if err != nil || len(packets) != 1 {
		t.Fatalf("send packets=%+v err=%v", packets, err)
	}
	event, err = callee.Handle(packets[0])
	if err != nil || string(event.Data) != "hello" || len(event.Packets) != 1 || event.Packets[0].Opcode != OpcodeInformationAcknowledge {
		t.Fatalf("data event=%+v err=%v", event, err)
	}
	if _, err := caller.Handle(event.Packets[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := caller.Send([]byte("again")); err != nil {
		t.Fatal("ack did not free transmit window:", err)
	}
}

func TestCircuitReassemblesFragmentsAndSignalsGap(t *testing.T) {
	sender, _ := NewCircuit(1, 1, 4)
	receiver, _ := NewCircuit(2, 2, 4)
	user, _ := ax25.ParseAddress("N0CALL")
	node, _ := ax25.ParseAddress("N0NODE")
	request, _ := sender.Open(ConnectRequest{OriginUser: user, OriginNode: node, Window: 4})
	ack, _ := receiver.Accept(request, 4)
	_, _ = sender.Handle(ack)
	packets, err := sender.Send(make([]byte, MaxInformationPayload+3))
	if err != nil || len(packets) != 2 {
		t.Fatalf("fragments=%d err=%v", len(packets), err)
	}
	event, err := receiver.Handle(packets[1])
	if err != nil || !event.NAK || len(event.Packets) != 1 || !event.Packets[0].NAK {
		t.Fatalf("gap event=%+v err=%v", event, err)
	}
	event, err = receiver.Handle(packets[0])
	if err != nil || len(event.Data) != 0 {
		t.Fatalf("first fragment event=%+v err=%v", event, err)
	}
	event, err = receiver.Handle(packets[1])
	if err != nil || len(event.Data) != MaxInformationPayload+3 || event.Data[MaxInformationPayload] != 0 {
		t.Fatalf("second fragment event=%+v err=%v", event, err)
	}
}

func TestCircuitRejectsInvalidWindows(t *testing.T) {
	if _, err := NewCircuit(1, 1, 0); err == nil {
		t.Fatal("zero window accepted")
	}
	if _, err := NewCircuit(1, 1, 128); err == nil {
		t.Fatal("window above modulo half accepted")
	}
}

func TestCircuitIgnoresAcknowledgementOutsideTransmitWindow(t *testing.T) {
	c, _ := NewCircuit(1, 1, 2)
	user, _ := ax25.ParseAddress("N0CALL")
	node, _ := ax25.ParseAddress("N0NODE")
	request, _ := c.Open(ConnectRequest{OriginUser: user, OriginNode: node, Window: 2})
	peer, _ := NewCircuit(2, 2, 2)
	ack, _ := peer.Accept(request, 2)
	_, _ = c.Handle(ack)
	if _, err := c.Send([]byte("x")); err != nil {
		t.Fatal(err)
	}
	_, err := c.Handle(Packet{CircuitIndex: 1, CircuitID: 1, RXSequence: 99, Opcode: OpcodeInformationAcknowledge})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send([]byte("y")); err != nil {
		t.Fatal("invalid ACK changed transmit state:", err)
	}
}

func TestBridgeAcknowledgesAndForwardsCompleteData(t *testing.T) {
	leftSender, _ := NewCircuit(1, 1, 2)
	rightSender, _ := NewCircuit(2, 2, 2)
	user, _ := ax25.ParseAddress("N0CALL")
	node, _ := ax25.ParseAddress("N0NODE")
	leftRequest, _ := leftSender.Open(ConnectRequest{OriginUser: user, OriginNode: node, Window: 2})
	left, _ := NewCircuit(3, 3, 2)
	leftAck, _ := left.Accept(leftRequest, 2)
	_, _ = leftSender.Handle(leftAck)
	rightRequest, _ := rightSender.Open(ConnectRequest{OriginUser: user, OriginNode: node, Window: 2})
	right, _ := NewCircuit(4, 4, 2)
	rightAck, _ := right.Accept(rightRequest, 2)
	_, _ = rightSender.Handle(rightAck)
	bridge, err := NewBridge(left, right)
	if err != nil {
		t.Fatal(err)
	}
	packets, _ := leftSender.Send([]byte("through"))
	event, err := bridge.Handle(BridgeLeft, packets[0])
	if err != nil || len(event.SameSide) != 1 || len(event.OtherSide) != 1 {
		t.Fatalf("bridge event=%+v err=%v", event, err)
	}
	remoteEvent, err := rightSender.Handle(event.OtherSide[0])
	if err != nil || string(remoteEvent.Data) != "through" {
		t.Fatalf("remote event=%+v err=%v", remoteEvent, err)
	}
}
