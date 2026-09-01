package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameHallConfigurationAndSSIDZero(t *testing.T) {
	c, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\nexperimental: {services: true}\ngame_hall: {enabled: true, callsign: SP5ME, ssid: 0, language: en, invite_timeout_seconds: 60}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !c.GameHall.Enabled || c.GameHall.SSID != 0 || c.GameHall.Callsign != "SP5ME" || c.GameHall.Language != "en" {
		t.Fatalf("game hall configuration was not preserved: %+v", c.GameHall)
	}
}

func TestGameHallRejectsInvalidTimeout(t *testing.T) {
	_, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\nexperimental: {services: true}\ngame_hall: {enabled: true, callsign: SP5ME, ssid: 14, language: pl, invite_timeout_seconds: 5}\n"))
	if err == nil {
		t.Fatal("invalid invitation timeout was accepted")
	}
}

func TestGameHallSSIDDefaultsOnlyWhenOmitted(t *testing.T) {
	c, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\ngame_hall: {enabled: false}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.GameHall.SSID != 14 {
		t.Fatalf("default SSID=%d", c.GameHall.SSID)
	}
}

func TestWebDefaultsAndPasswordHashIsPrivate(t *testing.T) {
	c, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Web.Listen != "0.0.0.0:8080" || c.Web.Username != "admin" || len(c.Web.AllowedAddresses) != 1 {
		t.Fatalf("unexpected web defaults: %+v", c.Web)
	}
	c.Web.PasswordHash = "secret-hash"
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-hash") || strings.Contains(string(b), "PasswordHash") {
		t.Fatal("password hash exposed in JSON configuration model")
	}
}

func TestWebAllowedAddressesAcceptHostname(t *testing.T) {
	_, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080, allowed_addresses: [operator.local]}\n"))
	if err != nil {
		t.Fatalf("hostname allow rule rejected: %v", err)
	}
}

func TestPortProxyJSONFieldsRoundTrip(t *testing.T) {
	var enabled = false
	want := Config{Ports: []Port{{ID: "p1", Type: "kiss-tcp", Enabled: &enabled, TNCProxyEnabled: true, TNCProxyPort: 8101}}}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Config
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Ports[0].Enabled == nil || *got.Ports[0].Enabled || !got.Ports[0].TNCProxyEnabled || got.Ports[0].TNCProxyPort != 8101 {
		t.Fatalf("unexpected port after JSON round trip: %+v", got.Ports[0])
	}
}

func TestExampleConfiguration(t *testing.T) {
	c, err := Load("../../configs/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Node.Enabled || c.Node.Alias == "" {
		t.Fatal("node not configured")
	}
	if !c.BBS.Enabled || c.BBS.Callsign == "" {
		t.Fatal("BBS not configured")
	}
}

