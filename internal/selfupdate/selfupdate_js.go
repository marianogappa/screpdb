//go:build js

package selfupdate

import (
	"context"
	"errors"
)

var errNotAvailable = errors.New("self-update is not available in the browser preview")

func CheckStatus(context.Context) (Status, error) { return Status{Tier: TierNone}, errNotAvailable }
func Apply(context.Context) (string, error)        { return "", errNotAvailable }
func CleanupOldBinary()                            {}
func IsRestart() bool                              { return false }
func RestartEnvKV() string                         { return "" }
func Restart() error                               { return errNotAvailable }
