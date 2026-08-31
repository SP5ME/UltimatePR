package bbs

import "testing"

func TestParseHierarchicalAddressTAPRX34(t *testing.T) {
	tests := []struct {
		value, area, region, country, continent string
	}{
		{"WA6GVD.CA.USA.NOAM", "", "CA", "USA", "NOAM"},
		{"EA2CMO.EAZ.ESP.EURO", "", "EAZ", "ESP", "EURO"},
		{"OH6RBV.#VAA.FIN.EURO", "VAA", "", "FIN", "EURO"},
		{"SP5AAA.#PL.POL.EURO", "PL", "", "POL", "EURO"},
	}
	for _, tt := range tests {
		a, err := ParseHierarchicalAddress(tt.value)
		if err != nil {
			t.Fatalf("ParseHierarchicalAddress(%q): %v", tt.value, err)
		}
		if a.Area != tt.area || a.Region != tt.region || a.Country != tt.country || a.Continent != tt.continent || a.String() != tt.value {
			t.Fatalf("ParseHierarchicalAddress(%q) = %#v", tt.value, a)
		}
	}
}

func TestParseHierarchicalAddressRejectsNonTAPRForms(t *testing.T) {
	for _, value := range []string{
		"SP5AAA.#PL.POL.EU",
		"SP5AAA-8.#PL.POL.EURO",
		"SP5AAA.#TOOLONG.POL.EURO",
		"SP5AAA.#PL.POL.UNKNOWN",
	} {
		if _, err := ParseHierarchicalAddress(value); err == nil {
			t.Fatalf("accepted invalid TAPR address %q", value)
		}
	}
}

func TestDistributionDesignatorIsNotHierarchicalAddress(t *testing.T) {
	for _, value := range []string{"POL", "EU", "ALL"} {
		if got, err := ParseDistributionDesignator(value); err != nil || got != value {
			t.Fatalf("ParseDistributionDesignator(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"SP5AAA.#PL.POL.EURO", "#PL", "TOO-LONG"} {
		if _, err := ParseDistributionDesignator(value); err == nil {
			t.Fatalf("accepted invalid distribution designator %q", value)
		}
	}
}
