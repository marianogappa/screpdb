.PHONY: openapi-generate ui-build build release cross-binaries

REL_LDFLAGS := -s -w

openapi-generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/oapi-codegen.yaml api/openapi/dashboard.v1.yaml
	go run ./internal/dashboard/tools/gen_openapi_bridge

ui-build:
	cd internal/dashboard/frontend && npm ci && npm run build

build: ui-build
	go build -o screpdb .

release: ui-build
	go build -trimpath -ldflags "-s -w" -o screpdb .

# Release-style cross-compiles for GitHub Releases: Windows CLI + dashboard; Linux/Darwin root CLI only (linux amd64 name unchanged).
cross-binaries: ui-build
	mkdir -p dist
	env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-windows-amd64.exe .
	env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS) -H=windowsgui" -o dist/screpdb-dashboard-windows-amd64.exe ./cmd/windows-dashboard
	env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-linux-amd64 .
	env GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-linux-arm64 .
	env GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-darwin-amd64 .
	env GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-darwin-arm64 .
