package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/packet-radio/ultimatepr/internal/bbs"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	"github.com/packet-radio/ultimatepr/internal/transport"
)

//go:embed openapi.yaml docs.html
var assets embed.FS

type Token struct {
	Name, Hash string
	Scopes     []string
}
type SessionDTO struct {
	ID             string     `json:"id"`
	RemoteCallsign string     `json:"remote_callsign,omitempty"`
	Port           string     `json:"port,omitempty"`
	State          string     `json:"state"`
	Direction      string     `json:"direction,omitempty"`
	ConnectedSince *time.Time `json:"connected_since,omitempty"`
}
type NodeStatus struct {
	Enabled   bool `json:"enabled"`
	Neighbors int  `json:"neighbors,omitempty"`
	Routes    int  `json:"routes,omitempty"`
	Services  int  `json:"services,omitempty"`
}
type DigipeaterStatus struct {
	Enabled      bool       `json:"enabled"`
	Repeated     uint64     `json:"repeated_frames"`
	LastActivity *time.Time `json:"last_activity,omitempty"`
}

type Config struct {
	Callsign, Mode, Version string
	Tokens                  []Token
	Ports                   func() []transport.Status
	MHeard                  *mheard.Store
	Monitor                 *monitor.Store
	Sessions                func() []SessionDTO
	Node                    func() NodeStatus
	BBS                     func() *bbs.Store
	Digipeater              func() DigipeaterStatus
	Broker                  *Broker
}

type Server struct {
	cfg     Config
	log     *slog.Logger
	started time.Time
	tokens  []token
}
type token struct {
	name   string
	hash   [32]byte
	scopes map[string]bool
}

func New(cfg Config, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, log: log, started: time.Now()}
	if s.cfg.Broker == nil {
		s.cfg.Broker = NewBroker()
	}
	for _, in := range cfg.Tokens {
		b, err := hex.DecodeString(strings.TrimSpace(in.Hash))
		if err != nil || len(b) != 32 {
			continue
		}
		var h [32]byte
		copy(h[:], b)
		scopes := map[string]bool{}
		for _, scope := range in.Scopes {
			scopes[strings.TrimSpace(scope)] = true
		}
		s.tokens = append(s.tokens, token{name: in.Name, hash: h, scopes: scopes})
	}
	return s
}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/health", s.health)
	m.Handle("GET /api/v1/status", s.require("status.read", http.HandlerFunc(s.status)))
	m.Handle("GET /api/v1/ports", s.require("ports.read", http.HandlerFunc(s.ports)))
	m.Handle("GET /api/v1/ports/{id}", s.require("ports.read", http.HandlerFunc(s.port)))
	m.Handle("GET /api/v1/mheard", s.require("mheard.read", http.HandlerFunc(s.mheardList)))
	m.Handle("GET /api/v1/mheard/summary", s.require("mheard.read", http.HandlerFunc(s.mheardSummary)))
	m.Handle("GET /api/v1/mheard/{callsign}", s.require("mheard.read", http.HandlerFunc(s.mheardOne)))
	m.Handle("GET /api/v1/sessions", s.require("sessions.read", http.HandlerFunc(s.sessions)))
	m.Handle("GET /api/v1/monitor", s.require("monitor.read", http.HandlerFunc(s.monitor)))
	m.Handle("GET /api/v1/node/status", s.require("node.read", http.HandlerFunc(s.node)))
	m.Handle("GET /api/v1/bbs/status", s.require("bbs.read", http.HandlerFunc(s.bbsStatus)))
	m.Handle("GET /api/v1/digipeater/status", s.require("digipeater.read", http.HandlerFunc(s.digipeater)))
	m.Handle("GET /api/v1/events", s.require("events.read", http.HandlerFunc(s.events)))
	m.HandleFunc("GET /api/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		b, _ := assets.ReadFile("openapi.yaml")
		_, _ = w.Write(b)
	})
	m.HandleFunc("GET /api/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		b, _ := assets.ReadFile("docs.html")
		_, _ = w.Write(b)
	})
	return m
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (s *Server) require(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")) == "" {
			apiError(w, 401, "unauthorized", "Bearer token required")
			return
		}
		sum := sha256.Sum256([]byte(strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))))
		for _, t := range s.tokens {
			if subtle.ConstantTimeCompare(sum[:], t.hash[:]) == 1 {
				if !t.scopes[scope] && !t.scopes["admin"] {
					apiError(w, 403, "forbidden", "Token lacks required scope: "+scope)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
		}
		s.log.Warn("API authentication failed", "remote", r.RemoteAddr)
		apiError(w, 401, "unauthorized", "Invalid bearer token")
	})
}

