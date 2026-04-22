package dashboardrun

import (
	"os"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/spf13/pflag"
)

// Options holds CLI flags for starting the dashboard server.
type Options struct {
	SQLitePath string
	AIVendor   string
	AIAPIKey   string
	AIModel    string
	Port       int
}

// RegisterFlags binds dashboard flags to fs (Cobra command flags or a standalone pflag set).
func RegisterFlags(fs *pflag.FlagSet, o *Options) {
	fs.StringVarP(&o.SQLitePath, "sqlite-path", "s", "screp.db", "SQLite database file path.")
	fs.StringVarP(&o.AIVendor, "ai-vendor", "v", "", "Which AI to use (OPENAI|ANTHROPIC|GEMINI). Defaults to OPENAI.")
	fs.StringVarP(&o.AIAPIKey, "ai-api-key", "k", "", "An API KEY from the AI vendor in order to prompt for widget creation.")
	fs.StringVarP(&o.AIModel, "ai-model", "m", "", "The AI model to use.")
	fs.IntVarP(&o.Port, "port", "p", 8000, "Dashboard server port")
}

// ResolveAIVendor picks a vendor from env API keys when explicit is empty; explicit non-empty wins (uppercased).
func ResolveAIVendor(explicit string) string {
	var vendor string
	if os.Getenv("GEMINI_API_KEY") != "" {
		vendor = dashboard.AIVendorGemini
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		vendor = dashboard.AIVendorAnthropic
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		vendor = dashboard.AIVendorOpenAI
	}
	if explicit != "" {
		vendor = strings.ToUpper(explicit)
	}

	return vendor
}

// NormalizeAfterParse sets AIVendor from flags and env (call after Parse).
func (o *Options) NormalizeAfterParse() {
	o.AIVendor = ResolveAIVendor(o.AIVendor)
}
