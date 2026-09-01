package transport

import (
	"bytes"
	"testing"
)

func TestPacketPreservesPortChannelAndData(t *testing.T) {
	data := []byte{0x01, 0x02}
	p := Packet{InterfaceID: "kiss-main", PortID: "uhf-9600", Channel: 1, Data: data}
	if p.InterfaceID != "kiss-main" || p.PortID != "uhf-9600" || p.Channel != 1 || !bytes.Equal(p.Data, data) {
		t.Fatalf("packet context was not preserved: %+v", p)
	}
}

func TestPacketsFromDifferentChannelsRemainDistinct(t *testing.T) {
	packets := []Packet{
		{InterfaceID: "kiss-main", PortID: "vhf-1200", Channel: 0, Data: []byte("vhf")},
		{InterfaceID: "kiss-main", PortID: "uhf-9600", Channel: 1, Data: []byte("uhf")},
	}
	if packets[0].Channel == packets[1].Channel || packets[0].PortID == packets[1].PortID {
		t.Fatalf("channel/port identity collapsed: %+v", packets)
	}
}
