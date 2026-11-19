//go:build !windows

package config

import "os"

func renameFile(from, to string) error {
	return os.Rename(from, to)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
