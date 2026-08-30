package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// featureFlagGamingSession gates the Gaming Session view: the summary of the
// run of games you have just played. It is a preview, so it defaults off.
const featureFlagGamingSession = "gaming_session"

// knownFeatureFlags is the allowlist. Writes to anything outside it are
// rejected, so a stale client cannot litter the settings row with keys nothing
// reads, and a flag that is retired stops being settable the moment it leaves
// this list.
var knownFeatureFlags = map[string]struct{}{
	featureFlagGamingSession: {},
}

func (d *Dashboard) featureFlags(ctx context.Context) (map[string]bool, error) {
	raw, err := d.dbStore.GetFeatureFlagsJSON(ctx, globalReplayFilterConfigKey)
	if err != nil {
		return nil, fmt.Errorf("loading feature flags: %w", err)
	}
	stored := map[string]bool{}
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		// A malformed value is treated as "everything off" rather than an
		// error: a preview switch is never worth failing a page load over.
		stored = map[string]bool{}
	}
	flags := make(map[string]bool, len(knownFeatureFlags))
	for key := range knownFeatureFlags {
		flags[key] = stored[key]
	}
	return flags, nil
}

func (d *Dashboard) featureFlagEnabled(ctx context.Context, key string) bool {
	flags, err := d.featureFlags(ctx)
	if err != nil {
		return false
	}
	return flags[key]
}

func (d *Dashboard) setFeatureFlag(ctx context.Context, key string, enabled bool) (map[string]bool, error) {
	if _, ok := knownFeatureFlags[key]; !ok {
		return nil, fmt.Errorf("unknown feature flag %q", key)
	}
	flags, err := d.featureFlags(ctx)
	if err != nil {
		return nil, err
	}
	flags[key] = enabled
	encoded, err := json.Marshal(flags)
	if err != nil {
		return nil, err
	}
	if err := d.dbStore.SetFeatureFlagsJSON(ctx, globalReplayFilterConfigKey, string(encoded)); err != nil {
		return nil, fmt.Errorf("saving feature flags: %w", err)
	}
	return flags, nil
}

func (d *Dashboard) handlerFeatureFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := d.featureFlags(r.Context())
	if err != nil {
		http.Error(w, "failed to load feature flags", http.StatusInternalServerError)
		return
	}
	writeFeatureFlags(w, flags)
}

func (d *Dashboard) handlerSetFeatureFlag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	flags, err := d.setFeatureFlag(r.Context(), strings.TrimSpace(req.Key), req.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeFeatureFlags(w, flags)
}

func writeFeatureFlags(w http.ResponseWriter, flags map[string]bool) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		FeatureFlags map[string]bool `json:"feature_flags"`
	}{FeatureFlags: flags})
}
