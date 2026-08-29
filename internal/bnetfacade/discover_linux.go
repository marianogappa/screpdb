//go:build linux

package bnetfacade

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loopbackListeningPorts parses /proc/net/tcp to enumerate TCP sockets in the
// LISTEN state (st=0A) bound to 127.0.0.1 (local_address 0100007F). This
// avoids depending on lsof, which may not be installed on minimal Linux systems.
func loopbackListeningPorts() ([]int, error) {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, fmt.Errorf("read /proc/net/tcp: %w", err)
	}
	return parseProcNetTCP(string(data)), nil
}

func parseProcNetTCP(data string) []int {
	seen := map[int]bool{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "sl") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// st column (index 3): 0A = LISTEN
		if fields[3] != "0A" {
			continue
		}
		// local_address column (index 1): ADDR:PORT in hex, little-endian
		addr, port, ok := parseProcNetAddr(fields[1])
		if !ok {
			continue
		}
		if addr == "7f000001" {
			seen[port] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	return ports
}

// parseProcNetAddr parses "0100007F:1F90" into ("7f000001", 8080, true).
// The IP is stored little-endian in /proc/net/tcp; we reverse it to
// standard big-endian hex for comparison.
func parseProcNetAddr(s string) (string, int, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	ipHex := parts[0]
	portHex := parts[1]
	if len(ipHex) != 8 {
		return "", 0, false
	}
	ipBytes, err := hex.DecodeString(ipHex)
	if err != nil || len(ipBytes) != 4 {
		return "", 0, false
	}
	// Reverse from little-endian to big-endian.
	reversed := fmt.Sprintf("%02x%02x%02x%02x", ipBytes[3], ipBytes[2], ipBytes[1], ipBytes[0])
	port, err := strconv.ParseInt(portHex, 16, 32)
	if err != nil {
		return "", 0, false
	}
	return reversed, int(port), true
}
