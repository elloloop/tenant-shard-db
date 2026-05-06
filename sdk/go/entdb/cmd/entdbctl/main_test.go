package main

import "testing"

// TestRootCmd_HasAllSubcommands keeps the v1 surface honest: if
// a subcommand silently disappears, this test breaks before
// anyone tags a release. The list mirrors README.md.
func TestRootCmd_HasAllSubcommands(t *testing.T) {
	want := map[string]bool{
		"health":  false,
		"tenants": false,
		"schema":  false,
		"nodes":   false,
		"edges":   false,
		"graph":   false,
		"search":  false,
	}
	for _, c := range newRootCmd().Commands() {
		// "completion" and "help" are auto-added by cobra; ignore.
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered on root", name)
		}
	}
}
