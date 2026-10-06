//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func updateProcessRunning(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	return err == nil, err
}

func cleanupUpdateHelper(path string) {
	if strings.HasPrefix(filepath.Base(path), "model-switcher-updater-") && filepath.Clean(filepath.Dir(path)) == filepath.Clean(os.TempDir()) {
		_ = os.Remove(path)
	}
}
