//go:build js

package dashboard

import (
	"net/http"

	"github.com/marianogappa/screpdb/internal/models"
)

type digestMemo struct{}

func (d *Dashboard) applyEngineToGameDetail(*workflowGameDetail) {}

func (d *Dashboard) applyEngineEvents(*workflowGameDetail, *models.MapContextLayout) {}

func (d *Dashboard) handlerEngineSnapshot(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "no engine data in the browser demo", http.StatusNotFound)
}

func (d *Dashboard) applyEngineHotkeys(int64, *hotkeyTimelinePayload) {}

type engineLayoutMemo struct{}
