package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/packet-radio/ultimatepr/internal/bbs"
	"github.com/packet-radio/ultimatepr/internal/language"
)

type Server struct {
	Listen, Callsign, Alias, Language string
	Router                            *Router
	BBS                               *bbs.Server
	Handlers                          map[string]func(string, string, *bufio.Scanner, io.Writer)
	Ports                             []string
	Log                               *slog.Logger
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
		wg.Add(1)
		go func() { defer wg.Done(); defer c.Close(); s.Serve(c, c) }()
	}
}
func (s *Server) Serve(r io.Reader, w io.Writer) {
	lang := language.Normalize(s.Language)
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1024), 64*1024)
	fmt.Fprintf(w, "\r\n%s:%s NODE UltimatePR %s\r\n%s", s.Alias, s.Callsign, bbs.BuildVersion, language.T(lang, "node_call"))
	if !in.Scan() {
		return
	}
	call := strings.ToUpper(strings.TrimSpace(in.Text()))
	s.serveCall(call, lang, in, w)
}

// ServeAX25 serves an already identified AX.25 station. Unlike Telnet, an
// AX.25 connected-mode session obtains the callsign from the link header and
// must not ask the operator to type it again.
func (s *Server) ServeAX25(call string, r io.Reader, w io.Writer) {
	lang := language.Normalize(s.Language)
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1024), 64*1024)
	fmt.Fprintf(w, "\r\n%s:%s NODE UltimatePR %s\r\n", s.Alias, s.Callsign, bbs.BuildVersion)
	s.serveCall(strings.ToUpper(strings.TrimSpace(call)), lang, in, w)
}

func (s *Server) serveCall(call, lang string, in *bufio.Scanner, w io.Writer) {
	if call == "" {
		fmt.Fprint(w, language.T(lang, "invalid_call"))
		return
	}
	if s.BBS != nil && s.BBS.Store != nil {
		if saved := s.BBS.Store.Language(call); saved != "" {
			lang = language.Normalize(saved)
		}
	}
	fmt.Fprintf(w, language.T(lang, "hello_node"), call)
	for {
		fmt.Fprintf(w, "%s> ", s.Alias)
		if !in.Scan() {
			return
		}
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		cmd := strings.ToUpper(f[0])
		switch cmd {
		case "?", "H", "HELP":
			fmt.Fprint(w, language.T(lang, "node_help"))
		case "LANG", "LANGUAGE":
			if len(f) < 2 || (strings.ToUpper(f[1]) != "PL" && strings.ToUpper(f[1]) != "EN") {
				fmt.Fprint(w, language.T(lang, "lang_usage"))
				continue
			}
			lang = language.Normalize(f[1])
			if s.BBS != nil && s.BBS.Store != nil {
				_ = s.BBS.Store.SetLanguage(call, lang)
			}
			if lang == "en" {
				fmt.Fprint(w, language.T(lang, "lang_en"))
			} else {
				fmt.Fprint(w, language.T(lang, "lang_pl"))
			}
		case "N", "NODES":
			s.nodes(w, lang)
		case "R", "ROUTES":
			s.routes(w, lang)
		case "P", "PORTS":
			for _, p := range s.Ports {
				fmt.Fprintf(w, "%-16s UP/CONFIGURED\r\n", p)
			}
		case "S", "SERVICES":
			s.services(w)
		case "BBS", "AI":
			if !s.runService(cmd, call, lang, in, w) {
				fmt.Fprint(w, language.T(lang, "bbs_unavailable"))
			}
		case "C", "CONNECT":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_connect"))
				continue
			}
			if service, ok := s.Router.Service(f[1]); ok && s.runService(service.Command, call, lang, in, w) {
				continue
			}
			n, route, err := s.Router.Resolve(f[1])
			if err != nil {
				fmt.Fprintf(w, language.T(lang, "no_route"), strings.ToUpper(f[1]))
			} else {
				fmt.Fprintf(w, language.T(lang, "route_found"), route.Destination, n.Callsign, n.Port, route.Quality)
			}
		case "B", "BYE", "Q", "QUIT":
			fmt.Fprint(w, "73!\r\n")
			return
		default:
			fmt.Fprint(w, language.T(lang, "unknown"))
		}
	}
}
func (s *Server) runService(command, call, lang string, in *bufio.Scanner, w io.Writer) bool {
	service, ok := s.Router.Service(command)
	if !ok {
		return false
	}
	if h := s.Handlers[strings.ToUpper(service.Command)]; h != nil {
		h(call, lang, in, w)
		fmt.Fprint(w, language.T(lang, "returned"))
		return true
	}
	if strings.EqualFold(service.Command, "BBS") && s.BBS != nil {
		s.BBS.ServeSessionLanguage(call, lang, in, w)
		fmt.Fprint(w, language.T(lang, "returned"))
		return true
	}
	return false
}
func (s *Server) nodes(w io.Writer, lang string) {
	ns := s.Router.Neighbors()
	if len(ns) == 0 {
		fmt.Fprint(w, language.T(lang, "no_neighbors"))
		return
	}
	for _, n := range ns {
		fmt.Fprintf(w, "%-10s %-10s PORT %-14s Q %3d\r\n", n.ID, n.Callsign, n.Port, n.Quality)
	}
}
func (s *Server) routes(w io.Writer, lang string) {
	rs := s.Router.Routes()
	if len(rs) == 0 {
		fmt.Fprint(w, language.T(lang, "no_routes"))
		return
	}
	for _, r := range rs {
		fmt.Fprintf(w, "%-10s VIA %-10s Q %3d\r\n", r.Destination, r.Via, r.Quality)
	}
}
func (s *Server) services(w io.Writer) {
	for _, x := range s.Router.Services() {
		fmt.Fprintf(w, "%-8s %-10s %s\r\n", x.Command, x.Callsign, x.Name)
	}
}
