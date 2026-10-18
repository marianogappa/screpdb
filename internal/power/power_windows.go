package power

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
	procSetThreadPriority    = kernel32.NewProc("SetThreadPriority")
)

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

const (
	acOffline            = 0
	threadPriorityLowest = -2
)

func onBattery() bool {
	var s systemPowerStatus
	ok, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	return ok != 0 && s.ACLineStatus == acOffline
}

// LowerThreadPriority locks the goroutine to its OS thread and lowers that thread's priority. The
// goroutine must not unlock: the thread then dies with it.
func LowerThreadPriority() {
	runtime.LockOSThread()
	priority := int32(threadPriorityLowest)
	_, _, _ = procSetThreadPriority.Call(uintptr(windows.CurrentThread()), uintptr(priority))
}
