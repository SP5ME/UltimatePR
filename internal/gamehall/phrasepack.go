package gamehall

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type PhraseEntry struct{ Category, Phrase string }
type PhrasePack struct {
	ID, Name, Language, Version string
	Entries                     []PhraseEntry
}

func LoadPhrasePack(r io.Reader) (PhrasePack, error) {
	var p PhrasePack
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			kv := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(line, "#")), "=", 2)
			if len(kv) == 2 {
				switch strings.TrimSpace(kv[0]) {
				case "id":
					p.ID = strings.TrimSpace(kv[1])
				case "name":
					p.Name = strings.TrimSpace(kv[1])
				case "language":
					p.Language = strings.TrimSpace(kv[1])
				case "version":
					p.Version = strings.TrimSpace(kv[1])
				}
			}
			continue
		}
		kv := strings.SplitN(line, "|", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" || strings.TrimSpace(kv[1]) == "" {
			return p, fmt.Errorf("invalid phrase entry")
		}
		p.Entries = append(p.Entries, PhraseEntry{strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])})
	}
	return p, s.Err()
}

var defaultPhrasePack = PhrasePack{Name: "Krotkofalarstwo", Language: "pl", Version: "1", Entries: []PhraseEntry{{"Lacznosc", "PACKET RADIO"}, {"Sprzet", "RADIOTELEFON"}, {"Anteny", "ANTENA YAGI"}, {"Propagacja", "FALA JONOSFERYCZNA"}}}
