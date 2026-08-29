//go:build darwin

package bnetfacade

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// loopbackListeningPorts uses macOS's BSD lsof to enumerate TCP sockets in the
// LISTEN state bound to loopback addresses.
func loopbackListeningPorts() ([]int, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-n", "-P").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lsof: %w: %s", err, out)
	}
	return parseLsofOutput(string(out)), nil
}

func parseLsofOutput(output string) []int {
	seen := map[int]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := fields[8]
		if !isLoopbackLsofName(name) {
			continue
		}
		port := extractPortFromLsofName(name)
		if port > 0 {
			seen[port] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	return ports
}

func isLoopbackLsofName(name string) bool {
	return strings.HasPrefix(name, "127.0.0.1:") ||
		strings.HasPrefix(name, "[::1]:") ||
		strings.HasPrefix(name, "localhost:")
}

func extractPortFromLsofName(name string) int {
	idx := strings.LastIndex(name, ":")
	if idx < 0 {
		return 0
	}
	portStr := strings.TrimSuffix(name[idx+1:], " (LISTEN)")
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return p
}
