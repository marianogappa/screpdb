//go:build js

package dashboard

import "net/http"

func (d *Dashboard) handlerHotkeyMap(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
