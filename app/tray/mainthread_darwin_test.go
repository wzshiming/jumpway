//go:build darwin

package tray

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
)

var mainThreadInlineCalls int
var mainThreadInlineResult int32

func init() {
	runtime.LockOSThread()
}

func TestMain(tests *testing.M) {
	runtime.LockOSThread()
	mainThread.once.Do(mainThread.init)
	if mainThread.err != nil {
		fmt.Fprintln(os.Stderr, mainThread.err)
		os.Exit(1)
	}
	result, err := mainThread.isMain()
	if err != nil || result != 1 {
		fmt.Fprintf(os.Stderr, "pthread_main_np on TestMain = %d, err = %v\n", result, err)
		os.Exit(1)
	}
	runOnMain(func() {
		mainThreadInlineCalls++
		mainThreadInlineResult, err = mainThread.isMain()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(tests.Run())
}

func TestRunOnMainInline(test *testing.T) {
	if mainThreadInlineCalls != 1 || mainThreadInlineResult != 1 {
		test.Fatalf("inline calls = %d, main thread = %d", mainThreadInlineCalls, mainThreadInlineResult)
	}
	if mainThread.queue == nil || mainThread.dispatch == nil || mainThread.pthread == nil || mainThread.callback == 0 {
		test.Fatal("libdispatch or pthread symbol was not resolved")
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
