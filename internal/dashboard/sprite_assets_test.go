//go:build !js

package dashboard

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerSpriteMetricsReturnsKnownSprites(t *testing.T) {
	d := &Dashboard{}
	for range 2 {
		rec := httptest.NewRecorder()
		d.handlerSpriteMetrics(rec, httptest.NewRequest(http.MethodGet, "/sprite-metrics?names=marine,%20goliath,,lurker@burrowed,unknownthing", nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
		}
		var got map[string]spriteMetric
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if _, ok := got["unknownthing"]; ok {
			t.Error("unknown sprite was answered")
		}
		for _, name := range []string{"marine", "goliath", "lurker@burrowed"} {
			m, ok := got[name]
			if !ok {
				t.Errorf("%s missing from %v", name, got)
				continue
			}
			if m.W <= 0 || m.H <= 0 || m.AX < 0 || m.AX > m.W || m.AY < 0 || m.AY > m.H {
				t.Errorf("%s metric out of its image: %+v", name, m)
			}
		}
		if s := got["marine"].Shadow; s == nil || s.W <= 0 || s.H <= 0 {
			t.Errorf("marine shadow = %+v", s)
		}
	}
}

func TestComputeSpriteMetricUnknown(t *testing.T) {
	if m := computeSpriteMetric("definitely_not_a_unit"); m != nil {
		t.Fatalf("got %+v", m)
	}
	if m, ok := spriteMetricOf("definitely_not_a_unit"); ok || m != (spriteMetric{}) {
		t.Fatalf("got %+v %v", m, ok)
	}
	if m, ok := spriteMetricOf("definitely_not_a_unit"); ok || m != (spriteMetric{}) {
		t.Fatalf("cached miss: got %+v %v", m, ok)
	}
}

func TestTilesetFolder(t *testing.T) {
	cases := map[int]string{-1: "", 0: "badlands", 4: "jungle", 7: "twilight", 8: ""}
	for i, want := range cases {
		if got := tilesetFolder(i); got != want {
			t.Errorf("tilesetFolder(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestHandlerCreepArt(t *testing.T) {
	d := &Dashboard{}
	serve := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		d.handlerCreepArt(rec, httptest.NewRequest(http.MethodGet, "/creep-art?"+query, nil))
		return rec
	}
	for _, kind := range []string{"texture", "edges"} {
		rec := serve("tileset=jungle&kind=" + kind)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s: status %d, content type %q", kind, rec.Code, rec.Header().Get("Content-Type"))
		}
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("%s: not a png: %v", kind, err)
		}
		if kind == "edges" && img.Bounds().Dy() != 32 {
			t.Errorf("edges height = %d, want 32", img.Bounds().Dy())
		}
	}
	if rec := serve("tileset=moon&kind=texture"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown tileset: status %d", rec.Code)
	}
	if rec := serve("tileset=jungle&kind=sparkles"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad kind: status %d", rec.Code)
	}
}
