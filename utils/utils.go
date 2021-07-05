package utils

import (
	_ "unsafe"
)

//go:linkname IsClosedConnError github.com/wzshiming/bridge/internal/common.IsClosedConnError
func IsClosedConnError(err error) bool // implemented in github.com/wzshiming/bridge/internal/common
