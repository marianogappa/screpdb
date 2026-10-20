//go:build !windows && !js

package selfupdate

// SelfInstall is Windows-only: elsewhere the shell and Homebrew installers
// already put the binary in place.
func SelfInstall() bool { return false }
