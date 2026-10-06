package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestUpdateSourceAndVersionOrdering(t *testing.T) {
	t.Setenv("MODELSWITCHER_UPDATE_URL", "")
	if !strings.HasSuffix(updateManifestURL(), "/releases/latest/download/latest.json") {
		t.Fatal("default source must use the GitHub Release manifest")
	}
	t.Setenv("MODELSWITCHER_UPDATE_URL", " https://example.com/latest.json ")
	if got := updateManifestURL(); got != "https://example.com/latest.json" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v0.2.0", "0.1.0", 1}, {"0.2.0", "0.2.0-rc.1", 1},
		{"0.2.0-alpha.2", "0.2.0-alpha.10", -1}, {"0.2.0-alpha.1", "0.2.0-alpha", 1},
		{"0.2.0-1", "0.2.0-alpha", -1}, {"0.2.0+build.1", "0.2.0+build.2", 0},
		{"0.10.0", "0.9.0", 1}, {"1.0.0", "2.0.0", -1},
	} {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("%s vs %s: %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	for _, bad := range []string{"", "0.2", "01.2.0", "0.2.0-01", "0.2.0-", "bad"} {
		if validUpdateVersion(bad) {
			t.Errorf("accepted invalid version %q", bad)
		}
	}
}

func TestCheckForUpdateReadsRedirectedReleaseManifest(t *testing.T) {
	manifest := UpdateManifest{Version: "99.0.0", Notes: "release notes", Windows: UpdateArtifact{URL: "https://example.com/app.exe", SHA256: strings.Repeat("a", 64)}, MacOS: UpdateArtifact{URL: "https://example.com/app.zip", SHA256: strings.Repeat("b", 64)}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest/download/latest.json" {
			http.Redirect(w, r, "/download/v99.0.0/latest.json", http.StatusFound)
			return
		}
		_ = json.NewEncoder(w).Encode(manifest)
	}))
	defer server.Close()
	t.Setenv("MODELSWITCHER_UPDATE_URL", server.URL+"/latest/download/latest.json")
	info, err := NewApp().CheckForUpdate()
	if err != nil || !info.UpdateAvailable || info.LatestVersion != manifest.Version || info.Notes != manifest.Notes {
		t.Fatalf("%+v: %v", info, err)
	}
	expected := manifest.Windows.URL
	if runtime.GOOS == "darwin" {
		expected = manifest.MacOS.URL
	}
	if info.DownloadURL != expected {
		t.Fatal(info.DownloadURL)
	}
}

func TestDownloadUpdateVerifiesBytesAndCleansFailedDownloads(t *testing.T) {
	downloadDir := t.TempDir()
	t.Setenv("TMP", downloadDir)
	t.Setenv("TMPDIR", downloadDir)
	data := []byte("new application bytes")
	digest := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	info := UpdateInfo{DownloadURL: server.URL, SHA256: hex.EncodeToString(digest[:])}
	path, err := downloadUpdate(info)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("%q: %v", got, err)
	}
	for _, sum := range []string{"invalid", strings.Repeat("0", 64)} {
		info.SHA256 = sum
		if path, err := downloadUpdate(info); err == nil || path != "" {
			t.Fatalf("bad hash accepted: %s %v", path, err)
		}
	}
	if entries, err := os.ReadDir(downloadDir); err != nil || len(entries) != 1 {
		t.Fatalf("failed downloads left temporary files: %v %v", entries, err)
	}
}

