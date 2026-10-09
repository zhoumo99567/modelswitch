package main

import (
	"errors"
	"os"
	"path/filepath"
)

var errAlreadyRunning = errors.New("Model Switcher 已经在运行")

// acquireSingleInstance keeps one OS-level lock for the lifetime of the GUI
// process. The lock file is intentionally retained after exit: the kernel lock
// is released with the file descriptor, so a crash cannot leave a stale lock.
func acquireSingleInstance() (func(), error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(base, "ModelSwitcher")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	return acquireSingleInstanceAt(filepath.Join(directory, "instance.lock"))
}
