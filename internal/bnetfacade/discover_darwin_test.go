//go:build darwin

package bnetfacade

import "testing"

func TestParseLsofOutput(t *testing.T) {
	sample := `COMMAND     PID     USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
StarCraf  12345 testuser    8u  IPv4 0xabcdef1234567890      0t0  TCP 127.0.0.1:50250 (LISTEN)
StarCraf  12345 testuser    9u  IPv4 0xabcdef1234567891      0t0  TCP 127.0.0.1:50251 (LISTEN)
nginx     99999 testuser    6u  IPv4 0xabcdef1234567892      0t0  TCP *:80 (LISTEN)
sshd      11111 testuser    3u  IPv4 0xabcdef1234567893      0t0  TCP 192.168.1.1:22 (LISTEN)
`
	ports := parseLsofOutput(sample)
	portSet := map[int]bool{}
	for _, p := range ports {
		portSet[p] = true
	}
	if !portSet[50250] {
		t.Errorf("expected port 50250 in results, got %v", ports)
	}
	if !portSet[50251] {
		t.Errorf("expected port 50251 in results, got %v", ports)
	}
	if portSet[80] {
		t.Errorf("port 80 (wildcard bind) should not appear in loopback results")
	}
	if portSet[22] {
		t.Errorf("port 22 (non-loopback) should not appear in loopback results")
	}
}

func TestParseLsofOutput_IPv6Loopback(t *testing.T) {
	sample := `COMMAND     PID     USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
StarCraf  12345 testuser    8u  IPv6 0xabcdef1234567890      0t0  TCP [::1]:50250 (LISTEN)
`
	ports := parseLsofOutput(sample)
	portSet := map[int]bool{}
	for _, p := range ports {
		portSet[p] = true
	}
	if !portSet[50250] {
		t.Errorf("expected port 50250 for [::1] loopback, got %v", ports)
	}
}

func TestParseLsofOutput_Empty(t *testing.T) {
	ports := parseLsofOutput("")
	if len(ports) != 0 {
		t.Errorf("expected 0 ports for empty input, got %v", ports)
	}
}
