package fileops

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
)

// GetDefaultReplayDir returns the default replay directory
func GetDefaultReplayDir() string {
	for _, strategy := range findReplayDirStrategies {
		dir, ok, err := strategy()
		if !ok || err != nil {
			continue
		}
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		return dir
	}
	return ""
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
		return fmt.Sprintf("%s/Library/Application Support/Blizzard/StarCraft/Maps/Replays/AutoSave", user.HomeDir), true, nil
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
