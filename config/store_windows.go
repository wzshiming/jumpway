//go:build windows

package config

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

const (
	renameAttempts = 20
	renameBackoff  = 50 * time.Millisecond
)

// renameFile retries while a handle opened without FILE_SHARE_DELETE, as os.Open does, blocks the rename.
func renameFile(from, to string) error {
	err := os.Rename(from, to)
	for range renameAttempts - 1 {
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			break
		}
		time.Sleep(renameBackoff)
		err = os.Rename(from, to)
	}
	return err
}
