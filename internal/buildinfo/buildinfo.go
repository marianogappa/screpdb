package buildinfo

// Version is set at build time via -ldflags "-X github.com/marianogappa/screpdb/internal/buildinfo.Version=vX.Y.Z".
// Defaults to "dev" for local non-release builds.
var Version = "dev"

// Commit is the short git commit SHA the binary was built from, set at build
// time via -ldflags "-X github.com/marianogappa/screpdb/internal/buildinfo.Commit=abc1234".
// Defaults to "unknown" for local non-release builds. Surfaced in the dashboard
// footer and crash reports so testers can pinpoint exactly which build they ran.
var Commit = "unknown"