func TestBBSDefaultsAndPeerDirectionCompatibility(t *testing.T) {
	c, err := Parse([]byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
bbs:
  enabled: true
  listen: 127.0.0.1:8023
  database: data/bbs.json
  callsign: SP5ME
  ssid: 8
  language: pl
  forwarding:
    peers:
      - {id: legacy, callsign: SR5DDD, transport: telnet, host: bbs.example, port: 7300, enabled: true, private_routes: ["#PL.POL.EURO"]}
      - {id: receive-only, callsign: SR5EEE, transport: telnet, host: bbs2.example, port: 7300, enabled: true, send: false, receive: true, to_calls: [SP5ME], hierarchical_routes: ["#PL.POL.EURO"]}
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.BBS.SysopCallsign != "SP5ME" || c.BBS.MaxSessions != 10 || c.BBS.Housekeeping.BulletinRetentionDays != 90 || c.BBS.Housekeeping.PersonalRetentionDays != 180 {
		t.Fatalf("BBS defaults = %+v", c.BBS)
	}
	if c.BBS.Forwarding.Peers[0].Send != nil || c.BBS.Forwarding.Peers[0].Receive != nil {
		t.Fatal("legacy peer directions should remain unspecified and default-enabled")
	}
	if c.BBS.Forwarding.Peers[1].Send == nil || *c.BBS.Forwarding.Peers[1].Send || c.BBS.Forwarding.Peers[1].Receive == nil || !*c.BBS.Forwarding.Peers[1].Receive {
		t.Fatalf("peer direction = %+v", c.BBS.Forwarding.Peers[1])
	}
}

func TestBBSRejectsDuplicatePeerAndInvalidRetention(t *testing.T) {
	_, err := Parse([]byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
bbs:
  enabled: true
  listen: 127.0.0.1:8023
  database: data/bbs.json
  callsign: SP5ME
  language: pl
  housekeeping: {bulletin_retention_days: -1, personal_retention_days: 180, log_retention_days: 30}
  forwarding:
    peers:
      - {id: same, callsign: SR5DDD, transport: telnet, host: one.example, port: 7300}
      - {id: same, callsign: SR5EEE, transport: telnet, host: two.example, port: 7300}
`))
	if err == nil {
		t.Fatal("invalid BBS peer/retention configuration accepted")
	}
}

func TestTerminalMessageDefaults(t *testing.T) {
	c, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(c.Application.WelcomeMessage) == "" || strings.TrimSpace(c.Application.AwayMessage) == "" || strings.TrimSpace(c.Application.GoodbyeMessage) == "" || strings.TrimSpace(c.Application.InfoMessage) == "" {
		t.Fatalf("terminal message defaults missing: %+v", c.Application)
	}
	if !strings.Contains(c.Application.AwayMessage, "Operatora nie ma") {
		t.Fatalf("away message does not report operator absence: %q", c.Application.AwayMessage)
	}
}

func TestTerminalAX25Defaults(t *testing.T) {
	c, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Application.TerminalEOL != "cr" || c.Application.AX25T1Seconds != 10 || c.Application.AX25T3Seconds != 300 || c.Application.AX25N2 != 10 || c.Application.AX25N1 != 256 {
		t.Fatalf("terminal AX.25 defaults = %+v", c.Application)
	}
}

func TestRejectsInvalidTerminalAX25Settings(t *testing.T) {
	_, err := Parse([]byte("application: {terminal_eol: invalid, ax25_t1_seconds: 10, ax25_t3_seconds: 5, ax25_n2: 10, ax25_n1: 256}\nserver: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\n"))
	if err == nil {
		t.Fatal("invalid terminal AX.25 settings accepted")
	}
}

func TestLegacyWelcomeGetsCommandHintWithoutOverwritingCustomWelcome(t *testing.T) {
	legacy, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\napplication: {welcome_message: 'Witaj {REMOTE}, de {CALL}.'}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(legacy.Application.WelcomeMessage, "/I") || !strings.Contains(legacy.Application.WelcomeMessage, "/MH") {
		t.Fatalf("legacy welcome was not migrated: %q", legacy.Application.WelcomeMessage)
	}

	custom, err := Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080}\napplication: {welcome_message: 'Moje powitanie'}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if custom.Application.WelcomeMessage != "Moje powitanie" {
		t.Fatalf("custom welcome was overwritten: %q", custom.Application.WelcomeMessage)
	}
}

func TestBeaconViaConfiguration(t *testing.T) {
	raw := []byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
beacon: {enabled: true, destination: BEACON, via: "SR5AAA-1, SR5BBB-2", text: hi, interval_minutes: 10}
node: {enabled: true, alias: SP5ME, listen: 127.0.0.1:8010, language: pl}
bbs: {enabled: true, listen: 127.0.0.1:8023, forward_listen: 127.0.0.1:8024, database: data/bbs.json, callsign: SP5ME, language: pl, beacon_via: "SR5CCC-3"}
`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Beacon.Via != "SR5AAA-1, SR5BBB-2" || c.BBS.BeaconVia != "SR5CCC-3" {
		t.Fatalf("unexpected beacon via fields: %+v %+v", c.Beacon, c.BBS)
	}
}

func TestPortEnabledDefaultsAndDisable(t *testing.T) {
	raw := []byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - {id: p1, type: kiss-tcp, host: 127.0.0.1, port: 8001, kiss_port: 7, max_frame_bytes: 4096, reconnect_seconds: 5}
  - {id: p2, type: kiss-tcp, enabled: false, max_frame_bytes: 4096, reconnect_seconds: 5}
`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ports) != 2 {
		t.Fatalf("unexpected ports count: %d", len(c.Ports))
	}
	if c.Ports[0].Enabled == nil || !*c.Ports[0].Enabled {
		t.Fatal("enabled port did not default to true")
	}
	if c.Ports[0].KISSPort != 7 {
		t.Fatalf("KISS port = %d, want 7", c.Ports[0].KISSPort)
	}
	if c.Ports[1].Enabled == nil || *c.Ports[1].Enabled {
		t.Fatal("disabled port was not preserved")
	}
}

func TestKISSMultiportMappingIsPreserved(t *testing.T) {
	raw := []byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - id: kiss-main
    interface_id: kiss-main
    type: kiss-tcp
    host: 127.0.0.1
    port: 8001
    channels: {0: vhf, 1: uhf}
    max_frame_bytes: 4096
    reconnect_seconds: 5
`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ports[0].InterfaceID != "kiss-main" || c.Ports[0].Channels[0] != "vhf" || c.Ports[0].Channels[1] != "uhf" {
		t.Fatalf("unexpected KISS mapping: %+v", c.Ports[0])
	}
}

func TestLegacyPortConfigStillUsesSingleLogicalPort(t *testing.T) {
	c, err := Parse([]byte(`server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - {id: vhf, type: kiss-tcp, host: 127.0.0.1, port: 8001, kiss_port: 0, max_frame_bytes: 4096, reconnect_seconds: 5}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Ports[0].InterfaceID != "" || len(c.Ports[0].Channels) != 0 {
		t.Fatalf("legacy config unexpectedly changed: %+v", c.Ports[0])
	}
}

func TestRejectsKISSPortAbove15(t *testing.T) {
	raw := []byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - {id: p1, type: kiss-tcp, host: 127.0.0.1, port: 8001, kiss_port: 16, max_frame_bytes: 4096, reconnect_seconds: 5}
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("KISS port above 15 accepted")
	}
}

func TestAcceptsTNCProxy(t *testing.T) {
	c, err := Parse([]byte(`server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - {id: p1, type: kiss-tcp, host: 127.0.0.1, port: 8001, tncproxy_enabled: true, tncproxy_listen: 127.0.0.1:8101, max_frame_bytes: 4096, reconnect_seconds: 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.Ports[0].TNCProxyEnabled || c.Ports[0].TNCProxyListen != "127.0.0.1:8101" {
		t.Fatalf("unexpected proxy config: %+v", c.Ports[0])
	}
	if c.Ports[0].TNCProxyPort != 8101 {
		t.Fatalf("proxy port = %d, want 8101", c.Ports[0].TNCProxyPort)
	}
}

func TestTNCProxyDefaultsToPort8101(t *testing.T) {
	c, err := Parse([]byte(`server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
ports:
  - {id: p1, type: kiss-tcp, host: 127.0.0.1, port: 8001, tncproxy_enabled: true, max_frame_bytes: 4096, reconnect_seconds: 5}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Ports[0].TNCProxyPort != 8101 {
		t.Fatalf("proxy port = %d, want 8101", c.Ports[0].TNCProxyPort)
	}
}

func TestSaveRejectsInvalidWithoutChangingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	good, err := os.ReadFile("../../configs/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err = Save(p, good); err != nil {
		t.Fatal(err)
	}
	if err = Save(p, good); err != nil {
		t.Fatalf("overwrite valid configuration: %v", err)
	}
	if err = Save(p, []byte("server: [")); err == nil {
		t.Fatal("invalid YAML accepted")
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(good) {
		t.Fatal("valid configuration was changed")
	}
}

func TestSaveModelRoundTrip(t *testing.T) {
	c, err := Load("../../configs/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err = SaveModel(p, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Node.Alias != c.Node.Alias || got.BBS.Callsign != c.BBS.Callsign {
		t.Fatal("model changed during save")
	}
}

func TestCleanFirstRunModes(t *testing.T) {
	station := New(ModeStation, "N0CALL", "AA00AA", "Example", "en", 0, 7, 8)
	if err := station.Validate(); err != nil {
		t.Fatalf("station config: %v", err)
	}
	if station.Node.Enabled || station.BBS.Enabled {
		t.Fatal("station mode enabled NODE or BBS")
	}
	full := New(ModeFull, "N0CALL", "AA00AA", "Example", "en", 0, 7, 8)
	if err := full.Validate(); err != nil {
		t.Fatalf("full config: %v", err)
	}
	if !full.Node.Enabled || !full.BBS.Enabled {
		t.Fatal("full mode did not enable NODE and BBS")
	}
	full.BBS.Enabled = false
	if err := full.Validate(); err != nil {
		t.Fatalf("independent NODE/BBS configuration was rejected: %v", err)
	}
}

func TestInvalidLocatorRejected(t *testing.T) {
	for _, locator := range []string{"INVALID", "AA00AA00", "KO02A", "KO02A1", "KO02JD1"} {
		c := New(ModeStation, "N0CALL", locator, "Example", "en", 0, 7, 8)
		if err := c.Validate(); err == nil {
			t.Fatalf("invalid locator %q was accepted", locator)
		}
	}
}

func TestValidLocatorAccepted(t *testing.T) {
	for _, locator := range []string{"KO02JD", "ko02mg"} {
		c := New(ModeStation, "N0CALL", locator, "Example", "en", 0, 7, 8)
		if err := c.Validate(); err != nil {
			t.Fatalf("valid locator %q was rejected: %v", locator, err)
		}
	}
}

func TestUPRDDefaultsOnLegacyConfigButHonorsExplicitDisable(t *testing.T) {
	legacy, err := Parse([]byte("server: {callsign: N0CALL}\nterminal: {callsign: N0CALL}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.UPRD.Enabled {
		t.Fatal("UPRD should be enabled for a legacy config without an uprd section")
	}
	disabled, err := Parse([]byte("server: {callsign: N0CALL}\nterminal: {callsign: N0CALL}\nuprd: {enabled: false, interval_seconds: 600, mheard_limit: 5}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if disabled.UPRD.Enabled {
		t.Fatal("explicitly disabled UPRD was enabled")
	}
}

func TestStableChannelMigratesToMain(t *testing.T) {
	raw := []byte("application: {update_channel: stable}\nserver: {callsign: N0CALL}\nterminal: {callsign: N0CALL}\n")
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Application.UpdateChannel != "main" {
		t.Fatalf("channel = %q", c.Application.UpdateChannel)
	}
}
