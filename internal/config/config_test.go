package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
