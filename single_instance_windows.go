//go:build windows

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func acquireSingleInstanceAt(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	overlapped := &windows.Overlapped{}
	const lockFileExclusive = 0x00000002
	const lockFileFailImmediately = 0x00000001
	if err := windows.LockFileEx(windows.Handle(file.Fd()), lockFileExclusive|lockFileFailImmediately, 0, 1, 0, overlapped); err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, errAlreadyRunning
		}
		return nil, err
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
		_ = file.Close()
	}, nil
}
