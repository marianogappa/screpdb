package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getFeatureFlags(t *testing.T, r http.Handler) map[string]bool {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/custom/feature-flags", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get feature flags: %d %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		FeatureFlags map[string]bool `json:"feature_flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("feature flags json: %v", err)
	}
	return payload.FeatureFlags
}

func putFeatureFlag(t *testing.T, r http.Handler, key string, enabled bool) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"key": key, "enabled": enabled})
	req := httptest.NewRequest(http.MethodPut, "/api/custom/feature-flags", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestFeatureFlagsRoundTrip(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	flags := getFeatureFlags(t, r)
	if _, ok := flags[featureFlagCompleteReplays]; !ok {
		t.Fatalf("known flag missing from payload: %v", flags)
	}

	if rec := putFeatureFlag(t, r, featureFlagCompleteReplays, true); rec.Code != http.StatusOK {
		t.Fatalf("enable flag: %d %s", rec.Code, rec.Body.String())
	}
	t.Cleanup(func() { putFeatureFlag(t, r, featureFlagCompleteReplays, false) })
	if flags := getFeatureFlags(t, r); !flags[featureFlagCompleteReplays] {
		t.Fatalf("flag did not persist: %v", flags)
	}

	if rec := putFeatureFlag(t, r, "not_a_flag", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown flag must 400, got %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/custom/feature-flags", bytes.NewReader([]byte("{")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body must 400, got %d", rec.Code)
	}
}

func TestGamingSessionEndpointAlwaysAvailable(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/custom/gaming-session", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("gaming session: %d %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("gaming session returned invalid JSON: %s", rec.Body.String())
	}
}
