package bbs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/packet-radio/ultimatepr/internal/language"
	"github.com/packet-radio/ultimatepr/internal/lineinput"
)

type Server struct {
	Listen, Title, Node, Address, Language string
	WelcomeMessage, GoodbyeMessage         string
	NewUserMessage, InfoMessage, Prompt    string
	SysopCallsign                          string
	MaxSessions                            int
	ForwardPeers                           []ForwardPeer
	MaxForwardMessages                     int
	Store                                  *Store
	Log                                    *slog.Logger
	OnMessage                              func()
	chatMu                                 sync.Mutex
	chat                                   map[*chatClient]struct{}
	activeSessions                         atomic.Int32
}

type chatClient struct {
	call string
	w    io.Writer
	mu   sync.Mutex
}

func (c *chatClient) send(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = io.WriteString(c.w, text)
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() { <-ctx.Done(); _ = ln.Close() }()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !s.acquireSession() {
			_, _ = io.WriteString(c, "BBS is busy. Please try again later.\r\n")
			_ = c.Close()
			continue
		}
		wg.Add(1)
		go func() { defer wg.Done(); defer s.releaseSession(); defer c.Close(); s.Serve(c, c) }()
	}
}

func (s *Server) Serve(r io.Reader, w io.Writer) {
	in := lineinput.NewScanner(r)
	in.Buffer(make([]byte, 1024), 64*1024)
	lang := language.Normalize(s.Language)
	fmt.Fprintf(w, "\r\n%s [%s] UltimatePR %s\r\n%s", s.Title, s.Node, BuildVersion, language.T(lang, "callsign"))
	if !in.Scan() {
		return
	}
	call, err := normalizeCall(in.Text())
	if err != nil {
		fmt.Fprint(w, language.T(lang, "invalid_call"))
		return
	}
	if err = s.Store.Register(call); err != nil {
		fmt.Fprint(w, language.T(lang, "db_error"))
		return
	}
	s.ServeSessionLanguage(call, lang, in, w)
}

// ServeAX25 serves a station whose identity was obtained from the connected
// AX.25 link. No additional text callsign prompt is used on radio links.
func (s *Server) ServeAX25(call string, r io.Reader, w io.Writer) {
	if !s.acquireSession() {
		_, _ = io.WriteString(w, "BBS is busy. Please try again later.\r\n")
		return
	}
	defer s.releaseSession()
	in := lineinput.NewScanner(r)
	in.Buffer(make([]byte, 1024), 64*1024)
	lang := language.Normalize(s.Language)
	fmt.Fprintf(w, "\r\n%s [%s] UltimatePR %s\r\n", s.Title, s.Node, BuildVersion)
	s.ServeSessionLanguage(call, lang, in, w)
}

