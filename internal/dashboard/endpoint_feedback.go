package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/marianogappa/screpdb/internal/feedbackfacade"
)

const maxFeedbackRequestBytes = 14 << 20

type feedbackRequest struct {
	Message     string                  `json:"message"`
	Contact     string                  `json:"contact"`
	Language    string                  `json:"language"`
	Screenshots []string                `json:"screenshots"`
	Replays     []feedbackRequestReplay `json:"replays"`
}

type feedbackRequestReplay struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

var sendFeedback = feedbackfacade.Send

// handlerFeedback only accepts application/json because a cross-site page can
// POST a form to localhost without a CORS preflight, but not JSON.
func (d *Dashboard) handlerFeedback(w http.ResponseWriter, r *http.Request) {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeFeedbackResult(w, http.StatusUnsupportedMediaType, "invalid")
		return
	}
	var req feedbackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFeedbackRequestBytes)).Decode(&req); err != nil {
		writeFeedbackResult(w, http.StatusBadRequest, "invalid")
		return
	}
	submission := feedbackfacade.Submission{Message: req.Message, Contact: req.Contact, Language: req.Language}
	for _, encoded := range req.Screenshots {
		shot, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			writeFeedbackResult(w, http.StatusBadRequest, "invalid")
			return
		}
		submission.Screenshots = append(submission.Screenshots, shot)
	}
	for _, replay := range req.Replays {
		data, err := base64.StdEncoding.DecodeString(replay.Data)
		if err != nil {
			writeFeedbackResult(w, http.StatusBadRequest, "invalid")
			return
		}
		submission.Replays = append(submission.Replays, feedbackfacade.Replay{Name: replay.Name, Data: data})
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 90*time.Second)
	defer cancel()
	err := sendFeedback(ctx, submission)
	switch {
	case err == nil:
		writeFeedbackResult(w, http.StatusOK, "sent")
	case errors.Is(err, feedbackfacade.ErrInvalid):
		writeFeedbackResult(w, http.StatusBadRequest, "invalid")
	case errors.Is(err, feedbackfacade.ErrRateLimited):
		writeFeedbackResult(w, http.StatusTooManyRequests, "rate_limited")
	default:
		log.Printf("sending feedback failed: %v", err)
		writeFeedbackResult(w, http.StatusBadGateway, "failed")
	}
}

func writeFeedbackResult(w http.ResponseWriter, status int, result string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"result": result})
}
