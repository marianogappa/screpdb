package dashboard

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard url missing"))
		return
	}

	err := d.deleteDashboard(d.ctx, dashboardURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error deleting dashboard: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}

