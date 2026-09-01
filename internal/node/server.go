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

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/language"
	"github.com/packet-radio/ultimatepr/internal/lineinput"
	"github.com/packet-radio/ultimatepr/internal/service"
)

type Server struct {
	Listen, Callsign, Alias, Language string
	WelcomeMessage, GoodbyeMessage    string
	Router                            *Router
	Registry                          *service.Registry
	Version                           string
	LanguageLookup                    func(string) string
	SetLanguage                       func(string, string) error
	Ports                             []string
	Log                               *slog.Logger
	// Connect bridges the current terminal to a resolved remote node or
	// station. It is kept outside the node package so routing owns no radio I/O.
	Connect func(string, Neighbor, Route, io.Reader, io.Writer) error
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
	s.serveWithContext(service.ServiceContext{Context: context.Background(), Reader: r, Writer: w, EntryType: service.EntryTCP})
}

func (s *Server) ServeContext(ctx service.ServiceContext) {
	s.serveWithContext(ctx)
}

func (s *Server) serveWithContext(ctx service.ServiceContext) {
	lang := language.Normalize(s.Language)
	stream := &singleByteReader{r: ctx.Reader}
	in := lineinput.NewScanner(stream)
	in.Buffer(make([]byte, 1024), 64*1024)
	fmt.Fprintf(ctx.Writer, "\r\n%s:%s NODE UltimatePR %s\r\n%s", s.Alias, s.Callsign, s.Version, language.T(lang, "node_call"))
	if ctx.EntryType == service.EntryAX25 || ctx.EntryType == service.EntryNode {
		s.serveCall(ctx.RemoteCall.String(), lang, in, stream, ctx)
		return
	}
	if !in.Scan() {
		return
	}
	call := strings.ToUpper(strings.TrimSpace(in.Text()))
	if parsed, err := ax25.ParseAddress(call); err == nil {
		ctx.RemoteCall = parsed
	}
	s.serveCall(call, lang, in, stream, ctx)
}

// ServeAX25 serves an already identified AX.25 station. Unlike Telnet, an
// AX.25 connected-mode session obtains the callsign from the link header and
// must not ask the operator to type it again.
func (s *Server) ServeAX25(call string, r io.Reader, w io.Writer) {
	remote, _ := ax25.ParseAddress(call)
	local, _ := ax25.ParseAddress(s.Callsign)
	s.ServeContext(service.ServiceContext{Context: context.Background(), LocalCall: local, RemoteCall: remote, Reader: r, Writer: w, EntryType: service.EntryAX25})
}

func remoteAddress(call string) ax25.Address { remote, _ := ax25.ParseAddress(call); return remote }

func (s *Server) serveCall(call, lang string, in *bufio.Scanner, stream io.Reader, ctx service.ServiceContext) {
	w := ctx.Writer
	if call == "" {
		fmt.Fprint(w, language.T(lang, "invalid_call"))
		return
	}
	if s.LanguageLookup != nil {
		if saved := s.LanguageLookup(call); saved != "" {
			lang = language.Normalize(saved)
		}
	}
	if strings.TrimSpace(s.WelcomeMessage) == "" {
		fmt.Fprintf(w, language.T(lang, "hello_node"), call)
	} else {
		fmt.Fprint(w, expandSessionMessage(s.WelcomeMessage, s.Callsign, call), "\r\n")
	}
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
			if s.SetLanguage != nil {
				_ = s.SetLanguage(call, lang)
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
		default:
			if !s.runService(cmd, call, lang, stream, ctx) {
				if s.Registry != nil && s.Registry.Has(cmd) {
					fmt.Fprint(w, language.T(lang, "bbs_unavailable"))
				} else {
					fmt.Fprint(w, language.T(lang, "unknown"))
				}
			}
		case "C", "CONNECT":
			if len(f) < 2 {
				fmt.Fprint(w, language.T(lang, "usage_connect"))
				continue
			}
			n, route, err := s.Router.Resolve(f[1])
			if err != nil {
				fmt.Fprintf(w, language.T(lang, "no_route"), strings.ToUpper(f[1]))
			} else if s.Connect != nil {
				if err := s.Connect(f[1], n, route, stream, w); err != nil {
					fmt.Fprintf(w, "CONNECT failed: %v\r\n", err)
				}
				return
			} else {
				fmt.Fprintf(w, language.T(lang, "route_found"), route.Destination, n.Callsign, n.Port, route.Quality)
			}
		case "B", "BYE", "Q", "QUIT":
			if strings.TrimSpace(s.GoodbyeMessage) == "" {
				fmt.Fprint(w, "73!\r\n")
			} else {
				fmt.Fprint(w, expandSessionMessage(s.GoodbyeMessage, s.Callsign, call), "\r\n")
			}
			return
		}
	}
}

func expandSessionMessage(message, local, remote string) string {
	return strings.NewReplacer("{CALL}", local, "{REMOTE}", remote).Replace(strings.TrimSpace(message))
}

// singleByteReader prevents the command scanner from reading past the command
// line. The remaining bytes must stay available when CONNECT switches to the
// transparent session bridge.
type singleByteReader struct{ r io.Reader }

func (r *singleByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n, err := r.r.Read(p[:1])
	return n, err
}
func (s *Server) runService(command, call, lang string, stream io.Reader, parent service.ServiceContext) bool {
	if s.Registry == nil {
		return false
	}
	registration, ok := s.Registry.ByAlias(command)
	if !ok {
		registration, ok = s.Registry.ByID(command)
	}
	if !ok || !registration.NodeVisible {
		return false
	}
	ctx := parent
	ctx.Reader, ctx.Writer, ctx.EntryType = stream, parent.Writer, service.EntryNode
	if ctx.Context == nil {
		ctx.Context = context.Background()
	}
	if ctx.RemoteCall.Callsign == "" {
		ctx.RemoteCall = remoteAddress(call)
	}
	_ = s.Registry.Serve(registration.Service.ID(), ctx)
	fmt.Fprint(parent.Writer, language.T(lang, "returned"))
	return true
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
	if s.Registry == nil {
		return
	}
	for _, registration := range s.Registry.ListNodeVisible() {
		fmt.Fprintf(w, "%-8s %-10s %s\r\n", strings.Join(registration.Aliases, ","), registration.Callsign.String(), registration.Service.ID())
	}
}
