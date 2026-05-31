package spec

// Regenerate SPECIFICATION.md (at the repo root) from the Go source of truth.
// Run `go generate ./...` after changing any golden value, then commit the
// result. The guard test (spec_guard_test.go) fails CI if the committed file is
// stale.
//
//go:generate go run ./tools/genspec
