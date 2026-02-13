package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardSPA_RootReturnsIndexHTML(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("root status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("expected html doctype in root response")
	}
}

func TestDashboardSPA_NonexistentPathFallsBackToIndexHTML(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboards/default/widgets/999", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("fallback status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("expected html doctype in fallback response")
	}
}

func TestDashboardSPA_APIRoutesStillWork(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("api health status %d: %s", rec.Code, rec.Body.String())
	}
}
