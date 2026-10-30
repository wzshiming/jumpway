package tundev

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
)

// frameMax bounds the JSON payload of a frame.
const frameMax = 65536

// request asks the helper for a TUN; the helper validates it and computes the routes itself.
type request struct {
	Addresses []netip.Prefix `json:"addresses"`
	MTU       int            `json:"mtu"`
	Bypass    []netip.Prefix `json:"bypass,omitempty"`
}

// reply answers a request; the TUN's descriptor travels with it unless Error is set.
type reply struct {
	Name     string   `json:"name"`
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

func writeFrame(w io.Writer, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(payload) > frameMax {
		return fmt.Errorf("tundev: frame payload of %d bytes exceeds %d", len(payload), frameMax)
	}
	frame := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(payload)), uint32(len(payload)))
	_, err = w.Write(append(frame, payload...))
	return err
}

func readFrame(r io.Reader, v any) error {
	var size [4]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n > frameMax {
		return fmt.Errorf("tundev: frame payload of %d bytes exceeds %d", n, frameMax)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}
