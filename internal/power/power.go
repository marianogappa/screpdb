// Package power answers the two questions a background job needs to be a good citizen: is StarCraft:
// Remastered running, and how to run the current OS thread at low priority.
package power

// StarCraftRunning reports whether StarCraft: Remastered is running. It answers false when the platform
// cannot tell.
func StarCraftRunning() bool { return starCraftRunning() }
