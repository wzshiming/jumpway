//go:build windows

package netproc

import (
	"context"
	"encoding/binary"
	"net/netip"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getExtendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

type tcpRow struct {
	State      uint32
	LocalAddr  [4]byte
	LocalPort  uint32
	RemoteAddr [4]byte
	RemotePort uint32
	PID        uint32
}

type tcp6Row struct {
	LocalAddr   [16]byte
	LocalScope  uint32
	LocalPort   uint32
	RemoteAddr  [16]byte
	RemoteScope uint32
	RemotePort  uint32
	State       uint32
	PID         uint32
}

func readTCPTable(ctx context.Context, family uint32) ([]byte, error) {
	buffer := make([]byte, 4)
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		size := uint32(len(buffer))
		result, _, _ := getExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)),
			0, uintptr(family), 4, 0,
		)
		if result == 0 {
			return buffer[:min(int(size), len(buffer))], nil
		}
		if syscall.Errno(result) != windows.ERROR_INSUFFICIENT_BUFFER {
			return nil, syscall.Errno(result)
		}
		buffer = make([]byte, max(size, 4))
	}
	return nil, windows.ERROR_INSUFFICIENT_BUFFER
}

// Owners returns the visible established TCP socket owners.
func Owners(ctx context.Context) (map[netip.AddrPort]Process, error) {
	table4, err := readTCPTable(ctx, windows.AF_INET)
	if err != nil {
		return nil, err
	}
	table6, err := readTCPTable(ctx, windows.AF_INET6)
	if err != nil {
		return nil, err
	}
	owners := make(map[netip.AddrPort]Process)
	names := make(map[uint32]string)
	processForPID := func(pid uint32) Process {
		name, ok := names[pid]
		if !ok {
			handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
			if err == nil {
				buffer := make([]uint16, 32768)
				size := uint32(len(buffer))
				if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err == nil {
					name = filepath.Base(windows.UTF16ToString(buffer[:size]))
				}
				windows.CloseHandle(handle)
			}
			names[pid] = name
		}
		return Process{PID: int(pid), Name: name}
	}
	if len(table4) >= 4+24 {
		count := min(uint64(binary.LittleEndian.Uint32(table4[:4])), uint64((len(table4)-4)/24))
		for _, row := range unsafe.Slice((*tcpRow)(unsafe.Pointer(&table4[4])), int(count)) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if row.State != 5 {
				continue
			}
			port := uint16(row.LocalPort)
			address := netip.AddrPortFrom(netip.AddrFrom4(row.LocalAddr).Unmap(), port>>8|port<<8)
			owners[address] = processForPID(row.PID)
		}
	}
	if len(table6) >= 4+56 {
		count := min(uint64(binary.LittleEndian.Uint32(table6[:4])), uint64((len(table6)-4)/56))
		for _, row := range unsafe.Slice((*tcp6Row)(unsafe.Pointer(&table6[4])), int(count)) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if row.State != 5 {
				continue
			}
			port := uint16(row.LocalPort)
			address := netip.AddrPortFrom(netip.AddrFrom16(row.LocalAddr).Unmap(), port>>8|port<<8)
			owners[address] = processForPID(row.PID)
		}
	}
	return owners, nil
}
