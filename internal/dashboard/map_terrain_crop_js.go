//go:build js

package dashboard

import "net/http"

// The browser demo cannot render terrain crops; GameMap keeps the full-map image there.
func (d *Dashboard) handlerMapTerrainCrop(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
