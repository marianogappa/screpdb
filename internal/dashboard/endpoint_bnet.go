package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (d *Dashboard) handlerBnetStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.getBnetStatus())
}

func (d *Dashboard) handlerBnetToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	d.setBnetDisabled(req.Disabled)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.getBnetStatus())
}

// handlerBnetCountryCodes answers, from cache only, which of the given players
// we now know a country for. It never fetches, so a page can poll it while a
// backfill runs and paint flags in as they land without spending any bridge
// budget of its own. Pending reports whether a backfill is still in flight,
// which is how the caller knows to keep polling.
func (d *Dashboard) handlerBnetCountryCodes(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("players")
	var playerKeys []string
	seen := map[string]struct{}{}
	for _, name := range strings.Split(raw, ",") {
		key := normalizePlayerKey(name)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		playerKeys = append(playerKeys, key)
	}
	if len(playerKeys) > bnetCountryCodeLookupMaxPlayers {
		playerKeys = playerKeys[:bnetCountryCodeLookupMaxPlayers]
	}
	codes, err := d.countryCodesByPlayerKeys(playerKeys)
	if err != nil {
		http.Error(w, "country code lookup failed", http.StatusInternalServerError)
		return
	}
	if codes == nil {
		codes = map[string]string{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		CountryCodes map[string]string `json:"country_codes"`
		Pending      bool              `json:"pending"`
	}{CountryCodes: codes, Pending: d.bnetBackfillActive.Load() > 0})
}

// bnetCountryCodeLookupMaxPlayers bounds the query the poll endpoint builds, so
// a hand-crafted URL cannot turn one request into an unbounded IN clause.
const bnetCountryCodeLookupMaxPlayers = 200
