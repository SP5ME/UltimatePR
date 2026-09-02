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
	ID           int64                   `json:"id"`
	Type         string                  `json:"type"`
	From         string                  `json:"from"`
	To           string                  `json:"to"`
	At           string                  `json:"at,omitempty"`
	Distribution string                  `json:"distribution,omitempty"`
	MID          string                  `json:"mid"`
	BID          string                  `json:"bid,omitempty"`
	Subject      string                  `json:"subject"`
	Body         string                  `json:"body"`
	Routing      []string                `json:"routing_headers,omitempty"`
	CreatedAt    time.Time               `json:"created_at"`
	ReadBy       []string                `json:"read_by,omitempty"`
	DeletedBy    []string                `json:"deleted_by,omitempty"`
	Forward      map[string]ForwardState `json:"forward,omitempty"`
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

// WhitePageEntry is routing knowledge about a user. It is intentionally kept
// separate from the local profile: a profile is operator-supplied state, while
// White Pages may later be learned from a BBS exchange.
type WhitePageEntry struct {
	Callsign  string    `json:"callsign"`
	HomeBBS   string    `json:"home_bbs"`
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source"`
}

type dataFile struct {
	SchemaVersion int                       `json:"schema_version"`
	NextID        int64                     `json:"next_id"`
	Users         map[string]string         `json:"users"`
	Languages     map[string]string         `json:"languages,omitempty"`
	Profiles      map[string]UserProfile    `json:"profiles,omitempty"`
	WhitePages    map[string]WhitePageEntry `json:"white_pages,omitempty"`
	Messages      []Message                 `json:"messages"`
}

type Store struct {
	mu           sync.RWMutex
	path         string
	data         dataFile
	maxBodyBytes int
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, maxBodyBytes: 131072, data: dataFile{SchemaVersion: 3, NextID: 1, Users: map[string]string{}, Languages: map[string]string{}, Profiles: map[string]UserProfile{}, WhitePages: map[string]WhitePageEntry{}}}
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
	if s.data.WhitePages == nil {
		s.data.WhitePages = map[string]WhitePageEntry{}
	}
	if s.migrateTAPRIdentifiers() {
		if err := s.saveLocked(); err != nil {
			return nil, fmt.Errorf("migrate BBS database: %w", err)
		}
	}
	return s, nil
}

// SetMaxBodyBytes applies the configured storage guard to newly stored mail.
// A non-positive value keeps the safe default.
func (s *Store) SetMaxBodyBytes(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > 0 {
		s.maxBodyBytes = limit
	}
}

func (s *Store) migrateTAPRIdentifiers() bool {
	changed := s.data.SchemaVersion != 3
	for i := range s.data.Messages {
		m := &s.data.Messages[i]
		m.MID = strings.ToUpper(strings.TrimSpace(m.MID))
		m.BID = strings.ToUpper(strings.TrimSpace(m.BID))
		if m.MID == "" {
			m.MID = m.BID
			changed = true
		}
		if m.Type == "P" && m.BID == m.MID {
			m.BID = ""
			changed = true
		}
		if m.Type == "B" && m.BID == "" {
			m.BID = m.MID
			changed = true
		}
		if m.Type == "B" && m.Distribution == "" {
			m.Distribution = strings.ToUpper(strings.TrimSpace(m.To))
			m.To = "ALL"
			changed = true
		}
	}
	for call, profile := range s.data.Profiles {
		if profile.HomeBBS == "" {
			continue
		}
		if _, exists := s.data.WhitePages[call]; !exists {
			updated := profile.LastSeen
			if updated.IsZero() {
				updated = profile.CreatedAt
			}
			if updated.IsZero() {
				updated = time.Now().UTC()
			}
			s.data.WhitePages[call] = WhitePageEntry{Callsign: call, HomeBBS: profile.HomeBBS, UpdatedAt: updated, Source: "local-profile"}
			changed = true
		}
	}
	s.data.SchemaVersion = 3
	return changed
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
	if wp, ok := s.data.WhitePages[call]; ok && wp.HomeBBS != "" {
		return call + "@" + wp.HomeBBS
	}
	return call
}
func (s *Store) SaveProfile(p UserProfile) error {
	call, err := normalizeCall(p.Callsign)
	if err != nil {
		return err
	}
	if p.HomeBBS != "" {
		if _, err := ParseHierarchicalAddress(p.HomeBBS); err != nil {
			return errors.New("invalid Home BBS")
		}
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
	if p.HomeBBS != "" {
		s.data.WhitePages[call] = WhitePageEntry{Callsign: call, HomeBBS: p.HomeBBS, UpdatedAt: now, Source: "local-profile"}
	}
	return s.saveLocked()
}

func (s *Store) WhitePage(call string) (WhitePageEntry, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data.WhitePages[call]
	return entry, ok
}

func (s *Store) WhitePages() []WhitePageEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]WhitePageEntry, 0, len(s.data.WhitePages))
	for _, entry := range s.data.WhitePages {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Callsign < entries[j].Callsign })
	return entries
}

