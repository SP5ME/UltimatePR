package terminalcodec

import (
	"bytes"
	"testing"
)

func TestCP437RoundTrip(t *testing.T) {
	c, err := New("cp437")
	if err != nil {
		t.Fatal(err)
	}
	want := "┌─┐\r\n│A│\r\n└─┘"
	wire, err := c.Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Decode(wire); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAutoUsesUTF8OrCP437(t *testing.T) {
	c, _ := New("auto")
	if got := c.Decode([]byte("Zażółć")); got != "Zażółć" {
		t.Fatalf("UTF-8: %q", got)
	}
	cp1250, _ := New("windows-1250")
	polishWire, _ := cp1250.Encode("Zażółć gęślą jaźń")
	if got := c.Decode(polishWire); got != "Zażółć gęślą jaźń" {
		t.Fatalf("CP1250 fallback: %q", got)
	}
	cp437, _ := New("cp437")
	wire, _ := cp437.Encode("─")
	if bytes.Equal(wire, []byte("─")) {
		t.Fatal("test fixture is not legacy encoded")
	}
	first := c.Decode(wire)
	if got := first + c.Decode([]byte("A")); got != "─A" {
		t.Fatalf("CP437 fallback: %q", got)
	}
}

func TestAliasesAndUnsupportedEncoding(t *testing.T) {
	c, err := New("CP1250")
	if err != nil || c.Name() != "windows-1250" {
		t.Fatalf("codec=%#v error=%v", c, err)
	}
	if _, err = New("petscii"); err == nil {
		t.Fatal("unsupported encoding accepted")
	}
}

func TestAutoPreservesUTF8SplitAcrossPackets(t *testing.T) {
	c, _ := New("auto")
	wire := []byte("żółć")
	if got := c.Decode(wire[:1]); got != "" {
		t.Fatalf("first partial rune produced %q", got)
	}
	if got := c.Decode(wire[1:]); got != "żółć" {
		t.Fatalf("joined UTF-8: %q", got)
	}
}
