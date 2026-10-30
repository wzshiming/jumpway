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
	if err != nil {
		return reply{}, nil, err
	}
	var f *os.File
	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	for _, m := range msgs {
		fds, _ := unix.ParseUnixRights(&m)
		for _, fd := range fds {
			if f == nil {
				f = os.NewFile(uintptr(fd), "tun")
			} else {
				unix.Close(fd)
			}
		}
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
