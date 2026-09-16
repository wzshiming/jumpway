//go:build linux

package netproc

import (
	"bytes"
	"context"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Owners returns the visible established TCP socket owners.
func Owners(ctx context.Context) (map[netip.AddrPort]Process, error) {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, err
	}
	addresses := parseProcNet(bytes.NewReader(data))
	if data, err := os.ReadFile("/proc/net/tcp6"); err == nil {
		maps.Copy(addresses, parseProcNet(bytes.NewReader(data)))
	}
	owners := make(map[netip.AddrPort]Process)
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		root := filepath.Join("/proc", entry.Name())
		fds, err := os.ReadDir(filepath.Join(root, "fd"))
		if err != nil {
			continue
		}
		name, _ := os.ReadFile(filepath.Join(root, "comm"))
		process := Process{PID: pid, Name: strings.TrimSpace(string(name))}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(root, "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode, err := strconv.ParseUint(target[len("socket:["):len(target)-1], 10, 64)
			if err != nil {
				continue
			}
			if address, ok := addresses[inode]; ok {
				owners[address] = process
			}
		}
	}
	return owners, nil
}
