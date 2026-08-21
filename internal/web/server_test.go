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

func TestExpandTerminalMessageAtSendTime(t *testing.T) {
	cfg := Config{TerminalCallsign: "SP5ME", TerminalSSID: 3, OperatorName: "Miki", ApplicationLocator: "KO02MD", ApplicationQTH: "Warszawa"}
	values := terminalMacroContext(callsign(cfg.TerminalCallsign, cfg.TerminalSSID), "SQ9ABC-7", cfg)

	got := expandTerminalMessage("Czesc {REMOTE}, tu {NAME} z {QTH}, {LOC}, de {CALL}.\r\n", values)
	want := "Czesc SQ9ABC-7, tu Miki z Warszawa, KO02MD, de SP5ME-3.\r\n"
	if got != want {
		t.Fatalf("send-time macro expansion = %q, want %q", got, want)
	}

	plain := "Tekst {NIEZNANE} pozostaje bez zmian.\r\n"
	if got := expandTerminalMessage(plain, values); got != plain {
		t.Fatalf("plain message changed: %q", got)
	}
}

func TestPrepareTerminalMessageReplacesPolishCharacters(t *testing.T) {
	values := map[string]string{"NAME": "Mikołaj"}
	got := prepareTerminalMessage("Zażółć gęślą jaźń, {NAME}!\r\n", values)
	want := "Zazolc gesla jazn, Mikolaj!\r\n"
	if got != want {
		t.Fatalf("prepared terminal message = %q, want %q", got, want)
	}
}

func TestTerminalRemoteCommandsRequireSlashAndSeparateLine(t *testing.T) {
	tests := map[string]string{
		"/i":        "info",
		" /MH ":     "mheard",
		"/h":        "help",
		"/?":        "help",
		"I":         "",
		"INFO":      "",
		"MH":        "",
		"tekst /I":  "",
		"/MH teraz": "",
	}
	for input, want := range tests {
		if got := terminalRemoteCommand(input); got != want {
			t.Errorf("terminalRemoteCommand(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTerminalHelpListsRemoteCommands(t *testing.T) {
	help := terminalHelpResponse()
	for _, command := range []string{"/I", "/MH", "/H", "/?"} {
		if !strings.Contains(help, command) {
			t.Errorf("help does not list %s: %q", command, help)
		}
	}
}
