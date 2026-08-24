// Package netallow matches client IP addresses against IP, CIDR, and hostname rules.
package netallow

import (
	"net"
	"strings"
)

// ParseIP parses a plain IP address and an IPv6 address carrying an interface
// zone, as returned for link-local TCP clients (for example fe80::1%eth0).
func ParseIP(raw string) net.IP {
	raw = strings.Trim(strings.TrimSpace(raw), "[]")
	if zone := strings.LastIndexByte(raw, '%'); zone >= 0 && strings.Contains(raw[:zone], ":") {
		raw = raw[:zone]
	}
	return net.ParseIP(raw)
}

// ValidRule reports whether rule is an IP address, CIDR network, or hostname.
func ValidRule(rule string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return false
	}
	if ParseIP(rule) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(rule); err == nil {
		return true
	}
	return validHostname(rule)
}

// Allowed reports whether ip matches at least one IP, CIDR, or hostname rule.
// Hostnames are resolved when checked so DNS changes do not require a restart.
func Allowed(ip net.IP, rules []string) bool {
	if ip == nil {
		return false
	}
	for _, raw := range rules {
		rule := strings.TrimSpace(raw)
		if rule == "0.0.0.0" && ip.To4() != nil {
			return true
		}
		if rule == "::" && ip.To4() == nil {
			return true
		}
		if exact := ParseIP(rule); exact != nil && exact.Equal(ip) {
			return true
		}
		if _, network, err := net.ParseCIDR(rule); err == nil {
			if network.Contains(ip) {
				return true
			}
			continue
		}
		if !validHostname(rule) {
			continue
		}
		addresses, err := net.LookupIP(strings.TrimSuffix(rule, "."))
		if err != nil {
			continue
		}
		for _, address := range addresses {
			if address.Equal(ip) {
				return true
			}
		}
	}
	return false
}

func validHostname(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
				return false
			}
		}
	}
	return true
}
