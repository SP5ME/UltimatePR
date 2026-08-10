package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/packet-radio/modernbbs/internal/bbs"
	"github.com/packet-radio/modernbbs/internal/session"
)

//go:embed static/*
var assets embed.FS

type Config struct {
	Listen           string
	NodeCallsign     string
	NodeSSID         uint8
	TerminalCallsign string
	TerminalSSID     uint8
	Ports            []string
	Radio            *session.Manager
	BBSListen        string
	BBS              *bbs.Store
}

type Server struct {
	cfg       Config
	log       *slog.Logger
	started   time.Time
	wsClients atomic.Int64
}

func New(cfg Config, log *slog.Logger) *Server {
	return &Server{cfg: cfg, log: log, started: time.Now()}
}

func (s *Server) Run(ctx context.Context) error {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /ws/terminal", s.terminal)
	mux.HandleFunc("GET /api/bbs/messages", s.bbsList)
	mux.HandleFunc("POST /api/bbs/messages", s.bbsSend)
	mux.HandleFunc("GET /api/bbs/messages/{id}", s.bbsRead)
	mux.HandleFunc("DELETE /api/bbs/messages/{id}", s.bbsDelete)

	h := securityHeaders(mux)
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

func (s *Server) SetBBS(store *bbs.Store) { s.cfg.BBS = store }

type bbsRequest struct { Call string `json:"call"`; Type string `json:"type"`; To string `json:"to"`; Subject string `json:"subject"`; Body string `json:"body"` }
func (s *Server) bbsList(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil { http.Error(w,"BBS disabled",http.StatusServiceUnavailable); return }
	call:=r.URL.Query().Get("call"); if err:=s.cfg.BBS.Register(call);err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return}
	w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(s.cfg.BBS.List(call,r.URL.Query().Get("type")=="B"))
}
func (s *Server) bbsSend(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil { http.Error(w,"BBS disabled",http.StatusServiceUnavailable); return }
	var q bbsRequest; if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,128<<10)).Decode(&q);err!=nil{http.Error(w,"Invalid request",http.StatusBadRequest);return}
	m,err:=s.cfg.BBS.Send(strings.ToUpper(q.Type),q.Call,q.To,q.Subject,q.Body);if err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return}
	w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusCreated);_=json.NewEncoder(w).Encode(m)
}
func (s *Server) bbsRead(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil { http.Error(w,"BBS disabled",http.StatusServiceUnavailable); return }
	id,err:=strconv.ParseInt(r.PathValue("id"),10,64);if err!=nil{http.Error(w,"Invalid id",http.StatusBadRequest);return}
	m,err:=s.cfg.BBS.Read(r.URL.Query().Get("call"),id);if err!=nil{http.Error(w,err.Error(),http.StatusNotFound);return}
	w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(m)
}
func (s *Server) bbsDelete(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BBS == nil { http.Error(w,"BBS disabled",http.StatusServiceUnavailable); return }
	id,err:=strconv.ParseInt(r.PathValue("id"),10,64);if err==nil{err=s.cfg.BBS.Delete(r.URL.Query().Get("call"),id)};if err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};w.WriteHeader(http.StatusNoContent)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; style-src 'self'; script-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"node": callsign(s.cfg.NodeCallsign, s.cfg.NodeSSID), "terminal": callsign(s.cfg.TerminalCallsign, s.cfg.TerminalSSID),
		"ports": s.cfg.Ports, "uptime_seconds": int(time.Since(s.started).Seconds()), "terminal_clients": s.wsClients.Load(),
		"bbs_enabled": s.cfg.BBSListen != "",
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize: 4096, WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "http://"+r.Host || r.Header.Get("Origin") == "https://"+r.Host
	},
}

