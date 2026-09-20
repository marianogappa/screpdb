package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// featureFlagDisableCompleteReplays turns OFF the #341 enrichment worker, which
// downloads co-players' fuller recordings of multiplayer games the user left
// early. The worker is on by default now that the edge cases have soaked, so
// the switch reads as an opt-out.
//
// It is stated negatively on purpose. Flags default to false, and the feature
// has to default to on, so the stored zero value has to mean "enabled". A
// positively named flag would have needed every existing install to carry an
// explicit true, and anyone whose settings predate it would silently lose the
// feature.
//
// The old positive key, complete_replays, is deliberately not migrated: it left
// this allowlist, so it stops being read or written, and the people who had
// switched it on in Labs simply keep the behaviour they already had.
const featureFlagDisableCompleteReplays = "disable_complete_replays"

// knownFeatureFlags is the allowlist. Writes to anything outside it are
// rejected, so a stale client cannot litter the settings row with keys nothing
// reads, and a flag that is retired stops being settable the moment it leaves
// this list.
var knownFeatureFlags = map[string]struct{}{
	featureFlagDisableCompleteReplays: {},
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
