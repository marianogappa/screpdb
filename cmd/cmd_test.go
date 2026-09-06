package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHasSubcommands(t *testing.T) {
	want := map[string]bool{"mcp": false, "dashboard": false}
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

// TestIngestCommandIsGone pins the removal: ingest wrote to a SQLite database
// nothing reads any more, and bringing it back would bring the database with it.
func TestIngestCommandIsGone(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "ingest" {
			t.Fatal("the ingest subcommand is registered again")
		}
	}
}

func TestRootRunsDashboard(t *testing.T) {
	if rootCmd.RunE == nil {
		t.Fatal("rootCmd.RunE is nil; bare `screpdb` should launch the dashboard")
	}
}

func TestMCPFlagDefaults(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"port", "8000"},
		{"no-auto-start", "false"},
		{"replay-dir", ""},
		{"wait-for-load", "true"},
	}
	for _, tt := range tests {
		f := mcpCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("mcp flag %q not registered", tt.name)
			continue
		}
		if f.DefValue != tt.want {
			t.Errorf("mcp flag %q default = %q, want %q", tt.name, f.DefValue, tt.want)
		}
	}
	if sh := mcpCmd.Flags().ShorthandLookup("p"); sh == nil || sh.Name != "port" {
		t.Error("mcp shorthand -p should map to port")
	}
}

// TestMCPHasNoDatabaseFlag pins that `screpdb mcp` no longer opens a database:
// it reads the running server's API instead.
func TestMCPHasNoDatabaseFlag(t *testing.T) {
	if f := mcpCmd.Flags().Lookup("sqlite-path"); f != nil {
		t.Error("mcp still registers a sqlite-path flag")
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
		legacy := cmd.Flags().Lookup("legacy-db-path")
		if legacy == nil || legacy.DefValue != "screp.db" {
			t.Errorf("%s legacy-db-path default wrong: %+v", cmd.Name(), legacy)
		}
	}
}

func TestDefaultDashboardOptions(t *testing.T) {
	opts := defaultDashboardOptions()
	if opts.LegacyDBPath != "screp.db" {
		t.Errorf("default LegacyDBPath = %q, want screp.db", opts.LegacyDBPath)
	}
	if opts.Port != 8000 {
		t.Errorf("default Port = %d, want 8000", opts.Port)
	}
}
