//go:build darwin

package tray

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

var mainThread = mainThreadDispatcher{path: "/usr/lib/libSystem.B.dylib"}

type mainThreadDispatcher struct {
	path                     string
	once                     sync.Once
	err                      error
	handle, queue            unsafe.Pointer
	dispatch, pthread        unsafe.Pointer
	dispatchCall, threadCall types.CallInterface
	callback                 uintptr
	mu                       sync.Mutex
	next                     uintptr
	pending                  map[uintptr]func()
}

func (dispatcher *mainThreadDispatcher) load() error {
	dispatcher.once.Do(func() { dispatcher.err = dispatcher.init() })
	return dispatcher.err
}

func (dispatcher *mainThreadDispatcher) init() (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("initialize main-thread dispatch: %v", recovered)
		}
	}()
	dispatcher.handle, err = ffi.LoadLibrary(dispatcher.path)
	if err != nil {
		return err
	}
	for _, symbol := range []struct {
		name   string
		target *unsafe.Pointer
	}{
		{"dispatch_sync_f", &dispatcher.dispatch},
		{"pthread_main_np", &dispatcher.pthread},
		{"_dispatch_main_q", &dispatcher.queue},
	} {
		*symbol.target, err = ffi.GetSymbol(dispatcher.handle, symbol.name)
		if err != nil {
			return err
		}
	}
	err = ffi.PrepareCallInterface(&dispatcher.dispatchCall, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor, types.PointerTypeDescriptor, types.PointerTypeDescriptor})
	if err != nil {
		return err
	}
	err = ffi.PrepareCallInterface(&dispatcher.threadCall, types.DefaultCall, types.SInt32TypeDescriptor, nil)
	if err != nil {
		return err
	}
	dispatcher.callback = ffi.NewCallback(func(token uintptr) {
		if fn := dispatcher.take(token); fn != nil {
			fn()
		}
	})
	return nil
}

func (dispatcher *mainThreadDispatcher) isMain() (int32, error) {
	var result int32
	_, err := ffi.CallFunction(&dispatcher.threadCall, dispatcher.pthread, unsafe.Pointer(&result), nil)
	return result, err
}

func (dispatcher *mainThreadDispatcher) register(fn func()) uintptr {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if dispatcher.pending == nil {
		dispatcher.pending = make(map[uintptr]func())
	}
	dispatcher.next++
	dispatcher.pending[dispatcher.next] = fn
	return dispatcher.next
}

func (dispatcher *mainThreadDispatcher) take(token uintptr) func() {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	fn := dispatcher.pending[token]
	delete(dispatcher.pending, token)
	return fn
}

// Callers must not hold a.mu, and the main thread must never wait on the actions worker: dispatch_sync_f would deadlock.
func runOnMain(fn func()) error {
	return mainThread.run(fn)
}

func (dispatcher *mainThreadDispatcher) run(fn func()) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := dispatcher.load(); err != nil {
		return err
	}
	onMain, err := dispatcher.isMain()
	if err != nil {
		return err
	}
	if onMain != 0 {
		fn()
		return nil
	}
	token := dispatcher.register(fn)
	_, err = ffi.CallFunction(&dispatcher.dispatchCall, dispatcher.dispatch, nil, []unsafe.Pointer{
		unsafe.Pointer(&dispatcher.queue), unsafe.Pointer(&token), unsafe.Pointer(&dispatcher.callback),
	})
	if err != nil {
		dispatcher.take(token)
		return err
	}
	return nil
}
