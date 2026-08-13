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

	"github.com/packet-radio/ultimatepr/internal/language"
)

type Message struct {
	ID        int64                   `json:"id"`
	Type      string                  `json:"type"`
	From      string                  `json:"from"`
	To        string                  `json:"to"`
	At        string                  `json:"at,omitempty"`
	BID       string                  `json:"bid"`
	Subject   string                  `json:"subject"`
	Body      string                  `json:"body"`
	CreatedAt time.Time               `json:"created_at"`
	ReadBy    []string                `json:"read_by,omitempty"`
	DeletedBy []string                `json:"deleted_by,omitempty"`
	Forward   map[string]ForwardState `json:"forward,omitempty"`
}
type ForwardState struct {
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	QueuedAt    time.Time `json:"queued_at,omitempty"`
	LastAttempt time.Time `json:"last_attempt,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
}
type UserProfile struct {
	Callsign  string    `json:"callsign"`
	Name      string    `json:"name,omitempty"`
	HomeBBS   string    `json:"home_bbs,omitempty"`
	QTH       string    `json:"qth,omitempty"`
	Locator   string    `json:"locator,omitempty"`
	Language  string    `json:"language,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
	Completed bool      `json:"completed"`
}

type dataFile struct {
	NextID    int64                  `json:"next_id"`
	Users     map[string]string      `json:"users"`
	Languages map[string]string      `json:"languages,omitempty"`
	Profiles  map[string]UserProfile `json:"profiles,omitempty"`
	Messages  []Message              `json:"messages"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data dataFile
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: dataFile{NextID: 1, Users: map[string]string{}, Languages: map[string]string{}, Profiles: map[string]UserProfile{}}}
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
	if s.data.Languages == nil {
		s.data.Languages = map[string]string{}
	}
	if s.data.Profiles == nil {
		s.data.Profiles = map[string]UserProfile{}
	}
	return s, nil
}

func (s *Store) Profile(call string) (UserProfile, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.data.Profiles[call]
	return p, ok
}
func (s *Store) AddressFor(call string) string {
	call = strings.ToUpper(strings.TrimSpace(call))
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.data.Profiles[call]; ok && p.HomeBBS != "" {
		return call + "@" + p.HomeBBS
	}
	return call
}
func (s *Store) SaveProfile(p UserProfile) error {
	call, err := normalizeCall(p.Callsign)
	if err != nil {
		return err
	}
	if p.HomeBBS != "" && !validBBSAddress(p.HomeBBS) {
		return errors.New("invalid Home BBS")
	}
	p.Callsign, p.Name, p.HomeBBS, p.QTH, p.Locator = call, language.ASCII(strings.TrimSpace(p.Name)), strings.ToUpper(strings.TrimSpace(p.HomeBBS)), language.ASCII(strings.TrimSpace(p.QTH)), strings.ToUpper(strings.TrimSpace(p.Locator))
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.LastSeen = now
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Profiles == nil {
		s.data.Profiles = map[string]UserProfile{}
	}
	s.data.Profiles[call] = p
	return s.saveLocked()
}

func (s *Store) Language(call string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Languages[strings.ToUpper(strings.TrimSpace(call))]
}
func (s *Store) SetLanguage(call, lang string) error {
	call, err := normalizeCall(call)
	if err != nil {
		return err
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != "pl" && lang != "en" {
		return errors.New("unsupported language")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Languages == nil {
		s.data.Languages = map[string]string{}
	}
	s.data.Languages[call] = lang
	return s.saveLocked()
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
	at := ""
	if parts := strings.SplitN(to, "@", 2); len(parts) == 2 {
		to = parts[0]
		at = strings.TrimSpace(parts[1])
		if at == "" {
			return Message{}, errors.New("BBS address after @ is required")
		}
	}
	if kind == "P" {
		if _, err := normalizeCall(to); err != nil {
			return Message{}, fmt.Errorf("recipient: %w", err)
		}
	}
	if at != "" && !validBBSAddress(at) {
		return Message{}, errors.New("invalid hierarchical BBS address")
	}
	if kind != "P" && kind != "B" {
		return Message{}, errors.New("message type must be P or B")
	}
	if strings.TrimSpace(subject) == "" {
		return Message{}, errors.New("subject is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	id := s.data.NextID
	base := strings.Split(from, "-")[0]
	bid := fmt.Sprintf("%d_%s", id, base)
	if len(bid) > 12 {
		bid = bid[:12]
	}
	m := Message{ID: id, Type: kind, From: from, To: to, At: at, BID: bid, Subject: language.ASCII(strings.TrimSpace(subject)), Body: language.ASCII(body), CreatedAt: now}
	s.data.NextID++
	s.data.Messages = append(s.data.Messages, m)
	return m, s.saveLocked()
}

func (s *Store) HasBID(bid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.data.Messages {
		if strings.EqualFold(m.BID, bid) {
			return true
		}
	}
	return false
}
func (s *Store) Import(m Message) (Message, bool, error) {
	if strings.TrimSpace(m.BID) == "" || len(m.BID) > 80 {
		return Message{}, false, errors.New("invalid BID")
	}
	if _, err := normalizeCall(m.From); err != nil {
		return Message{}, false, err
	}
	if m.Type == "P" {
		if _, err := normalizeCall(m.To); err != nil {
			return Message{}, false, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.data.Messages {
		if strings.EqualFold(x.BID, m.BID) {
			return x, true, nil
		}
	}
	m.ID = s.data.NextID
	s.data.NextID++
	m.Subject = language.ASCII(m.Subject)
	m.Body = language.ASCII(m.Body)
	m.ReadBy = nil
	m.DeletedBy = nil
	m.Forward = nil
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	s.data.Messages = append(s.data.Messages, m)
	return m, false, s.saveLocked()
}

func (m Message) IsRead(call string) bool {
	return contains(m.ReadBy, strings.ToUpper(strings.TrimSpace(call)))
}
func (s *Store) ListNew(call string) []Message {
	all := s.List(call, false)
	out := all[:0]
	for _, m := range all {
		if m.To == strings.ToUpper(strings.TrimSpace(call)) && !m.IsRead(call) {
			out = append(out, m)
		}
	}
	return out
}
func (s *Store) ListSent(call string) []Message {
	call = strings.ToUpper(strings.TrimSpace(call))
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Message
	for _, m := range s.data.Messages {
		if m.From == call && !contains(m.DeletedBy, call) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}
func (s *Store) Messages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Message, len(s.data.Messages))
	copy(out, s.data.Messages)
	return out
}
func (s *Store) QueueForward(peer string, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	for i := range s.data.Messages {
		m := &s.data.Messages[i]
		if !wanted[m.ID] {
			continue
		}
		if m.Forward == nil {
			m.Forward = map[string]ForwardState{}
		}
		if _, ok := m.Forward[peer]; !ok {
			m.Forward[peer] = ForwardState{Status: "pending", QueuedAt: now}
		}
	}
	return s.saveLocked()
}
func (s *Store) ForwardQueue(peer string, limit int) []Message {
	if limit < 1 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Message
	for _, m := range s.data.Messages {
		st, ok := m.Forward[peer]
		if ok && (st.Status == "pending" || st.Status == "failed") {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
func (s *Store) RecordForward(peer string, id int64, delivered bool, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Messages {
		m := &s.data.Messages[i]
		if m.ID != id {
			continue
		}
		if m.Forward == nil {
			m.Forward = map[string]ForwardState{}
		}
		st := m.Forward[peer]
		st.Attempts++
		st.LastAttempt = time.Now().UTC()
		st.LastError = reason
		if delivered {
			st.Status = "delivered"
			st.DeliveredAt = st.LastAttempt
			st.LastError = ""
		} else {
			st.Status = "failed"
		}
		m.Forward[peer] = st
		return s.saveLocked()
	}
	return errors.New("message not found")
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
func validBBSAddress(v string) bool {
	if len(v) < 1 || len(v) > 80 {
		return false
	}
	for _, r := range strings.ToUpper(v) {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '.' && r != '#' {
			return false
		}
	}
	return true
}
