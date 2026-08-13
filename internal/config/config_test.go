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
