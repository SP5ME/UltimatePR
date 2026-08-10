package bbs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	Listen, Title, Node string
	Store               *Store
	Log                 *slog.Logger
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
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1024), 64*1024)
	line := func(prompt string) (string, bool) {
		fmt.Fprint(w, prompt)
		if !in.Scan() {
			return "", false
		}
		return strings.TrimSpace(in.Text()), true
	}
	fmt.Fprintf(w, "\r\n%s [%s]\r\nCallsign: ", s.Title, s.Node)
	if !in.Scan() {
		return
	}
	call, err := normalizeCall(in.Text())
	if err != nil {
		fmt.Fprint(w, "Invalid callsign.\r\n")
		return
	}
	if err = s.Store.Register(call); err != nil {
		fmt.Fprintln(w, "Database error.")
		return
	}
	fmt.Fprintf(w, "Hello %s. Type H for help.\r\n", call)
	for {
		raw, ok := line(s.Node + "> ")
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
			fmt.Fprint(w, "H help | L private list | LB bulletins | R <id> read | S <call> send | SB <topic> bulletin | K <id> delete | B bye\r\n")
		case "L", "LM":
			s.printList(w, call, false)
		case "LB":
			s.printList(w, call, true)
		case "R", "READ":
			if len(f) < 2 {
				fmt.Fprint(w, "Usage: R <id>\r\n")
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e != nil {
				fmt.Fprint(w, "Invalid id.\r\n")
				continue
			}
			m, e := s.Store.Read(call, id)
			if e != nil {
				fmt.Fprintf(w, "Error: %v\r\n", e)
			} else {
				fmt.Fprintf(w, "#%d %s from %s to %s %s\r\n%s\r\n", m.ID, m.Type, m.From, m.To, m.Subject, m.Body)
			}
		case "K", "KILL":
			if len(f) < 2 {
				fmt.Fprint(w, "Usage: K <id>\r\n")
				continue
			}
			id, e := strconv.ParseInt(f[1], 10, 64)
			if e == nil {
				e = s.Store.Delete(call, id)
			}
			if e != nil {
				fmt.Fprintf(w, "Error: %v\r\n", e)
			} else {
				fmt.Fprint(w, "Deleted.\r\n")
			}
		case "S", "SEND":
			if len(f) < 2 {
				fmt.Fprint(w, "Usage: S <callsign>\r\n")
				continue
			}
			s.compose(in, w, call, "P", strings.ToUpper(f[1]))
		case "SB":
			to := "ALL"
			if len(f) > 1 {
				to = strings.ToUpper(f[1])
			}
			s.compose(in, w, call, "B", to)
		case "B", "BYE", "Q", "QUIT":
			fmt.Fprint(w, "73!\r\n")
			return
		default:
			fmt.Fprint(w, "Unknown command. Type H.\r\n")
		}
	}
}
func (s *Server) printList(w io.Writer, call string, b bool) {
	ms := s.Store.List(call, b)
	if len(ms) == 0 {
		fmt.Fprint(w, "No messages.\r\n")
		return
	}
	for _, m := range ms {
		fmt.Fprintf(w, "%5d %-1s %-9s %-9s %s\r\n", m.ID, m.Type, m.From, m.To, m.Subject)
	}
}
func (s *Server) compose(in *bufio.Scanner, w io.Writer, from, kind, to string) {
	fmt.Fprint(w, "Subject: ")
	if !in.Scan() {
		return
	}
	sub := strings.TrimSpace(in.Text())
	fmt.Fprint(w, "Enter text, finish with /EX on a separate line.\r\n")
	var lines []string
	for in.Scan() {
		if strings.EqualFold(strings.TrimSpace(in.Text()), "/EX") {
			break
		}
		lines = append(lines, in.Text())
	}
	m, e := s.Store.Send(kind, from, to, sub, strings.Join(lines, "\r\n"))
	if e != nil {
		fmt.Fprintf(w, "Error: %v\r\n", e)
		return
	}
	fmt.Fprintf(w, "Message #%d saved.\r\n", m.ID)
}
