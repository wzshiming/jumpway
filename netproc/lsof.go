package netproc

import (
	"bufio"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

func parseLsof(reader io.Reader) map[netip.AddrPort]Process {
	owners := make(map[netip.AddrPort]Process)
	var process Process
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ := strconv.Atoi(line[1:])
			process = Process{PID: pid}
		case 'c':
			process.Name = line[1:]
		case 'n':
			local, _, connected := strings.Cut(line[1:], "->")
			if !connected || process.PID <= 0 {
				continue
			}
			address, err := netip.ParseAddrPort(local)
			if err == nil {
				owners[netip.AddrPortFrom(address.Addr().Unmap(), address.Port())] = process
			}
		}
	}
	return owners
}
