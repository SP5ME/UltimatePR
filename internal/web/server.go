package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/packet-radio/ultimatepr/internal/bbs"
	appconfig "github.com/packet-radio/ultimatepr/internal/config"
	"github.com/packet-radio/ultimatepr/internal/history"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	"github.com/packet-radio/ultimatepr/internal/session"
	"github.com/packet-radio/ultimatepr/internal/terminalcodec"
	"github.com/packet-radio/ultimatepr/internal/transport"
)

//go:embed static/*
var assets embed.FS

type Config struct {
	Listen             string
	Username           string
	PasswordHash       string
	AllowedAddresses   []string
	NodeCallsign       string
	NodeSSID           uint8
	BBSCallsign        string
	BSSSID             uint8
	TerminalCallsign   string
	TerminalSSID       uint8
	OperatorName       string
	ApplicationLocator string
	ApplicationQTH     string
	TerminalWelcome    string
	TerminalGoodbye    string
	TerminalInfo       string
	Ports              []string
	PortStatus         func() []transport.Status
	NodeEnabled        bool
	Radio              *session.Hub
	MHeard             *mheard.Store
	History            *history.Store
	Monitor            *monitor.Store
	SendBeacon         func(context.Context) error
	BBSListen          string
	BBS                *bbs.Store
	ConfigPath         string
	ReconnectPort      func(string) error
	RequestRestart     func()
	Version            string
}

type Server struct {
	cfg         Config
	log         *slog.Logger
	started     time.Time
	wsClients   atomic.Int64
	notifyMu    sync.Mutex
	notifySeq   uint64
	notify      map[uint64]chan notification
	incomingSeq atomic.Uint64
	incomingMu  sync.Mutex
	incoming    map[uint64]*operatorSession
	authMu      sync.RWMutex
}

type notification struct {
	Type    string `json:"type"`
	Call    string `json:"call"`
	Service string `json:"service"`
	Port    string `json:"port"`
	ID      uint64 `json:"id,omitempty"`
}

type operatorSession struct {
	id      uint64
	call    string
	r       io.Reader
	w       io.Writer
	mu      sync.Mutex
	done    chan struct{}
	claimed chan struct{}
	once    sync.Once
}

func (s *operatorSession) close() {
	s.once.Do(func() {
		if c, ok := s.r.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := s.w.(io.Closer); ok {
			_ = c.Close()
		}
		close(s.done)
	})
}

func New(cfg Config, log *slog.Logger) *Server {
	return &Server{cfg: cfg, log: log, started: time.Now(), notify: make(map[uint64]chan notification), incoming: make(map[uint64]*operatorSession)}
}

// HasActiveBrowser reports whether at least one authenticated application page
// currently has its persistent notification connection open.
func (s *Server) HasActiveBrowser() bool {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	return len(s.notify) > 0
}

// ServeOperatorAX25 exposes a radio connection addressed specifically to the
// operator station in the web terminal. NODE and BBS links use other handlers.
func (s *Server) ServeOperatorAX25(remote string, r io.Reader, w io.Writer) {
	id := s.incomingSeq.Add(1)
	in := &operatorSession{id: id, call: remote, r: r, w: w, done: make(chan struct{}), claimed: make(chan struct{})}
	s.incomingMu.Lock()
	s.incoming[id] = in
	s.incomingMu.Unlock()
	e := notification{Type: "incoming", Call: remote, Service: "TERMINAL", ID: id}
	s.notifyMu.Lock()
	for _, ch := range s.notify {
		select {
		case ch <- e:
		default:
		}
	}
	s.notifyMu.Unlock()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-in.done:
	case <-timer.C:
		in.close()
	case <-in.claimed:
		<-in.done
	}
	s.incomingMu.Lock()
	delete(s.incoming, id)
	s.incomingMu.Unlock()
}

func (s *Server) claimOperatorSession(id uint64) *operatorSession {
	s.incomingMu.Lock()
	defer s.incomingMu.Unlock()
	in := s.incoming[id]
	delete(s.incoming, id)
	if in != nil {
		close(in.claimed)
	}
	return in
}

