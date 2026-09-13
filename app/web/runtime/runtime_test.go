package runtime

import (
	"sync"
	"testing"
)

type testRuntime struct{}

func (*testRuntime) Reload() error { return nil }

func (*testRuntime) Status() Status { return Status{Running: true} }

func TestGetBeforeSet(t *testing.T) {
	current := Get()
	if current == nil {
		t.Fatal("Get returned nil")
	}
	if err := current.Reload(); err == nil || err.Error() != "proxy runtime not available" {
		t.Fatalf("Reload returned %v, want proxy runtime not available", err)
	}
	if status := current.Status(); status != (Status{}) {
		t.Fatalf("Status returned %+v, want zero Status", status)
	}
}

func TestSet(t *testing.T) {
	previous := Get()
	t.Cleanup(func() { Set(previous) })

	current := &testRuntime{}
	Set(current)
	if Get() != current {
		t.Fatal("Get did not return the runtime passed to Set")
	}
	Set(nil)
	if Get() == nil {
		t.Fatal("Get returned nil after Set(nil)")
	}
}

func TestConcurrentGetSet(t *testing.T) {
	previous := Get()
	t.Cleanup(func() { Set(previous) })

	var workers sync.WaitGroup
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for attempt := 0; attempt < 100; attempt++ {
				Set(&testRuntime{})
				current := Get()
				if current == nil {
					t.Error("Get returned nil")
					return
				}
				_ = current.Reload()
				_ = current.Status()
			}
		}()
	}
	workers.Wait()
}
