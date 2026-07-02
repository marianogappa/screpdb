//go:build !windows

package main

// runLauncher is never called off Windows (winsandbox.ShouldLaunch is false
// there); this stub exists only so main stays cross-platform.
func runLauncher() int { return 0 }
