//go:build windows

package tray

import (
	_ "embed"

	"github.com/getlantern/systray"
)

type Config struct {
	Title   string
	Tooltip string
	Icon    []byte
	OnQuit  func()
}

//go:embed assets/tray.ico
var defaultIcon []byte

func DefaultIcon() []byte {
	return defaultIcon
}

func Supported() bool {
	return true
}

func Run(config Config) error {
	systray.Run(func() {
		if config.Title != "" {
			systray.SetTitle(config.Title)
		}
		if config.Tooltip != "" {
			systray.SetTooltip(config.Tooltip)
		}
		icon := config.Icon
		if len(icon) == 0 {
			icon = defaultIcon
		}
		if len(icon) > 0 {
			systray.SetIcon(icon)
		}

		quitItem := systray.AddMenuItem("Quit", "Quit screpdb dashboard")
		go func() {
			<-quitItem.ClickedCh
			if config.OnQuit != nil {
				config.OnQuit()
			}
			systray.Quit()
		}()
	}, func() {})

	return nil
}

func Quit() {
	systray.Quit()
}
