package history

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Limits struct{ MaxStations, MaxSessions, MaxLines, MaxBytes, RetentionDays int }
type Line struct {
	Time      time.Time `json:"time"`
	Direction string    `json:"direction"`
	Text      string    `json:"text"`
}
type Conversation struct {
	Key      string    `json:"key"`
	Station  string    `json:"station"`
	Mode     string    `json:"mode"`
	Port     string    `json:"port"`
	Digi     string    `json:"digi,omitempty"`
	LastSeen time.Time `json:"last_seen"`
	Sessions int       `json:"sessions"`
	Lines    []Line    `json:"lines"`
}
type fileData struct {
	Version       int            `json:"version,omitempty"`
	Conversations []Conversation `json:"conversations"`
}
type Store struct {
	mu     sync.Mutex
	path   string
	limits Limits
	items  map[string]*Conversation
}

func Open(path string, limits Limits) (*Store, error) {
	s := &Store{path: path, limits: limits, items: map[string]*Conversation{}}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(b) > 0 {
		var d fileData
		if err = json.Unmarshal(b, &d); err != nil {
			return nil, err
		}
		if d.Version < 2 {
			migrateLegacyLines(d.Conversations)
		}
		for i := range d.Conversations {
			c := d.Conversations[i]
			s.items[c.Key] = &c
		}
	}
	s.prune()
	return s, nil
}
func Key(mode, station, port, digi string) string {
	return strings.ToUpper(mode + "|" + station + "|" + port + "|" + digi)
}
func (s *Store) Connected(mode, station, port, digi string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := Key(mode, station, port, digi)
	c := s.items[key]
	if c == nil {
		c = &Conversation{Key: key, Station: station, Mode: mode, Port: port, Digi: digi}
		s.items[key] = c
	}
	c.Sessions++
	c.LastSeen = time.Now()
	s.pruneLocked()
	_ = s.saveLocked()
}
func (s *Store) Add(mode, station, port, digi, direction, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.items[Key(mode, station, port, digi)]
	if c == nil {
		return
	}
	// Store the terminal stream verbatim. Splitting it into non-empty lines used
	// to discard blank lines and packet-boundary information, corrupting ASCII
	// art when a conversation was reopened from history.
	for len(text) > 0 {
		n := len(text)
		if n > 512 {
			n = 512
			for n > 0 && n < len(text) && (text[n]&0xc0) == 0x80 {
				n--
			}
		}
		c.Lines = append(c.Lines, Line{time.Now(), direction, text[:n]})
		text = text[n:]
	}
	if len(c.Lines) > s.limits.MaxLines {
		c.Lines = c.Lines[len(c.Lines)-s.limits.MaxLines:]
	}
	c.LastSeen = time.Now()
	s.pruneLocked()
	_ = s.saveLocked()
}
func (s *Store) List() []Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	out := make([]Conversation, 0, len(s.items))
	for _, c := range s.items {
		v := *c
		if len(v.Lines) > 0 {
			v.Lines = []Line{v.Lines[len(v.Lines)-1]}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}
func (s *Store) Get(key string) (Conversation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.items[key]
	if !ok {
		return Conversation{}, false
	}
	v := *c
	v.Lines = append([]Line(nil), c.Lines...)
	return v, true
}
func (s *Store) prune() { s.mu.Lock(); defer s.mu.Unlock(); s.pruneLocked() }
func (s *Store) pruneLocked() {
	cut := time.Now().AddDate(0, 0, -s.limits.RetentionDays)
	for k, c := range s.items {
		if c.LastSeen.Before(cut) {
			delete(s.items, k)
		}
	}
	for len(s.items) > s.limits.MaxStations {
		var k string
		var t time.Time
		for key, c := range s.items {
			if k == "" || c.LastSeen.Before(t) {
				k, t = key, c.LastSeen
			}
		}
		delete(s.items, k)
	}
	for s.encodedSize() > s.limits.MaxBytes && len(s.items) > 0 {
		var k string
		var t time.Time
		for key, c := range s.items {
			if k == "" || c.LastSeen.Before(t) {
				k, t = key, c.LastSeen
			}
		}
		delete(s.items, k)
	}
}
func (s *Store) encodedSize() int { b, _ := json.Marshal(s.data()); return len(b) }
func (s *Store) data() fileData {
	d := fileData{Version: 2}
	for _, c := range s.items {
		d.Conversations = append(d.Conversations, *c)
	}
	return d
}

// Version 1 stored every non-empty terminal line as a separate record and
// discarded its delimiter. Version 2 stores the byte stream verbatim. Restore
// the missing delimiter while loading legacy data so the web UI can safely
// concatenate records without turning the whole conversation into one line.
func migrateLegacyLines(conversations []Conversation) {
	for conversationIndex := range conversations {
		for lineIndex := range conversations[conversationIndex].Lines {
			line := &conversations[conversationIndex].Lines[lineIndex]
			if line.Text != "" && !strings.HasSuffix(line.Text, "\r") && !strings.HasSuffix(line.Text, "\n") {
				line.Text += "\n"
			}
		}
	}
}
func (s *Store) saveLocked() error {
	if err := os.MkdirAll(dir(s.path), 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data(), "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func dir(p string) string {
	i := strings.LastIndexAny(p, "/\\")
	if i < 0 {
		return "."
	}
	return p[:i]
}
