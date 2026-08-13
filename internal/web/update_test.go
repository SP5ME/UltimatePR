package web

import "testing"

func TestBranchReleaseEndpoint(t *testing.T) {
	for channel, want := range map[string]string{
		"main": "https://api.github.com/repos/SP5ME/UltimatePR/releases/tags/main-latest",
		"dev":  "https://api.github.com/repos/SP5ME/UltimatePR/releases/tags/dev-latest",
	} {
		if got := branchReleaseEndpoint(channel); got != want {
			t.Fatalf("%s: %q", channel, got)
		}
	}
}
