package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialHelperPathUsesStableName(t *testing.T) {
	path, err := credentialHelperPath()
	if err != nil {
		t.Fatal(err)
	}
	want := "model-switcher-token"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if !strings.EqualFold(filepath.Base(path), want) {
		t.Fatalf("credential helper path %q does not use stable name %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}
