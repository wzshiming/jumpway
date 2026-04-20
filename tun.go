package jumpway

import (
	"context"
	"io"
	"net"
	"net/netip"
	"time"

	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/wzshiming/bridge"
)

// TunConfig holds configuration for the TUN device.
type TunConfig struct {
	Name         string
	MTU          uint32
	Inet4Address []netip.Prefix
	AutoRoute    bool
	Stack        string
}

// TunProxy manages a TUN device and network stack for global proxy.
type TunProxy struct {
	tunDevice tun.Tun
	stack     tun.Stack
}

// NewTunProxy creates and configures a new TUN-based global proxy.
func NewTunProxy(ctx context.Context, dialer bridge.Dialer, cfg TunConfig, log logger.Logger) (*TunProxy, error) {
	tunName := cfg.Name
	if tunName == "" {
		tunName = tun.CalculateInterfaceName("")
	}

	mtu := cfg.MTU
	if mtu == 0 {
		mtu = 9000
	}

	inet4Address := cfg.Inet4Address
	if len(inet4Address) == 0 {
		inet4Address = []netip.Prefix{netip.MustParsePrefix("198.18.0.1/16")}
	}

	tunOptions := tun.Options{
		Name:         tunName,
		MTU:          mtu,
		Inet4Address: inet4Address,
		AutoRoute:    cfg.AutoRoute,
		Logger:       log,
	}

	tunDevice, err := tun.New(tunOptions)
	if err != nil {
		return nil, err
	}

	handler := &tunHandler{
		dialer: dialer,
		logger: log,
	}

	stack, err := tun.NewStack(cfg.Stack, tun.StackOptions{
		Context:    ctx,
		Tun:        tunDevice,
		TunOptions: tunOptions,
		UDPTimeout: 5 * time.Minute,
		Handler:    handler,
		Logger:     log,
	})
	if err != nil {
		tunDevice.Close()
		return nil, err
	}

	return &TunProxy{
		tunDevice: tunDevice,
		stack:     stack,
	}, nil
}

// Start activates the TUN device and network stack.
func (t *TunProxy) Start() error {
	err := t.tunDevice.Start()
	if err != nil {
		return err
	}
	return t.stack.Start()
}

// Close shuts down the network stack and TUN device.
func (t *TunProxy) Close() error {
	err := t.stack.Close()
	if err2 := t.tunDevice.Close(); err == nil {
		err = err2
	}
	return err
}

// tunHandler implements tun.Handler to forward traffic through the proxy dialer.
type tunHandler struct {
	dialer bridge.Dialer
	logger logger.Logger
}

// PrepareConnection decides whether to route packets directly or through the stack.
// Returning nil lets all traffic go through the stack for proxying.
func (h *tunHandler) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr, routeContext tun.DirectRouteContext, timeout time.Duration) (tun.DirectRouteDestination, error) {
	return nil, nil
}

// NewConnectionEx handles new TCP connections from the TUN device.
func (h *tunHandler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	destAddr := destination.String()
	h.logger.Info("tun tcp connection: ", destAddr)

	remote, err := h.dialer.DialContext(ctx, "tcp", destAddr)
	if err != nil {
		h.logger.Error("tun tcp dial: ", err)
		if onClose != nil {
			onClose(err)
		}
		conn.Close()
		return
	}

	go func() {
		var err error
		defer func() {
			conn.Close()
			remote.Close()
			if onClose != nil {
				onClose(err)
			}
		}()
		err = relay(conn, remote)
	}()
}

// NewPacketConnectionEx handles new UDP connections from the TUN device.
func (h *tunHandler) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	destAddr := destination.String()
	h.logger.Info("tun udp connection: ", destAddr)

	remote, err := h.dialer.DialContext(ctx, "udp", destAddr)
	if err != nil {
		h.logger.Error("tun udp dial: ", err)
		if onClose != nil {
			onClose(err)
		}
		conn.Close()
		return
	}

	go func() {
		var err error
		defer func() {
			conn.Close()
			remote.Close()
			if onClose != nil {
				onClose(err)
			}
		}()

		done := make(chan struct{})

		// TUN -> remote
		go func() {
			defer func() {
				select {
				case <-done:
				default:
					close(done)
				}
			}()
			buffer := buf.NewPacket()
			defer buffer.Release()
			for {
				buffer.Reset()
				_, readErr := conn.ReadPacket(buffer)
				if readErr != nil {
					return
				}
				_, writeErr := remote.Write(buffer.Bytes())
				if writeErr != nil {
					return
				}
			}
		}()

		// remote -> TUN
		go func() {
			defer func() {
				select {
				case <-done:
				default:
					close(done)
				}
			}()
			rawBuf := make([]byte, 65535)
			for {
				n, readErr := remote.Read(rawBuf)
				if readErr != nil {
					return
				}
				buffer := buf.As(rawBuf[:n])
				writeErr := conn.WritePacket(buffer, destination)
				if writeErr != nil {
					return
				}
			}
		}()

		<-done
	}()
}

// relay copies data bidirectionally between two connections.
func relay(left, right net.Conn) error {
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(right, left)
		done <- err
	}()
	_, err := io.Copy(left, right)
	if err == nil {
		err = <-done
	}
	return err
}
