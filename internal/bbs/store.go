package bbs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Message struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	ReadBy    []string  `json:"read_by,omitempty"`
	DeletedBy []string  `json:"deleted_by,omitempty"`
}

type dataFile struct {
	NextID   int64             `json:"next_id"`
	Users    map[string]string `json:"users"`
	Messages []Message         `json:"messages"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data dataFile
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: dataFile{NextID: 1, Users: map[string]string{}}}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("decode BBS database: %w", err)
	}
	if s.data.NextID < 1 {
		s.data.NextID = 1
	}
	if s.data.Users == nil {
		s.data.Users = map[string]string{}
	}
	return s, nil
}

func (s *Store) Register(callsign string) error {
	call, err := normalizeCall(callsign)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Users[call]; ok {
		return nil
	}
	s.data.Users[call] = time.Now().UTC().Format(time.RFC3339)
	return s.saveLocked()
}

func (s *Store) Send(kind, from, to, subject, body string) (Message, error) {
	from, err := normalizeCall(from)
	if err != nil {
		return Message{}, err
	}
	to = strings.ToUpper(strings.TrimSpace(to))
	if kind == "P" {
		if _, err := normalizeCall(to); err != nil {
			return Message{}, fmt.Errorf("recipient: %w", err)
		}
	}
	if kind != "P" && kind != "B" {
		return Message{}, errors.New("message type must be P or B")
	}
	if strings.TrimSpace(subject) == "" {
		return Message{}, errors.New("subject is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := Message{ID: s.data.NextID, Type: kind, From: from, To: to, Subject: strings.TrimSpace(subject), Body: body, CreatedAt: time.Now().UTC()}
	s.data.NextID++
	s.data.Messages = append(s.data.Messages, m)
	return m, s.saveLocked()
}

func (s *Store) List(call string, bulletins bool) []Message {
	call = strings.ToUpper(call)
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Message
	for _, m := range s.data.Messages {
		visible := (bulletins && m.Type == "B") || (!bulletins && m.Type == "P" && (m.To == call || m.From == call))
		if visible && !contains(m.DeletedBy, call) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (s *Store) Read(call string, id int64) (Message, error) {
	call = strings.ToUpper(call)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Messages {
		m := &s.data.Messages[i]
		if m.ID == id {
			if m.Type == "P" && m.To != call && m.From != call {
				return Message{}, errors.New("message is not addressed to you")
			}
			if contains(m.DeletedBy, call) {
				return Message{}, errors.New("message not found")
			}
			if !contains(m.ReadBy, call) {
				m.ReadBy = append(m.ReadBy, call)
				if err := s.saveLocked(); err != nil {
					return Message{}, err
				}
			}
			return *m, nil
		}
	}
	return Message{}, errors.New("message not found")
}

func (s *Store) Delete(call string, id int64) error {
	call = strings.ToUpper(call)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Messages {
		m := &s.data.Messages[i]
		if m.ID == id {
			if m.Type == "P" && m.To != call && m.From != call {
				return errors.New("message is not yours")
			}
			if !contains(m.DeletedBy, call) {
				m.DeletedBy = append(m.DeletedBy, call)
			}
			return s.saveLocked()
		}
	}
	return errors.New("message not found")
}

func (s *Store) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func normalizeCall(v string) (string, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	base := strings.Split(v, "-")[0]
	if len(base) < 1 || len(base) > 6 {
		return "", errors.New("invalid callsign")
	}
	for _, r := range base {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return "", errors.New("invalid callsign")
		}
	}
	return v, nil
}
