//go:build linux

package bypass

import (
	"bufio"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// DefaultInterface returns the interface holding the family's default route, or ErrNoDefaultRoute when there is none.
func DefaultInterface(ipv6 bool) (*net.Interface, error) {
	path, parse := "/proc/net/route", parseRoute4
	if ipv6 {
		path, parse = "/proc/net/ipv6_route", parseRoute6
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name, err := parse(f)
	if err != nil {
		return nil, err
	}
	return net.InterfaceByName(name)
}

func parseRoute4(r io.Reader) (string, error) {
	name, best := "", uint64(0)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) < 8 || f[1] != "00000000" || f[7] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(f[3], 16, 32)
		if err != nil || flags&unix.RTF_UP == 0 {
			continue
		}
		metric, err := strconv.ParseUint(f[6], 10, 32)
		if err != nil {
			continue
		}
		if name == "" || metric < best {
			name, best = f[0], metric
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrNoDefaultRoute
	}
	return name, nil
}

func parseRoute6(r io.Reader) (string, error) {
	name, best := "", uint64(0)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		// The kernel keeps an unreachable default route on lo.
		if len(f) < 10 || f[1] != "00" || f[9] == "lo" {
			continue
		}
		metric, err := strconv.ParseUint(f[5], 16, 32)
		if err != nil {
			continue
		}
		flags, err := strconv.ParseUint(f[8], 16, 32)
		if err != nil || flags&unix.RTF_UP == 0 {
			continue
		}
		if name == "" || metric < best {
			name, best = f[9], metric
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrNoDefaultRoute
	}
	return name, nil
}

// SO_BINDTODEVICE needs no privilege since Linux 5.7.
var bindDefault bindFunc = func(c syscall.RawConn, iface boundIface, _ bool) error {
	var err error
	if cerr := c.Control(func(fd uintptr) {
		err = unix.BindToDevice(int(fd), iface.Name)
	}); cerr != nil {
		return cerr
	}
	return os.NewSyscallError("setsockopt", err)
}
