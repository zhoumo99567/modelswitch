package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIWhitelistAndDirectoryValidation(t *testing.T) {
	for _, id := range []string{"pi; echo injected", "../../../bin/sh", "unknown", ""} {
		if _, err := resolveCLI(id); err == nil {
			t.Fatalf("accepted command %q", id)
		}
	}
	if _, err := cliDirectory(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("accepted nonexistent working directory")
	}
	if _, err := cliDirectory("/tmp\nwrong"); err == nil {
		t.Fatal("accepted multiline directory")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := cliDirectory(file); err == nil {
		t.Fatal("accepted a file as directory")
	}
}
func TestCLILaunchQuotingAndPathResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "project's folder $(touch injected)")
	bin := filepath.Join(root, "bin's folder")
	for _, path := range []string{dir, bin} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(bin, "pi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s' \"$PWD\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	resolved, err := resolveCLI("pi")
	if err != nil || resolved != path {
		t.Fatalf("resolve: %s %v", resolved, err)
	}
	cmd := exec.Command("/bin/sh", "-c", cliShellScript(path, dir))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != dir {
		t.Fatalf("quote/working folder: %s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(root, "injected")); !os.IsNotExist(err) {
		t.Fatal("shell injection occurred")
	}
}
