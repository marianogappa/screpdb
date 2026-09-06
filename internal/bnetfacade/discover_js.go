//go:build js

package bnetfacade

import "errors"

func loopbackListeningPorts() ([]int, error) {
	return nil, errors.New("Battle.net discovery is not available in the browser")
}
