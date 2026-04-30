package buildinfo

// Version is set at build time via -ldflags "-X github.com/marianogappa/screpdb/internal/buildinfo.Version=vX.Y.Z".
// Defaults to "dev" for local non-release builds.
var Version = "dev"
