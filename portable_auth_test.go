package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPiCredentialCommandQuotesProfilePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	dir := filepath.Join(t.TempDir(), "folder's 空格")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "credential-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "$(echo unexpected)`echo unexpected` profiles.json")
	command, err := shellTokenCommand(helper, "profile-id", profile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := exec.Command("/bin/sh", "-c", strings.TrimPrefix(command, "!")).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	want := "--model-switcher-token\nprofile-id\n--profiles\n" + profile + "\n"
	if string(result) != want {
		t.Fatalf("profile argument was changed by shell parsing: %q", result)
	}
	if !modelSwitcherTokenCommand(command, "profile-id", profile) {
		t.Fatal("generated credential command rejected by Pi chat")
	}
	if modelSwitcherTokenCommand(command, "profile-id", profile+"-other") || modelSwitcherTokenCommand(command+"; echo extra", "profile-id", profile) {
		t.Fatal("unrelated profile or extra command accepted")
	}
}
