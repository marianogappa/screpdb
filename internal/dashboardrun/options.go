package dashboardrun

import (
	"github.com/spf13/pflag"
)

// Options holds CLI flags for starting the dashboard server.
type Options struct {
	SQLitePath string
	Port       int
	Headless   bool
}

// RegisterFlags binds dashboard flags to fs (Cobra command flags or a standalone pflag set).
func RegisterFlags(fs *pflag.FlagSet, o *Options) {
	fs.StringVarP(&o.SQLitePath, "sqlite-path", "s", "screp.db", "SQLite database file path.")
	fs.IntVarP(&o.Port, "port", "p", 8000, "Dashboard server port")
	fs.BoolVar(&o.Headless, "headless", false, "Run as an API-only server: don't serve the dashboard UI and don't open a browser")
}
