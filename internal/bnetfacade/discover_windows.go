//go:build windows

package bnetfacade

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MIB_TCP_STATE_LISTEN is the TCP state constant for listening sockets.
const mibTCPStateListen = 2

// MIB_TCP_ROW_OWNER_PID matches the Windows MIB_TCPROW_OWNER_PID structure.
type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}

// MIB_TCPTABLE_OWNER_PID is the table header returned by GetExtendedTcpTable.
type mibTCPTableOwnerPID struct {
	NumEntries uint32
	Table      [1]mibTCPRowOwnerPID
}

var iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
var procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")

// loopbackListeningPorts calls GetExtendedTcpTable to enumerate all TCP sockets
// in the LISTEN state bound to 127.0.0.1. This works from Low-integrity
// processes because the TCP table is a read-only query with no privilege
// requirement.
func loopbackListeningPorts() ([]int, error) {
	const afINET = 2 // AF_INET
	const tcpTableOwnerPIDAll = 5

	var size uint32
	// First call to get the required buffer size.
	ret, _, _ := procGetExtendedTcpTable.Call(
		0, uintptr(unsafe.Pointer(&size)), 1,
		afINET, tcpTableOwnerPIDAll, 0,
	)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) && ret != 0 {
		return nil, fmt.Errorf("GetExtendedTcpTable size query: error %d", ret)
	}

	buf := make([]byte, size)
	ret, _, _ = procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1,
		afINET, tcpTableOwnerPIDAll, 0,
	)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedTcpTable: error %d", ret)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	const rowSize = unsafe.Sizeof(mibTCPRowOwnerPID{})
	const loopbackAddr = 0x0100007F // 127.0.0.1 in network byte order (little-endian on x86)

	seen := map[int]bool{}
	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + uintptr(i)*rowSize
		if offset+rowSize > uintptr(len(buf)) {
			break
		}
		row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
		if row.State != mibTCPStateListen {
			continue
		}
		if row.LocalAddr != loopbackAddr {
			continue
		}
		// LocalPort is stored in network byte order (big-endian).
		port := int(binary.BigEndian.Uint16((*[4]byte)(unsafe.Pointer(&row.LocalPort))[:2]))
		seen[port] = true
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	return ports, nil
}
