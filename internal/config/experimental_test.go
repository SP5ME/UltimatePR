package config

import "testing"

func TestLegacyServicesBecomeExperimental(t *testing.T) {
	c, err := Parse([]byte(`application: {mode: station-node-bbs}
server: {callsign: SP5ME, ssid: 2}
terminal: {callsign: SP5ME, ssid: 0}
web: {listen: "127.0.0.1:8080", username: admin, allowed_addresses: [127.0.0.1]}
uprd: {enabled: true, interval_seconds: 600, mheard_limit: 5}
bbs: {enabled: true, listen: "127.0.0.1:8023", database: bbs.json, callsign: SP5ME, ssid: 8, language: pl}
node: {enabled: true, alias: SP5ME, listen: "127.0.0.1:8010", language: pl}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Experimental.Services || !c.Experimental.UPRD {
		t.Fatalf("legacy flags not migrated: %+v", c.Experimental)
	}
}

func TestServiceSwitchesAreIndependent(t *testing.T) {
	c := New(ModeFull, "SP5ME", "KO02JD", "Warsaw", "pl", 0, 2, 8)
	c.Experimental.Services = true
	c.Node.Enabled = false
	c.BBS.Enabled = false
	c.AI.Enabled = true
	if c.Node.Enabled || c.BBS.Enabled || !c.AI.Enabled {
		t.Fatal("independent service switches were not preserved")
	}
}
