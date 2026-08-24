package netallow

import (
	"net"
	"testing"
)

func TestAllowed(t *testing.T) {
	tests := []struct {
		name  string
		ip    string
		rules []string
		want  bool
	}{
		{name: "exact IP", ip: "192.0.2.10", rules: []string{"192.0.2.10"}, want: true},
		{name: "CIDR", ip: "192.0.2.10", rules: []string{"192.0.2.0/24"}, want: true},
		{name: "outside CIDR", ip: "198.51.100.10", rules: []string{"192.0.2.0/24"}},
		{name: "IPv4 wildcard", ip: "198.51.100.10", rules: []string{"0.0.0.0"}, want: true},
		{name: "hostname", ip: "127.0.0.1", rules: []string{"localhost"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Allowed(net.ParseIP(tt.ip), tt.rules); got != tt.want {
				t.Fatalf("Allowed(%q, %v) = %v, want %v", tt.ip, tt.rules, got, tt.want)
			}
		})
	}
}

func TestValidRule(t *testing.T) {
	for _, rule := range []string{"192.0.2.1", "192.0.2.0/24", "tnc.local", "raspberrypi", "host-name.example."} {
		if !ValidRule(rule) {
			t.Errorf("valid rule %q rejected", rule)
		}
	}
	for _, rule := range []string{"", "bad host!", "-host.local", "host..local"} {
		if ValidRule(rule) {
			t.Errorf("invalid rule %q accepted", rule)
		}
	}
}

func TestParseZonedIPv6(t *testing.T) {
	ip := ParseIP("fe80::1234%eth0")
	if ip == nil || !ip.Equal(net.ParseIP("fe80::1234")) {
		t.Fatalf("zoned IPv6 was not parsed: %v", ip)
	}
	if !Allowed(ip, []string{"::"}) {
		t.Fatal("IPv6 wildcard rejected zoned client address")
	}
}
