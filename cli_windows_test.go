//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsCLIHasInteractiveConsole(t *testing.T) {
	directory := t.TempDir()
	marker := filepath.Join(directory, "console.txt")
	wrapper := filepath.Join(directory, "pi.cmd")
	script := fmt.Sprintf("@echo off\r\npowershell.exe -NoLogo -NoProfile -Command \"[IO.File]::WriteAllText('%s', ([Console]::IsInputRedirected.ToString() + ',' + [Console]::IsOutputRedirected.ToString() + ',' + [Console]::IsErrorRedirected.ToString()))\"\r\n", strings.ReplaceAll(marker, "'", "''"))
	if err := os.WriteFile(wrapper, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	process, err := startCLIHost(wrapper, directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("taskkill.exe", "/PID", fmt.Sprint(process.Pid), "/T", "/F").Run()
		_ = process.Release()
	})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil && len(data) > 0 {
			if string(data) != "False,False,False" {
				t.Fatalf("CLI does not have an interactive console (stdin, stdout, stderr redirected): %s", data)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("CLI wrapper never started")
}
