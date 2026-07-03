package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHasSubcommands(t *testing.T) {
	want := map[string]bool{"ingest": false, "mcp": false, "dashboard": false}
	for _, c := range rootCmd.Commands() {
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

func TestRootRunsDashboard(t *testing.T) {
	if rootCmd.RunE == nil {
		t.Fatal("rootCmd.RunE is nil; bare `screpdb` should launch the dashboard")
	}
}

func TestIngestFlagDefaults(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"sqlite-path", "screp.db"},
		{"stop-after-n-reps", "0"},
		{"up-to-yyyy-mm-dd", ""},
		{"up-to-n-months", "0"},
		{"store-right-clicks", "false"},
		{"skip-hotkeys", "false"},
		{"clean", "false"},
		{"clean-dashboard", "false"},
	}
	for _, tt := range tests {
		f := ingestCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("ingest flag %q not registered", tt.name)
			continue
		}
		if f.DefValue != tt.want {
			t.Errorf("ingest flag %q default = %q, want %q", tt.name, f.DefValue, tt.want)
		}
	}
}

func TestIngestInputDirFlagRegistered(t *testing.T) {
	if f := ingestCmd.Flags().Lookup("input-dir"); f == nil {
		t.Error("ingest flag input-dir not registered")
	}
}

func TestIngestShorthands(t *testing.T) {
	shorthands := map[string]string{
		"i": "input-dir",
		"s": "sqlite-path",
		"n": "stop-after-n-reps",
		"d": "up-to-yyyy-mm-dd",
		"m": "up-to-n-months",
	}
	for sh, long := range shorthands {
		f := ingestCmd.Flags().ShorthandLookup(sh)
		if f == nil {
			t.Errorf("ingest shorthand -%s not registered", sh)
			continue
		}
		if f.Name != long {
			t.Errorf("ingest shorthand -%s maps to %q, want %q", sh, f.Name, long)
		}
	}
}

func TestMCPFlagDefaults(t *testing.T) {
	f := mcpCmd.Flags().Lookup("sqlite-path")
	if f == nil {
		t.Fatal("mcp flag sqlite-path not registered")
	}
	if f.DefValue != "screp.db" {
		t.Errorf("mcp sqlite-path default = %q, want %q", f.DefValue, "screp.db")
	}
	if sh := mcpCmd.Flags().ShorthandLookup("s"); sh == nil || sh.Name != "sqlite-path" {
		t.Error("mcp shorthand -s should map to sqlite-path")
	}
}

func TestDashboardFlagDefaults(t *testing.T) {
	for _, cmd := range []*cobra.Command{dashboardCmd, rootCmd} {
		port := cmd.Flags().Lookup("port")
		if port == nil {
			t.Errorf("%s flag port not registered", cmd.Name())
			continue
		}
		if port.DefValue != "8000" {
			t.Errorf("%s port default = %q, want 8000", cmd.Name(), port.DefValue)
		}
		sqlite := cmd.Flags().Lookup("sqlite-path")
		if sqlite == nil || sqlite.DefValue != "screp.db" {
			t.Errorf("%s sqlite-path default wrong: %+v", cmd.Name(), sqlite)
		}
	}
}

func TestDefaultDashboardOptions(t *testing.T) {
	opts := defaultDashboardOptions()
	if opts.SQLitePath != "screp.db" {
		t.Errorf("default SQLitePath = %q, want screp.db", opts.SQLitePath)
	}
	if opts.Port != 8000 {
		t.Errorf("default Port = %d, want 8000", opts.Port)
	}
}
