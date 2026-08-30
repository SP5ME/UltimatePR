package config

import (
	"strings"
)

const (
	ModeStation = "station"
	ModeFull    = "station-node-bbs"
)

// New creates a clean configuration without any operator-specific values.
func New(mode, callsign, locator, qth, language string, stationSSID, nodeSSID, bbsSSID uint8) Config {
	call := strings.ToUpper(strings.TrimSpace(callsign))
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang != "en" {
		lang = "pl"
	}
	full := mode == ModeFull
	c := Config{
		Application:  Application{Mode: mode, OperatorName: "", Locator: strings.ToUpper(strings.TrimSpace(locator)), QTH: strings.TrimSpace(qth), Language: lang, UpdateChannel: "main", TerminalEOL: "cr", AX25T1Seconds: 10, AX25T3Seconds: 300, AX25N2: 10, AX25N1: 256},
		Server:       Station{Callsign: call, SSID: nodeSSID},
		Terminal:     Station{Callsign: call, SSID: stationSSID},
		Web:          Web{Listen: "0.0.0.0:8080", Username: "admin", AllowedAddresses: []string{"0.0.0.0", "::"}},
		Beacon:       Beacon{Enabled: false, Destination: "BEACON", Text: "{LOC}", IntervalMinutes: 30},
		UPRD:         UPRD{Enabled: true, IntervalSeconds: 600, MHeardLimit: 5},
		Experimental: Experimental{UPRD: true, Map: true, Services: full},
		History:      History{Enabled: true, Database: "/var/lib/ultimatepr/history.json", MaxStations: 200, MaxSessionsPerStation: 50, MaxLinesPerStation: 2000, MaxBytes: 10485760, RetentionDays: 90},
		BBS: BBS{Enabled: full, Listen: "127.0.0.1:8023", ForwardListen: "127.0.0.1:8024", Database: "/var/lib/ultimatepr/bbs.json", Title: call + " BBS", Callsign: call, SSID: bbsSSID, Language: lang,
			Forwarding: BBSForwarding{Enabled: false, IntervalMinutes: 15, ConnectTimeoutSeconds: 15, SessionTimeoutSeconds: 120, MaxMessages: 50, MaxBodyBytes: 131072}},
		Node: Node{Enabled: full, Alias: shortAlias(call), Listen: "127.0.0.1:8010", Language: lang},
		AI:   AI{Enabled: false, Callsign: call, SSID: 12, Provider: "ollama", URL: "http://192.168.1.50:11434", Model: "qwen3:8b", TimeoutSeconds: 120, MaxResponseChars: 2000, MaxContext: 20, QueueSize: 8, Concurrency: 1},
	}
	c.applyTerminalMessageDefaults()
	if full {
		c.Node.Services = []NodeService{{Name: "BBS", Callsign: stationText(call, bbsSSID), Command: "BBS", Enabled: true}}
	}
	return c
}

func shortAlias(call string) string {
	if len(call) > 6 {
		return call[:6]
	}
	return call
}

func stationText(call string, ssid uint8) string {
	if ssid == 0 {
		return call
	}
	const digits = "0123456789"
	if ssid < 10 {
		return call + "-" + string(digits[ssid])
	}
	return call + "-1" + string(digits[ssid-10])
}

func (c *Config) applyTerminalMessageDefaults() {
	const legacyWelcome = "Witaj {REMOTE}, de {CALL}."
	const welcome = legacyWelcome + "\r\nDostepne komendy: /I, /MH, /H lub /?."
	if current := strings.TrimSpace(c.Application.WelcomeMessage); current == "" || current == legacyWelcome {
		c.Application.WelcomeMessage = welcome
	}
	if strings.TrimSpace(c.Application.AwayMessage) == "" {
		c.Application.AwayMessage = "Witaj {REMOTE}, de {CALL}.\r\nOperatora nie ma obecnie przy stacji.\r\nDostepne komendy: /I, /MH, /H lub /?."
	}
	if strings.TrimSpace(c.Application.GoodbyeMessage) == "" {
		c.Application.GoodbyeMessage = "73 {REMOTE}, de {CALL}."
	}
	if strings.TrimSpace(c.Application.InfoMessage) == "" {
		lines := []string{"Call: " + stationText(c.Terminal.Callsign, c.Terminal.SSID), "Imię: {NAME}"}
		if loc := strings.ToUpper(strings.TrimSpace(c.Application.Locator)); loc != "" {
			lines = append(lines, "LOC: "+loc)
		}
		if qth := strings.TrimSpace(c.Application.QTH); qth != "" {
			lines = append(lines, "QTH: "+qth)
		}
		lines = append(lines, "73")
		c.Application.InfoMessage = strings.Join(lines, "\r\n")
	}
}
