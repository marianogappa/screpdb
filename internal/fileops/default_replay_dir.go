package fileops

import (
	"errors"
	"fmt"
	"os/user"
	"runtime"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

var errDefaultReplayDirNotFound = errors.New("default replay directory not found")

func ResolveDefaultReplayDir() (string, error) {
	for _, strategy := range findReplayDirStrategies {
		dir, ok, err := strategy()
		if !ok || err != nil {
			continue
		}
		// Permit the candidate before validating: ValidateReplayDir stats/walks
		// it through the facade, which would otherwise reject this not-yet-known
		// OS-standard replay location once the facade is enforcing.
		_ = iofacade.AllowDir(dir)
		if err := ValidateReplayDir(dir); err != nil {
			continue
		}
		return dir, nil
	}
	return "", errDefaultReplayDirNotFound
}

// GetDefaultReplayDir returns the default replay directory
func GetDefaultReplayDir() string {
	dir, err := ResolveDefaultReplayDir()
	if err != nil {
		return ""
	}
	return dir
}

type findReplayDirStrategy func() (string, bool, error)

var (
	findReplayDirStrategies = []findReplayDirStrategy{
		strategyMacUser(),
		strategyWindowsUser(),
		strategyWindowsUserOld(),
		strategyOneDriveUser(),
	}
)

func strategyMacUser() func() (string, bool, error) {
	return func() (string, bool, error) {
		if runtime.GOOS == "windows" {
			return "", false, nil
		}
		user, err := user.Current()
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("%s/Library/Application Support/Blizzard/StarCraft/Maps/Replays", user.HomeDir), true, nil
	}
}

func strategyWindowsUser() func() (string, bool, error) {
	return func() (string, bool, error) {
		if runtime.GOOS != "windows" {
			return "", false, nil
		}
		user, err := user.Current()
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf(`%s\Documents\Starcraft\Maps\Replays`, user.HomeDir), true, nil
	}
}

func strategyOneDriveUser() func() (string, bool, error) {
	return func() (string, bool, error) {
		if runtime.GOOS != "windows" {
			return "", false, nil
		}
		user, err := user.Current()
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf(`%s\OneDrive\Documents\Starcraft\Maps\Replays`, user.HomeDir), true, nil
	}
}

func strategyWindowsUserOld() func() (string, bool, error) {
	return func() (string, bool, error) {
		if runtime.GOOS != "windows" {
			return "", false, nil
		}
		return `C:\Program Files (x86)\StarCraft\Maps\Replays`, true, nil
	}
}
