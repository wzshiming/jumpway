//go:build !windows

package config

func fileSafeKey(key string) string {
	return key
}
