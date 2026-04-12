package dashboard

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
)

type openAPIStrictAdapter struct {
	service dashboardservice.DashboardService
}

var _ apigen.StrictServerInterface = (*openAPIStrictAdapter)(nil)

func newOpenAPIStrictAdapter(service dashboardservice.DashboardService) *openAPIStrictAdapter {
	return &openAPIStrictAdapter{service: service}
}

func responseFromPayload[Req any, Resp any](ctx context.Context, req Req, fn func(context.Context, Req) (dashboardservice.HandlerResult, error), build func(any) Resp) (Resp, error) {
	var zero Resp
	payload, err := fn(ctx, req)
	if err != nil {
		return zero, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return build(payload), nil
}
