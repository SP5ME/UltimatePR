package bbs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorePersistsMail(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bbs.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Register("SP5ABC"); err != nil {
		t.Fatal(err)
	}
	m, err := s.Send("P", "SP5ABC", "SQ9XYZ", "Test", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != 1 {
		t.Fatalf("id=%d", m.ID)
	}
	if m.MID != "1_SP5ABC" || m.BID != "" {
		t.Fatalf("private identifiers: MID=%q BID=%q", m.MID, m.BID)
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Read("SQ9XYZ", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "Hello" {
		t.Fatalf("body=%q", got.Body)
	}
	if err = s2.Delete("SQ9XYZ", 1); err != nil {
		t.Fatal(err)
	}
	if len(s2.List("SQ9XYZ", false)) != 0 {
		t.Fatal("deleted message still visible")
	}
}

func TestBulletinVisibleToEveryone(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "bbs.json"))
	m, err := s.Send("B", "SP5ABC", "ALL", "Net", "Sunday 10:00")
	if err != nil {
		t.Fatal(err)
	}
	if m.MID == "" || m.BID != m.MID || m.To != "ALL" || m.Distribution != "ALL" {
		t.Fatalf("bulletin identifiers: MID=%q BID=%q", m.MID, m.BID)
	}
	if got := s.List("SQ9XYZ", true); len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
}

func TestOpenMigratesLegacyBIDToTAPRIdentifiers(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bbs.json")
	legacy := `{"next_id":3,"users":{},"messages":[` +
		`{"id":1,"type":"P","from":"SP5AAA","to":"SP5BBB","bid":"1_SP5AAA","subject":"P","body":""},` +
		`{"id":2,"type":"B","from":"SP5AAA","to":"ALL","bid":"2_SP5AAA","subject":"B","body":""}]}`
	if err := os.WriteFile(p, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	ms := s.Messages()
	if ms[0].MID != "1_SP5AAA" || ms[0].BID != "" || ms[1].MID != "2_SP5AAA" || ms[1].BID != "2_SP5AAA" || ms[1].To != "ALL" || ms[1].Distribution != "ALL" {
		t.Fatalf("migration result: %+v", ms)
	}
	raw, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(raw), `"schema_version": 2`) {
		t.Fatalf("migration not persisted: %v %s", err, raw)
	}
}

func TestUserLanguagePersists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bbs.json")
	s, _ := Open(p)
	if err := s.SetLanguage("SP5AAA", "en"); err != nil {
		t.Fatal(err)
	}
	again, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Language("sp5aaa"); got != "en" {
		t.Fatalf("language=%q", got)
	}
}
