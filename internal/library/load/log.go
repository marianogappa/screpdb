package load

import "fmt"

type LogLevel string

const (
	LogLevelInfo    LogLevel = "info"
	LogLevelSuccess LogLevel = "success"
	LogLevelWarn    LogLevel = "warn"
	LogLevelError   LogLevel = "error"
)

// LogEvent mirrors the shape the dashboard already streams to the browser, so
// the library can reuse the existing log panel without a translation layer.
type LogEvent struct {
	Level   LogLevel `json:"level"`
	Message string   `json:"message"`
	Append  bool     `json:"append,omitempty"`
}

func (l *Loader) logf(level LogLevel, format string, args ...any) {
	if l.opts.Log == nil {
		return
	}
	l.opts.Log(LogEvent{Level: level, Message: fmt.Sprintf(format, args...)})
}
