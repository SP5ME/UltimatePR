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

// TAPR requires $ (BID support) to be the final advertised feature. I carries
// a null station-identification line on transports such as TCP that do not
// otherwise identify the calling station.
func taprSID() string { return "[UltimatePR-" + BuildVersion + "-HI$]" }

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
	if p.From != "" {
		if _, err := normalizeCall(p.From); err != nil {
			return taprProposal{}, errors.New("invalid TAPR originator")
		}
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
	remoteCall := ""
	for {
		line, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		if line == "F>" {
			return s.sendTAPRReverse(conn, r, remoteCall)
		}
		if strings.HasPrefix(strings.TrimSpace(line), ";") {
			remoteCall = taprNullCall(line)
			if err := writeTAPRLine(conn, ">"); err != nil {
				return err
			}
			continue
		}
		p, err := parseTAPRSend(line)
		if err != nil {
			return err
		}
		if remoteCall != "" && !s.peerCanReceive(remoteCall) {
			if err := writeTAPRLine(conn, "NO forwarding receive disabled"); err != nil {
				return err
			}
			if err := writeTAPRLine(conn, ">"); err != nil {
				return err
			}
			continue
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
		from, err := taprOriginator(p, remoteCall)
		if err != nil {
			return err
		}
		m := Message{Type: p.Kind, From: from, To: p.To, At: p.At, MID: p.BID, BID: p.BID, Subject: subject, Body: body, Routing: routing}
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
	if hasTAPRFeature(sid, 'I') && strings.TrimSpace(f.LocalCall) != "" {
		if err := writeTAPRLine(conn, "; "+stripSSID(f.LocalCall)); err != nil {
			return err
		}
		if err := expectTAPRPrompt(r); err != nil {
			return err
		}
	}
	for _, m := range queue {
		if err := writeTAPRLine(conn, formatTAPRSend(m)); err != nil {
			return err
		}
		response, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		switch taprResponse(response) {
		case 'N':
			if err := expectTAPRPrompt(r); err != nil {
				return err
			}
			if err := f.Store.RecordForward(peerID, m.ID, true, ""); err != nil {
				return err
			}
			continue
		case 'O':
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
	if err := writeTAPRLine(conn, "F>"); err != nil {
		return err
	}
	return f.receiveTAPRReverse(conn, r, peerID)
}

// receiveTAPRReverse performs section 3 of the TAPR BBS specification. After
// the original master sends F>, it accepts proposals from the original slave;
// every acceptance or rejection is acknowledged with F>, not a normal prompt.
func (f *Forwarder) receiveTAPRReverse(conn io.ReadWriter, r *bufio.Reader, peerID string) error {
	peerCall := ""
	for _, peer := range f.Peers {
		if peer.ID == peerID {
			peerCall = peer.Callsign
			if !peer.CanReceive() {
				return f.rejectTAPRReverse(conn, r)
			}
			break
		}
	}
	for {
		line, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		if line == "F>" {
			return nil
		}
		proposal, err := parseTAPRSend(line)
		if err != nil {
			return err
		}
		if proposal.BID != "" && f.Store.HasBID(proposal.BID) {
			if err := writeTAPRLine(conn, "NO"); err != nil {
				return err
			}
			if err := writeTAPRLine(conn, "F>"); err != nil {
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
		from, err := taprOriginator(proposal, peerCall)
		if err != nil {
			return err
		}
		m := Message{Type: proposal.Kind, From: from, To: proposal.To, At: proposal.At, MID: proposal.BID, BID: proposal.BID, Subject: subject, Body: body, Routing: routing}
		if m.Type == "P" || m.Type == "T" {
			m.BID = ""
		}
		if m.Type == "B" {
			m.At, m.Distribution = "", proposal.At
		}
		if _, _, err := f.Store.Import(m); err != nil {
			return err
		}
		if err := writeTAPRLine(conn, "F>"); err != nil {
			return err
		}
	}
}

func (f *Forwarder) rejectTAPRReverse(conn io.Writer, r *bufio.Reader) error {
	for {
		line, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		if line == "F>" {
			return nil
		}
		if _, err := parseTAPRSend(line); err != nil {
			return err
		}
		if err := writeTAPRLine(conn, "NO forwarding receive disabled"); err != nil {
			return err
		}
		if err := writeTAPRLine(conn, "F>"); err != nil {
			return err
		}
	}
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

// sendTAPRReverse makes the original slave the sender after an F> from the
// original master. TCP lacks AX.25 caller identity, so queues are released
// only after the peer supplied a TAPR I-feature null identification line.
func (s *Server) sendTAPRReverse(w io.Writer, r *bufio.Reader, remoteCall string) error {
	peerID := s.peerIDForTAPRCall(remoteCall)
	if peerID == "" || !s.peerCanSend(remoteCall) {
		return writeTAPRLine(w, "F>")
	}
	limit := s.MaxForwardMessages
	if limit < 1 {
		limit = 50
	}
	if err := PrepareQueues(s.Store, s.ForwardPeers, limit); err != nil {
		return err
	}
	queue := s.Store.ForwardQueue(peerID, limit)
	for _, m := range queue {
		if err := writeTAPRLine(w, formatTAPRSend(m)); err != nil {
			return err
		}
		response, err := readTAPRLine(r)
		if err != nil {
			return err
		}
		switch taprResponse(response) {
		case 'N':
		case 'O':
			if err := writeTAPRMessage(w, m, s.Address); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected TAPR reverse response %q", response)
		}
		if err := expectTAPRReverse(r); err != nil {
			return err
		}
		if err := s.Store.RecordForward(peerID, m.ID, true, ""); err != nil {
			return err
		}
	}
	return writeTAPRLine(w, "F>")
}

func (s *Server) peerIDForTAPRCall(call string) string {
	call = stripSSID(call)
	if call == "" {
		return ""
	}
	for _, peer := range s.ForwardPeers {
		if strings.EqualFold(stripSSID(peer.Callsign), call) {
			return peer.ID
		}
	}
	return ""
}

func (s *Server) peerCanSend(call string) bool {
	for _, peer := range s.ForwardPeers {
		if strings.EqualFold(stripSSID(peer.Callsign), stripSSID(call)) {
			return peer.CanSend()
		}
	}
	return false
}

func (s *Server) peerCanReceive(call string) bool {
	for _, peer := range s.ForwardPeers {
		if strings.EqualFold(stripSSID(peer.Callsign), stripSSID(call)) {
			return peer.CanReceive()
		}
	}
	return true
}

func taprNullCall(line string) string {
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ";")))
	if len(fields) == 0 {
		return ""
	}
	call, err := normalizeCall(fields[0])
	if err != nil {
		return ""
	}
	return stripSSID(call)
}

func taprOriginator(p taprProposal, fallback string) (string, error) {
	from := p.From
	if from == "" {
		from = fallback
	}
	if from == "" {
		return "", errors.New("TAPR message has no originator")
	}
	return normalizeCall(from)
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

func expectTAPRReverse(r *bufio.Reader) error {
	line, err := readTAPRLine(r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "F>" {
		return fmt.Errorf("expected TAPR reverse acknowledgment, got %q", line)
	}
	return nil
}

func taprResponse(line string) byte {
	line = strings.TrimSpace(strings.ToUpper(line))
	if line == "" {
		return 0
	}
	if line[0] != 'O' && line[0] != 'N' {
		return 0
	}
	if len(line) > 1 && line[1] != ' ' && line != "OK" && line != "NO" {
		return 0
	}
	return line[0]
}

func hasTAPRFeature(sid string, feature byte) bool {
	sid = strings.TrimSpace(sid)
	if len(sid) < 5 || sid[0] != '[' || sid[len(sid)-1] != ']' {
		return false
	}
	part := sid[1 : len(sid)-1]
	lastDash := strings.LastIndex(part, "-")
	if lastDash < 1 {
		return false
	}
	features := strings.ToUpper(part[lastDash+1:])
	return strings.ContainsRune(features, rune(feature))
}

func isTAPRSID(line string) bool {
	line = strings.TrimSpace(line)
	return hasTAPRFeature(line, 'H') && hasTAPRFeature(line, '$')
}
