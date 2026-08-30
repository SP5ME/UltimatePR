package web

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	appconfig "github.com/packet-radio/ultimatepr/internal/config"
)

func TestPasswordHash(t *testing.T) {
	hash, err := hashPassword("new secure password")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "new secure password") || verifyPassword(hash, "wrong") {
		t.Fatal("password verification failed")
	}
	if !verifyPassword("", "packet") || verifyPassword("", "other") {
		t.Fatal("default password verification failed")
	}
}

func TestLoginDetailsCanChangeUsernameWithoutChangingPassword(t *testing.T) {
	hash, err := hashPassword("current-password")
	if err != nil {
		t.Fatal(err)
	}
	c, err := appconfig.Parse([]byte("server: {callsign: SP5ME}\nterminal: {callsign: SP5ME}\nweb: {listen: 127.0.0.1:8080, username: admin}\n"))
	if err != nil {
		t.Fatal(err)
	}
	c.Web.PasswordHash = hash
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err = appconfig.SaveModel(path, c); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Username: "admin", PasswordHash: hash, ConfigPath: path}}
	body, _ := json.Marshal(passwordRequest{Username: "operator", Current: "current-password"})
	w := httptest.NewRecorder()
	s.changePassword(w, httptest.NewRequest(http.MethodPut, "/api/application/password", bytes.NewReader(body)))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	saved, err := appconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Web.Username != "operator" || saved.Web.PasswordHash != hash || s.cfg.Username != "operator" || s.cfg.PasswordHash != hash {
		t.Fatalf("login details not updated safely: saved=%+v runtime=%+v", saved.Web, s.cfg)
	}
}

func TestAllowAddressesAcceptsZonedIPv6Remote(t *testing.T) {
	s := &Server{cfg: Config{AllowedAddresses: []string{"::"}}}
	req := httptest.NewRequest(http.MethodGet, "http://ultimatepr:8080/", nil)
	req.RemoteAddr = "[fe80::1234%eth0]:54321"
	recorder := httptest.NewRecorder()
	s.allowAddresses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("zoned IPv6 client rejected with status %d", recorder.Code)
	}
}

func TestPersistentSessionToken(t *testing.T) {
	now := time.Now()
	token, err := createSessionToken("hash-one", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !verifySessionToken("hash-one", token, now) {
		t.Fatal("valid token rejected")
	}
	if verifySessionToken("hash-two", token, now) {
		t.Fatal("token survived password change")
	}
	if verifySessionToken("hash-one", token, now.Add(2*time.Hour)) {
		t.Fatal("expired token accepted")
	}
}

func TestAddressAllowed(t *testing.T) {
	if !addressAllowed(net.ParseIP("192.168.1.20"), []string{"192.168.1.0/24"}) {
		t.Fatal("CIDR address rejected")
	}
	if addressAllowed(net.ParseIP("10.0.0.2"), []string{"192.168.1.0/24"}) {
		t.Fatal("address outside CIDR accepted")
	}
	if !addressAllowed(net.ParseIP("10.0.0.2"), []string{"0.0.0.0"}) {
		t.Fatal("IPv4 wildcard rejected")
	}
	if !addressAllowed(net.ParseIP("127.0.0.1"), []string{"localhost"}) {
		t.Fatal("hostname address rejected")
	}
}