func TestUpdateStagingAndReplacement(t *testing.T) {
	root := t.TempDir()
	sourceDir, targetDir := filepath.Join(root, "download"), filepath.Join(root, "application")
	for _, dir := range []string{sourceDir, targetDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	source, target := filepath.Join(sourceDir, "downloaded"), filepath.Join(targetDir, "app.exe")
	if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(targetDir, "profiles.json")
	if err := os.WriteFile(profile, []byte("user data"), 0600); err != nil {
		t.Fatal(err)
	}
	staged, err := stageUpdateExecutable(source, targetDir, ".update-*.exe")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(staged) != targetDir {
		t.Fatal("staging must be on the destination volume")
	}
	if info, err := os.Stat(staged); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		t.Fatalf("helper is not executable: %v", err)
	}
	restarted := false
	err = replaceUpdateTarget(target, staged, func(path string) error {
		restarted = true
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "new" {
			t.Fatalf("restart saw %q: %v", data, err)
		}
		return nil
	})
	if err != nil || !restarted {
		t.Fatalf("restart: %v %v", restarted, err)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "new" {
		t.Fatal("staging consumed the downloaded file")
	}
	if data, err := os.ReadFile(profile); err != nil || string(data) != "user data" {
		t.Fatal("update touched user profiles")
	}
	if backups, _ := filepath.Glob(filepath.Join(targetDir, "*.backup-*")); len(backups) != 0 {
		t.Fatal(backups)
	}
}

func TestUpdateRollbackOnLaunchOrReplacementFailure(t *testing.T) {
	for _, missingStage := range []bool{false, true} {
		t.Run(map[bool]string{false: "launch failure", true: "replacement failure"}[missingStage], func(t *testing.T) {
			root := t.TempDir()
			target, staged := filepath.Join(root, "app.exe"), filepath.Join(root, "staged.exe")
			if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
				t.Fatal(err)
			}
			if !missingStage {
				if err := os.WriteFile(staged, []byte("new"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := replaceUpdateTarget(target, staged, func(string) error { return errors.New("cannot launch") })
			if err == nil || !strings.Contains(err.Error(), "已恢复原版本") {
				t.Fatalf("%v", err)
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "old" {
				t.Fatalf("old version lost: %q %v", data, err)
			}
		})
	}
}

func TestUpdateHelperWaitsForParentExit(t *testing.T) {
	if err := waitForUpdateParent(os.Getpid(), 0); err == nil {
		t.Fatal("helper must wait while the parent is alive")
	}
	cmd := hiddenCommand(os.Args[0], "-test.run=^TestUpdateWaitingChild$")
	cmd.Env = append(os.Environ(), "MODELSWITCHER_TEST_WAIT_CHILD=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := waitForUpdateParent(pid, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRollbackRestartsTheOriginalApplication(t *testing.T) {
	root := t.TempDir()
	target, staged := filepath.Join(root, "app.exe"), filepath.Join(root, "staged.exe")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	var attempts []string
	err := replaceUpdateTarget(target, staged, func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		attempts = append(attempts, string(data))
		if string(data) == "new" {
			return errors.New("new version cannot launch")
		}
		return nil
	})
	if err == nil || strings.Join(attempts, ",") != "new,old" {
		t.Fatalf("rollback did not restart the original: %v, %v", attempts, err)
	}
}

func TestUpdateWaitingChild(t *testing.T) {
	if os.Getenv("MODELSWITCHER_TEST_WAIT_CHILD") == "1" {
		return
	}
}

func updateTestZIP(t *testing.T, entries map[string]struct {
	body string
	mode os.FileMode
}) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, entry := range entries {
		header := &zip.FileHeader{Name: name}
		header.SetMode(entry.mode)
		out, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMacOSUpdateArchiveRejectsTraversalAndUnsafeLinks(t *testing.T) {
	entry := func(body string, mode os.FileMode) struct {
		body string
		mode os.FileMode
	} {
		return struct {
			body string
			mode os.FileMode
		}{body, mode}
	}
	valid := updateTestZIP(t, map[string]struct {
		body string
		mode os.FileMode
	}{
		"ModelSwitcher.app/Contents/MacOS/app":      entry("binary", 0755),
		"ModelSwitcher.app/Contents/Resources/link": entry("../MacOS/app", os.ModeSymlink|0777),
	})
	if name, err := validateMacOSUpdateArchive(valid); err != nil || name != "ModelSwitcher.app" {
		t.Fatalf("%s: %v", name, err)
	}
	for _, name := range []string{"../outside", "/absolute", "ModelSwitcher.app/../outside", "ModelSwitcher.app/Contents/../../outside", `ModelSwitcher.app\outside`, "other.app/file", "ModelSwitcher.app/Contents/Resources/link/child"} {
		entries := map[string]struct {
			body string
			mode os.FileMode
		}{
			"ModelSwitcher.app/Contents/MacOS/app":      entry("binary", 0755),
			"ModelSwitcher.app/Contents/Resources/link": entry("../MacOS/app", os.ModeSymlink|0777),
			name: entry("bad", 0644),
		}
		if _, err := validateMacOSUpdateArchive(updateTestZIP(t, entries)); err == nil {
			t.Errorf("accepted unsafe entry %q", name)
		}
	}
	for _, link := range []string{"../../../outside", "/absolute", "", `C:\outside`} {
		path := updateTestZIP(t, map[string]struct {
			body string
			mode os.FileMode
		}{"ModelSwitcher.app/link": entry(link, os.ModeSymlink|0777)})
		if _, err := validateMacOSUpdateArchive(path); err == nil {
			t.Errorf("accepted unsafe link %q", link)
		}
	}
	for _, entries := range []map[string]struct {
		body string
		mode os.FileMode
	}{
		{"ModelSwitcher.app/a": entry("b", os.ModeSymlink|0777), "ModelSwitcher.app/b": entry("a", os.ModeSymlink|0777)},
		{"ModelSwitcher.app/a": entry("subdir/b/../../outside", os.ModeSymlink|0777), "ModelSwitcher.app/subdir/b": entry("../dir", os.ModeSymlink|0777)},
	} {
		if _, err := validateMacOSUpdateArchive(updateTestZIP(t, entries)); err == nil {
			t.Fatal("accepted cyclic or escaping link chain")
		}
	}
}

func TestMacOSBundleExtractionPreservesExecutabilityAndLinks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("uses macOS ditto and PlistBuddy; also run by the macOS Actions job")
	}
	root := t.TempDir()
	bundle := filepath.Join(root, "ModelSwitcher.app")
	binDir := filepath.Join(bundle, "Contents", "MacOS")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>app</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "app"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("app", filepath.Join(binDir, "link")); err != nil {
		t.Fatal(err)
	}
	archive, stage := filepath.Join(root, "update.zip"), filepath.Join(root, "stage")
	if output, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", bundle, archive).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", output, err)
	}
	if err := os.Mkdir(stage, 0755); err != nil {
		t.Fatal(err)
	}
	unpacked, err := prepareMacOSUpdateBundle(archive, stage)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(unpacked, "Contents", "MacOS", "app")); err != nil || info.Mode().Perm()&0111 == 0 {
		t.Fatalf("lost executable mode: %v", err)
	}
	if link, err := os.Readlink(filepath.Join(unpacked, "Contents", "MacOS", "link")); err != nil || link != "app" {
		t.Fatalf("lost link: %q %v", link, err)
	}
}
