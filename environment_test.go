package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestNodeVersionCompatibility(t *testing.T) {
	for version, want := range map[string]bool{"v22.19.0": true, "22.19.1": true, "v24.0.0": true, "v26.1.0": true, "v22.18.9": false, "v20.19.0": false, "v22.19.0-rc.1": false, "v22.bad.0": false, "": false} {
		if got := compatibleNodeVersion(version); got != want {
			t.Errorf("%q: got %v, want %v", version, got, want)
		}
	}
}

func TestSelectNodeReleaseUsesCompatibleLTSAndPlatform(t *testing.T) {
	var releases []nodeRelease
	err := json.Unmarshal([]byte(`[
	 {"version":"v26.1.0","lts":false,"files":["win-x64-zip"]},
	 {"version":"v24.9.0","lts":"Krypton","files":["win-x64-zip","win-arm64-zip","osx-x64-tar","osx-arm64-tar"]},
	 {"version":"v22.18.0","lts":"Jod","files":["win-x64-zip"]}
	]`), &releases)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ platform, arch, file string }{
		{"windows", "amd64", "node-v24.9.0-win-x64.zip"},
		{"windows", "arm64", "node-v24.9.0-win-arm64.zip"},
		{"darwin", "amd64", "node-v24.9.0-darwin-x64.tar.gz"},
		{"darwin", "arm64", "node-v24.9.0-darwin-arm64.tar.gz"},
	} {
		version, file, err := selectNodeRelease(releases, tc.platform, tc.arch)
		if err != nil || version != "v24.9.0" || file != tc.file {
			t.Fatalf("%s/%s: %q %q %v", tc.platform, tc.arch, version, file, err)
		}
	}
	if _, _, err = selectNodeRelease(releases, "windows", "386"); err == nil {
		t.Fatal("unsupported arch accepted")
	}
	if _, _, err = selectNodeRelease(releases[2:], "windows", "amd64"); err == nil {
		t.Fatal("incompatible LTS accepted")
	}
}

func TestNodeDownloadVerifiesChecksumAndHandlesHTTPFailure(t *testing.T) {
	payload := []byte("fixture node archive")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write(payload)
	}))
	defer server.Close()
	progress := func(string, int) {}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if err := downloadNodeArchive(context.Background(), server.URL, t.TempDir()+"/node.zip", hash, progress); err != nil {
		t.Fatal(err)
	}
	if err := downloadNodeArchive(context.Background(), server.URL, t.TempDir()+"/node.zip", strings.Repeat("0", 64), progress); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	if _, err := dependencyHTTPBytes(context.Background(), server.URL+"/missing", 100); err == nil {
		t.Fatal("HTTP error accepted")
	}
	if _, err := dependencyHTTPBytes(context.Background(), server.URL, 2); err == nil {
		t.Fatal("oversized response accepted")
	}
	if got, err := releaseChecksum([]byte(hash+"  node.zip\n"), "node.zip"); err != nil || got != hash {
		t.Fatalf("checksum: %s %v", got, err)
	}
	if _, err := releaseChecksum([]byte(hash+"  other.zip\n"), "node.zip"); err == nil {
		t.Fatal("wrong archive checksum accepted")
	}
}

func TestNodeArchiveRejectsTraversalAndPreservesFiles(t *testing.T) {
	for _, name := range []string{"node/../../outside", "../outside", "/node/bin/node", "node\\..\\outside", "node/C:/outside", "other/bin/node"} {
		if _, err := nodeArchivePath(t.TempDir(), name, "node"); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, unsafe := range []bool{false, true} {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		name := "node/bin/node"
		if unsafe {
			name = "node/../../outside"
		}
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte("runtime"))
		writer.Close()
		archive := filepath.Join(t.TempDir(), "node.zip")
		os.WriteFile(archive, buffer.Bytes(), 0600)
		root := filepath.Join(t.TempDir(), "unpacked")
		err = extractNodeArchive(archive, root, "node")
		if unsafe {
			if err == nil {
				t.Fatal("extracted traversal archive")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "bin", "node"))
		if err != nil || string(data) != "runtime" {
			t.Fatalf("extraction: %s %v", data, err)
		}
	}
}

func TestNodeTarLinksStayInsideInstallation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS archive links")
	}
	for _, target := range []string{"../lib/npm-cli.js", "../../../outside", "/outside"} {
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(gz)
		writer.WriteHeader(&tar.Header{Name: "node/lib/npm-cli.js", Typeflag: tar.TypeReg, Mode: 0755, Size: 3})
		writer.Write([]byte("npm"))
		writer.WriteHeader(&tar.Header{Name: "node/bin/npm", Typeflag: tar.TypeSymlink, Linkname: target})
		writer.Close()
		gz.Close()
		archive := filepath.Join(t.TempDir(), "node.tar.gz")
		os.WriteFile(archive, buffer.Bytes(), 0600)
		err := extractNodeArchive(archive, filepath.Join(t.TempDir(), "unpacked"), "node")
		if (target == "../lib/npm-cli.js") != (err == nil) {
			t.Fatalf("link %q: %v", target, err)
		}
	}
}

