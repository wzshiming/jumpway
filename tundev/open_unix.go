//go:build darwin || linux

package tundev

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

var createTUN = func(name string, mtu int) (tun.Device, error) { return tun.CreateTUN(name, mtu) }

// Open prompts for administrator privileges unless the process is root.
func Open(ctx context.Context, cfg Config) (*Device, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}
	if os.Geteuid() == 0 {
		return openLocal(ctx, cfg)
	}

	dir, err := os.MkdirTemp("", "jumpway-tun-")
	if err == nil && len(dir+"/s") > 100 {
		os.Remove(dir)
		dir, err = os.MkdirTemp("/tmp", "jwt-")
	}
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	sock := dir + "/s"
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: sock, Net: "unix"})
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	if err := ln.SetDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return nil, err
	}
	for _, p := range []string{exe, sock} {
		if strings.ContainsFunc(p, func(r rune) bool { return r < 0x20 }) {
			return nil, fmt.Errorf("tundev: control character in path %q", p)
		}
	}

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(cctx, "/usr/bin/osascript", "-e", appleScript(exe, sock))
	} else {
		cmd = exec.CommandContext(cctx, "/usr/bin/pkexec", exe, "tun-helper", sock)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		// No helper connects once its launcher is gone, so the accept gives up.
		ln.Close()
		close(exited)
	}()

	conn, err := ln.AcceptUnix()
	ln.Close()
	if err != nil {
		cancel()
		<-exited
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case errors.Is(err, net.ErrClosed):
			return nil, classifyExit(runtime.GOOS, cmd.ProcessState.ExitCode(), stderr.String())
		}
		return nil, fmt.Errorf("tundev: waiting for the tun helper: %w", err)
	}

	uid, err := peerUID(conn)
	if err == nil && uid != 0 {
		err = fmt.Errorf("peer runs as uid %d, not root", uid)
	}
	var rep reply
	var f *os.File
	if err == nil {
		conn.SetDeadline(time.Now().Add(60 * time.Second))
		if err = writeFrame(conn, request{Addresses: cfg.Addresses, MTU: cfg.MTU, Bypass: cfg.Bypass}); err == nil {
			rep, f, err = recvReply(conn)
		}
	}
	// The helper exits on reading EOF.
	conn.Close()
	os.RemoveAll(dir)
	if err != nil {
		cancel()
	}
	<-exited
	if err == nil && f == nil {
		err = errors.New("reply carries no TUN descriptor")
	}
	if err != nil {
		return nil, fmt.Errorf("tundev: tun helper: %w", err)
	}
	dev, err := deviceFromFile(f)
	if err != nil {
		return nil, fmt.Errorf("tundev: %w", err)
	}
	return &Device{Device: dev, Interface: rep.Name, Warnings: rep.Warnings}, nil
}

// openLocal creates and configures the TUN in this process, which must be root.
func openLocal(ctx context.Context, cfg Config) (*Device, error) {
	dev, err := createTUN(createTUNName, cfg.MTU)
	if err != nil {
		return nil, err
	}
	name, err := dev.Name()
	var warnings []string
	if err == nil {
		warnings, err = configure(ctx, runtime.GOOS, defaultTools(runtime.GOOS), name, cfg, runCommand)
	}
	if err != nil {
		dev.Close()
		return nil, err
	}
	return &Device{Device: dev, Interface: name, Warnings: warnings}, nil
}
