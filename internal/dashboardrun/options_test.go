package dashboardrun

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestRegisterFlagsDefaultsAndParsing(t *testing.T) {
	var opts Options
	fs := pflag.NewFlagSet("dashboard", pflag.ContinueOnError)
	RegisterFlags(fs, &opts)

	if opts.SQLitePath != "screp.db" || opts.Port != 8000 || opts.Headless || opts.ReplayDir != "" {
		t.Fatalf("defaults = %+v", opts)
	}
	if err := fs.Parse([]string{"--replay-dir", "/replays", "--headless", "-p", "9000"}); err != nil {
		t.Fatal(err)
	}
	if opts.ReplayDir != "/replays" || !opts.Headless || opts.Port != 9000 {
		t.Fatalf("parsed = %+v", opts)
	}
}
