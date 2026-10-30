package power

import (
	"runtime"

	"golang.org/x/sys/unix"
)

func starCraftRunning() bool { return false }

const lowNice = 10

// LowerThreadPriority locks the goroutine to its OS thread and renices only that thread, which Linux
// allows through its thread id. The goroutine must not unlock: the thread then dies with it.
func LowerThreadPriority() {
	runtime.LockOSThread()
	_ = unix.Setpriority(unix.PRIO_PROCESS, unix.Gettid(), lowNice)
}
