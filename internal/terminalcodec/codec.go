package terminalcodec

import (
	"strings"
	"sync"
	"unicode/utf8"
)

const Default = "utf-8"

type Codec struct {
	name    string
	mu      sync.Mutex
	pending []byte
}

func Supported() []string { return []string{Default} }

func New(name string) (*Codec, error) {
	_ = name
	return &Codec{name: Default}, nil
}

func (c *Codec) Name() string { return c.name }

func (c *Codec) Decode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	combined := append(append([]byte(nil), c.pending...), data...)
	complete, pending := splitTrailingUTF8(combined)
	c.pending = append(c.pending[:0], pending...)
	if utf8.Valid(complete) {
		return string(complete)
	}
	return strings.ToValidUTF8(string(complete), "�")
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
	return []byte(text), nil
}
