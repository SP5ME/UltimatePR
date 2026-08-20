package terminalcodec

import "testing"

func TestUTF8RoundTrip(t *testing.T) {
	c, err := New("cp437")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Name(); got != "utf-8" {
		t.Fatalf("codec name = %q, want utf-8", got)
	}
	want := "Zażółć gęślą jaźń\r\nUnicode: ✓"
	wire, err := c.Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(wire); got != want {
		t.Fatalf("encoded = %q, want %q", got, want)
	}
	if got := c.Decode(wire); got != want {
		t.Fatalf("decoded = %q, want %q", got, want)
	}
}

func TestUTF8PreservesSplitRuneAcrossPackets(t *testing.T) {
	c, _ := New("auto")
	wire := []byte("Zażółć")
	if got := c.Decode(wire[:3]); got != "Za" {
		t.Fatalf("first packet = %q, want %q", got, "Za")
	}
	if got := c.Decode(wire[3:]); got != "żółć" {
		t.Fatalf("second packet = %q, want %q", got, "żółć")
	}
}

func TestSupportedListsOnlyUTF8(t *testing.T) {
	got := Supported()
	if len(got) != 1 || got[0] != "utf-8" {
		t.Fatalf("supported encodings = %#v", got)
	}
}
