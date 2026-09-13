package runtime

import (
	"errors"
	"sync"
)

// Status is what the web UI shows about the running proxy.
type Status struct {
	Address string `json:"address"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

type Runtime interface {
	Reload() error
	Status() Status
}

var (
	mu      sync.RWMutex
	current Runtime
)

func Set(r Runtime) {
	mu.Lock()
	current = r
	mu.Unlock()
}

func Get() Runtime {
	mu.RLock()
	r := current
	mu.RUnlock()
	if r == nil {
		return unavailableRuntime{}
	}
	return r
}

type unavailableRuntime struct{}

func (unavailableRuntime) Reload() error {
	return errors.New("proxy runtime not available")
}

func (unavailableRuntime) Status() Status {
	return Status{}
}
