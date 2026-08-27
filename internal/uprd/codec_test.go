package uprd

import (
	"strings"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestPayloadRoundTripAndBaseCall(t *testing.T) {
	source := ax25.Address{Callsign: "SP5ME", SSID: 7}
	payload, err := EncodePayload(source, "ko02md", []string{"SQ9MDD-1", "SR5DDD", "SP7ABC"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePayload(payload, source)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Locator != "KO02MD" || strings.Join(parsed.MHeard, ",") != "SQ9MDD,SR5DDD,SP7ABC" {
		t.Fatalf("unexpected payload: %#v", parsed)
	}
	if parsed.Shift < 1 || parsed.Shift > 9 {
		t.Fatalf("shift out of range: %d", parsed.Shift)
	}
}

func TestPayloadValidation(t *testing.T) {
	source := ax25.Address{Callsign: "SP5ME"}
	if _, err := EncodePayload(source, "KO02M", nil, 5); err == nil {
		t.Fatal("expected invalid locator")
	}
	valid, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := EncodePayload(source, "KO02MD", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePayload(empty, source); err != nil {
		t.Fatal("empty MHEARD should remain valid: ", err)
	}
	if _, err := ParsePayload("BAD|KO02MD|SQ9MDD|EXTRA", source); err == nil {
		t.Fatal("expected field count error")
	}
	if _, err := ParsePayload(valid, ax25.Address{Callsign: "SP7ABC"}); err == nil {
		t.Fatal("expected callsign mismatch")
	}
}

func TestEncodingWrapsAlphabet(t *testing.T) {
	encoded, err := encodeBaseCall("Z9", 1)
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "0A" {
		t.Fatalf("got %q, want 0A", encoded)
	}
	decoded, err := decodeBaseCall(encoded, 1)
	if err != nil || decoded != "Z9" {
		t.Fatalf("decode got %q, %v", decoded, err)
	}
}
