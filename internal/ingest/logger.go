package ingest

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

type LogLevel string

const (
	LogLevelInfo     LogLevel = "info"
	LogLevelSuccess  LogLevel = "success"
	LogLevelWarn     LogLevel = "warn"
	LogLevelError    LogLevel = "error"
	LogLevelProgress LogLevel = "progress"
)

type LogEvent struct {
	Level   LogLevel `json:"level"`
	Message string   `json:"message"`
	Append  bool     `json:"append,omitempty"`
	Key     string   `json:"key,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type Logger struct {
	mu       sync.Mutex
	writer   io.Writer
	useColor bool
	onEvent  func(LogEvent)
	lineOpen bool
}

func NewLogger(writer io.Writer, useColor bool, onEvent func(LogEvent)) *Logger {
	return &Logger{
		writer:   writer,
		useColor: useColor,
		onEvent:  onEvent,
	}
}

func (l *Logger) Infof(key, format string, args ...any) {
	l.printf(LogLevelInfo, key, format, args...)
}

func (l *Logger) Successf(key, format string, args ...any) {
	l.printf(LogLevelSuccess, key, format, args...)
}

func (l *Logger) Warnf(key, format string, args ...any) {
	l.printf(LogLevelWarn, key, format, args...)
}

func (l *Logger) Errorf(key, format string, args ...any) {
	l.printf(LogLevelError, key, format, args...)
}

func (l *Logger) Progress() {
	l.emit(LogEvent{
		Level:   LogLevelProgress,
		Message: ".",
		Append:  true,
	})
}

func (l *Logger) printf(level LogLevel, key, format string, args ...any) {
	l.emit(LogEvent{
		Level:   level,
		Message: fmt.Sprintf(format, args...),
		Key:     key,
		Args:    stringifyArgs(args),
	})
}

func stringifyArgs(args []any) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = fmt.Sprint(arg)
	}
	return out
}

func (l *Logger) emit(event LogEvent) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		if event.Append {
			if _, err := io.WriteString(l.writer, event.Message); err == nil {
				l.lineOpen = true
			}
		} else {
			if l.lineOpen {
				_, _ = io.WriteString(l.writer, "\n")
				l.lineOpen = false
			}
			line := fmt.Sprintf("%s %s\n", time.Now().Format("15:04:05"), l.colorize(event.Level, event.Message))
			_, _ = io.WriteString(l.writer, line)
		}
	}

	if l.onEvent != nil {
		l.onEvent(event)
	}
}

func (l *Logger) colorize(level LogLevel, message string) string {
	if !l.useColor {
		return message
	}

	switch level {
	case LogLevelSuccess:
		return color.GreenString(message)
	case LogLevelWarn:
		return color.YellowString(message)
	case LogLevelError:
		return color.RedString(message)
	case LogLevelProgress:
		return color.CyanString(message)
	default:
		return color.CyanString(message)
	}
}

func trimLogMessage(message string) string {
	return strings.TrimRight(message, "\n")
}
