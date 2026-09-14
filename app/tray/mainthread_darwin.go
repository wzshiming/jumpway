//go:build darwin

package tray

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"github.com/wzshiming/jumpway/log"
)

var mainThread mainThreadDispatcher

type mainThreadDispatcher struct {
	once                     sync.Once
	err                      error
	library, queue           unsafe.Pointer
	dispatch, pthread        unsafe.Pointer
	dispatchCall, threadCall types.CallInterface
	callback                 uintptr
	mu                       sync.Mutex
	next                     uintptr
	pending                  map[uintptr]func()
}

func (dispatcher *mainThreadDispatcher) init() {
	defer func() {
		if recovered := recover(); recovered != nil {
			dispatcher.err = fmt.Errorf("initialize main-thread dispatch: %v", recovered)
		}
	}()
	dispatcher.library, dispatcher.err = ffi.LoadLibrary("/usr/lib/libSystem.B.dylib")
	if dispatcher.err != nil {
		return
	}
	for _, symbol := range []struct {
		name   string
		target *unsafe.Pointer
	}{
		{"dispatch_sync_f", &dispatcher.dispatch},
		{"pthread_main_np", &dispatcher.pthread},
		{"_dispatch_main_q", &dispatcher.queue},
	} {
		*symbol.target, dispatcher.err = ffi.GetSymbol(dispatcher.library, symbol.name)
		if dispatcher.err != nil {
			return
		}
	}
	dispatcher.err = ffi.PrepareCallInterface(&dispatcher.dispatchCall, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor, types.PointerTypeDescriptor, types.PointerTypeDescriptor})
	if dispatcher.err != nil {
		return
	}
	dispatcher.err = ffi.PrepareCallInterface(&dispatcher.threadCall, types.DefaultCall, types.SInt32TypeDescriptor, nil)
	if dispatcher.err != nil {
		return
	}
	dispatcher.callback = ffi.NewCallback(func(token uintptr) uintptr {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if result, err := dispatcher.isMain(); err != nil {
			log.Error(err, "runOnMain")
		} else if result == 0 {
			log.Error(errors.New("menu rebuild ran off the main thread"), "runOnMain")
		}
		if fn := dispatcher.take(token); fn != nil {
			fn()
		}
		return 0
	})
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

// Callers must not hold a.mu or block the main goroutine waiting on the worker.
func runOnMain(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	mainThread.once.Do(mainThread.init)
	if mainThread.err != nil {
		log.Error(mainThread.err, "runOnMain")
		fn()
		return
	}
	result, err := mainThread.isMain()
	if err != nil || result != 0 {
		if err != nil {
			log.Error(err, "runOnMain")
		}
		fn()
		return
	}
	token := mainThread.register(fn)
	_, err = ffi.CallFunction(&mainThread.dispatchCall, mainThread.dispatch, nil, []unsafe.Pointer{
		unsafe.Pointer(&mainThread.queue), unsafe.Pointer(&token), unsafe.Pointer(&mainThread.callback),
	})
	if err != nil {
		log.Error(err, "runOnMain")
		if pending := mainThread.take(token); pending != nil {
			pending()
		}
	}
}
