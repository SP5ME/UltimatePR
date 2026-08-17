package history

import (
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
	if !ok || c.Sessions != 1 || len(c.Lines) != 1 || c.Lines[0].Text != "one\r\n\r\ntwo\rthree" {
		t.Fatalf("conversation=%+v", c)
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
	if got := c.Lines[0].Text + c.Lines[1].Text; got != "  ASCII\r\n\r\n   ART\r\n" {
		t.Fatalf("history changed terminal stream: %q", got)
	}
}
