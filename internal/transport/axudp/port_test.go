package axudp

import "testing"

func TestFCSRoundTrip(t *testing.T) {
	body := []byte{0x82, 0xa0, 0xa4, 0xa6, 0x40, 0x40, 0x60, 0x03, 0xf0, 'H', 'I'}
	wire := appendFCS(append([]byte(nil), body...))
	if !validFCS(wire) {
		t.Fatal("valid FCS rejected")
	}
	wire[3] ^= 1
	if validFCS(wire) {
		t.Fatal("damaged frame accepted")
	}
}
func TestAllowList(t *testing.T) {
	p := New(Config{AllowFrom: []string{"192.0.2.0/24"}}, nil)
	if !p.allowed([]byte{192, 0, 2, 4}) {
		t.Fatal("CIDR address rejected")
	}
	if p.allowed([]byte{198, 51, 100, 1}) {
		t.Fatal("foreign address accepted")
	}
}
