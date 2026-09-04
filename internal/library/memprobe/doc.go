// Package memprobe measures what a loaded replay library costs in memory.
//
// The corpus is held for the life of the process, so its size per replay is a
// design constraint rather than a detail, and it cannot be read off a running
// server: the heap also holds one-time costs (the marker registry, the
// embedded map data) and whatever the parses left behind. These probes
// separate those out. They are skipped unless pointed at a replay folder:
//
//	PROBE_FOLDER=~/path/to/replays go test ./internal/library/memprobe/ -v
package memprobe
