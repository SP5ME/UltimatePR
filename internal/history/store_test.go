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
	s.Add("tnc", "SR5DDD", "radio", "", "rx", "one\r\ntwo\rthree")
	c, ok := s.Get(Key("tnc", "SR5DDD", "radio", ""))
	if !ok || c.Sessions != 1 || len(c.Lines) != 2 || c.Lines[1].Text != "three" {
		t.Fatalf("conversation=%+v", c)
	}
}
