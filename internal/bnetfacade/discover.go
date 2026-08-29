package bnetfacade

import (
	"context"
	"fmt"
	"sort"
)

// DiscoverBridgeAddr enumerates loopback listening ports using platform-specific
// methods and probes each for the SC:R web-api bridge. It returns the first
// address that responds (200 or 401), or an error if no bridge is found.
func DiscoverBridgeAddr(ctx context.Context) (string, error) {
	ports, err := loopbackListeningPorts()
	if err != nil {
		return "", fmt.Errorf("bnetfacade: port discovery: %w", err)
	}
	sort.Ints(ports)
	for _, port := range ports {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		state := ProbeBridge(ctx, addr)
		if state == BridgeConnected || state == BridgeOffline {
			return addr, nil
		}
	}
	return "", fmt.Errorf("bnetfacade: no SC:R bridge found among %d loopback ports", len(ports))
}
