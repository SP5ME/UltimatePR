package uprd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Payload struct {
	EncodedCall     string
	Status          byte
	OperatorPresent bool
	Locator         string
	MHeard          []string
	Shift           int
}

func BaseCall(call string) string {
	call = strings.ToUpper(strings.TrimSpace(call))
	if i := strings.IndexByte(call, '-'); i >= 0 {
		call = call[:i]
	}
	return call
}

func NormalizeLocator(locator string) string {
	return strings.ToUpper(strings.TrimSpace(locator))
}

func ValidLocator(locator string) bool {
	locator = NormalizeLocator(locator)
	if len(locator) != 6 {
		return false
	}
	for i, r := range locator {
		switch {
		case i < 2:
			if r < 'A' || r > 'Z' {
				return false
			}
		case i < 4:
			if r < '0' || r > '9' {
				return false
			}
		default:
			if r < 'A' || r > 'Z' {
				return false
			}
		}
	}
	return true
}

func sanitizeMHeard(calls []string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}
	out := make([]string, 0, min(limit, len(calls)))
	seen := make(map[string]struct{}, len(calls))
	for _, raw := range calls {
		call := BaseCall(raw)
		if call == "" {
			continue
		}
		if _, ok := seen[call]; ok {
			continue
		}
		if len(call) > 6 {
			continue
		}
		if _, err := ax25.ParseAddress(call); err != nil {
			continue
		}
		seen[call] = struct{}{}
		out = append(out, call)
		if len(out) == limit {
			break
		}
	}
	return out
}

func shiftFor(locator string, heard []string) int {
	text := NormalizeLocator(locator) + "|" + strings.Join(heard, ",")
	sum := 0
	for i := 0; i < len(text); i++ {
		sum += int(text[i])
	}
	return (sum % 9) + 1
}

func encodeBaseCall(base string, shift int) (string, error) {
	base = BaseCall(base)
	if base == "" {
		return "", fmt.Errorf("empty callsign")
	}
	if shift < 1 || shift > 9 {
		return "", fmt.Errorf("invalid shift")
	}
	var out strings.Builder
	out.Grow(len(base))
	for _, r := range base {
		idx := strings.IndexRune(alphabet, r)
		if idx < 0 {
			return "", fmt.Errorf("invalid callsign")
		}
		out.WriteByte(alphabet[(idx+shift)%len(alphabet)])
	}
	return out.String(), nil
}

func decodeBaseCall(encoded string, shift int) (string, error) {
	encoded = BaseCall(encoded)
	if encoded == "" {
		return "", fmt.Errorf("empty callsign")
	}
	if shift < 1 || shift > 9 {
		return "", fmt.Errorf("invalid shift")
	}
	var out strings.Builder
	out.Grow(len(encoded))
	for _, r := range encoded {
		idx := strings.IndexRune(alphabet, r)
		if idx < 0 {
			return "", fmt.Errorf("invalid callsign")
		}
		pos := idx - shift
		if pos < 0 {
			pos += len(alphabet)
		}
		out.WriteByte(alphabet[pos])
	}
	return out.String(), nil
}

func EncodePayload(source ax25.Address, locator string, heard []string, limit int, operatorPresent bool) ([]byte, error) {
	locator = NormalizeLocator(locator)
	if locator != "" && !ValidLocator(locator) {
		return nil, fmt.Errorf("invalid locator")
	}
	list := sanitizeMHeard(heard, limit)
	plainHeard := strings.Join(list, ",")
	shift := shiftFor(locator, list)
	encCall, err := encodeBaseCall(source.Callsign, shift)
	if err != nil {
		return nil, err
	}
	status := byte(0)
	if !operatorPresent {
		status = 0x01
	}
	payload := make([]byte, 0, len(encCall)+len(locator)+len(plainHeard)+4)
	payload = append(payload, encCall...)
	payload = append(payload, '|', status, '|')
	payload = append(payload, locator...)
	payload = append(payload, '|')
	payload = append(payload, plainHeard...)
	return payload, nil
}

func ParsePayload(payload []byte, source ax25.Address) (Payload, error) {
	firstSep := bytes.IndexByte(payload, '|')
	if firstSep < 1 || firstSep+2 >= len(payload) || payload[firstSep+2] != '|' {
		return Payload{}, fmt.Errorf("invalid UPRD payload")
	}
	encCall := BaseCall(string(payload[:firstSep]))
	status := payload[firstSep+1]
	parts := strings.Split(string(payload[firstSep+3:]), "|")
	if len(parts) != 2 {
		return Payload{}, fmt.Errorf("invalid UPRD payload")
	}
	locator := NormalizeLocator(parts[0])
	if locator != "" && !ValidLocator(locator) {
		return Payload{}, fmt.Errorf("invalid locator")
	}
	heard := sanitizeMHeard(strings.Split(parts[1], ","), 10)
	shift := shiftFor(locator, heard)
	decoded, err := decodeBaseCall(encCall, shift)
	if err != nil {
		return Payload{}, err
	}
	if decoded != BaseCall(source.String()) {
		return Payload{}, fmt.Errorf("callsign mismatch")
	}
	return Payload{
		EncodedCall:     encCall,
		Status:          status,
		OperatorPresent: status&0x01 == 0,
		Locator:         locator,
		MHeard:          heard,
		Shift:           shift,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
