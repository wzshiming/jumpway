package jumpway

import (
	"context"
	"net"
	"syscall"

	"github.com/wzshiming/bridge"
	"golang.org/x/sys/unix"
)

// tunFWMark is the firewall mark used to exempt proxy traffic from TUN routing.
// 0x6a77 is "jw" (JumpWay) in ASCII — chosen to be unique and unlikely to
// conflict with other applications' fwmark usage.
const tunFWMark = 0x6a77

// NewMarkedDialer returns a bridge.Dialer that sets SO_MARK on outgoing
// sockets so that the kernel routes them through the real interface instead
// of back into the TUN device.
func NewMarkedDialer() bridge.Dialer {
	return &markedDialer{}
}

type markedDialer struct{}

func (d *markedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	nd := net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, tunFWMark)
			})
			return err
		},
	}
	return nd.DialContext(ctx, network, address)
}
