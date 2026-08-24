package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnlyConnectedConversationReceivesLines(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.json"), Limits{MaxStations: 2, MaxSessions: 10, MaxLines: 2, MaxBytes: 4096, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	s.Add("tnc", "FAILED", "radio", "", "rx", "must not exist")
	if len(s.List()) != 0 {
		t.Fatal("failed attempt entered history")
	}
	s.Connected("tnc", "SR5DDD", "radio", "")
	s.Add("tnc", "SR5DDD", "radio", "", "rx", "one\r\n\r\ntwo\rthree")
	c, ok := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if !ok || c.Sessions != 1 || len(c.Lines) != 2 || c.Lines[0].Kind != "connected" || c.Lines[1].Text != "one\r\n\r\ntwo\rthree" {
		t.Fatalf("conversation=%+v", c)
	}
}

func TestLegacyHistoryRestoresLineEndings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	legacy := `{"conversations":[{"key":"TNC|SR5DDD|RADIO|","station":"SR5DDD","mode":"tnc","port":"radio","last_seen":"2026-08-19T10:00:00Z","sessions":1,"lines":[{"time":"2026-08-19T10:00:00Z","direction":"rx","text":"  ASCII"},{"time":"2026-08-19T10:00:01Z","direction":"rx","text":"   ART"}]}]}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, Limits{MaxStations: 2, MaxSessions: 10, MaxLines: 10, MaxBytes: 4096, RetentionDays: 3650})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if !ok || len(c.Lines) != 2 || c.Lines[0].Text != "  ASCII\n" || c.Lines[1].Text != "   ART\n" {
		t.Fatalf("legacy conversation not migrated: %+v", c)
	}
}

func TestUnversionedRawHistoryDoesNotSplitPacketFragments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	legacy := `{"conversations":[{"key":"TNC|SR5DDD|RADIO|","station":"SR5DDD","mode":"tnc","port":"radio","last_seen":"2026-08-19T10:00:00Z","sessions":1,"lines":[{"time":"2026-08-19T10:00:00Z","direction":"rx","text":"fragment-without-a-delimiter"},{"time":"2026-08-19T10:00:01Z","direction":"rx","text":"continues-here\r\nnext-line\r\n"}]}]}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, Limits{MaxStations: 2, MaxSessions: 10, MaxLines: 10, MaxBytes: 4096, RetentionDays: 3650})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if !ok || len(c.Lines) != 2 || c.Lines[0].Text != "fragment-without-a-delimiter" {
		t.Fatalf("raw packet boundary was changed: %+v", c)
	}
}

func TestHistoryPreservesPacketBoundariesAndBlankLines(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.json"), Limits{MaxStations: 2, MaxSessions: 10, MaxLines: 10, MaxBytes: 4096, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	s.Connected("tnc", "SR5DDD", "radio", "")
	s.Add("tnc", "SR5DDD", "radio", "", "rx", "  ASCII\r\n")
	s.Add("tnc", "SR5DDD", "radio", "", "rx", "\r\n   ART\r\n")
	c, _ := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if got := c.Lines[1].Text + c.Lines[2].Text; got != "  ASCII\r\n\r\n   ART\r\n" {
		t.Fatalf("history changed terminal stream: %q", got)
	}
}

func TestHistoryStoresConnectionLifecycle(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.json"), Limits{MaxStations: 2, MaxSessions: 10, MaxLines: 10, MaxBytes: 4096, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	sessionID := s.Connected("tnc", "SR5DDD", "radio", "")
	s.Add("tnc", "SR5DDD", "radio", "", "rx", "hello")
	s.Disconnected(sessionID)
	s.Disconnected(sessionID)
	c, _ := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if len(c.Lines) != 3 || c.Lines[0].Kind != "connected" || c.Lines[1].Text != "hello" || c.Lines[2].Kind != "disconnected" {
		t.Fatalf("unexpected lifecycle: %+v", c.Lines)
	}
}
