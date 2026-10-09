//go:build darwin && cgo

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalExecutablePathNativeResolver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file with 空格's")
	if err := os.WriteFile(path, []byte("probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	original, ok := originalExecutablePath(path)
	if !ok {
		t.Fatal("Security.framework original-path resolver unavailable")
	}
	actual, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := os.Stat(path)
	if !os.SameFile(actual, expected) {
		t.Fatalf("resolver changed the file identity: %s", original)
	}
	if _, ok := originalExecutablePath(filepath.Join(t.TempDir(), "missing")); ok {
		t.Fatal("resolver accepted a nonexistent path")
	}
}