func (s *Server) Run(ctx context.Context) error {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("PUT /api/application/password", s.changePassword)
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/mheard", s.mheard)
	mux.HandleFunc("GET /api/history", s.historyList)
	mux.HandleFunc("DELETE /api/history", s.historyClear)
	mux.HandleFunc("GET /api/history/{key}", s.historyGet)
	mux.HandleFunc("DELETE /api/history/{key}", s.historyDelete)
	mux.HandleFunc("GET /api/monitor", s.monitor)
	mux.HandleFunc("DELETE /api/monitor", s.monitorClear)
	mux.HandleFunc("POST /api/beacon", s.beacon)
	mux.HandleFunc("GET /ws/terminal", s.terminal)
	mux.HandleFunc("GET /ws/notifications", s.notifications)
	mux.HandleFunc("GET /api/bbs/messages", s.bbsList)
	mux.HandleFunc("POST /api/bbs/messages", s.bbsSend)
	mux.HandleFunc("GET /api/bbs/messages/{id}", s.bbsRead)
	mux.HandleFunc("DELETE /api/bbs/messages/{id}", s.bbsDelete)
	mux.HandleFunc("GET /api/config", s.configGet)
	mux.HandleFunc("PUT /api/config", s.configPut)
	mux.HandleFunc("GET /api/config/backup", s.configBackup)
	mux.HandleFunc("POST /api/config/restore", s.configRestore)
	mux.HandleFunc("GET /api/update", s.updateStatus)
	mux.HandleFunc("GET /api/update/status", s.updateJobStatus)
	mux.HandleFunc("PUT /api/update/channel", s.updateChannel)
	mux.HandleFunc("POST /api/update/apply", s.updateApply)
	mux.HandleFunc("GET /api/config/model", s.configModelGet)
	mux.HandleFunc("PUT /api/config/model", s.configModelPut)
	mux.HandleFunc("POST /api/ports/{id}/reconnect", s.portReconnect)

	h := securityHeaders(s.allowAddresses(s.authenticate(mux)))
	srv := &http.Server{Addr: s.cfg.Listen, Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { s.log.Info("web listening", "address", "http://"+s.cfg.Listen); done <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	s.notifyMu.Lock()
	s.notifySeq++
	id := s.notifySeq
	ch := make(chan notification, 8)
	s.notify[id] = ch
	s.notifyMu.Unlock()
	defer func() { s.notifyMu.Lock(); delete(s.notify, id); s.notifyMu.Unlock() }()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-ch:
			if err := ws.WriteJSON(e); err != nil {
				return
			}
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) historyList(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.History == nil {
		_ = json.NewEncoder(w).Encode([]history.Conversation{})
		return
	}
	_ = json.NewEncoder(w).Encode(s.cfg.History.List())
}
func (s *Server) historyGet(w http.ResponseWriter, r *http.Request) {
	if s.cfg.History == nil {
		http.NotFound(w, r)
		return
	}
	c, ok := s.cfg.History.Get(r.PathValue("key"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}
func (s *Server) historyDelete(w http.ResponseWriter, r *http.Request) {
	if s.cfg.History == nil {
		http.NotFound(w, r)
		return
	}
	if err := s.cfg.History.Delete(r.PathValue("key")); err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) historyClear(w http.ResponseWriter, r *http.Request) {
	if s.cfg.History == nil {
		http.NotFound(w, r)
		return
	}
	if err := s.cfg.History.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) monitor(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.Monitor == nil {
		_ = json.NewEncoder(w).Encode([]monitor.Entry{})
		return
	}
	_ = json.NewEncoder(w).Encode(s.cfg.Monitor.List())
}
func (s *Server) monitorClear(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.Monitor != nil {
		s.cfg.Monitor.Clear()
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) beacon(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SendBeacon == nil {
		http.Error(w, "Beacon unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := s.cfg.SendBeacon(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) mheard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.MHeard == nil {
		_ = json.NewEncoder(w).Encode([]mheard.Entry{})
		return
	}
	_ = json.NewEncoder(w).Encode(s.cfg.MHeard.List())
}

func (s *Server) configModelGet(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}
func (s *Server) configModelPut(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	var c appconfig.Config
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&c); err != nil {
		http.Error(w, "Invalid configuration", http.StatusBadRequest)
		return
	}
	current, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Web.PasswordHash = current.Web.PasswordHash
	if err := validateWebListener(current.Web.Listen, c.Web.Listen); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := appconfig.SaveModel(s.cfg.ConfigPath, c); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.log.Info("configuration model saved", "path", s.cfg.ConfigPath)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"saved": true, "restart_required": true})
	if s.cfg.RequestRestart != nil {
		go func() {
			time.Sleep(250 * time.Millisecond)
			s.cfg.RequestRestart()
		}()
	}
}

func (s *Server) configGet(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(b)
}
func (s *Server) configBackup(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ultimatepr-config-backup.yaml"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}
func (s *Server) configRestore(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Configuration too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err = validateWebConfigChange(s.cfg.ConfigPath, b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = appconfig.Save(s.cfg.ConfigPath, b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.log.Info("configuration restored", "path", s.cfg.ConfigPath)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"saved":true,"restart_required":true}`))
	if s.cfg.RequestRestart != nil {
		go func() { time.Sleep(250 * time.Millisecond); s.cfg.RequestRestart() }()
	}
}
func (s *Server) configPut(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ConfigPath == "" {
		http.Error(w, "Configuration unavailable", http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/yaml") {
		http.Error(w, "Content-Type must be application/yaml", http.StatusUnsupportedMediaType)
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Configuration too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err = validateWebConfigChange(s.cfg.ConfigPath, b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err = appconfig.Save(s.cfg.ConfigPath, b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.log.Info("configuration saved", "path", s.cfg.ConfigPath)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"saved": true, "restart_required": true})
}

func (s *Server) SetBBS(store *bbs.Store) { s.cfg.BBS = store }

func validateWebConfigChange(path string, raw []byte) error {
	current, err := appconfig.Load(path)
	if err != nil {
		return err
	}
	candidate, err := appconfig.Parse(raw)
	if err != nil {
		return err
	}
	return validateWebListener(current.Web.Listen, candidate.Web.Listen)
}

func validateWebListener(current, candidate string) error {
	if candidate == current {
		return nil
	}
	currentHost, currentPort, currentErr := net.SplitHostPort(current)
	candidateHost, candidatePort, candidateErr := net.SplitHostPort(candidate)
	if candidateErr != nil {
		return fmt.Errorf("panel WWW: nieprawidłowy adres %q: %w", candidate, candidateErr)
	}
	if currentErr == nil && currentPort == candidatePort {
		if candidateHost == "" || candidateHost == "0.0.0.0" || candidateHost == "::" || candidateHost == currentHost {
			return nil
		}
		if _, err := net.ResolveTCPAddr("tcp", candidate); err != nil {
			return fmt.Errorf("panel WWW nie może użyć adresu %q: %w", candidate, err)
		}
		return nil
	}
	listener, err := net.Listen("tcp", candidate)
	if err != nil {
		return fmt.Errorf("panel WWW nie może otworzyć adresu %q; ustawienia nie zostały zapisane: %w", candidate, err)
	}
	_ = listener.Close()
	return nil
}

type bbsRequest struct {
	Call    string `json:"call"`
	Type    string `json:"type"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (s *Server) bbsList(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil {
		http.Error(w, "BBS disabled", http.StatusServiceUnavailable)
		return
	}
	call := r.URL.Query().Get("call")
	if err := s.cfg.BBS.Register(call); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.cfg.BBS.List(call, r.URL.Query().Get("type") == "B"))
}
func (s *Server) bbsSend(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil {
		http.Error(w, "BBS disabled", http.StatusServiceUnavailable)
		return
	}
	var q bbsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&q); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	m, err := s.cfg.BBS.Send(strings.ToUpper(q.Type), q.Call, q.To, q.Subject, q.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(m)
}
func (s *Server) bbsRead(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil {
		http.Error(w, "BBS disabled", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}
	m, err := s.cfg.BBS.Read(r.URL.Query().Get("call"), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}
func (s *Server) bbsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil {
		http.Error(w, "BBS disabled", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		err = s.cfg.BBS.Delete(r.URL.Query().Get("call"), id)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; style-src 'self'; script-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	ports := []transport.Status{}
	if s.cfg.PortStatus != nil {
		ports = s.cfg.PortStatus()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version": s.cfg.Version, "node": callsign(s.cfg.NodeCallsign, s.cfg.NodeSSID), "bbs": callsign(s.cfg.BBSCallsign, s.cfg.BSSSID), "terminal": callsign(s.cfg.TerminalCallsign, s.cfg.TerminalSSID),
		"ports": s.cfg.Ports, "uptime_seconds": int(time.Since(s.started).Seconds()), "terminal_clients": s.wsClients.Load(),
		"bbs_enabled": s.cfg.BBSListen != "", "node_enabled": s.cfg.NodeEnabled, "port_status": ports,
	})
}

func (s *Server) portReconnect(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReconnectPort == nil {
		http.Error(w, "port reconnect is unavailable", http.StatusNotImplemented)
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		http.Error(w, "port id is required", http.StatusBadRequest)
		return
	}
	if err := s.cfg.ReconnectPort(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize: 4096, WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "http://"+r.Host || r.Header.Get("Origin") == "https://"+r.Host
	},
}

type clientMessage struct {
	Type     string `json:"type"`
	Mode     string `json:"mode"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TNC      string `json:"tnc"`
	Target   string `json:"target"`
	Digi     string `json:"digi"`
	Data     string `json:"data"`
	Encoding string `json:"encoding"`
	Inbound  uint64 `json:"inbound"`
	ID       string `json:"id"`
}
type serverMessage struct {
	Type   string `json:"type"`
	State  string `json:"state,omitempty"`
	Data   string `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
	ID     string `json:"id,omitempty"`
	Packet int    `json:"packet,omitempty"`
	Total  int    `json:"total,omitempty"`
}

func terminalMessageFromEvent(e session.Event) serverMessage {
	codec, _ := terminalcodec.New(terminalcodec.Default)
	return terminalMessageFromEventWithCodec(e, codec)
}

func terminalMessageFromEventWithCodec(e session.Event, codec *terminalcodec.Codec) serverMessage {
	msg := serverMessage{Type: e.Type, State: string(e.State)}
	if len(e.Data) > 0 {
		msg.Data = codec.Decode(e.Data)
	} else {
		msg.Data = e.Message
	}
	return msg
}

type safeWS struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *safeWS) write(v serverMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func (s *Server) terminal(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	out := &safeWS{conn: ws}
	s.wsClients.Add(1)
	defer s.wsClients.Add(-1)
	ws.SetReadLimit(16 * 1024)
	_ = out.write(serverMessage{Type: "state", State: "idle", Data: "Terminal gotowy. Wybierz TNC / Radio lub lokalny BBS.\r\n"})
	var remote net.Conn
	activeMode := ""
	historyStation, historyPort, historyDigi := "", "", ""
	historyConnected := false
	var historySession uint64
	var cancelRadio func()
	var radioSession *session.Manager
	var releaseRadio func()
	var cancelKeepAlive context.CancelFunc
	var incoming *operatorSession
	terminalCodec, _ := terminalcodec.New(terminalcodec.Default)
	terminalTXCodec, _ := terminalcodec.New(terminalcodec.Default)
	remoteCommandBuffer := &terminalLineBuffer{}
	var sendMu sync.Mutex
	terminalCall := callsign(s.cfg.TerminalCallsign, s.cfg.TerminalSSID)
	expandReply := func(template, remote string) string {
		return terminalReplyText(template, terminalMacroContext(terminalCall, remote, s.cfg))
	}
	terminalWelcome := s.cfg.TerminalWelcome
	terminalGoodbye := s.cfg.TerminalGoodbye
	terminalInfo := s.cfg.TerminalInfo
	handleRemoteCommand := func(text string, reply func(string)) {
		for _, line := range remoteCommandBuffer.Push(text) {
			switch terminalRemoteCommand(line) {
			case "mheard":
				if s.cfg.MHeard != nil {
					reply(formatMHeardResponse(s.cfg.MHeard.List()))
				} else {
					reply("Brak odebranych stacji.\r\n")
				}
			case "info":
				reply(expandReply(terminalInfo, historyStation))
			case "help":
				reply(terminalHelpResponse())
			}
		}
	}
	sendIncomingReply := func(text string, id string, remote string) {
		text = expandReply(text, remote)
		if text == "" || incoming == nil {
			return
		}
		if id == "" {
			id = fmt.Sprintf("auto-%d", time.Now().UnixNano())
		}
		wireData, encodeErr := terminalTXCodec.Encode(text)
		if encodeErr != nil {
			_ = out.write(serverMessage{Type: "error", Error: encodeErr.Error()})
			return
		}
		_ = out.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "sending"})
		incoming.mu.Lock()
		if _, err := incoming.w.Write(wireData); err != nil {
			incoming.mu.Unlock()
			_ = out.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "error", Error: err.Error()})
			_ = out.write(serverMessage{Type: "error", Error: err.Error()})
			return
		}
		incoming.mu.Unlock()
		_ = out.write(serverMessage{Type: "tx_packet", ID: id, Packet: 1, Total: 1, Data: text, State: "sent"})
	}
	sendRadioReply := func(text string) {
		text = terminalResponseText(text)
		if text == "" || radioSession == nil {
			return
		}
		wireData, encodeErr := terminalTXCodec.Encode(text)
		if encodeErr != nil {
			_ = out.write(serverMessage{Type: "error", Error: encodeErr.Error()})
			return
		}
		txID := fmt.Sprintf("auto-%d", time.Now().UnixNano())
		progress := func(p session.SendPacketProgress) {
			_ = out.write(serverMessage{Type: "tx_packet", ID: txID, Packet: p.Packet, Total: p.Total, Data: terminalTXCodec.Decode(p.Data), State: p.State, Error: p.Error})
		}
		go func() {
			sendMu.Lock()
			if err := radioSession.SendWithProgress(r.Context(), wireData, progress); err != nil {
				sendMu.Unlock()
				_ = out.write(serverMessage{Type: "error", Error: err.Error()})
				return
			}
			sendMu.Unlock()
		}()
	}
	remoteDone := make(chan struct{}, 1)
	closeRemote := func() {
		if remote != nil {
			_ = remote.Close()
			remote = nil
		}
		select {
		case remoteDone <- struct{}{}:
		default:
		}
	}
	defer closeRemote()
	defer func() {
		if incoming != nil {
			incoming.close()
		}
	}()
	defer func() {
		if cancelRadio != nil {
			cancelRadio()
		}
	}()
	defer func() {
		if cancelKeepAlive != nil {
			cancelKeepAlive()
		}
		if radioSession != nil && radioSession.State() != session.Disconnected {
			closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = radioSession.Disconnect(closeCtx)
			cancel()
		}
	}()
	defer func() {
		if releaseRadio != nil {
			releaseRadio()
		}
	}()
	defer func() {
		if s.cfg.History != nil {
			s.cfg.History.Disconnected(historySession)
		}
	}()
	for {
		var m clientMessage
		if err := ws.ReadJSON(&m); err != nil {
			return
		}
		switch m.Type {
		case "connect":
			if s.cfg.History != nil {
				s.cfg.History.Disconnected(historySession)
			}
			historySession = 0
			selectedCodec, _ := terminalcodec.New(terminalcodec.Default)
			terminalCodec = selectedCodec
			terminalTXCodec, _ = terminalcodec.New(terminalcodec.Default)
			// Keep the operator workflow in TNC/Radio, but short-circuit a call
			// addressed to this server's own BBS. No AX.25 frame reaches the TNC.
			localBBS := callsign(s.cfg.BBSCallsign, s.cfg.BSSSID)
			if m.Mode == "tnc" && s.cfg.BBSListen != "" && strings.EqualFold(strings.TrimSpace(m.Target), localBBS) {
				m.Mode = "bbs"
			}
			if m.Mode == "tnc" && strings.TrimSpace(m.Target) == "" {
				_ = out.write(serverMessage{Type: "state", State: "error", Error: "Podaj znak korespondenta."})
				continue
			}
			closeRemote()
			if cancelRadio != nil {
				cancelRadio()
				cancelRadio = nil
			}
			if releaseRadio != nil {
				if cancelKeepAlive != nil {
					cancelKeepAlive()
					cancelKeepAlive = nil
				}
				if radioSession != nil && radioSession.State() != session.Disconnected {
					closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					_ = radioSession.Disconnect(closeCtx)
					cancel()
				}
				releaseRadio()
				releaseRadio = nil
				radioSession = nil
			}
			activeMode = m.Mode
			historyConnected = false
			historyStation, historyPort, historyDigi = m.Target, m.TNC, strings.ToUpper(m.Digi)
			if m.Mode == "incoming" {
				incoming = s.claimOperatorSession(m.Inbound)
				if incoming == nil {
					_ = out.write(serverMessage{Type: "state", State: "error", Error: "Polaczenie przychodzace nie jest juz dostepne."})
					continue
				}
				historyStation, historyPort, historyDigi, historyConnected = incoming.call, "radio", "", true
				if s.cfg.History != nil {
					historySession = s.cfg.History.Connected("tnc", historyStation, historyPort, historyDigi)
				}
				_ = out.write(serverMessage{Type: "state", State: "connected", Data: "Polaczenie przychodzace od " + incoming.call + "\r\n"})
				sendIncomingReply(terminalWelcome, fmt.Sprintf("welcome-%d", time.Now().UnixNano()), incoming.call)
				go s.copyOperatorToWS(out, incoming, historyStation, terminalCodec, historySession)
				continue
			}
			if m.Mode == "tnc" {
				if s.cfg.Radio == nil {
					_ = out.write(serverMessage{Type: "state", State: "error", Error: "Session Manager AX.25 jest niedostepny."})
					continue
				}
				radioSession, releaseRadio = s.cfg.Radio.NewSession()
				keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())
				cancelKeepAlive = keepAliveCancel
				go radioSession.KeepAlive(keepAliveCtx, 5*time.Minute)
				events, cancel := radioSession.Subscribe()
				cancelRadio = cancel
				sessionCodec := terminalCodec
				go func() {
					initialState := true
					for e := range events {
						// A freshly created manager publishes its initial disconnected
						// snapshot before Connect advances to awaiting_connection. Sending
						// that snapshot to the browser makes it close the WebSocket and the
						// deferred cleanup immediately transmits DISC after SABM.
						if initialState {
							initialState = false
							if e.Type == "state" && e.State == session.Disconnected {
								continue
							}
						}
						if e.State == session.Connected && !historyConnected && s.cfg.History != nil {
							historyConnected = true
							historySession = s.cfg.History.Connected("tnc", historyStation, historyPort, historyDigi)
						}
						if e.Type == "data" && historyConnected && s.cfg.History != nil {
							s.cfg.History.Add("tnc", historyStation, historyPort, historyDigi, "rx", sessionCodec.Decode(e.Data))
						}
						if e.Type == "data" {
							handleRemoteCommand(sessionCodec.Decode(e.Data), sendRadioReply)
						}
						if e.State == session.Disconnected && strings.EqualFold(strings.TrimSpace(e.Message), "Zdalna stacja rozlaczyla sesje") {
							_ = out.write(serverMessage{Type: "data", State: "connected", Data: expandReply(terminalGoodbye, historyStation)})
						}
						if e.State == session.Disconnected && historyConnected && s.cfg.History != nil {
							s.cfg.History.Disconnected(historySession)
							historyConnected = false
						}
						msg := terminalMessageFromEventWithCodec(e, sessionCodec)
						if e.State == session.Disconnected && !historyConnected {
							msg.Error = string(e.Message)
						}
						_ = out.write(msg)
					}
				}()
				go func() {
					cctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
					defer cancel()
					via := strings.FieldsFunc(strings.ToUpper(m.Digi), func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
					if err := radioSession.Connect(cctx, m.TNC, m.Target, via...); err != nil {
						_ = out.write(serverMessage{Type: "state", State: "error", Error: err.Error()})
					}
				}()
				continue
			}
			if m.Mode != "bbs" {
				_ = out.write(serverMessage{Type: "error", Error: "Nieznany tryb terminala."})
				continue
			}
			if s.cfg.BBSListen == "" {
				_ = out.write(serverMessage{Type: "error", Error: "Lokalny BBS jest wylaczony."})
				continue
			}
			address := s.cfg.BBSListen
			dialCtx, dialCancel := context.WithTimeout(r.Context(), 8*time.Second)
			conn, err := (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext(dialCtx, "tcp", address)
			dialCancel()
			if err != nil {
				_ = out.write(serverMessage{Type: "state", State: "error", Error: "Polaczenie nieudane: " + err.Error()})
				continue
			}
			remote = conn
			historyStation = "Lokalny BBS"
			historyPort, historyDigi, historyConnected = address, "", true
			if s.cfg.History != nil {
				historySession = s.cfg.History.Connected(m.Mode, historyStation, historyPort, historyDigi)
			}
			done := remoteDone
			_ = out.write(serverMessage{Type: "state", State: "connected", Data: "Polaczono z " + conn.RemoteAddr().String() + "\r\n"})
			go s.copyTelnetToWS(out, conn, done, "bbs", historyStation, historyPort, historySession)
		case "data":
			m.Data = prepareTerminalMessage(m.Data, terminalMacroContext(terminalCall, historyStation, s.cfg))
			if historyConnected && s.cfg.History != nil {
				s.cfg.History.Add(activeMode, historyStation, historyPort, historyDigi, "tx", m.Data)
			}
			if activeMode == "incoming" && incoming != nil {
				wireData, encodeErr := terminalTXCodec.Encode(m.Data)
				if encodeErr != nil {
					_ = out.write(serverMessage{Type: "error", Error: encodeErr.Error()})
					continue
				}
				_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "sending"})
				incoming.mu.Lock()
				if _, err := incoming.w.Write(wireData); err != nil {
					incoming.mu.Unlock()
					_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "error", Error: err.Error()})
					_ = out.write(serverMessage{Type: "error", Error: err.Error()})
				} else {
					incoming.mu.Unlock()
					_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "sent"})
				}
				continue
			}
			if activeMode == "tnc" && radioSession != nil {
				wireData, encodeErr := terminalTXCodec.Encode(m.Data)
				if encodeErr != nil {
					_ = out.write(serverMessage{Type: "error", Error: encodeErr.Error()})
					continue
				}
				progress := func(p session.SendPacketProgress) {
					_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: p.Packet, Total: p.Total, Data: terminalTXCodec.Decode(p.Data), State: p.State, Error: p.Error})
				}
				sendMu.Lock()
				if err := radioSession.SendWithProgress(r.Context(), wireData, progress); err != nil {
					sendMu.Unlock()
					_ = out.write(serverMessage{Type: "error", Error: err.Error()})
				} else {
					sendMu.Unlock()
				}
				continue
			}
			if remote == nil {
				_ = out.write(serverMessage{Type: "error", Error: "Brak aktywnego polaczenia."})
				continue
			}
			if len(m.Data) > 4096 {
				_ = out.write(serverMessage{Type: "error", Error: "Dane sa zbyt dlugie."})
				continue
			}
			_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "sending"})
			if _, err := remote.Write([]byte(m.Data)); err != nil {
				_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "error", Error: err.Error()})
				closeRemote()
				_ = out.write(serverMessage{Type: "state", State: "error", Error: err.Error()})
			} else {
				_ = out.write(serverMessage{Type: "tx_packet", ID: m.ID, Packet: 1, Total: 1, Data: m.Data, State: "sent"})
			}
		case "disconnect":
			goodbye := expandReply(terminalGoodbye, historyStation)
			if incoming != nil {
				sendIncomingReply(terminalGoodbye, fmt.Sprintf("goodbye-%d", time.Now().UnixNano()), historyStation)
				incoming.close()
				incoming = nil
			}
			if activeMode == "tnc" && radioSession != nil {
				if cancelKeepAlive != nil {
					cancelKeepAlive()
					cancelKeepAlive = nil
				}
				if radioSession.State() == session.Connected && strings.TrimSpace(goodbye) != "" {
					wireData, encodeErr := terminalTXCodec.Encode(terminalResponseText(goodbye))
					if encodeErr == nil {
						sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						sendMu.Lock()
						if err := radioSession.Send(sendCtx, wireData); err != nil {
							_ = out.write(serverMessage{Type: "error", Error: "Nie wyslano pozegnania: " + err.Error()})
						}
						sendMu.Unlock()
						cancel()
					}
				}
				closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = radioSession.Disconnect(closeCtx)
				cancel()
			}
			if activeMode == "bbs" && remote != nil && strings.TrimSpace(goodbye) != "" {
				_, _ = remote.Write([]byte(terminalResponseText(goodbye)))
			}
			closeRemote()
			if s.cfg.History != nil {
				s.cfg.History.Disconnected(historySession)
			}
			historySession = 0
			historyConnected = false
			activeMode = ""
			_ = out.write(serverMessage{Type: "state", State: "idle", Data: "Rozlaczono.\r\n"})
		}
	}
}