type clientMessage struct {
	Type   string `json:"type"`
	Mode   string `json:"mode"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	TNC    string `json:"tnc"`
	Target string `json:"target"`
	Data   string `json:"data"`
}
type serverMessage struct {
	Type  string `json:"type"`
	State string `json:"state,omitempty"`
	Data  string `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
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
	_ = out.write(serverMessage{Type: "state", State: "idle", Data: "Terminal gotowy. Wybierz Telnet lub TNC.\r\n"})
	var remote net.Conn
	activeMode := ""
	var cancelRadio func()
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
		if cancelRadio != nil {
			cancelRadio()
		}
	}()
	for {
		var m clientMessage
		if err := ws.ReadJSON(&m); err != nil {
			return
		}
		switch m.Type {
		case "connect":
			closeRemote()
			if cancelRadio != nil {
				cancelRadio()
				cancelRadio = nil
			}
			activeMode = m.Mode
			if m.Mode == "tnc" {
				if s.cfg.Radio == nil {
					_ = out.write(serverMessage{Type: "state", State: "error", Error: "Session Manager AX.25 jest niedostępny."})
					continue
				}
				events, cancel := s.cfg.Radio.Subscribe()
				cancelRadio = cancel
				go func() {
					for e := range events {
						_ = out.write(serverMessage{Type: e.Type, State: string(e.State), Data: string(e.Data), Error: e.Message})
					}
				}()
				go func() {
					cctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
					defer cancel()
					if err := s.cfg.Radio.Connect(cctx, m.TNC, m.Target); err != nil {
						_ = out.write(serverMessage{Type: "state", State: "error", Error: err.Error()})
					}
				}()
				continue
			}
			if m.Mode != "telnet" && m.Mode != "bbs" {
				_ = out.write(serverMessage{Type: "error", Error: "Nieznany tryb terminala."})
				continue
			}
			address := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
			if m.Mode == "bbs" {
				if s.cfg.BBSListen == "" {
					_ = out.write(serverMessage{Type: "error", Error: "Lokalny BBS jest wyłączony."})
					continue
				}
				address = s.cfg.BBSListen
			} else if err := validateTelnet(m.Host, m.Port); err != nil {
				_ = out.write(serverMessage{Type: "error", Error: err.Error()})
				continue
			}
			conn, err := net.DialTimeout("tcp", address, 8*time.Second)
			if err != nil {
				_ = out.write(serverMessage{Type: "state", State: "error", Error: "Połączenie nieudane: " + err.Error()})
				continue
			}
			remote = conn
			done := remoteDone
			_ = out.write(serverMessage{Type: "state", State: "connected", Data: "Połączono z " + conn.RemoteAddr().String() + "\r\n"})
			go s.copyTelnetToWS(out, conn, done)
		case "data":
			if activeMode == "tnc" && s.cfg.Radio != nil {
				if err := s.cfg.Radio.Send(r.Context(), []byte(m.Data)); err != nil {
					_ = out.write(serverMessage{Type: "error", Error: err.Error()})
				}
				continue
			}
			if remote == nil {
				_ = out.write(serverMessage{Type: "error", Error: "Brak aktywnego połączenia."})
				continue
			}
			if len(m.Data) > 4096 {
				_ = out.write(serverMessage{Type: "error", Error: "Dane są zbyt długie."})
				continue
			}
			if _, err := remote.Write([]byte(m.Data)); err != nil {
				closeRemote()
				_ = out.write(serverMessage{Type: "state", State: "error", Error: err.Error()})
			}
		case "disconnect":
			if activeMode == "tnc" && s.cfg.Radio != nil {
				go s.cfg.Radio.Disconnect(context.Background())
			}
			closeRemote()
			activeMode = ""
			_ = out.write(serverMessage{Type: "state", State: "idle", Data: "Rozłączono.\r\n"})
		}
	}
}

func (s *Server) copyTelnetToWS(ws *safeWS, conn net.Conn, done <-chan struct{}) {
	filter := newTelnetFilter(conn)
	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		n, err := conn.Read(buf)
		if n > 0 {
			data := filter.Feed(buf[:n])
			if len(data) > 0 {
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
			_ = ws.write(serverMessage{Type: "state", State: "idle", Data: "\r\nPołączenie zakończone.\r\n"})
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
		return errors.New("Port musi być w zakresie 1–65535.")
	}
	return nil
}
func callsign(call string, ssid uint8) string {
	if ssid == 0 {
		return call
	}
	return call + "-" + strconv.Itoa(int(ssid))
}
