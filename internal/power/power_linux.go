package power

import (
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const powerSupplyDir = "/sys/class/power_supply"

func onBattery() bool {
	if iofacade.AllowReadOnlyDir(powerSupplyDir) != nil {
		return false
	}
	entries, err := iofacade.ReadDir(powerSupplyDir)
	if err != nil {
		return false
	}
	sawMains, discharging := false, false
	for _, e := range entries {
		read := func(name string) string {
			b, _ := iofacade.ReadFile(filepath.Join(powerSupplyDir, e.Name(), name))
			return strings.TrimSpace(string(b))
		}
		switch read("type") {
		case "Mains", "USB":
			sawMains = true
			if read("online") == "1" {
				return false
			}
		case "Battery":
			discharging = discharging || read("status") == "Discharging"
		}
	}
	return sawMains || discharging
}

const lowNice = 10

// LowerThreadPriority locks the goroutine to its OS thread and renices only that thread, which Linux
// allows through its thread id. The goroutine must not unlock: the thread then dies with it.
func LowerThreadPriority() {
	runtime.LockOSThread()
	_ = unix.Setpriority(unix.PRIO_PROCESS, unix.Gettid(), lowNice)
}
