//go:build windows

package config

import "strings"

var fileNameEscaper = strings.NewReplacer("%", "%25", ":", "%3A", "*", "%2A", "?", "%3F", `"`, "%22", "<", "%3C", ">", "%3E", "|", "%7C")

// fileSafeKey percent-encodes characters Windows forbids in file names, and % so keys stay distinct.
func fileSafeKey(key string) string {
	return fileNameEscaper.Replace(key)
}
