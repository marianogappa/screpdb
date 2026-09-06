package dashboardrun

import (
	"github.com/spf13/pflag"
)

// Options holds CLI flags for starting the dashboard server.
type Options struct {
	// LegacyDBPath points at the screp.db a pre-library release wrote. It is
	// read once, read-only, to carry the user's folder, filters and
	// Battle.net caches over; nothing else in screpdb opens a database.
	LegacyDBPath string
	ReplayDir    string
	Port         int
	Headless     bool
}

// RegisterFlags binds dashboard flags to fs (Cobra command flags or a standalone pflag set).
func RegisterFlags(fs *pflag.FlagSet, o *Options) {
	fs.StringVar(&o.LegacyDBPath, "legacy-db-path", "screp.db", "Pre-2.0 screp.db to import settings and Battle.net caches from once. Read-only.")
	fs.StringVar(&o.ReplayDir, "replay-dir", "", "Replay folder to read. Defaults to the saved folder, then to the StarCraft one.")
	fs.IntVarP(&o.Port, "port", "p", 8000, "Dashboard server port")
	fs.BoolVar(&o.Headless, "headless", false, "Run as an API-only server: don't serve the dashboard UI and don't open a browser")
}
