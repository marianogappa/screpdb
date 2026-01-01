package dashboard

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (d *Dashboard) handlerDeleteDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardWidgetID, err := strconv.Atoi(vars["wid"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("dashboard widget id missing or should be numeric"))
		return
	}

	err = d.deleteDashboardWidget(d.ctx, int64(dashboardWidgetID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error deleting dashboard widget: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}

