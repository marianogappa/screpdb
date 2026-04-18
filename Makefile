.PHONY: openapi-generate map-images-sync ui-build build release

openapi-generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/oapi-codegen.yaml api/openapi/dashboard.v1.yaml
	go run ./internal/dashboard/tools/gen_openapi_bridge

map-images-sync:
	@module_dir="$$(go list -m -f '{{.Dir}}' github.com/marianogappa/scmapanalyzer 2>/dev/null || true)"; \
	src_dir="$$module_dir/map-images"; \
	dst_dir="internal/dashboard/frontend/public/map-images"; \
	mkdir -p "$$dst_dir"; \
	if [ -d "$$src_dir" ]; then \
		cp -f "$$src_dir"/* "$$dst_dir"/; \
	elif compgen -G "$$dst_dir/*" >/dev/null; then \
		echo "map images source directory not found: $$src_dir; using checked-in assets from $$dst_dir"; \
	else \
		echo "map images source directory not found: $$src_dir"; \
		exit 1; \
	fi

ui-build: map-images-sync
	cd internal/dashboard/frontend && npm ci && npm run build

build: ui-build
	go build -o screpdb .

release: ui-build
	go build -trimpath -ldflags "-s -w" -o screpdb .
