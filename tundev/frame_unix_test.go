//go:build darwin || linux

package tundev

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"
)

func TestFrameRoundTrip(t *testing.T) {
	cfg := ttDefaultConfig()
	want := request{Addresses: cfg.Addresses, MTU: cfg.MTU, Bypass: cfg.Bypass}
	var buf bytes.Buffer
	if err := writeFrame(&buf, want); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}
	frame := buf.Bytes()
	if len(frame) < 4 || int(binary.BigEndian.Uint32(frame)) != len(frame)-4 || !json.Valid(frame[4:]) {
		t.Fatalf("frame %q is not a 4-byte big-endian length followed by that many bytes of JSON", frame)
	}
	var got request
	// One byte per Read, as a stream socket may deliver it.
	if err := readFrame(iotest.OneByteReader(bytes.NewReader(frame)), &got); err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if !slices.Equal(got.Addresses, want.Addresses) || got.MTU != want.MTU || !slices.Equal(got.Bypass, want.Bypass) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

// ttFrame crafts a frame whose header announces length bytes, whatever the payload's actual size.
func ttFrame(length int, payload string) []byte {
	return append(binary.BigEndian.AppendUint32(nil, uint32(length)), payload...)
}

// ttJSONString returns a JSON string literal exactly n bytes long.
func ttJSONString(n int) string {
	return `"` + strings.Repeat("a", n-2) + `"`
}

func TestFramePayloadLimitIs65536Bytes(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, strings.Repeat("a", 65537-2)); err == nil || buf.Len() != 0 {
		t.Errorf("writeFrame of a 65537-byte payload = %v after writing %d bytes, want an error and nothing written", err, buf.Len())
	}
	for _, tc := range []struct {
		size    int
		wantErr bool
	}{
		{65536, false},
		{65537, true},
	} {
		var got string
		err := readFrame(bytes.NewReader(ttFrame(tc.size, ttJSONString(tc.size))), &got)
		if tc.wantErr && err == nil {
			t.Errorf("readFrame accepted a %d-byte payload", tc.size)
		}
		if !tc.wantErr && (err != nil || len(got) != tc.size-2) {
			t.Errorf("readFrame of a %d-byte payload = %v with %d bytes decoded, want it accepted", tc.size, err, len(got))
		}
	}
}

func TestReadFrameRejectsShortInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"empty", nil},
		{"partial header", []byte{0, 0}},
		{"payload shorter than its header", ttFrame(10, `"abc"`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			if err := readFrame(bytes.NewReader(tc.input), &got); err == nil {
				t.Errorf("readFrame(%q) = nil with %q decoded, want an error", tc.input, got)
			}
		})
	}
}

// ttSocketPair returns both ends of a connected AF_UNIX stream socket pair whose I/O fails after 30 s.
func ttSocketPair(t *testing.T) (tray, helper *net.UnixConn) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	var conns [2]*net.UnixConn
	for i, fd := range fds {
		f := os.NewFile(uintptr(fd), fmt.Sprintf("tt-socketpair-%d", i))
		c, err := net.FileConn(f)
		f.Close()
		if err != nil {
			t.Fatalf("FileConn: %v", err)
		}
		t.Cleanup(func() { c.Close() })
		uc, ok := c.(*net.UnixConn)
		if !ok {
			t.Fatalf("FileConn returned %T, want *net.UnixConn", c)
		}
		if err := uc.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			t.Fatalf("SetDeadline: %v", err)
		}
		conns[i] = uc
	}
	return conns[0], conns[1]
}

// ttSend runs sendReply on its own goroutine so a reply larger than the socket buffer cannot deadlock the test.
func ttSend(helper *net.UnixConn, rep reply, f *os.File) <-chan error {
	sent := make(chan error, 1)
	go func() { sent <- sendReply(helper, rep, f) }()
	return sent
}

func ttWaitSent(t *testing.T, sent <-chan error) {
	t.Helper()
	select {
	case err := <-sent:
		if err != nil {
			t.Errorf("sendReply: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Error("sendReply did not return within 30s")
	}
}

func TestReplyPassesFileDescriptor(t *testing.T) {
	tray, helper := ttSocketPair(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	want := reply{Name: "utun9", Warnings: []string{"ipv6: tt warning"}}
	sent := ttSend(helper, want, r)
	got, f, err := recvReply(tray)
	if err != nil {
		t.Fatalf("recvReply: %v", err)
	}
	ttWaitSent(t, sent)
	if got.Name != want.Name || !slices.Equal(got.Warnings, want.Warnings) {
		t.Errorf("recvReply = %+v, want %+v", got, want)
	}
	if f == nil {
		t.Fatal("recvReply returned no file for a reply that carried one")
	}
	defer f.Close()

	const msg = "bytes through the passed descriptor"
	if _, err := io.WriteString(w, msg); err != nil {
		t.Fatalf("write to the pipe: %v", err)
	}
	read := make(chan string, 1)
	go func() {
		buf := make([]byte, len(msg))
		n, _ := io.ReadFull(f, buf)
		read <- string(buf[:n])
	}()
	select {
	case data := <-read:
		if data != msg {
			t.Errorf("read %q from the received file, want %q", data, msg)
		}
	case <-time.After(30 * time.Second):
		t.Error("nothing arrived through the received file within 30s")
	}
}

func TestRecvReplyReturnsHelperError(t *testing.T) {
	tray, helper := ttSocketPair(t)
	const msg = "tt-helper-error: route add failed"
	sent := ttSend(helper, reply{Error: msg}, nil)
	_, f, err := recvReply(tray)
	ttWaitSent(t, sent)
	if f != nil {
		f.Close()
		t.Error("recvReply returned a file for an error reply that carried none")
	}
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("recvReply error = %v, want one containing %q", err, msg)
	}
}
