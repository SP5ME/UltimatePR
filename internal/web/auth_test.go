package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
