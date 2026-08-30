package lineinput

import (
	"strings"
	"testing"
)

func TestScannerAcceptsPacketRadioLineEndings(t *testing.T) {
	scanner := NewScanner(strings.NewReader("one\rtwo\nthree\r\nfour"))
	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	want := []string{"one", "two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("lines=%q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d=%q, want %q", i, got[i], want[i])
		}
	}
}
