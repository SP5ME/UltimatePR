package bbs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/packet-radio/ultimatepr/internal/language"
)

func taprSID() string { return "[UltimatePR-" + BuildVersion + "-H$]" }

type taprProposal struct {
	Kind, To, At, From, BID string
}

func parseTAPRSend(line string) (taprProposal, error) {
	f := strings.Fields(strings.ToUpper(strings.TrimSpace(line)))
	if len(f) < 2 || len(f[0]) != 2 || f[0][0] != 'S' || !strings.Contains("PBT", f[0][1:]) {
		return taprProposal{}, errors.New("invalid TAPR send command")
	}
	p := taprProposal{Kind: f[0][1:], To: f[1]}
	for i := 2; i < len(f); i++ {
		switch f[i] {
		case "@":
			i++
			if i >= len(f) {
				return taprProposal{}, errors.New("missing TAPR @ address")
			}
			p.At = f[i]
		case "<":
			i++
			if i >= len(f) {
				return taprProposal{}, errors.New("missing TAPR originator")
			}
			p.From = f[i]
		default:
			if strings.HasPrefix(f[i], "$") {
				p.BID = strings.TrimPrefix(f[i], "$")
			} else {
				return taprProposal{}, errors.New("unexpected TAPR send field")
			}
		}
	}
	if _, err := normalizeCall(p.From); err != nil {
		return taprProposal{}, errors.New("invalid TAPR originator")
	}
	if p.BID != "" && !validMessageIdentifier(p.BID) {
		return taprProposal{}, errors.New("invalid TAPR BID")
	}
	switch p.Kind {
	case "P", "T":
		if _, err := normalizeCall(p.To); err != nil {
			return taprProposal{}, err
		}
		if p.At != "" {
			if _, err := ParseHierarchicalAddress(p.At); err != nil {
				return taprProposal{}, err
			}
		}
		if p.Kind == "T" && p.BID != "" {
			return taprProposal{}, errors.New("TAPR NTS traffic must not carry BID")
		}
	case "B":
		if p.BID == "" {
			return taprProposal{}, errors.New("TAPR bulletin requires BID")
		}
		if _, err := ParseDistributionDesignator(p.At); err != nil {
			return taprProposal{}, err
		}
	}
	return p, nil
}

func formatTAPRSend(m Message) string {
	line := "S" + m.Type + " " + m.To
	if m.Type == "B" && m.Distribution != "" {
		line += " @ " + m.Distribution
	} else if m.At != "" {
		line += " @ " + m.At
	}
	line += " < " + stripSSID(m.From)
	if m.BID != "" {
		line += " $" + m.BID
	}
	return line
}

func (s *Server) serveTAPRForward(conn io.ReadWriter) error {
	r := bufio.NewReader(conn)
	if err := writeTAPRLine(conn, taprSID()); err != nil {
		return err
	}
	remoteSID, err := readTAPRLine(r)
	if err != nil {
		return err
	}
	if !isTAPRSID(remoteSID) {
		return errors.New("remote system did not send a TAPR SID")
	}
	if err := writeTAPRLine(conn, ">"); err != nil {
		return err
	}
	for {
		line, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		if line == "F>" {
			return nil
		}
		p, err := parseTAPRSend(line)
		if err != nil {
			return err
		}
		if p.BID != "" && s.Store.HasBID(p.BID) {
			if err := writeTAPRLine(conn, "NO"); err != nil {
				return err
			}
			if err := writeTAPRLine(conn, ">"); err != nil {
				return err
			}
			continue
		}
		if err := writeTAPRLine(conn, "OK"); err != nil {
			return err
		}
		subject, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		raw, err := readUntilTAPREOM(r)
		if err != nil {
			return err
		}
		routing, body := splitTAPRPayload(string(raw))
		m := Message{Type: p.Kind, From: p.From, To: p.To, At: p.At, MID: p.BID, BID: p.BID, Subject: subject, Body: body, Routing: routing}
		if p.Kind == "P" || p.Kind == "T" {
			m.BID = ""
		}
		if p.Kind == "B" {
			m.At, m.Distribution = "", p.At
		}
		if _, _, err := s.Store.Import(m); err != nil {
			return err
		}
		if err := writeTAPRLine(conn, ">"); err != nil {
			return err
		}
	}
}

func (f *Forwarder) exchangeTAPRMaster(conn io.ReadWriter, peerID string, queue []Message) error {
	r := bufio.NewReader(conn)
	sid, err := readTAPRLine(r)
	if err != nil {
		return err
	}
	if !isTAPRSID(sid) {
		return errors.New("remote system did not send a TAPR SID")
	}
	if err := writeTAPRLine(conn, taprSID()); err != nil {
		return err
	}
	if err := expectTAPRPrompt(r); err != nil {
		return err
	}
	for _, m := range queue {
		if err := writeTAPRLine(conn, formatTAPRSend(m)); err != nil {
			return err
		}
		response, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		fields := strings.Fields(strings.ToUpper(response))
		if len(fields) == 0 {
			return errors.New("empty TAPR response")
		}
		switch fields[0] {
		case "NO":
			if err := expectTAPRPrompt(r); err != nil {
				return err
			}
			if err := f.Store.RecordForward(peerID, m.ID, true, ""); err != nil {
				return err
			}
			continue
		case "OK":
		default:
			return fmt.Errorf("unexpected TAPR response %q", response)
		}
		if err := writeTAPRMessage(conn, m, f.LocalAddress); err != nil {
			return err
		}
		if err := expectTAPRPrompt(r); err != nil {
			return err
		}
		if err := f.Store.RecordForward(peerID, m.ID, true, ""); err != nil {
			return err
		}
	}
	return writeTAPRLine(conn, "F>")
}

func writeTAPRMessage(w io.Writer, m Message, localAddress string) error {
	if _, err := ParseHierarchicalAddress(localAddress); err != nil {
		return fmt.Errorf("local TAPR BBS address: %w", err)
	}
	if err := writeTAPRLine(w, language.ASCII(m.Subject)); err != nil {
		return err
	}
	number := ((m.ID - 1) % 65535) + 1
	header := fmt.Sprintf("R:%sZ %d@%s", time.Now().UTC().Format("060102/1504"), number, strings.ToUpper(localAddress))
	lines := append([]string{header}, m.Routing...)
	body := strings.ReplaceAll(language.ASCII(m.Body), "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r")
	_, err := io.WriteString(w, strings.Join(lines, "\r")+"\r\r"+body+"\r\x1a\r")
	return err
}

func splitTAPRPayload(raw string) ([]string, string) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.SplitN(raw, "\n\n", 2)
	if len(parts) != 2 {
		return nil, strings.Trim(raw, "\n")
	}
	var routing []string
	for _, line := range strings.Split(parts[0], "\n") {
		if strings.HasPrefix(line, "R:") {
			routing = append(routing, line)
		}
	}
	return routing, strings.Trim(parts[1], "\n")
}

func expectTAPRPrompt(r *bufio.Reader) error {
	line, err := readTAPRLine(r)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(strings.TrimSpace(line), ">") {
		return fmt.Errorf("expected TAPR prompt, got %q", line)
	}
	return nil
}

func isTAPRSID(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && strings.Contains(line, "H") && strings.Contains(line, "$")
}
