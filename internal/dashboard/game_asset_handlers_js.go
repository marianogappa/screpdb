//go:build js

package dashboard

import "net/http"

func (d *Dashboard) handlerGameAssetUnit(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (d *Dashboard) handlerGameAssetBuilding(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (d *Dashboard) handlerSpriteMetrics(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (d *Dashboard) handlerCreepArt(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (d *Dashboard) handlerGameAssetMap(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
