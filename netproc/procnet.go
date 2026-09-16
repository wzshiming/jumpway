package netproc

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

func parseProcNet(reader io.Reader) map[uint64]netip.AddrPort {
	addresses := make(map[uint64]netip.AddrPort)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[3] != "01" {
			continue
		}
		host, portText, ok := strings.Cut(fields[1], ":")
		if !ok || (len(host) != 8 && len(host) != 32) || len(portText) != 4 {
			continue
		}
		addressBytes, err := hex.DecodeString(host)
		if err != nil {
			continue
		}
		port, err := strconv.ParseUint(portText, 16, 16)
		if err != nil {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		for offset := 0; offset < len(addressBytes); offset += 4 {
			word := addressBytes[offset : offset+4]
			binary.BigEndian.PutUint32(word, binary.NativeEndian.Uint32(word))
		}
		var address netip.Addr
		if len(addressBytes) == 4 {
			address = netip.AddrFrom4([4]byte(addressBytes))
		} else {
			address = netip.AddrFrom16([16]byte(addressBytes))
		}
		addresses[inode] = netip.AddrPortFrom(address.Unmap(), uint16(port))
	}
	return addresses
}
