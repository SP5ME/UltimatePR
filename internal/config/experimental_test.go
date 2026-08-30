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
	if !c.Experimental.Node || !c.Experimental.BBS || !c.Experimental.UPRD {
		t.Fatalf("legacy flags not migrated: %+v", c.Experimental)
	}
}

func TestExplicitExperimentalDisableIsPreserved(t *testing.T) {
	c := New(ModeFull, "SP5ME", "KO02JD", "Warsaw", "pl", 0, 2, 8)
	c.Experimental.Node = false
	c.Experimental.BBS = false
	if c.Experimental.Node || c.Experimental.BBS {
		t.Fatal("disabled services were enabled")
	}
}
