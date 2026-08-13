package web

import (
	"net"
	"testing"
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
}
