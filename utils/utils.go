package utils

import (
	"errors"
	"net"
	"strings"
)

// IsClosedConnError reports whether err was caused by operating on a closed connection.
func IsClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) ||
		strings.Contains(strings.ToLower(err.Error()), net.ErrClosed.Error())
}
