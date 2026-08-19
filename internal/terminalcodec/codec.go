package terminalcodec

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
	"unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const Default = "auto"

var supported = []string{"auto", "utf-8", "cp437", "cp850", "windows-1250", "iso-8859-2"}
var autoFallbacks = []encoding.Encoding{
	charmap.Windows1250,
	charmap.ISO8859_2,
	charmap.CodePage850,
	charmap.CodePage437,
}

type Codec struct {
	name     string
	encoding encoding.Encoding
	mu       sync.Mutex
	pending  []byte
}

func Supported() []string { return append([]string(nil), supported...) }

func New(name string) (*Codec, error) {
	name = normalize(name)
	var enc encoding.Encoding
	switch name {
	case "auto", "utf-8":
	case "cp437":
		enc = charmap.CodePage437
	case "cp850":
		enc = charmap.CodePage850
	case "windows-1250":
		enc = charmap.Windows1250
	case "iso-8859-2":
		enc = charmap.ISO8859_2
	default:
		return nil, fmt.Errorf("unsupported terminal encoding %q", name)
	}
	return &Codec{name: name, encoding: enc}, nil
}

func (c *Codec) Name() string { return c.name }

func (c *Codec) Decode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.name == "utf-8" || c.name == "auto" {
		combined := append(append([]byte(nil), c.pending...), data...)
		c.pending = c.pending[:0]
		complete, pending := splitTrailingUTF8(combined)
		if utf8.Valid(complete) {
			c.pending = append(c.pending, pending...)
			return string(complete)
		}
		if c.name == "utf-8" {
			c.pending = append(c.pending, pending...)
			return strings.ToValidUTF8(string(complete), "�")
		}
		data = combined
		best := decodeLegacyAuto(data)
		if best != "" {
			return best
		}
	}
	enc := c.encoding
	if c.name == "auto" {
		enc = charmap.Windows1250
	}
	out, _, err := transform.Bytes(enc.NewDecoder(), data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "�")
	}
	return string(out)
}

func decodeLegacyAuto(data []byte) string {
	best := ""
	bestScore := -1 << 30
	for _, enc := range autoFallbacks {
		out, _, err := transform.Bytes(enc.NewDecoder(), data)
		if err != nil {
			continue
		}
		score := scoreDecodedText(string(out))
		if score > bestScore {
			bestScore = score
			best = string(out)
		}
	}
	return best
}

func scoreDecodedText(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case r == utf8.RuneError:
			score -= 8
		case r == '\n' || r == '\r' || r == '\t' || r == ' ':
			score += 1
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			score += 4
		case isBoxDrawingRune(r):
			score += 5
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			score += 2
		case unicode.IsControl(r):
			score -= 4
		case unicode.IsPrint(r):
			score += 1
		default:
			score -= 2
		}
	}
	return score
}

func isBoxDrawingRune(r rune) bool {
	return r >= 0x2500 && r <= 0x257F
}

func splitTrailingUTF8(data []byte) ([]byte, []byte) {
	start := len(data)
	for i := len(data) - 1; i >= 0 && len(data)-i <= utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if !utf8.FullRune(data[i:]) {
				start = i
			}
			break
		}
	}
	return data[:start], data[start:]
}

func (c *Codec) Encode(text string) ([]byte, error) {
	if c.name == "auto" || c.name == "utf-8" {
		return []byte(text), nil
	}
	out, _, err := transform.Bytes(encoding.ReplaceUnsupported(c.encoding.NewEncoder()), []byte(text))
	return out, err
}

func normalize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "", "automatic", "autodetect":
		return Default
	case "utf8":
		return "utf-8"
	case "ibm437", "ibm-437":
		return "cp437"
	case "ibm850", "ibm-850":
		return "cp850"
	case "cp1250", "windows1250":
		return "windows-1250"
	case "latin2", "latin-2", "iso8859-2":
		return "iso-8859-2"
	default:
		return name
	}
}
