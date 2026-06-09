.PHONY: openapi-generate spec-generate ui-build ui-test build release cross-binaries windows-syso clean-windows-syso

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
REL_LDFLAGS := -s -w -X github.com/marianogappa/screpdb/internal/buildinfo.Version=$(VERSION) -X github.com/marianogappa/screpdb/internal/buildinfo.Commit=$(COMMIT)

# Extract a plain MAJOR.MINOR.PATCH triple for the Windows PE FixedFileInfo
# numeric version fields. Only a real semver tag (vX.Y.Z[-...]) yields a number;
# untagged builds (git describe falls back to a bare commit SHA) have no
# meaningful numeric version, so default to 0.0.0. This avoids feeding the SHA
# to awk, which would misparse e.g. "182e64f" as scientific notation (1.82e66)
# or a 7-digit SHA as an out-of-range major, both of which goversioninfo rejects.
NUMVER := $(shell echo "$(VERSION)" | grep -Eo '^v?[0-9]+\.[0-9]+\.[0-9]+' | sed 's/^v//')
VER_MAJOR := $(shell echo "$(NUMVER)" | awk -F. '{print int($$1)+0}')
VER_MINOR := $(shell echo "$(NUMVER)" | awk -F. '{print int($$2)+0}')
VER_PATCH := $(shell echo "$(NUMVER)" | awk -F. '{print int($$3)+0}')

GOVERSIONINFO := go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0
VERSIONINFO_JSON := build/windows/versioninfo.json
WINDOWS_ICON := build/windows/icon.ico
SYSO_VER_FLAGS := \
	-icon $(WINDOWS_ICON) \
	-file-version "$(VERSION)" \
	-product-version "$(VERSION)" \
	-ver-major $(VER_MAJOR) -ver-minor $(VER_MINOR) -ver-patch $(VER_PATCH) -ver-build 0 \
	-product-ver-major $(VER_MAJOR) -product-ver-minor $(VER_MINOR) -product-ver-patch $(VER_PATCH) -product-ver-build 0

openapi-generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/oapi-codegen.yaml api/openapi/dashboard.v1.yaml
	go run ./internal/dashboard/tools/gen_openapi_bridge

# Regenerate SPECIFICATION.md from the Go source of truth. Equivalent to
# `go generate ./...` for the spec package; CI's `go test ./...` fails if stale.
spec-generate:
	go run ./internal/spec/tools/genspec

ui-build:
	cd internal/dashboard/frontend && npm ci && npm run build

# Frontend unit tests (route/state contracts). Zero extra deps: node --test.
ui-test:
	cd internal/dashboard/frontend && npm ci && npm test

build: ui-build
	go build -ldflags "$(REL_LDFLAGS)" -o screpdb .

release: ui-build
	go build -trimpath -ldflags "$(REL_LDFLAGS)" -o screpdb .

# Generate Windows PE resource files (icon + version info) for both Windows binaries.
# Output filenames use the *_windows_amd64.syso suffix so Go links them only into windows/amd64 builds.
windows-syso:
	$(GOVERSIONINFO) $(SYSO_VER_FLAGS) \
		-description "screpdb - StarCraft replay analysis tool (CLI)" \
		-original-name "screpdb-windows-amd64.exe" \
		-product-name "screpdb" \
		-o resource_windows_amd64.syso \
		$(VERSIONINFO_JSON)
	$(GOVERSIONINFO) $(SYSO_VER_FLAGS) \
		-description "screpdb dashboard - StarCraft replay analysis dashboard" \
		-original-name "screpdb-dashboard-windows-amd64.exe" \
		-product-name "screpdb dashboard" \
		-o cmd/windows-dashboard/resource_windows_amd64.syso \
		$(VERSIONINFO_JSON)

clean-windows-syso:
	rm -f resource_windows_amd64.syso cmd/windows-dashboard/resource_windows_amd64.syso

# Release-style cross-compiles for GitHub Releases: Windows CLI + dashboard; Linux/Darwin root CLI only (linux amd64 name unchanged).
cross-binaries: ui-build windows-syso
	mkdir -p dist
	env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-windows-amd64.exe .
	env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS) -H=windowsgui" -o dist/screpdb-dashboard-windows-amd64.exe ./cmd/windows-dashboard
	env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-linux-amd64 .
	env GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-linux-arm64 .
	env GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-darwin-amd64 .
	env GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(REL_LDFLAGS)" -o dist/screpdb-darwin-arm64 .
