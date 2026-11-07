//go:build windows

package config

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func lockExclusive(t *testing.T, path string) windows.Handle {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile(%s) failed: %v", path, err)
	}
	return h
}

func TestLoadRawRetriesSharingViolation(t *testing.T) {
	s := NewStore(t.TempDir())
	want := []byte("proxies: {}\n")
	if err := s.SaveRaw(want); err != nil {
		t.Fatal(err)
	}

	h := lockExclusive(t, s.Path())
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := s.LoadRaw()
		done <- result{data, err}
	}()
	time.Sleep(150 * time.Millisecond)
	if err := windows.CloseHandle(h); err != nil {
		t.Fatal(err)
	}

	r := <-done
	if r.err != nil {
		t.Fatalf("LoadRaw() failed after the exclusive handle was released: %v", r.err)
	}
	if !bytes.Equal(r.data, want) {
		t.Fatalf("LoadRaw() = %q, want %q", r.data, want)
	}
}

func TestLoadRawGivesUpAfterRetryBudget(t *testing.T) {
	attempts, backoff := retryAttempts, retryBackoff
	t.Cleanup(func() { retryAttempts, retryBackoff = attempts, backoff })
	retryAttempts, retryBackoff = 3, 10*time.Millisecond

	s := NewStore(t.TempDir())
	if err := s.SaveRaw([]byte("proxies: {}\n")); err != nil {
		t.Fatal(err)
	}
	h := lockExclusive(t, s.Path())
	defer windows.CloseHandle(h)

	_, err := s.LoadRaw()
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("LoadRaw() error = %v, want ERROR_SHARING_VIOLATION", err)
	}
}
