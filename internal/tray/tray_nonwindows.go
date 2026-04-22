//go:build !windows

package tray

import "errors"

type Config struct {
	Title   string
	Tooltip string
	Icon    []byte
	OnQuit  func()
}

func DefaultIcon() []byte {
	return nil
}

func Supported() bool {
	return false
}

func Run(config Config) error {
	return errors.New("system tray is only supported on windows")
}

func Quit() {}
