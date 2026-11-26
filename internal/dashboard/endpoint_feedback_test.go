package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/feedbackfacade"
)

func TestHandlerFeedback(t *testing.T) {
	shot := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	validBody := `{"message":"hi","contact":"me","language":"en","screenshots":["` + base64.StdEncoding.EncodeToString(shot) + `"]}`
	cases := []struct {
		name        string
		contentType string
		body        string
		sendErr     error
		wantStatus  int
		wantResult  string
	}{
		{"sent", "application/json", validBody, nil, http.StatusOK, "sent"},
		{"form posts are refused", "application/x-www-form-urlencoded", validBody, nil, http.StatusUnsupportedMediaType, "invalid"},
		{"bad json", "application/json", "{", nil, http.StatusBadRequest, "invalid"},
		{"bad base64", "application/json", `{"message":"hi","screenshots":["!!"]}`, nil, http.StatusBadRequest, "invalid"},
		{"invalid", "application/json", validBody, feedbackfacade.ErrInvalid, http.StatusBadRequest, "invalid"},
		{"rate limited", "application/json", validBody, feedbackfacade.ErrRateLimited, http.StatusTooManyRequests, "rate_limited"},
		{"upstream failure", "application/json", validBody, errors.New("boom"), http.StatusBadGateway, "failed"},
	}
	original := sendFeedback
	defer func() { sendFeedback = original }()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got feedbackfacade.Submission
			sendFeedback = func(_ context.Context, s feedbackfacade.Submission) error {
				got = s
				return c.sendErr
			}
			req := httptest.NewRequest(http.MethodPost, "/api/custom/feedback", strings.NewReader(c.body))
			req.Header.Set("Content-Type", c.contentType)
			rec := httptest.NewRecorder()
			(&Dashboard{}).handlerFeedback(rec, req)
			if rec.Code != c.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, c.wantStatus)
			}
			var resp map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp["result"] != c.wantResult {
				t.Fatalf("result %q, want %q", resp["result"], c.wantResult)
			}
			if c.name == "sent" && (got.Message != "hi" || got.Contact != "me" || len(got.Screenshots) != 1 || string(got.Screenshots[0]) != string(shot)) {
				t.Fatalf("unexpected submission %+v", got)
			}
		})
	}
}
