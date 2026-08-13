package web

import (
	"testing"

	"github.com/packet-radio/ultimatepr/internal/session"
)

func TestTerminalMessageFromEventUsesDataForPayload(t *testing.T) {
	msg := terminalMessageFromEvent(session.Event{Type: "data", Data: []byte("Hello\r\n")})
	if msg.Type != "data" {
		t.Fatalf("Type = %q, want %q", msg.Type, "data")
	}
	if msg.Data != "Hello\r\n" {
		t.Fatalf("Data = %q, want %q", msg.Data, "Hello\r\n")
	}
	if msg.Error != "" {
		t.Fatalf("Error = %q, want empty", msg.Error)
	}
}

func TestTerminalMessageFromEventFallsBackToMessage(t *testing.T) {
	msg := terminalMessageFromEvent(session.Event{Type: "state", State: session.Disconnected, Message: "Rozlaczono."})
	if msg.Type != "state" {
		t.Fatalf("Type = %q, want %q", msg.Type, "state")
	}
	if msg.State != string(session.Disconnected) {
		t.Fatalf("State = %q, want %q", msg.State, session.Disconnected)
	}
	if msg.Data != "Rozlaczono." {
		t.Fatalf("Data = %q, want %q", msg.Data, "Rozlaczono.")
	}
}
