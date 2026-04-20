.PHONY: openapi-generate ui-build build release

openapi-generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/oapi-codegen.yaml api/openapi/dashboard.v1.yaml
	go run ./internal/dashboard/tools/gen_openapi_bridge

ui-build:
	cd internal/dashboard/frontend && npm ci && npm run build

build: ui-build
	go build -o screpdb .

release: ui-build
	go build -trimpath -ldflags "-s -w" -o screpdb .
