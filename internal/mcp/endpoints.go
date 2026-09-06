package mcp

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// endpoint is one read-only GET path an MCP client may reach.
type endpoint struct {
	template    string
	segments    []string
	summary     string
	queryParams []string
}

// exposed is the slice of the dashboard API the MCP tools can read. The
// dashboard serves three dozen paths; most drive the UI (images, colors,
// Battle.net, self-update) or mutate local state, and one of them —
// /api/games/{replayID}/see — launches the game client. Answering questions
// about the corpus needs only what is listed here, so the tool refuses
// everything else rather than trusting the model to stay inside the useful
// subset.
//
// TestExposedSurfaceMatchesTheSpec cross-checks this table against
// api/openapi/dashboard.v1.yaml, so a parameter added there cannot silently go
// missing here and a path removed there cannot linger.
var exposed = []endpoint{
	{
		template: "/api/health",
		summary:  "Corpus size, screpdb version and replay-folder load progress.",
	},
	{
		template:    "/api/games",
		summary:     "The games list. Filters are repeatable; omit them for the newest games.",
		queryParams: []string{"duration", "featuring", "limit", "map", "map_kind", "matchup", "offset", "player"},
	},
	{
		template: "/api/games/{replayID}",
		summary:  "The whole game report: players, build orders, timings, events, chat, map.",
	},
	{
		template: "/api/games/{replayID}/hotkeys",
		summary:  "Per-player hotkey timeline and map for one game.",
	},
	{
		template:    "/api/players",
		summary:     "The players list with game counts, races and APM.",
		queryParams: []string{"last_played", "limit", "name", "offset", "only_5_plus", "sort_by", "sort_dir"},
	},
	{
		template: "/api/players/{playerKey}",
		summary:  "One player's summary across the corpus.",
	},
	{
		template: "/api/players/{playerKey}/last-games",
		summary:  "That player's most recent games.",
	},
	{
		template: "/api/players/{playerKey}/chat-summary",
		summary:  "What that player says in game.",
	},
	{
		template:    "/api/players/{playerKey}/insight",
		summary:     "One computed insight for that player; type selects which.",
		queryParams: []string{"type"},
	},
	{
		template: "/api/players/{playerKey}/hotkey-signature",
		summary:  "That player's hotkey habits, the basis of playstyle fingerprinting.",
	},
	{
		template: "/api/players/{playerKey}/insights/apm-histogram",
		summary:  "That player's APM distribution.",
	},
	{
		template:    "/api/players/{playerKey}/insights/unit-production-cadence",
		summary:     "That player's army production rhythm.",
		queryParams: []string{"filter"},
	},
	{
		template: "/api/players/insights/apm-histogram",
		summary:  "APM distribution across the corpus.",
	},
	{
		template:    "/api/players/insights/unit-production-cadence",
		summary:     "Army production rhythm across the corpus.",
		queryParams: []string{"filter", "limit", "min_games"},
	},
	{
		template: "/api/players/insights/viewport-multitasking",
		summary:  "Screen-switching rate across the corpus.",
	},
	{
		template: "/api/custom/markers/definitions",
		summary:  "The marker and game-event vocabulary: labels, keys and algorithm version.",
	},
}

func init() {
	for i := range exposed {
		exposed[i].segments = splitPath(exposed[i].template)
		sort.Strings(exposed[i].queryParams)
	}
}

func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}

func isPlaceholder(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// match reports whether a concrete request path is this endpoint.
func (e endpoint) match(segments []string) bool {
	if len(segments) != len(e.segments) {
		return false
	}
	for i, want := range e.segments {
		if isPlaceholder(want) {
			if segments[i] == "" {
				return false
			}
			continue
		}
		if segments[i] != want {
			return false
		}
	}
	return true
}

func (e endpoint) checkQuery(values url.Values) error {
	allowed := map[string]bool{}
	for _, name := range e.queryParams {
		allowed[name] = true
	}
	var unknown []string
	for name := range values {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	if len(e.queryParams) == 0 {
		return fmt.Errorf("%s takes no query parameters; got %s", e.template, strings.Join(unknown, ", "))
	}
	return fmt.Errorf("%s does not accept %s; it accepts %s",
		e.template, strings.Join(unknown, ", "), strings.Join(e.queryParams, ", "))
}

// resolve validates a caller-supplied path (optionally carrying a query
// string) against the exposed surface and returns it normalized.
func resolve(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("path is required, e.g. /api/games?limit=20")
	}
	if strings.Contains(trimmed, "://") {
		return "", fmt.Errorf("pass a path, not a URL; got %q", raw)
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("could not parse %q: %w", raw, err)
	}

	segments := splitPath(parsed.Path)
	for _, e := range exposed {
		if !e.match(segments) {
			continue
		}
		if err := e.checkQuery(parsed.Query()); err != nil {
			return "", err
		}
		return parsed.String(), nil
	}
	return "", fmt.Errorf("%s is not an exposed endpoint; call get_api_schema for the ones that are", parsed.Path)
}

// renderSchema lists the exposed endpoints for the schema tool.
func renderSchema() string {
	sorted := make([]endpoint, len(exposed))
	copy(sorted, exposed)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].template < sorted[j].template })

	var b strings.Builder
	b.WriteString("Reachable endpoints (all GET, all read-only):\n\n")
	for _, e := range sorted {
		fmt.Fprintf(&b, "  GET %s\n", e.template)
		if e.summary != "" {
			fmt.Fprintf(&b, "      %s\n", e.summary)
		}
		if len(e.queryParams) > 0 {
			fmt.Fprintf(&b, "      query: %s\n", strings.Join(e.queryParams, ", "))
		}
	}
	return b.String()
}