func (s *Server) ServeSession(call string, in *bufio.Scanner, w io.Writer) {
	s.ServeSessionLanguage(call, s.Language, in, w)
}
func (s *Server) ServeSessionLanguage(call, lang string, in *bufio.Scanner, w io.Writer) {
	lang = language.Normalize(lang)
	call, err := normalizeCall(call)
	if err != nil {
		fmt.Fprint(w, language.T(lang, "invalid_call"))
		return
	}
	if err = s.Store.Register(call); err != nil {
		fmt.Fprint(w, language.T(lang, "db_error"))
		return
	}
	if saved := s.Store.Language(call); saved != "" {
		lang = language.Normalize(saved)
	}
	profile, exists := s.Store.Profile(call)
	if !exists || !profile.Completed {
		if strings.TrimSpace(s.NewUserMessage) != "" {
			fmt.Fprint(w, s.expandMessage(s.NewUserMessage, call), "\r\n")
		}
		profile = s.registerProfile(call, lang, profile, in, w)
	}
	profile.LastSeen = time.Now().UTC()
	_ = s.Store.SaveProfile(profile)
	if strings.TrimSpace(s.WelcomeMessage) == "" {
		fmt.Fprintf(w, language.T(lang, "hello_bbs"), call)
	} else {
		fmt.Fprint(w, s.expandMessage(s.WelcomeMessage, call), "\r\n")
	}
	line := func(prompt string) (string, bool) {
		fmt.Fprint(w, prompt)
		if !in.Scan() {
			return "", false
		}
		return strings.TrimSpace(in.Text()), true
	}
	for {
		prompt := s.expandMessage(s.Prompt, call)
		if strings.TrimSpace(prompt) == "" {
			prompt = s.Node + "> "
		}
		raw, ok := line(prompt)
		if !ok {
			return
		}
		f := strings.Fields(raw)
		if len(f) == 0 {
			continue
		}
		cmd := strings.ToUpper(f[0])
		switch cmd {
		case "H", "HELP", "?":
			fmt.Fprint(w, language.T(lang, "bbs_help"))
		case "I", "INFO":
			if strings.TrimSpace(s.InfoMessage) == "" {
				fmt.Fprintf(w, "%s [%s] %s\r\n", s.Title, s.Node, s.Address)
			} else {
				fmt.Fprint(w, s.expandMessage(s.InfoMessage, call), "\r\n")
			}
		case "PROFILE":
			s.printProfile(w, profile)
		case "NAME":
			if len(f) < 2 {
				fmt.Fprintf(w, "Name: %s\r\n", profile.Name)
				continue
			}
			profile.Name = strings.Join(f[1:], " ")
			_ = s.Store.SaveProfile(profile)
			fmt.Fprint(w, "OK\r\n")
		case "HOMEBBS":
			if len(f) < 2 {
				fmt.Fprintf(w, "Home BBS: %s\r\n", profile.HomeBBS)
				continue
			}
			profile.HomeBBS = strings.ToUpper(f[1])
			if e := s.Store.SaveProfile(profile); e != nil {
				fmt.Fprintf(w, "Error: %v\r\n", e)
			} else {
				fmt.Fprint(w, "OK\r\n")
			}
		case "QTH":
			if len(f) < 2 {
				fmt.Fprintf(w, "QTH: %s\r\n", profile.QTH)
				continue
			}
			profile.QTH = strings.Join(f[1:], " ")
			_ = s.Store.SaveProfile(profile)
			fmt.Fprint(w, "OK\r\n")
		case "LOC", "LOCATOR":
			if len(f) < 2 {
				fmt.Fprintf(w, "Locator: %s\r\n", profile.Locator)
				continue
			}
			profile.Locator = strings.ToUpper(f[1])
			_ = s.Store.SaveProfile(profile)
			fmt.Fprint(w, "OK\r\n")
		case "C", "T", "TALK", "CONV", "CONVERS":
			s.convers(call, in, w, lang)
		case "LANG", "LANGUAGE":
			if len(f) < 2 || (strings.ToUpper(f[1]) != "PL" && strings.ToUpper(f[1]) != "EN") {
				fmt.Fprint(w, language.T(lang, "lang_usage"))
				continue
			}
			lang = language.Normalize(f[1])
			_ = s.Store.SetLanguage(call, lang)
			profile.Language = lang
			_ = s.Store.SaveProfile(profile)
			fmt.Fprint(w, language.T(lang, "lang_set"))
		case "L", "LM":
			s.printList(w, call, false, lang)
		case "N", "NEW", "RN":
			ms := s.Store.ListNew(call)
			if len(ms) == 0 {
				fmt.Fprint(w, language.T(lang, "no_new"))
			} else {
				s.printMessages(w, call, ms)
			}
		case "LS":
			ms := s.Store.ListSent(call)
			if len(ms) == 0 {
				fmt.Fprint(w, language.T(lang, "no_messages"))
			} else {
				s.printMessages(w, call, ms)
			}
		case "LB":
			s.printList(w, call, true, lang)
		case "R", "READ":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_read"))
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e != nil {
				fmt.Fprint(w, language.T(lang, "invalid_id"))
				continue
			}
			m, e := s.Store.Read(call, id)
			if e != nil {
				fmt.Fprintf(w, language.T(lang, "error"), e)
			} else {
				to := m.To
				if m.At != "" {
					to += "@" + m.At
				} else if m.Distribution != "" {
					to += "@" + m.Distribution
				}
				identifier := "MID " + m.MID
				if m.BID != "" {
					identifier += " BID " + m.BID
				}
				fmt.Fprintf(w, "#%d %s from %s to %s %s %s\r\n%s\r\n", m.ID, m.Type, m.From, to, identifier, m.Subject, m.Body)
			}
		case "K", "KILL":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_kill"))
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e == nil {
				e = s.Store.Delete(call, id)
			}
			if e != nil {
				fmt.Fprintf(w, language.T(lang, "error"), e)
			} else {
				fmt.Fprint(w, language.T(lang, "deleted"))
			}
		case "S", "SP", "SEND":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_send"))
				continue
			}
			to := strings.ToUpper(f[1])
			if !strings.Contains(to, "@") {
				to = s.Store.AddressFor(to)
			}
			s.compose(in, w, call, "P", to, lang)
		case "ST":
			if len(f) < 2 {
				fmt.Fprint(w, "Usage: ST <call or call@BBS>\r\n")
				continue
			}
			to := strings.ToUpper(f[1])
			if !strings.Contains(to, "@") {
				to = s.Store.AddressFor(to)
			}
			s.compose(in, w, call, "T", to, lang)
		case "RE", "REPLY":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_reply"))
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e != nil {
				fmt.Fprint(w, language.T(lang, "invalid_id"))
				continue
			}
			m, e := s.Store.Read(call, id)
			if e != nil {
				fmt.Fprintf(w, language.T(lang, "error"), e)
				continue
			}
			s.composePreset(in, w, call, m.From, fmt.Sprintf(language.T(lang, "reply_subject"), m.Subject), lang)
		case "FS", "FSTATUS":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_forward"))
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e != nil {
				fmt.Fprint(w, language.T(lang, "invalid_id"))
				continue
			}
			m, e := s.Store.Read(call, id)
			if e != nil {
				fmt.Fprintf(w, language.T(lang, "error"), e)
				continue
			}
			if len(m.Forward) == 0 {
				fmt.Fprint(w, language.T(lang, "forward_none"))
				continue
			}
			for peer, st := range m.Forward {
				fmt.Fprintf(w, language.T(lang, "forward_line"), peer, st.Status, st.Attempts, st.LastError)
			}
		case "SB":
			to := "ALL"
			if len(f) > 1 {
				to = strings.ToUpper(f[1])
			}
			s.compose(in, w, call, "B", to, lang)
		case "B", "BYE", "Q", "QUIT":
			if strings.TrimSpace(s.GoodbyeMessage) == "" {
				fmt.Fprint(w, "73!\r\n")
			} else {
				fmt.Fprint(w, s.expandMessage(s.GoodbyeMessage, call), "\r\n")
			}
			return
		default:
			fmt.Fprint(w, language.T(lang, "unknown"))
		}
	}
}

