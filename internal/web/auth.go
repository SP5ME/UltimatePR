package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/packet-radio/ultimatepr/internal/config"
	"github.com/packet-radio/ultimatepr/internal/netallow"
)

const (
	sessionCookie  = "ultimatepr_session"
	passwordRounds = 120000
	sessionMaxAge  = 30 * 24 * time.Hour
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	Current string `json:"current_password"`
	New     string `json:"new_password"`
	Confirm string `json:"confirm_password"`
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login.html" || r.URL.Path == "/login.css" || r.URL.Path == "/login.js" || r.URL.Path == "/api/login" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err == nil && s.validSession(cookie.Value) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login.html", http.StatusSeeOther)
	})
}

func (s *Server) allowAddresses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := netallow.ParseIP(host)
		if ip == nil || !addressAllowed(ip, s.cfg.AllowedAddresses) {
			http.Error(w, "Address not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func addressAllowed(ip net.IP, rules []string) bool {
	return netallow.Allowed(ip, rules)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var q loginRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	s.authMu.RLock()
	username, hash := s.cfg.Username, s.cfg.PasswordHash
	s.authMu.RUnlock()
	if subtle.ConstantTimeCompare([]byte(q.Username), []byte(username)) != 1 || !verifyPassword(hash, q.Password) {
		time.Sleep(250 * time.Millisecond)
		http.Error(w, "Nieprawidłowy użytkownik lub hasło", http.StatusUnauthorized)
		return
	}
	token, err := createSessionToken(hash, time.Now().Add(sessionMaxAge))
	if err != nil {
		http.Error(w, "Cannot create session", http.StatusInternalServerError)
		return
	}
	s.authMu.Lock()
	if s.activeSession != "" && s.activeSession != token {
		s.sessionSwitch = true
	}
	s.activeSession = token
	s.authMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionMaxAge.Seconds()), Expires: time.Now().Add(sessionMaxAge)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.authMu.Lock()
		if s.activeSession == cookie.Value {
			s.activeSession = ""
		}
		s.authMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) validSession(token string) bool {
	s.authMu.RLock()
	hash := s.cfg.PasswordHash
	active := s.activeSession
	s.authMu.RUnlock()
	if active != "" && token != active {
		return false
	}
	return verifySessionToken(hash, token, time.Now())
}

func createSessionToken(passwordHash string, expires time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%d.%s", expires.Unix(), base64.RawURLEncoding.EncodeToString(nonce))
	mac := hmac.New(sha256.New, sessionSigningKey(passwordHash))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifySessionToken(passwordHash, token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	fields := strings.Split(string(payload), ".")
	if len(fields) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || now.Unix() >= expires {
		return false
	}
	mac := hmac.New(sha256.New, sessionSigningKey(passwordHash))
	_, _ = mac.Write(payload)
	return hmac.Equal(sig, mac.Sum(nil))
}

func sessionSigningKey(passwordHash string) []byte {
	sum := sha256.Sum256([]byte("ultimatepr-session-v1\x00" + passwordHash))
	return sum[:]
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var q passwordRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if q.New != q.Confirm {
		http.Error(w, "Nowe hasła nie są identyczne", http.StatusBadRequest)
		return
	}
	if len(q.New) < 4 || len(q.New) > 128 {
		http.Error(w, "Hasło musi mieć od 4 do 128 znaków", http.StatusBadRequest)
		return
	}
	s.authMu.RLock()
	valid := verifyPassword(s.cfg.PasswordHash, q.Current)
	s.authMu.RUnlock()
	if !valid {
		http.Error(w, "Obecne hasło jest nieprawidłowe", http.StatusUnauthorized)
		return
	}
	hash, err := hashPassword(q.New)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Web.PasswordHash = hash
	if err = appconfig.SaveModel(s.cfg.ConfigPath, c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.authMu.Lock()
	s.cfg.PasswordHash = hash
	s.authMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	digest := deriveKey([]byte(password), salt, passwordRounds, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordRounds, hex.EncodeToString(salt), hex.EncodeToString(digest)), nil
}

func verifyPassword(encoded, password string) bool {
	if encoded == "" {
		return subtle.ConstantTimeCompare([]byte(password), []byte("packet")) == 1
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	rounds, err := strconv.Atoi(parts[1])
	salt, errSalt := hex.DecodeString(parts[2])
	want, errHash := hex.DecodeString(parts[3])
	if err != nil || errSalt != nil || errHash != nil || rounds < 10000 || rounds > 1000000 || len(want) != 32 {
		return false
	}
	got := deriveKey([]byte(password), salt, rounds, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func deriveKey(password, salt []byte, rounds, size int) []byte {
	result := make([]byte, 0, size)
	for block := uint32(1); len(result) < size; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < rounds; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		result = append(result, t...)
	}
	return result[:size]
}
