//go:build darwin || linux

package tundev

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Helper runs the tun-helper subcommand that Open launches as root.
func Helper(args []string) int {
	if len(args) != 1 || !filepath.IsAbs(args[0]) {
		fmt.Fprintln(os.Stderr, "usage: tun-helper <absolute unix socket path>")
		return 2
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "tun-helper: must run as root")
		return 2
	}
	// CreateTUN would write the interface name to the file this names.
	os.Unsetenv("WG_TUN_NAME_FILE")
	c, err := net.DialTimeout("unix", args[0], 5*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tun-helper:", err)
		return 1
	}
	conn := c.(*net.UnixConn)
	defer conn.Close()

	var req request
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := readFrame(conn, &req); err != nil {
		fmt.Fprintln(os.Stderr, "tun-helper: read request:", err)
		return 1
	}
	cfg := Config{Addresses: req.Addresses, MTU: req.MTU, Bypass: req.Bypass}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var dev *Device
	err = validate(cfg)
	if err == nil {
		dev, err = openLocal(ctx, cfg)
	}
	// A client that stops reading must not stall the root helper.
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tun-helper:", err)
		sendReply(conn, reply{Error: err.Error()}, nil)
		return 1
	}
	if err := sendReply(conn, reply{Name: dev.Interface, Warnings: dev.Warnings}, dev.File()); err != nil {
		dev.Close()
		fmt.Fprintln(os.Stderr, "tun-helper: send reply:", err)
		return 1
	}
	// Open closes the connection once it holds the descriptor.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	io.Copy(io.Discard, conn)
	return 0
}
