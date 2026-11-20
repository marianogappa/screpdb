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
	req := httptest.NewRequest(http.MethodGet, "/games/999/some-deep-link", nil)
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

func TestDashboardHeadless_NoSPAButAPIWorks(t *testing.T) {
	dash := newTestDashboard(t)
	dash.headless = true
	router := dash.setupRouter()

	// UI paths must NOT serve the SPA in headless mode.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("headless root status = %d, want 404", rec.Code)
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("headless root should not serve the SPA html")
	}

	// API routes must still work.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("headless api health status %d: %s", rec.Code, rec.Body.String())
	}
}
