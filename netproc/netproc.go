// Package netproc identifies the processes that own local TCP sockets.
package netproc

// Process identifies the local process that owns a TCP socket.
type Process struct {
	PID int `json:"pid"`
	// Name may be empty when the process cannot be inspected.
	Name string `json:"name"`
}
