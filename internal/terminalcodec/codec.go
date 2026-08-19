package terminalcodec

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const Default = "auto"

var supported = []string{"auto", "utf-8", "cp437", "cp850", "windows-1250", "iso-8859-2"}

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
	}
	enc := c.encoding
	if c.name == "auto" {
		enc = charmap.CodePage437
	}
	out, _, err := transform.Bytes(enc.NewDecoder(), data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "�")
	}
	return string(out)
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
