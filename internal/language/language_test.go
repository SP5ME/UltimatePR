package language

import "testing"

func TestPolishTerminalCatalogIsASCII(t *testing.T) {
	for key, value := range pl {
		for _, r := range value {
			if r > 127 {
				t.Fatalf("message %s contains non-ASCII %q", key, r)
			}
		}
	}
}
func TestPolishTransliteration(t *testing.T) {
	if got := ASCII("Zażółć gęślą jaźń"); got != "Zazolc gesla jazn" {
		t.Fatalf("got %q", got)
	}
}
