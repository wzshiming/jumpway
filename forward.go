package jumpway

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/wzshiming/bridge"
)

// RunForward pipes every accepted connection to target through dialer until the listener fails.
func RunForward(ctx context.Context, listener net.Listener, dialer bridge.Dialer, target string) error {
	if target == "" {
		return errors.New("forward target is empty")
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func() {
			remote, err := dialer.DialContext(WithClientAddr(ctx, conn.RemoteAddr()), "tcp", target)
			if err != nil {
				conn.Close()
				return
			}
			tunnel(ctx, conn, remote)
		}()
	}
}

func tunnel(ctx context.Context, client, remote net.Conn) {
	closeBoth := func() {
		client.Close()
		remote.Close()
	}
	defer closeBoth()
	stop := context.AfterFunc(ctx, closeBoth)
	defer stop()
	done := make(chan bool, 2)
	copyConn := func(dst, src net.Conn) {
		buffer := BytesPool.Get()
		_, err := io.CopyBuffer(dst, src, buffer)
		BytesPool.Put(buffer)
		halfClosed := false
		if closer, ok := dst.(interface{ CloseWrite() error }); ok && err == nil {
			halfClosed = closer.CloseWrite() == nil
		}
		done <- halfClosed
	}
	go copyConn(client, remote)
	go copyConn(remote, client)
	if !<-done {
		closeBoth()
	}
	<-done
}
