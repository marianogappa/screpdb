package bnetfacade

import (
	"context"
	"fmt"
	"sort"
)

// DiscoverBridgeAddr enumerates loopback listening ports using platform-specific
// methods and probes each for the SC:R web-api bridge. Only a fully
// authenticated response (200 with JSON) confirms a port as SC:R's bridge —
// a 401 during discovery is ignored because other loopback services (e.g. the
// Battle.net Agent) may also return 401 on arbitrary paths. Once a bridge addr
// is discovered, the caller should use ProbeBridge to track state transitions
// including 401 (offline / not logged in).
func DiscoverBridgeAddr(ctx context.Context) (string, error) {
	ports, err := loopbackListeningPorts()
	if err != nil {
		return "", fmt.Errorf("bnetfacade: port discovery: %w", err)
	}
	sort.Ints(ports)
	for _, port := range ports {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		if ProbeBridge(ctx, addr) == BridgeConnected {
			return addr, nil
		}
	}
	return "", fmt.Errorf("bnetfacade: no SC:R bridge found among %d loopback ports", len(ports))
}
