package utils

import (
	_ "unsafe"
)

//go:linkname IsClosedConnError github.com/wzshiming/bridge/internal/netutils.IsClosedConnError
func IsClosedConnError(err error) bool // implemented in github.com/wzshiming/bridge/internal/netutils
