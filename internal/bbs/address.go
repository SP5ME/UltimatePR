package bbs

import (
	"errors"
	"strings"
)

// HierarchicalAddress is the TAPR x.3.4 BBS address:
// BBS.[#AREA.][REGION.]COUNTRY.CONTINENT.
type HierarchicalAddress struct {
	BBS, Area, Region, Country, Continent string
}

var taprContinents = map[string]struct{}{
	"EURO": {}, "MEDR": {}, "INDI": {}, "MDLE": {}, "SEAS": {}, "ASIA": {},
	"NOAM": {}, "CEAM": {}, "CARB": {}, "SOAM": {}, "AUNZ": {}, "EPAC": {},
	"NPAC": {}, "SPAC": {}, "WPAC": {}, "NAFR": {}, "CAFR": {}, "SAFR": {},
	"ANTR": {},
}

func ParseHierarchicalAddress(value string) (HierarchicalAddress, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	parts := strings.Split(value, ".")
	if len(parts) < 3 || len(parts) > 5 {
		return HierarchicalAddress{}, errors.New("expected BBS.[#AREA.][REGION.]COUNTRY.CONTINENT")
	}
	if !validCall(parts[0]) {
		return HierarchicalAddress{}, errors.New("invalid BBS callsign")
	}
	a := HierarchicalAddress{BBS: parts[0]}
	tail := parts[1:]
	if strings.HasPrefix(tail[0], "#") {
		a.Area = strings.TrimPrefix(tail[0], "#")
		if !validLocationPart(a.Area, 1, 6) {
			return HierarchicalAddress{}, errors.New("invalid area designator")
		}
		tail = tail[1:]
	}
	if len(tail) == 3 {
		a.Region = tail[0]
		if !validLocationPart(a.Region, 2, 4) {
			return HierarchicalAddress{}, errors.New("region must contain 2 to 4 letters or digits")
		}
		tail = tail[1:]
	}
	if len(tail) != 2 || !validLocationPart(tail[0], 3, 3) {
		return HierarchicalAddress{}, errors.New("country must be a 3-character TAPR/ISO designator")
	}
	a.Country, a.Continent = tail[0], tail[1]
	if _, ok := taprContinents[a.Continent]; !ok {
		return HierarchicalAddress{}, errors.New("unknown TAPR continent designator")
	}
	if len(strings.Join(parts[1:], ".")) > 31 {
		return HierarchicalAddress{}, errors.New("hierarchical location exceeds 31 characters")
	}
	return a, nil
}

func (a HierarchicalAddress) String() string {
	parts := []string{a.BBS}
	if a.Area != "" {
		parts = append(parts, "#"+a.Area)
	}
	if a.Region != "" {
		parts = append(parts, a.Region)
	}
	return strings.Join(append(parts, a.Country, a.Continent), ".")
}

// ParseDistributionDesignator validates the routing token used in the @ field
// of a TAPR bulletin. It is deliberately separate from a BBS address.
func ParseDistributionDesignator(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !validLocationPart(value, 1, 6) {
		return "", errors.New("distribution designator must contain 1 to 6 letters or digits")
	}
	return value, nil
}

func validCall(value string) bool { return validLocationPart(value, 1, 6) }

func validLocationPart(value string, minLen, maxLen int) bool {
	if len(value) < minLen || len(value) > maxLen {
		return false
	}
	for _, r := range value {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
