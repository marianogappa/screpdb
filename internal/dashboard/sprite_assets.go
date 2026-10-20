//go:build !js

package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
)

// spriteMetric places a sprite: its size and the pixel the unit's position falls on, in sprite pixels
// (two per map pixel). Shadow is the same for the shadow image, absent when the sprite casts none.
type spriteMetric struct {
	W      int           `json:"w"`
	H      int           `json:"h"`
	AX     int           `json:"ax"`
	AY     int           `json:"ay"`
	Shadow *spriteMetric `json:"shadow,omitempty"`
}

var spriteMetrics sync.Map

// handlerSpriteMetrics answers names=marine,lurker@burrowed with each sprite's metric; unknown names are left out.
func (d *Dashboard) handlerSpriteMetrics(w http.ResponseWriter, r *http.Request) {
	out := map[string]spriteMetric{}
	for _, raw := range strings.Split(r.URL.Query().Get("names"), ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if m, ok := spriteMetricOf(name); ok {
			out[name] = m
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(out)
}

func spriteMetricOf(name string) (spriteMetric, bool) {
	if v, ok := spriteMetrics.Load(name); ok {
		m, found := v.(*spriteMetric)
		return derefMetric(m), found && m != nil
	}
	m := computeSpriteMetric(name)
	spriteMetrics.Store(name, m)
	return derefMetric(m), m != nil
}

func derefMetric(m *spriteMetric) spriteMetric {
	if m == nil {
		return spriteMetric{}
	}
	return *m
}

func computeSpriteMetric(name string) *spriteMetric {
	key, variant, _ := strings.Cut(name, "@")
	_, query, ok := resolveGameAssetIconQuery(key)
	if !ok {
		return nil
	}
	opts := scmapanalyzer.UnitImageOptions{Variant: scmapanalyzer.UnitVariant(variant)}
	img, err := scmapanalyzer.UnitOrBuildingImage(query, opts)
	if err != nil {
		return nil
	}
	m := &spriteMetric{W: img.Bounds().Dx(), H: img.Bounds().Dy(), AX: img.Bounds().Dx() / 2, AY: img.Bounds().Dy() / 2}
	if x, y, ok := scmapanalyzer.UnitImageAnchor(query, opts); ok {
		m.AX, m.AY = x, y
	}
	shadowOpts := opts
	shadowOpts.Shadow = true
	if shadow, err := scmapanalyzer.UnitShadowImage(query, opts.Variant); err == nil {
		if x, y, ok := scmapanalyzer.UnitImageAnchor(query, shadowOpts); ok {
			m.Shadow = &spriteMetric{W: shadow.Bounds().Dx(), H: shadow.Bounds().Dy(), AX: x, AY: y}
		}
	}
	return m
}

// tilesetFolders are BW's tilesets by the index a map stores.
var tilesetFolders = [8]string{"badlands", "platform", "install", "ashworld", "jungle", "desert", "ice", "twilight"}

func tilesetFolder(index int) string {
	if index < 0 || index >= len(tilesetFolders) {
		return ""
	}
	return tilesetFolders[index]
}

// handlerCreepArt serves a tileset's creep texture (kind=texture) or its edge frames (kind=edges).
func (d *Dashboard) handlerCreepArt(w http.ResponseWriter, r *http.Request) {
	tileset := strings.TrimSpace(r.URL.Query().Get("tileset"))
	known := false
	for _, t := range tilesetFolders {
		known = known || t == tileset
	}
	if !known {
		http.NotFound(w, r)
		return
	}
	var data []byte
	var err error
	switch r.URL.Query().Get("kind") {
	case "texture":
		data, err = scmapanalyzer.CreepTexturePNG(tileset)
	case "edges":
		data, err = scmapanalyzer.CreepEdgesPNG(tileset)
	default:
		http.Error(w, "bad kind", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}
