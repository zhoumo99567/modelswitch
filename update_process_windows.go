//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

func updateProcessRunning(pid int) (bool, error) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	return status == uint32(windows.WAIT_TIMEOUT), err
}

func cleanupUpdateHelper(path string) {
	// Windows cannot unlink the running helper. A hidden cleanup process waits
	// for this exact PID and removes only the known temporary helper file.
	if !strings.HasPrefix(filepath.Base(path), "model-switcher-updater-") || !strings.EqualFold(filepath.Clean(filepath.Dir(path)), filepath.Clean(os.TempDir())) {
		return
	}
	cmd := hiddenCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `Wait-Process -Id ([int]$env:MODELSWITCHER_HELPER_PID) -ErrorAction SilentlyContinue; Remove-Item -LiteralPath $env:MODELSWITCHER_HELPER_PATH -Force -ErrorAction SilentlyContinue`)
	cmd.Env = append(os.Environ(), "MODELSWITCHER_HELPER_PID="+strconv.Itoa(os.Getpid()), "MODELSWITCHER_HELPER_PATH="+path)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