func (s *Server) copyOperatorToWS(ws *safeWS, in *operatorSession, station string, codec *terminalcodec.Codec, historySession uint64) {
	if s.cfg.History != nil {
		defer s.cfg.History.Disconnected(historySession)
	}
	buf := make([]byte, 4096)
	commandBuffer := &terminalLineBuffer{}
	terminalCall := callsign(s.cfg.TerminalCallsign, s.cfg.TerminalSSID)
	for {
		n, err := in.r.Read(buf)
		if n > 0 {
			data := codec.Decode(buf[:n])
			if s.cfg.History != nil {
				s.cfg.History.Add("tnc", station, "radio", "", "rx", data)
			}
			if ws.write(serverMessage{Type: "data", State: "connected", Data: data}) != nil {
				in.close()
				return
			}
			for _, line := range commandBuffer.Push(data) {
				switch terminalRemoteCommand(line) {
				case "mheard":
					if s.cfg.MHeard != nil {
						writeOperatorReply(ws, in, codec, formatMHeardResponse(s.cfg.MHeard.List()))
					} else {
						writeOperatorReply(ws, in, codec, "Brak odebranych stacji.\r\n")
					}
				case "info":
					writeOperatorReply(ws, in, codec, terminalReplyText(s.cfg.TerminalInfo, terminalMacroContext(terminalCall, station, s.cfg)))
				case "help":
					writeOperatorReply(ws, in, codec, terminalHelpResponse())
				}
			}
		}
		if err != nil {
			if goodbye := terminalReplyText(s.cfg.TerminalGoodbye, terminalMacroContext(terminalCall, station, s.cfg)); goodbye != "" {
				_ = ws.write(serverMessage{Type: "data", State: "connected", Data: goodbye})
			}
			_ = ws.write(serverMessage{Type: "state", State: "idle", Data: "Stacja zdalna zakonczyla polaczenie.\r\n"})
			in.close()
			return
		}
	}
}

func (s *Server) copyTelnetToWS(ws *safeWS, conn net.Conn, done <-chan struct{}, mode, station, port string, historySession uint64) {
	if s.cfg.History != nil {
		defer s.cfg.History.Disconnected(historySession)
	}
	filter := newTelnetFilter(conn)
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			data := filter.Feed(buf[:n])
			if len(data) > 0 {
				if s.cfg.History != nil {
					s.cfg.History.Add(mode, station, port, "", "rx", string(data))
				}
				if e := ws.write(serverMessage{Type: "data", Data: string(data)}); e != nil {
					return
				}
			}
		}
		if err != nil {
			select {
			case <-done:
				return
			default:
			}
			_ = ws.write(serverMessage{Type: "state", State: "idle", Data: "\r\nPolaczenie zakonczone.\r\n"})
			return
		}
	}
}

func validateTelnet(host string, port int) error {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return errors.New("Podaj poprawny host.")
	}
	if port < 1 || port > 65535 {
		return errors.New("Port musi byc w zakresie 1-65535.")
	}
	return nil
}
func callsign(call string, ssid uint8) string {
	if ssid == 0 {
		return call
	}
	return call + "-" + strconv.Itoa(int(ssid))
}
