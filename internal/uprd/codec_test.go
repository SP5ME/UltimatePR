package uprd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

func TestPayloadRoundTripAndBaseCall(t *testing.T) {
	source := ax25.Address{Callsign: "SP5ME", SSID: 7}
	payload, err := EncodePayload(source, "ko02md", []string{"SQ9MDD-1", "SR5DDD", "SP7ABC"}, 5, true)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePayload(payload, source)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Locator != "KO02MD" || strings.Join(parsed.MHeard, ",") != "SQ9MDD,SR5DDD,SP7ABC" || parsed.Status != 0x00 || !parsed.OperatorPresent {
		t.Fatalf("unexpected payload: %#v", parsed)
	}
	if parsed.Shift < 1 || parsed.Shift > 9 {
		t.Fatalf("shift out of range: %d", parsed.Shift)
	}
}

func TestPayloadValidation(t *testing.T) {
	source := ax25.Address{Callsign: "SP5ME"}
	if _, err := EncodePayload(source, "KO02M", nil, 5, true); err == nil {
		t.Fatal("expected invalid locator")
	}
	valid, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD"}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := EncodePayload(source, "KO02MD", nil, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePayload(empty, source); err != nil {
		t.Fatal("empty MHEARD should remain valid: ", err)
	}
	if _, err := ParsePayload([]byte("BAD|\x00|KO02MD|SQ9MDD|EXTRA"), source); err == nil {
		t.Fatal("expected field count error")
	}
	if _, err := ParsePayload(valid, ax25.Address{Callsign: "SP7ABC"}); err == nil {
		t.Fatal("expected callsign mismatch")
	}
}

func TestStatusIsBinaryAndBitZeroIsParsed(t *testing.T) {
	source := ax25.Address{Callsign: "SP5ME"}
	presentPayload, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD", "SR5DDD"}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	absentPayload, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD", "SR5DDD"}, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(presentPayload[:bytes.IndexByte(presentPayload, '|')], absentPayload[:bytes.IndexByte(absentPayload, '|')]) {
		t.Fatalf("encoded callsign changed with status: % X vs % X", presentPayload, absentPayload)
	}
	for _, tc := range []struct {
		status  byte
		present bool
	}{{0x00, true}, {0x01, false}} {
		payload, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD", "SR5DDD"}, 10, tc.present)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(payload, []byte{'|', tc.status, '|'}) {
			t.Fatalf("payload=% X, status=%02X", payload, tc.status)
		}
		parsed, err := ParsePayload(payload, source)
		if err != nil || parsed.OperatorPresent != tc.present {
			t.Fatalf("status=%02X parsed=%+v err=%v", tc.status, parsed, err)
		}
		if len(parsed.MHeard) != 2 {
			t.Fatalf("heard list truncated: %#v", parsed.MHeard)
		}
	}

	future, err := EncodePayload(source, "KO02MD", []string{"SQ9MDD"}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	future[bytes.IndexByte(future, '|')+1] = 0x05
	parsed, err := ParsePayload(future, source)
	if err != nil || parsed.OperatorPresent {
		t.Fatalf("future status parsed=%+v err=%v", parsed, err)
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
