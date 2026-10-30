package power

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procSetThreadPriority = kernel32.NewProc("SetThreadPriority")
)

const threadPriorityLowest = -2

// starCraftRunning reads the process names from a Toolhelp snapshot, which needs no handle to the
// processes themselves, so it works from the Low-integrity worker.
func starCraftRunning() bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		if isStarCraftProcess(windows.UTF16ToString(entry.ExeFile[:])) {
			return true
		}
	}
	return false
}

// LowerThreadPriority locks the goroutine to its OS thread and lowers that thread's priority. The
// goroutine must not unlock: the thread then dies with it.
func LowerThreadPriority() {
	runtime.LockOSThread()
	priority := int32(threadPriorityLowest)
	_, _, _ = procSetThreadPriority.Call(uintptr(windows.CurrentThread()), uintptr(priority))
}
