.PHONY: ui-build build release

ui-build:
	cd internal/dashboard/frontend && npm ci && npm run build

build: ui-build
	go build -o screpdb .

release: ui-build
	go build -trimpath -ldflags "-s -w" -o screpdb .
