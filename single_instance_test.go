package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSingleInstanceLockRejectsSecondOwnerAndRecoversAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	release, err := acquireSingleInstanceAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := acquireSingleInstanceAt(path); !errors.Is(err, errAlreadyRunning) {
		t.Fatalf("second owner error = %v, want %v", err, errAlreadyRunning)
	}
	release()
	third, err := acquireSingleInstanceAt(path)
	if err != nil {
		t.Fatal(err)
	}
	third()
}