func (s *Store) UpsertWhitePage(entry WhitePageEntry) error {
	call, err := normalizeCall(entry.Callsign)
	if err != nil {
		return err
	}
	if _, err := ParseHierarchicalAddress(entry.HomeBBS); err != nil {
		return fmt.Errorf("White Pages Home BBS: %w", err)
	}
	entry.Callsign = call
	entry.HomeBBS = strings.ToUpper(strings.TrimSpace(entry.HomeBBS))
	entry.Source = language.ASCII(strings.TrimSpace(entry.Source))
	if entry.Source == "" {
		return errors.New("White Pages source is required")
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data.WhitePages[call]; ok && existing.UpdatedAt.After(entry.UpdatedAt) {
		return nil
	}
	if s.data.WhitePages == nil {
		s.data.WhitePages = map[string]WhitePageEntry{}
	}
	s.data.WhitePages[call] = entry
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
	distribution := ""
	if parts := strings.SplitN(to, "@", 2); len(parts) == 2 {
		to = parts[0]
		at = strings.TrimSpace(parts[1])
		if at == "" {
			return Message{}, errors.New("BBS address after @ is required")
		}
	}
	if kind == "P" || kind == "T" {
		if _, err := normalizeCall(to); err != nil {
			return Message{}, fmt.Errorf("recipient: %w", err)
		}
	}
	if kind != "P" && kind != "B" && kind != "T" {
		return Message{}, errors.New("message type must be P, B or T")
	}
	if len(language.ASCII(strings.TrimSpace(subject))) < 1 || len(language.ASCII(strings.TrimSpace(subject))) > 79 {
		return Message{}, errors.New("subject length must be 1..79 ASCII characters")
	}
	body = language.ASCII(body)
	if len(body) > s.bodyLimitBytes() {
		return Message{}, fmt.Errorf("message body exceeds max_body_bytes (%d)", s.bodyLimitBytes())
	}
	if (kind == "P" || kind == "T") && at != "" {
		if _, err := ParseHierarchicalAddress(at); err != nil {
			return Message{}, fmt.Errorf("invalid hierarchical BBS address: %w", err)
		}
	}
	if kind == "B" {
		if at != "" {
			return Message{}, errors.New("bulletin distribution must not use a BBS address")
		}
		var err error
		distribution, err = ParseDistributionDesignator(to)
		if err != nil {
			return Message{}, err
		}
		to = "ALL"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	id := s.data.NextID
	base := strings.Split(from, "-")[0]
	mid := fmt.Sprintf("%d_%s", id, base)
	if len(mid) > 12 {
		mid = mid[:12]
	}
	bid := ""
	if kind == "B" {
		bid = mid
	}
	m := Message{ID: id, Type: kind, From: from, To: to, At: at, Distribution: distribution, MID: mid, BID: bid, Subject: language.ASCII(strings.TrimSpace(subject)), Body: body, CreatedAt: now}
	s.data.NextID++
	s.data.Messages = append(s.data.Messages, m)
	return m, s.saveLocked()
}

func (s *Store) HasBID(bid string) bool {
	if strings.TrimSpace(bid) == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.data.Messages {
		if strings.EqualFold(m.BID, bid) {
			return true
		}
	}
	return false
}
func (s *Store) HasMID(mid string) bool {
	if strings.TrimSpace(mid) == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.data.Messages {
		if strings.EqualFold(m.MID, mid) {
			return true
		}
	}
	return false
}
func (s *Store) Import(m Message) (Message, bool, error) {
	m.MID = strings.ToUpper(strings.TrimSpace(m.MID))
	m.BID = strings.ToUpper(strings.TrimSpace(m.BID))
	if m.MID != "" && !validMessageIdentifier(m.MID) {
		return Message{}, false, errors.New("invalid MID")
	}
	if m.BID != "" && !validMessageIdentifier(m.BID) {
		return Message{}, false, errors.New("invalid BID")
	}
	if _, err := normalizeCall(m.From); err != nil {
		return Message{}, false, err
	}
	if m.Type == "P" || m.Type == "T" {
		if _, err := normalizeCall(m.To); err != nil {
			return Message{}, false, err
		}
		if m.At != "" {
			if _, err := ParseHierarchicalAddress(m.At); err != nil {
				return Message{}, false, err
			}
		}
	}
	if m.Type != "P" && m.Type != "B" && m.Type != "T" {
		return Message{}, false, errors.New("message type must be P, B or T")
	}
	if m.Type == "B" && m.BID == "" {
		return Message{}, false, errors.New("bulletin BID is required")
	}
	if m.Type == "T" && m.BID != "" {
		return Message{}, false, errors.New("TAPR NTS traffic must not carry BID")
	}
	if m.Type == "B" {
		if _, err := ParseDistributionDesignator(m.To); err != nil {
			return Message{}, false, fmt.Errorf("bulletin category: %w", err)
		}
		var err error
		m.Distribution, err = ParseDistributionDesignator(m.Distribution)
		if err != nil {
			return Message{}, false, err
		}
	}
	if len(language.ASCII(strings.TrimSpace(m.Subject))) < 1 || len(language.ASCII(strings.TrimSpace(m.Subject))) > 79 {
		return Message{}, false, errors.New("subject length must be 1..79 ASCII characters")
	}
	m.Body = language.ASCII(m.Body)
	if len(m.Body) > s.bodyLimitBytes() {
		return Message{}, false, fmt.Errorf("message body exceeds max_body_bytes (%d)", s.bodyLimitBytes())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.MID == "" {
		m.MID = generatedMessageID(s.data.NextID, m.From)
	}
	for _, x := range s.data.Messages {
		if (m.BID != "" && strings.EqualFold(x.BID, m.BID)) || strings.EqualFold(x.MID, m.MID) {
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

func (s *Store) bodyLimitBytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.maxBodyBytes < 1 {
		return 131072
	}
	return s.maxBodyBytes
}

// RemoveExpired removes old bulletins and personal/traffic mail. Messages
// with any undelivered forwarding state are retained until delivery succeeds.
func (s *Store) RemoveExpired(now time.Time, bulletinDays, personalDays int) (bulletins, personal int, err error) {
	if bulletinDays < 1 || personalDays < 1 {
		return 0, 0, nil
	}
	cutB, cutP := now.Add(-time.Duration(bulletinDays)*24*time.Hour), now.Add(-time.Duration(personalDays)*24*time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.data.Messages[:0]
	for _, m := range s.data.Messages {
		cutoff := cutP
		if m.Type == "B" {
			cutoff = cutB
		}
		expired := !m.CreatedAt.IsZero() && m.CreatedAt.Before(cutoff)
		if expired && (m.Type == "B" || m.Type == "P" || m.Type == "T") && !hasUndeliveredForward(m.Forward) {
			if m.Type == "B" {
				bulletins++
			} else {
				personal++
			}
			continue
		}
		kept = append(kept, m)
	}
	if bulletins == 0 && personal == 0 {
		return 0, 0, nil
	}
	s.data.Messages = kept
	return bulletins, personal, s.saveLocked()
}

func hasUndeliveredForward(states map[string]ForwardState) bool {
	for _, state := range states {
		if state.Status != "delivered" {
			return true
		}
	}
	return false
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
		visible := (bulletins && m.Type == "B") || (!bulletins && (m.Type == "P" || m.Type == "T") && (m.To == call || m.From == call))
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
			if (m.Type == "P" || m.Type == "T") && m.To != call && m.From != call {
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
			if (m.Type == "P" || m.Type == "T") && m.To != call && m.From != call {
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
func validMessageIdentifier(v string) bool {
	if len(v) < 1 || len(v) > 12 || strings.ContainsAny(v, " \t\r\n") {
		return false
	}
	for _, r := range v {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func generatedMessageID(id int64, from string) string {
	mid := fmt.Sprintf("%d_%s", id, strings.Split(strings.ToUpper(strings.TrimSpace(from)), "-")[0])
	if len(mid) > 12 {
		mid = mid[:12]
	}
	return mid
}
