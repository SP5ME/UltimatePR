// Package netrom contains the interoperable NET/ROM network-layer formats.
package netrom

import (
	"fmt"
	"strings"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

const (
	PID                    byte = 0xCF
	RoutingMarker          byte = 0xFF
	MnemonicSize                = 6
	DestinationWireSize         = 21 // AX.25 address + mnemonic + AX.25 address + quality
	MaxRoutingDestinations      = 11
)

type Destination struct {
	Callsign ax25.Address
	Mnemonic string
	Neighbor ax25.Address
	Quality  uint8
}

type RoutingBroadcast struct {
	Sender       string
	Destinations []Destination
}

func (b RoutingBroadcast) Encode() ([]byte, error) {
	sender, err := mnemonic(b.Sender)
	if err != nil {
		return nil, err
	}
	if len(b.Destinations) == 0 {
		return nil, fmt.Errorf("NET/ROM broadcast has no destinations")
	}
	if len(b.Destinations) > MaxRoutingDestinations {
		return nil, fmt.Errorf("NET/ROM broadcast has too many destinations")
	}
	out := []byte{RoutingMarker}
	out = append(out, sender...)
	for _, d := range b.Destinations {
		if err := d.Callsign.Validate(); err != nil {
			return nil, fmt.Errorf("destination callsign: %w", err)
		}
		if err := d.Neighbor.Validate(); err != nil {
			return nil, fmt.Errorf("neighbor callsign: %w", err)
		}
		call, err := packAddress(d.Callsign)
		if err != nil {
			return nil, err
		}
		via, err := packAddress(d.Neighbor)
		if err != nil {
			return nil, err
		}
		out = append(out, call...)
		name, err := mnemonic(d.Mnemonic)
		if err != nil {
			return nil, err
		}
		out = append(out, name...)
		out = append(out, via...)
		out = append(out, d.Quality)
	}
	return out, nil
}

func DecodeRouting(data []byte) (RoutingBroadcast, error) {
	if len(data) < 1+MnemonicSize+DestinationWireSize || data[0] != RoutingMarker {
		return RoutingBroadcast{}, fmt.Errorf("not a NET/ROM routing broadcast")
	}
	if (len(data)-1-MnemonicSize)%DestinationWireSize != 0 {
		return RoutingBroadcast{}, fmt.Errorf("invalid NET/ROM routing broadcast length")
	}
	sender := strings.TrimSpace(string(data[1 : 1+MnemonicSize]))
	if _, err := mnemonic(sender); err != nil {
		return RoutingBroadcast{}, err
	}
	out := RoutingBroadcast{Sender: sender}
	for pos := 1 + MnemonicSize; pos < len(data); pos += DestinationWireSize {
		call, err := unpackAddress(data[pos : pos+7])
		if err != nil {
			return RoutingBroadcast{}, err
		}
		name := strings.TrimSpace(string(data[pos+7 : pos+13]))
		if _, err := mnemonic(name); err != nil {
			return RoutingBroadcast{}, err
		}
		via, err := unpackAddress(data[pos+13 : pos+20])
		if err != nil {
			return RoutingBroadcast{}, err
		}
		out.Destinations = append(out.Destinations, Destination{Callsign: call, Mnemonic: name, Neighbor: via, Quality: data[pos+20]})
	}
	return out, nil
}

func mnemonic(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if len(value) > MnemonicSize {
		return nil, fmt.Errorf("NET/ROM mnemonic must be at most %d characters", MnemonicSize)
	}
	for _, r := range value {
		if r < 0x20 || r > 0x7E {
			return nil, fmt.Errorf("NET/ROM mnemonic contains a non-printable character")
		}
	}
	return []byte(fmt.Sprintf("%-6s", value)), nil
}

func packAddress(a ax25.Address) ([]byte, error) {
	// NET/ROM routing entries use the normal seven-byte AX.25 address format,
	// with reserved command/repeated bits clear and the end bit unset.
	b, err := ax25.Encode(ax25.Frame{Destination: a, Source: ax25.Address{Callsign: "N0CALL"}, Type: ax25.TypeUI, PID: ptr(0)})
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), b[:7]...), nil
}

func unpackAddress(b []byte) (ax25.Address, error) {
	if len(b) != 7 {
		return ax25.Address{}, fmt.Errorf("invalid NET/ROM address length")
	}
	// Decode a minimal frame so the AX.25 package remains the sole address
	// encoding authority.
	template, err := ax25.Encode(ax25.Frame{Destination: ax25.Address{Callsign: "N0CALL"}, Source: ax25.Address{Callsign: "N0CALL"}, Type: ax25.TypeUI, PID: ptr(0)})
	if err != nil {
		return ax25.Address{}, err
	}
	address := append([]byte(nil), b...)
	address[6] &^= 1 // network-header EOA is not an AX.25 address-list EOA
	frame := append(append([]byte(nil), address...), template[7:14]...)
	frame = append(frame, 0x03, 0xF0)
	f, err := ax25.Decode(frame)
	if err != nil {
		return ax25.Address{}, err
	}
	return f.Destination, nil
}

func ptr(v byte) *byte { return &v }
