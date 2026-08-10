package bbs

import (
	"path/filepath"
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
	if _, err := s.Send("B", "SP5ABC", "ALL", "Net", "Sunday 10:00"); err != nil {
		t.Fatal(err)
	}
	if got := s.List("SQ9XYZ", true); len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
}
