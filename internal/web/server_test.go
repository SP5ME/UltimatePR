package web

import (
	"strings"
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

func TestTerminalTemplateExpansion(t *testing.T) {
	cfg := Config{TerminalCallsign: "SP5ME", TerminalSSID: 0, OperatorName: "Jan", ApplicationLocator: "KO02JD", ApplicationQTH: "Warszawa"}
	got := terminalReplyText("Call: {CALL}\r\nImię: {NAME}\r\nLOC: {LOC}\r\nQTH: {QTH}\r\n", terminalMacroContext(callsign(cfg.TerminalCallsign, cfg.TerminalSSID), "SQ9ABC", cfg))
	want := "Call: SP5ME\r\nImię: Jan\r\nLOC: KO02JD\r\nQTH: Warszawa\r\n"
	if got != want {
		t.Fatalf("expanded template = %q, want %q", got, want)
	}
	blank := Config{TerminalCallsign: "SP5ME", TerminalSSID: 0, ApplicationQTH: "Warszawa"}
	cleaned := terminalReplyText("Imię: {NAME}\r\nQTH: {QTH}\r\n", terminalMacroContext(callsign(blank.TerminalCallsign, blank.TerminalSSID), "SQ9ABC", blank))
	if strings.Contains(cleaned, "Imię:") || !strings.Contains(cleaned, "QTH: Warszawa") {
		t.Fatalf("blank macro handling failed: %q", cleaned)
	}
}
