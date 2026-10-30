//go:build !windows

package config

import "os"

func renameFile(from, to string) error {
	return os.Rename(from, to)
}