func TestManagedDependenciesAreFoundAndPropagatedToChildren(t *testing.T) {
	testHome := t.TempDir()
	if runtime.GOOS != "windows" {
		// os.UserConfigDir uses ~/Library/Application Support on macOS and
		// does not honor XDG_CONFIG_HOME there. Keep the fixture out of the
		// real user's ModelSwitcher/tools directory on every desktop OS.
		t.Setenv("HOME", testHome)
		t.Setenv("XDG_CONFIG_HOME", testHome)
	} else {
		t.Setenv("APPDATA", testHome)
		t.Setenv("USERPROFILE", testHome)
	}
	root := managedToolsRoot()
	runtimeDir := filepath.Join(root, "runtimes", "v24.9.0-123")
	bin := runtimeDir
	if runtime.GOOS != "windows" {
		bin = filepath.Join(bin, "bin")
	}
	os.MkdirAll(bin, 0700)
	os.WriteFile(filepath.Join(root, "node-version"), []byte("v24.9.0-123"), 0600)
	nodeName, cliName := "node", "pi"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
		cliName = "pi.cmd"
	}
	os.WriteFile(filepath.Join(bin, nodeName), []byte("fixture"), 0700)
	os.MkdirAll(managedNpmBin(), 0700)
	os.WriteFile(filepath.Join(managedNpmBin(), cliName), []byte("fixture"), 0700)
	node, err := resolveNode()
	if err != nil || node != filepath.Join(bin, nodeName) {
		t.Fatalf("managed Node: %q %v", node, err)
	}
	cli, err := resolveCLI("pi")
	if err != nil || cli != filepath.Join(managedNpmBin(), cliName) {
		t.Fatalf("managed CLI: %q %v", cli, err)
	}
	if !strings.Contains(dependencyPathPrefix(cli), bin) {
		t.Fatal("managed Node missing from launcher PATH")
	}
	for _, bad := range []string{"../outside", "v24.9.0/../../outside"} {
		os.WriteFile(filepath.Join(root, "node-version"), []byte(bad), 0600)
		if managedNodeDirectory() != "" {
			t.Fatal("unsafe runtime pointer accepted")
		}
	}
}

func TestDependencyInstallerRejectsUnknownAndConcurrentJobs(t *testing.T) {
	app := NewApp()
	if _, err := app.StartDependencyInstall("pi; echo unsafe"); err == nil {
		t.Fatal("unknown dependency accepted")
	}
	app.dependencyJob = DependencyInstallState{ID: "node", Status: "running"}
	if _, err := app.StartDependencyInstall("pi"); err == nil {
		t.Fatal("concurrent installation accepted")
	}
}

func TestPlatformInstallerFailureFallsBackAndCancellationStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installer")
	marker := filepath.Join(t.TempDir(), "args.txt")
	source := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + quoteCLIShell(marker) + "\nexit 1\n"
	platform, method := "darwin", "brew"
	if runtime.GOOS == "windows" {
		path += ".cmd"
		platform, method = "windows", "winget"
		source = "@echo off\r\necho %* > \"" + marker + "\"\r\nexit /b 1\r\n"
	}
	if err := os.WriteFile(path, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	fallbacks := 0
	fallback := func(context.Context, func(string, int)) error { fallbacks++; return nil }
	if err := installPlatformNodeUsing(context.Background(), path, method, platform, func(string, int) {}, fallback); err != nil {
		t.Fatal(err)
	}
	if fallbacks != 1 {
		t.Fatal("failed native install did not fall back")
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("native installer did not execute", err)
	}
	expected := "node@24"
	if runtime.GOOS == "windows" {
		expected = "OpenJS.NodeJS.LTS"
	}
	if !strings.Contains(string(data), expected) {
		t.Fatalf("wrong native package: %s", data)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := installPlatformNodeUsing(ctx, path, method, platform, func(string, int) {}, fallback); err == nil {
		t.Fatal("cancelled installation succeeded")
	}
	if fallbacks != 1 {
		t.Fatal("cancelled installation started another download")
	}
}

func TestEnvironmentDetectsActualNodeAndNpm(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node not available")
	}
	state := NewApp().DetectEnvironment()
	if len(state.Tools) != 4 {
		t.Fatalf("expected four dependencies: %#v", state)
	}
	if !state.Tools[0].Installed || state.Tools[0].Version == "" {
		t.Fatalf("Node detection: %#v", state.Tools[0])
	}
}

// Opt-in verification downloads the official packages into a disposable user
// directory. It never changes the real user's tools or PATH.
func TestLiveDependencyInstallation(t *testing.T) {
	if os.Getenv("MODELSWITCHER_LIVE_DEPENDENCY_TEST") != "1" {
		t.Skip("set MODELSWITCHER_LIVE_DEPENDENCY_TEST=1 for official downloads")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("desktop installer")
	}
	root := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", root)
	} else {
		t.Setenv("HOME", root)
	}
	t.Setenv("npm_config_cache", filepath.Join(root, "npm-cache"))
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	for _, id := range []string{"node", "pi", "codex"} {
		t.Run(id, func(t *testing.T) {
			if err := installDependencyWithNodeInstaller(ctx, id, func(message string, progress int) { t.Logf("%d%% %s", progress, message) }, installManagedNode); err != nil {
				t.Fatal(err)
			}
			info := detectDependency(id)
			if !info.Ready || !info.Managed {
				t.Fatalf("installation not ready/managed: %#v", info)
			}
			t.Logf("%s %s", info.Name, info.Version)
		})
		if t.Failed() {
			break
		}
	}
}
