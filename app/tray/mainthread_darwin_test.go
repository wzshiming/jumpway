//go:build darwin

package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// mainRunLoopErr is set when TestMain cannot pump the main run loop; the hop tests fail with it.
var mainRunLoopErr error

func init() {
	runtime.LockOSThread()
}

func TestMain(tests *testing.M) {
	if err := mainThread.load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result, err := mainThread.isMain()
	if err != nil || result != 1 {
		fmt.Fprintf(os.Stderr, "pthread_main_np on TestMain = %d, err = %v\n", result, err)
		os.Exit(1)
	}
	pump, err := newMainRunLoopPump()
	if err != nil {
		mainRunLoopErr = err
		os.Exit(tests.Run())
	}
	code := make(chan int, 1)
	go func() { code <- tests.Run() }()
	for {
		select {
		case exit := <-code:
			os.Exit(exit)
		default:
			pump()
		}
	}
}

// CFRunLoopRunInMode in the default mode drains the main dispatch queue the same way [NSApp run] does.
func newMainRunLoopPump() (func(), error) {
	library, err := ffi.LoadLibrary("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	if err != nil {
		return nil, err
	}
	run, err := ffi.GetSymbol(library, "CFRunLoopRunInMode")
	if err != nil {
		return nil, err
	}
	defaultMode, err := ffi.GetSymbol(library, "kCFRunLoopDefaultMode")
	if err != nil {
		return nil, err
	}
	var call types.CallInterface
	err = ffi.PrepareCallInterface(&call, types.DefaultCall, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor, types.DoubleTypeDescriptor, types.UInt8TypeDescriptor})
	if err != nil {
		return nil, err
	}
	mode := *(*unsafe.Pointer)(defaultMode)
	seconds := 0.05
	var returnAfterSourceHandled uint8
	return func() {
		var result int32
		_, _ = ffi.CallFunction(&call, run, unsafe.Pointer(&result), []unsafe.Pointer{
			unsafe.Pointer(&mode), unsafe.Pointer(&seconds), unsafe.Pointer(&returnAfterSourceHandled),
		})
	}, nil
}

type hopResult struct {
	caller, onMain         int32
	callerErr, hopErr, err error
	calls                  int
}

func hopFromWorker(test *testing.T, lockThread bool) {
	if mainRunLoopErr != nil {
		test.Fatal(mainRunLoopErr)
	}
	done := make(chan hopResult, 1)
	go func() {
		var result hopResult
		if lockThread {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			result.caller, result.callerErr = mainThread.isMain()
		}
		result.err = runOnMain(func() {
			result.calls++
			result.onMain, result.hopErr = mainThread.isMain()
		})
		done <- result
	}()
	select {
	case result := <-done:
		if result.err != nil {
			test.Fatalf("runOnMain returned %v", result.err)
		}
		if lockThread && (result.callerErr != nil || result.caller != 0) {
			test.Fatalf("caller pthread_main_np = %d, err = %v; expected a non-main thread", result.caller, result.callerErr)
		}
		if result.calls != 1 || result.hopErr != nil || result.onMain != 1 {
			test.Fatalf("calls = %d, pthread_main_np in fn = %d, err = %v", result.calls, result.onMain, result.hopErr)
		}
	case <-time.After(5 * time.Second):
		test.Fatal("runOnMain did not return within 5s")
	}
}

func TestRunOnMainHop(test *testing.T) {
	test.Run("locked worker", func(test *testing.T) { hopFromWorker(test, true) })
	test.Run("unlocked worker", func(test *testing.T) { hopFromWorker(test, false) })
}

func TestRunOnMainNested(test *testing.T) {
	if mainRunLoopErr != nil {
		test.Fatal(mainRunLoopErr)
	}
	type nestedResult struct {
		outer, inner       int32
		outerErr, innerErr error
		outerHop, innerHop error
		calls              int
	}
	done := make(chan nestedResult, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		var result nestedResult
		result.outerHop = runOnMain(func() {
			result.outer, result.outerErr = mainThread.isMain()
			result.innerHop = runOnMain(func() {
				result.inner, result.innerErr = mainThread.isMain()
				result.calls++
			})
		})
		done <- result
	}()
	select {
	case result := <-done:
		if result.outerHop != nil || result.innerHop != nil {
			test.Fatalf("runOnMain errors: outer = %v, inner = %v", result.outerHop, result.innerHop)
		}
		if result.outerErr != nil || result.innerErr != nil {
			test.Fatalf("pthread_main_np errors: outer = %v, inner = %v", result.outerErr, result.innerErr)
		}
		if result.outer != 1 || result.inner != 1 || result.calls != 1 {
			test.Fatalf("outer main = %d, inner main = %d, inner calls = %d", result.outer, result.inner, result.calls)
		}
	case <-time.After(5 * time.Second):
		test.Fatal("nested runOnMain did not return within 5s")
	}
}

func TestRunOnMainInitError(test *testing.T) {
	dispatcher := mainThreadDispatcher{path: filepath.Join(test.TempDir(), "missing.dylib")}
	calls := 0
	first := dispatcher.run(func() { calls++ })
	if first == nil {
		test.Fatal("run with a missing library returned nil")
	}
	if second := dispatcher.run(func() { calls++ }); second != first {
		test.Fatalf("second run error = %v, want the first error %v", second, first)
	}
	if loaded := dispatcher.load(); loaded != first {
		test.Fatalf("load error = %v, want %v", loaded, first)
	}
	if calls != 0 {
		test.Fatalf("fn was called %d times", calls)
	}
}

func TestMainThreadTokens(test *testing.T) {
	var dispatcher mainThreadDispatcher
	const count = 32
	tokens := make(chan uintptr, count)
	var workers sync.WaitGroup
	var called int
	for worker := 0; worker < count; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			tokens <- dispatcher.register(func() { called++ })
		}()
	}
	workers.Wait()
	close(tokens)
	seen := make(map[uintptr]bool)
	for token := range tokens {
		if token == 0 || seen[token] {
			test.Fatalf("invalid or duplicate token %d", token)
		}
		seen[token] = true
		fn := dispatcher.take(token)
		if fn == nil || dispatcher.take(token) != nil {
			test.Fatalf("token %d was not consumed exactly once", token)
		}
		fn()
	}
	if called != count || len(dispatcher.pending) != 0 || dispatcher.take(0) != nil {
		test.Fatalf("callbacks = %d, pending = %d", called, len(dispatcher.pending))
	}
}
