package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestBeaconViaConfiguration(t *testing.T) {
	raw := []byte(`
server: {callsign: SP5ME}
terminal: {callsign: SP5ME}
web: {listen: 127.0.0.1:8080}
beacon: {enabled: true, port: p1, destination: BEACON, via: "SR5AAA-1, SR5BBB-2", text: hi, interval_minutes: 10}
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
  - {id: p1, type: kiss-tcp, host: 127.0.0.1, port: 8001, channel: 0, max_frame_bytes: 4096, reconnect_seconds: 5}
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
	if c.Ports[1].Enabled == nil || *c.Ports[1].Enabled {
		t.Fatal("disabled port was not preserved")
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
	if err := full.Validate(); err == nil {
		t.Fatal("split NODE/BBS mode was accepted")
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
