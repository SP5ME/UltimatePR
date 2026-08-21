package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/packet-radio/ultimatepr/internal/language"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/terminalcodec"
)

type terminalLineBuffer struct {
	pending string
}

func (b *terminalLineBuffer) Push(text string) []string {
	if text == "" {
		return nil
	}
	b.pending += text
	lines := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(b.pending); i++ {
		switch b.pending[i] {
		case '\r', '\n':
			if i > start {
				lines = append(lines, strings.TrimSpace(b.pending[start:i]))
			}
			for i+1 < len(b.pending) && (b.pending[i+1] == '\r' || b.pending[i+1] == '\n') {
				i++
			}
			start = i + 1
		}
	}
	if start > 0 {
		b.pending = b.pending[start:]
	}
	return lines
}

func terminalResponseText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", "\r\n")
	return text + "\r\n"
}

func terminalMacroContext(localCall, remoteCall string, cfg Config) map[string]string {
	return map[string]string{
		"CALL":   strings.ToUpper(strings.TrimSpace(localCall)),
		"NAME":   strings.TrimSpace(cfg.OperatorName),
		"LOC":    strings.ToUpper(strings.TrimSpace(cfg.ApplicationLocator)),
		"QTH":    strings.TrimSpace(cfg.ApplicationQTH),
		"REMOTE": strings.ToUpper(strings.TrimSpace(remoteCall)),
	}
}

func expandTerminalTemplate(text string, values map[string]string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{"+key+"}", value)
	}
	text = strings.NewReplacer(replacements...).Replace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.Index(line, ":"); i >= 0 && strings.TrimSpace(line[i+1:]) == "" {
			continue
		}
		out = append(out, strings.Join(strings.Fields(line), " "))
	}
	return strings.Join(out, "\r\n")
}

func terminalReplyText(template string, values map[string]string) string {
	return terminalResponseText(expandTerminalTemplate(template, values))
}

// expandTerminalMessage expands configured station macros in operator-entered
// text immediately before it is sent. Messages without known macros are kept
// byte-for-byte unchanged.
func expandTerminalMessage(text string, values map[string]string) string {
	for key := range values {
		if strings.Contains(text, "{"+key+"}") {
			return terminalReplyText(text, values)
		}
	}
	return text
}

func prepareTerminalMessage(text string, values map[string]string) string {
	return language.ASCII(expandTerminalMessage(text, values))
}

func formatMHeardResponse(entries []mheard.Entry) string {
	if len(entries) == 0 {
		return "Brak odebranych stacji.\r\n"
	}
	if len(entries) > 10 {
		entries = entries[:10]
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "Ostatnie 10 stacji MHEARD:")
	for i, entry := range entries {
		seen := entry.LastSeen.Local().Format("2006-01-02 15:04:05")
		line := fmt.Sprintf("%d. %s", i+1, strings.ToUpper(strings.TrimSpace(entry.Callsign)))
		if port := strings.TrimSpace(entry.Port); port != "" {
			line += " [" + port + "]"
		}
		if entry.Indirect && strings.TrimSpace(entry.Via) != "" {
			line += " via " + strings.TrimSpace(entry.Via)
		}
		line += " " + seen
		lines = append(lines, line)
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func writeOperatorReply(ws *safeWS, in *operatorSession, codec *terminalcodec.Codec, text string) {
	text = terminalResponseText(text)
	if text == "" || ws == nil || in == nil || codec == nil {
		return
	}
	id := fmt.Sprintf("auto-%d", time.Now().UnixNano())
	wireData, encodeErr := codec.Encode(text)
	if encodeErr != nil {
		_ = ws.write(serverMessage{Type: "error", Error: encodeErr.Error()})
		return
	}
	_ = ws.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "sending"})
	in.mu.Lock()
	if _, err := in.w.Write(wireData); err != nil {
		in.mu.Unlock()
		_ = ws.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "error", Error: err.Error()})
		_ = ws.write(serverMessage{Type: "error", Error: err.Error()})
		return
	}
	in.mu.Unlock()
	_ = ws.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "sent"})
}
