package main

import (
	"encoding/json"
	"errors"
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

func TestTranslocatedCredentialHelperStaysOutsideBundle(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "AppTranslocation", "uuid", "d", "ModelSwitcher.app", "Contents", "MacOS", "model-switcher")
	legacy := filepath.Join(t.TempDir(), "Application Support", "ModelSwitcher", "profiles.json")
	if got := credentialHelperDestination(executable, "darwin", legacy); got != filepath.Join(filepath.Dir(legacy), "model-switcher-token") {
		t.Fatalf("helper would be stored in the app bundle or mount: %s", got)
	}
}

func TestCredentialHelperUsesExplicitProfileAndFailsIfMissing(t *testing.T) {
	isolateSkillStore(t)
	makeStore := func(path, key string) {
		t.Helper()
		data, _ := json.Marshal(storeFile{Profiles: []storedProfile{{ProfileView: ProfileView{ID: "same-id", APIKey: key}}}})
		if err := atomicWrite(path, data); err != nil {
			t.Fatal(err)
		}
	}
	makeStore(dataPath(), "wrong-portable-key")
	makeStore(legacyDataPath(), "wrong-user-key")
	selected := filepath.Join(t.TempDir(), "Download's 文件夹", "profiles.json")
	makeStore(selected, "expected-key")
	key, err := tokenForProfile("same-id", selected)
	if err != nil || key != "expected-key" {
		t.Fatalf("helper read a different profile: %v", err)
	}
	if _, err = tokenForProfile("same-id", filepath.Join(t.TempDir(), "missing.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing explicit file must not fall back to another key: %v", err)
	}
}
