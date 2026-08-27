package web

import "testing"

func TestBranchReleaseEndpoint(t *testing.T) {
	for channel, want := range map[string]string{
		"main": "https://github.com/SP5ME/UltimatePR/releases/download/main-latest/VERSION.txt",
		"dev":  "https://github.com/SP5ME/UltimatePR/releases/download/dev-latest/VERSION.txt",
	} {
		if got := branchReleaseEndpoint(channel); got != want {
			t.Fatalf("%s: %q", channel, got)
		}
	}
}
