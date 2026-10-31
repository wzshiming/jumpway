//go:build darwin || linux

package tundev

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func sendReply(conn *net.UnixConn, rep reply, f *os.File) error {
	var frame bytes.Buffer
	if err := writeFrame(&frame, rep); err != nil {
		return err
	}
	if f == nil {
		_, err := conn.Write(frame.Bytes())
		return err
	}
	rc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var n int
	// Unlike f.Fd, Control keeps the descriptor non-blocking, a flag the receiver shares.
	if cerr := rc.Control(func(fd uintptr) {
		n, _, err = conn.WriteMsgUnix(frame.Bytes(), unix.UnixRights(int(fd)), nil)
	}); cerr != nil {
		return cerr
	}
	if err == nil && n < frame.Len() {
		_, err = conn.Write(frame.Bytes()[n:])
	}
	return err
}

func recvReply(conn *net.UnixConn) (reply, *os.File, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	// A failed read may still have delivered descriptors, which must not leak.
	msgs, perr := unix.ParseSocketControlMessage(oob[:oobn])
	var f *os.File
	for _, m := range msgs {
		fds, rerr := unix.ParseUnixRights(&m)
		if perr == nil {
			perr = rerr
		}
		for _, fd := range fds {
			// On darwin, net sets close-on-exec only after a successful read.
			unix.CloseOnExec(fd)
			if f == nil {
				f = os.NewFile(uintptr(fd), "tun")
			} else {
				unix.Close(fd)
			}
		}
	}
	if err == nil {
		err = perr
	}
	var rep reply
	if err == nil {
		err = readFrame(io.MultiReader(bytes.NewReader(buf[:n]), conn), &rep)
	}
	if err == nil && rep.Error != "" {
		err = errors.New(rep.Error)
	}
	if err != nil {
		if f != nil {
			f.Close()
		}
		return reply{}, nil, err
	}
	return rep, f, nil
}
