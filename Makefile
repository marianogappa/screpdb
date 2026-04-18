.PHONY: openapi-generate map-images-sync ui-build build release

openapi-generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/oapi-codegen.yaml api/openapi/dashboard.v1.yaml
	go run ./internal/dashboard/tools/gen_openapi_bridge

map-images-sync:
	@src_dir="$$(go list -m -f '{{.Dir}}' github.com/marianogappa/scmapanalyzer)/map-images"; \
	if [ ! -d "$$src_dir" ]; then \
		echo "map images source directory not found: $$src_dir"; \
		exit 1; \
	fi; \
	mkdir -p internal/dashboard/frontend/public/map-images; \
	cp -f "$$src_dir"/* internal/dashboard/frontend/public/map-images/

ui-build: map-images-sync
	cd internal/dashboard/frontend && npm ci && npm run build

build: ui-build
	go build -o screpdb .

release: ui-build
	go build -trimpath -ldflags "-s -w" -o screpdb .
