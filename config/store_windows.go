//go:build windows

package config

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

var (
	retryAttempts = 20
	retryBackoff  = 50 * time.Millisecond
)

// renameFile retries while a handle opened without FILE_SHARE_DELETE, as os.Open does, blocks the rename.
func renameFile(from, to string) error {
	err := os.Rename(from, to)
	for range retryAttempts - 1 {
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			break
		}
		time.Sleep(retryBackoff)
		err = os.Rename(from, to)
	}
	return err
}

// readFile retries while a concurrent renameFile is replacing the file.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	for range retryAttempts - 1 {
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			break
		}
		time.Sleep(retryBackoff)
		data, err = os.ReadFile(path)
	}
	return data, err
}