func (s *Server) portStatuses() []transport.Status {
	if s.cfg.Ports == nil {
		return []transport.Status{}
	}
	return s.cfg.Ports()
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	issues := []string{}
	for _, p := range s.portStatuses() {
		if p.Enabled && !p.Connected {
			issues = append(issues, "port "+p.ID+" disconnected")
		}
	}
	status := "ok"
	if len(issues) > 0 {
		status = "degraded"
	}
	writeJSON(w, 200, map[string]any{"status": status, "issues": issues})
}
func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	ps := s.portStatuses()
	mh := []mheard.Entry{}
	if s.cfg.MHeard != nil {
		mh = s.cfg.MHeard.List()
	}
	frames := []monitor.Entry{}
	if s.cfg.Monitor != nil {
		frames = s.cfg.Monitor.List()
	}
	rx, tx := 0, 0
	for _, f := range frames {
		if strings.EqualFold(f.Direction, "RX") {
			rx++
		} else if strings.EqualFold(f.Direction, "TX") {
			tx++
		}
	}
	sessions := []SessionDTO{}
	if s.cfg.Sessions != nil {
		sessions = s.cfg.Sessions()
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "version": s.cfg.Version, "callsign": s.cfg.Callsign, "mode": s.cfg.Mode, "uptime_seconds": int64(time.Since(s.started).Seconds()), "active_sessions": len(sessions), "rx_frames": rx, "tx_frames": tx, "heard_stations": len(mh), "ports": ps})
}
func (s *Server) ports(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"items": s.portStatuses()})
}
func (s *Server) port(w http.ResponseWriter, r *http.Request) {
	for _, p := range s.portStatuses() {
		if p.ID == r.PathValue("id") {
			writeJSON(w, 200, p)
			return
		}
	}
	apiError(w, 404, "not_found", "Port not found")
}
func (s *Server) mheardList(w http.ResponseWriter, _ *http.Request) {
	v := []mheard.Entry{}
	if s.cfg.MHeard != nil {
		v = s.cfg.MHeard.List()
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (s *Server) mheardOne(w http.ResponseWriter, r *http.Request) {
	call := strings.ToUpper(r.PathValue("callsign"))
	if s.cfg.MHeard != nil {
		for _, e := range s.cfg.MHeard.List() {
			if strings.EqualFold(e.Callsign, call) {
				writeJSON(w, 200, e)
				return
			}
		}
	}
	apiError(w, 404, "not_found", "Station not found")
}
func (s *Server) mheardSummary(w http.ResponseWriter, _ *http.Request) {
	v := []mheard.Entry{}
	if s.cfg.MHeard != nil {
		v = s.cfg.MHeard.List()
	}
	now := time.Now()
	h1, h24 := 0, 0
	var last *mheard.Entry
	seen := map[string]bool{}
	for i := range v {
		e := &v[i]
		seen[e.Callsign] = true
		if now.Sub(e.LastSeen) <= time.Hour {
			h1++
		}
		if now.Sub(e.LastSeen) <= 24*time.Hour {
			h24++
		}
		if last == nil || e.LastSeen.After(last.LastSeen) {
			last = e
		}
	}
	out := map[string]any{"unique_stations": len(seen), "heard_1h": h1, "heard_24h": h24}
	if last != nil {
		out["last_callsign"] = last.Callsign
		out["last_seen"] = last.LastSeen
	}
	writeJSON(w, 200, out)
}
func (s *Server) sessions(w http.ResponseWriter, _ *http.Request) {
	v := []SessionDTO{}
	if s.cfg.Sessions != nil {
		v = s.cfg.Sessions()
	}
	writeJSON(w, 200, map[string]any{"items": v})
}
func (s *Server) monitor(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 {
			apiError(w, 400, "invalid_parameter", "limit must be a positive integer")
			return
		}
		limit = n
	}
	if limit > 500 {
		limit = 500
	}
	direction := strings.ToUpper(r.URL.Query().Get("direction"))
	port := r.URL.Query().Get("port")
	all := []monitor.Entry{}
	if s.cfg.Monitor != nil {
		all = s.cfg.Monitor.List()
	}
	out := make([]monitor.Entry, 0, limit)
	for _, e := range all {
		if direction != "" && strings.ToUpper(e.Direction) != direction {
			continue
		}
		if port != "" && e.Port != port {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, 200, map[string]any{"items": out, "limit": limit})
}
func (s *Server) node(w http.ResponseWriter, _ *http.Request) {
	v := NodeStatus{}
	if s.cfg.Node != nil {
		v = s.cfg.Node()
	}
	writeJSON(w, 200, v)
}
func (s *Server) bbsStatus(w http.ResponseWriter, _ *http.Request) {
	store := (*bbs.Store)(nil)
	if s.cfg.BBS != nil {
		store = s.cfg.BBS()
	}
	if store == nil {
		writeJSON(w, 200, map[string]any{"enabled": false})
		return
	}
	messages := store.Messages()
	pending := 0
	for _, m := range messages {
		for _, st := range m.Forward {
			if st.Status == "pending" || st.Status == "failed" {
				pending++
			}
		}
	}
	writeJSON(w, 200, map[string]any{"enabled": true, "messages": len(messages), "forward_queue": pending})
}
func (s *Server) digipeater(w http.ResponseWriter, _ *http.Request) {
	v := DigipeaterStatus{}
	if s.cfg.Digipeater != nil {
		v = s.cfg.Digipeater()
	}
	writeJSON(w, 200, v)
}

var eventsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return o == "" || o == "http://"+r.Host || o == "https://"+r.Host
}}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	// Subscribe before the websocket handshake completes so a publisher cannot
	// race the client immediately after Dial returns.
	ch, cancel := s.cfg.Broker.Subscribe(64)
	c, err := eventsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		cancel()
		return
	}
	defer cancel()
	defer c.Close()
	s.log.Info("API WebSocket connected", "remote", r.RemoteAddr)
	defer s.log.Info("API WebSocket disconnected", "remote", r.RemoteAddr)
	for e := range ch {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err = c.WriteJSON(e); err != nil {
			return
		}
	}
}