func (s *Server) expandMessage(message, remote string) string {
	return strings.NewReplacer("{CALL}", s.Node, "{REMOTE}", remote, "{TITLE}", s.Title, "{SYSOP}", s.SysopCallsign, "{ADDRESS}", s.Address).Replace(message)
}

func (s *Server) acquireSession() bool {
	limit := s.MaxSessions
	if limit < 1 {
		limit = 10
	}
	for {
		current := s.activeSessions.Load()
		if current >= int32(limit) {
			return false
		}
		if s.activeSessions.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (s *Server) releaseSession() { s.activeSessions.Add(-1) }

func (s *Server) registerProfile(call, lang string, p UserProfile, in *bufio.Scanner, w io.Writer) UserProfile {
	p.Callsign = call
	if language.Normalize(lang) == "pl" {
		fmt.Fprint(w, "Pierwsze polaczenie - konfiguracja profilu.\r\nImie: ")
	} else {
		fmt.Fprint(w, "First connection - profile setup.\r\nName: ")
	}
	if in.Scan() {
		p.Name = strings.TrimSpace(in.Text())
	}
	if p.Name == "" {
		p.Name = call
	}
	if language.Normalize(lang) == "pl" {
		fmt.Fprintf(w, "Domyslny BBS [%s]: ", s.Address)
	} else {
		fmt.Fprintf(w, "Home BBS [%s]: ", s.Address)
	}
	if in.Scan() {
		p.HomeBBS = strings.ToUpper(strings.TrimSpace(in.Text()))
	}
	if p.HomeBBS == "" {
		p.HomeBBS = strings.ToUpper(strings.TrimSpace(s.Address))
	}
	if language.Normalize(lang) == "pl" {
		fmt.Fprint(w, "QTH (opcjonalnie, wpisz POMIN aby pominac): ")
	} else {
		fmt.Fprint(w, "QTH (optional, type SKIP to skip): ")
	}
	if in.Scan() {
		p.QTH = optionalProfileValue(in.Text())
	}
	if language.Normalize(lang) == "pl" {
		fmt.Fprint(w, "Locator (opcjonalnie, wpisz POMIN aby pominac): ")
	} else {
		fmt.Fprint(w, "Locator (optional, type SKIP to skip): ")
	}
	if in.Scan() {
		p.Locator = strings.ToUpper(optionalProfileValue(in.Text()))
	}
	p.Language, p.Completed = language.Normalize(lang), true
	if err := s.Store.SaveProfile(p); err != nil {
		p.Completed = false
		fmt.Fprintf(w, "Error: %v\r\n", err)
	} else if language.Normalize(lang) == "pl" {
		fmt.Fprint(w, "Profil zapisany.\r\n")
	} else {
		fmt.Fprint(w, "Profile saved.\r\n")
	}
	return p
}

func optionalProfileValue(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToUpper(value) {
	case "POMIN", "POMIŃ", "SKIP":
		return ""
	default:
		return value
	}
}
func (s *Server) printProfile(w io.Writer, p UserProfile) {
	fmt.Fprintf(w, "Call: %s\r\nName: %s\r\nHome BBS: %s\r\nQTH: %s\r\nLocator: %s\r\nLanguage: %s\r\n", p.Callsign, p.Name, p.HomeBBS, p.QTH, p.Locator, strings.ToUpper(p.Language))
}
func (s *Server) convers(call string, in *bufio.Scanner, w io.Writer, lang string) {
	c := &chatClient{call: call, w: w}
	s.chatMu.Lock()
	if s.chat == nil {
		s.chat = map[*chatClient]struct{}{}
	}
	s.chat[c] = struct{}{}
	s.chatMu.Unlock()
	s.chatBroadcast("* " + call + " joined CONV\r\n")
	defer func() {
		s.chatMu.Lock()
		delete(s.chat, c)
		s.chatMu.Unlock()
		s.chatBroadcast("* " + call + " left CONV\r\n")
	}()
	c.send("CONV: /WHO users, /EX leave\r\n")
	for {
		c.send("conv> ")
		if !in.Scan() {
			return
		}
		text := strings.TrimSpace(in.Text())
		if strings.EqualFold(text, "/EX") || strings.EqualFold(text, "/EXIT") {
			return
		}
		if strings.EqualFold(text, "/WHO") {
			s.chatMu.Lock()
			var calls []string
			for x := range s.chat {
				calls = append(calls, x.call)
			}
			s.chatMu.Unlock()
			sort.Strings(calls)
			c.send("Users: " + strings.Join(calls, ", ") + "\r\n")
			continue
		}
		if text != "" {
			s.chatBroadcast("[" + call + "] " + language.ASCII(text) + "\r\n")
		}
	}
}
func (s *Server) chatBroadcast(text string) {
	s.chatMu.Lock()
	all := make([]*chatClient, 0, len(s.chat))
	for c := range s.chat {
		all = append(all, c)
	}
	s.chatMu.Unlock()
	for _, c := range all {
		c.send(text)
	}
}
func (s *Server) printList(w io.Writer, call string, b bool, lang string) {
	ms := s.Store.List(call, b)
	if len(ms) == 0 {
		fmt.Fprint(w, language.T(lang, "no_messages"))
		return
	}
	s.printMessages(w, call, ms)
}
func (s *Server) printMessages(w io.Writer, call string, ms []Message) {
	for _, m := range ms {
		mark := "R"
		if !m.IsRead(call) && m.To == strings.ToUpper(call) {
			mark = "N"
		}
		to := m.To
		if m.At != "" {
			to += "@" + m.At
		} else if m.Distribution != "" {
			to += "@" + m.Distribution
		}
		fmt.Fprintf(w, "%5d %s %-1s %-9s %-20s %s\r\n", m.ID, mark, m.Type, m.From, to, m.Subject)
	}
}
func (s *Server) compose(in *bufio.Scanner, w io.Writer, from, kind, to, lang string) {
	fmt.Fprint(w, language.T(lang, "subject"))
	if !in.Scan() {
		return
	}
	sub := strings.TrimSpace(in.Text())
	fmt.Fprint(w, language.T(lang, "body"))
	var lines []string
	for in.Scan() {
		if strings.EqualFold(strings.TrimSpace(in.Text()), "/EX") {
			break
		}
		lines = append(lines, in.Text())
	}
	m, e := s.Store.Send(kind, from, to, sub, strings.Join(lines, "\r\n"))
	if e != nil {
		fmt.Fprintf(w, language.T(lang, "error"), e)
		return
	}
	if s.OnMessage != nil {
		s.OnMessage()
	}
	fmt.Fprintf(w, language.T(lang, "saved"), m.ID)
}

func (s *Server) composePreset(in *bufio.Scanner, w io.Writer, from, to, subject, lang string) {
	fmt.Fprintf(w, "%s%s\r\n", language.T(lang, "subject"), subject)
	fmt.Fprint(w, language.T(lang, "body"))
	var lines []string
	for in.Scan() {
		if strings.EqualFold(strings.TrimSpace(in.Text()), "/EX") {
			break
		}
		lines = append(lines, in.Text())
	}
	m, e := s.Store.Send("P", from, to, subject, strings.Join(lines, "\r\n"))
	if e != nil {
		fmt.Fprintf(w, language.T(lang, "error"), e)
		return
	}
	if s.OnMessage != nil {
		s.OnMessage()
	}
	fmt.Fprintf(w, language.T(lang, "saved"), m.ID)
}
