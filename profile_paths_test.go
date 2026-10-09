package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProfilePathsTranslocationReadsAndWritesOriginalSibling(t *testing.T) {
	root := t.TempDir()
	downloads := filepath.Join(root, "Downloads", "Model's Switcher files")
	executable := filepath.Join(downloads, "Model's Switcher.app", "Contents", "MacOS", "ModelSwitcher")
	translocated := filepath.Join(string(filepath.Separator), "private", "var", "folders", "AppTranslocation", "test-uuid", "d", "Model's Switcher.app", "Contents", "MacOS", "ModelSwitcher")
	legacy := filepath.Join(root, "Library", "Application Support", "ModelSwitcher", "profiles.json")
	paths := profilePathsForExecutable(translocated, "darwin", legacy, func(path string) (string, bool) {
		if path != translocated {
			t.Fatalf("resolved unexpected executable %q", path)
		}
		return executable, true
	}, func(dir string) bool {
		if dir != downloads {
			t.Fatalf("checked bundle or translocated directory instead of original sibling directory: %q", dir)
		}
		return true
	})
	want := filepath.Join(downloads, "profiles.json")
	if paths.portable != want || paths.writable != want {
		t.Fatalf("profile paths = %+v, want original sibling %q for both reads and writes", paths, want)
	}
	if err := atomicWrite(legacy, []byte(`{"profiles":[{"id":"old-user-profile"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(want, []byte(`{"profiles":[{"id":"portable-profile"}]}`)); err != nil {
		t.Fatal(err)
	}
	path, data := readFirstProfileCandidateForTest(t, paths.candidates("", legacy))
	if path != want || !strings.Contains(string(data), "portable-profile") {
		t.Fatalf("legacy configuration shadowed original sibling: path=%q data=%s", path, data)
	}
	updated := []byte(`{"profiles":[{"id":"updated-portable-profile"}]}`)
	if err := atomicWrite(paths.writable, updated); err != nil {
		t.Fatal(err)
	}
	path, data = readFirstProfileCandidateForTest(t, paths.candidates("", legacy))
	if path != want || string(data) != string(updated) {
		t.Fatalf("saved sibling configuration was not immediately readable: path=%q data=%s", path, data)
	}
}

func TestProfilePathsUnresolvedTranslocationUsesUserDirectory(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "user-config", "ModelSwitcher", "profiles.json")
	executable := filepath.Join(root, "AppTranslocation", "uuid", "d", "ModelSwitcher.app", "Contents", "MacOS", "ModelSwitcher")
	cases := []struct {
		name     string
		original string
		resolved bool
	}{
		{name: "resolver unavailable"},
		{name: "relative original rejected", original: "Downloads/ModelSwitcher", resolved: true},
		{name: "translocated original rejected", original: executable, resolved: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			paths := profilePathsForExecutable(executable, "darwin", legacy, func(string) (string, bool) {
				return test.original, test.resolved
			}, func(string) bool {
				t.Fatal("unresolved translocation must not probe its read-only directory")
				return false
			})
			if paths.portable != "" || paths.writable != legacy {
				t.Fatalf("unresolved translocation paths = %+v, want only user configuration %q", paths, legacy)
			}
			for _, candidate := range paths.candidates("", legacy) {
				if translocatedPath(candidate) {
					t.Fatalf("read-only translocation candidate survived: %q", candidate)
				}
			}
		})
	}
}

func TestProfilePathsReadOnlyCopiesHaveIndependentWritableStores(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "user-config", "ModelSwitcher", "profiles.json")
	makePaths := func(folder string) profilePaths {
		original := filepath.Join(root, folder, "ModelSwitcher.app", "Contents", "MacOS", "ModelSwitcher")
		return profilePathsForExecutable(original, "darwin", legacy, func(string) (string, bool) {
			t.Fatal("ordinary executable must not require translocation resolution")
			return "", false
		}, func(string) bool { return false })
	}
	first, second := makePaths("read-only-copy-1"), makePaths("read-only-copy-2")
	if first.writable == second.writable || first.writable == legacy || second.writable == legacy {
		t.Fatalf("different portable copies share a writable configuration: first=%+v second=%+v legacy=%q", first, second, legacy)
	}
	for _, paths := range []profilePaths{first, second} {
		if relative, err := filepath.Rel(filepath.Join(filepath.Dir(legacy), "portable"), paths.writable); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("fallback is outside portable user storage: %q", paths.writable)
		}
	}
	if again := makePaths("read-only-copy-1"); !reflect.DeepEqual(again, first) {
		t.Fatalf("fallback changes between launches: first=%+v again=%+v", first, again)
	}
	if err := atomicWrite(first.portable, []byte(`{"profiles":[{"id":"first-original"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(second.portable, []byte(`{"profiles":[{"id":"second-original"}]}`)); err != nil {
		t.Fatal(err)
	}
	path, _ := readFirstProfileCandidateForTest(t, first.candidates("", legacy))
	if path != first.portable {
		t.Fatalf("before migration read %q, want original %q", path, first.portable)
	}
	if err := atomicWrite(first.writable, []byte(`{"profiles":[{"id":"first-saved"}]}`)); err != nil {
		t.Fatal(err)
	}
	path, data := readFirstProfileCandidateForTest(t, first.candidates("", legacy))
	if path != first.writable || !strings.Contains(string(data), "first-saved") {
		t.Fatalf("saved fallback was not preferred on reload: path=%q data=%s", path, data)
	}
	path, data = readFirstProfileCandidateForTest(t, second.candidates("", legacy))
	if path != second.portable || !strings.Contains(string(data), "second-original") {
		t.Fatalf("saving the first copy changed the second copy: path=%q data=%s", path, data)
	}
}

func TestProfilePathsUseNearestMacBundleAndNativeExecutableDirectory(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "user-config", "profiles.json")
	cases := []struct {
		name       string
		platform   string
		executable string
		directory  string
	}{
		{name: "mac bundle", platform: "darwin", executable: filepath.Join(root, "ModelSwitcher.app", "Contents", "MacOS", "ModelSwitcher"), directory: root},
		{name: "nested app folder", platform: "darwin", executable: filepath.Join(root, "archive.app", "downloads", "ModelSwitcher.app", "Contents", "MacOS", "ModelSwitcher"), directory: filepath.Join(root, "archive.app", "downloads")},
		{name: "uppercase bundle extension", platform: "darwin", executable: filepath.Join(root, "ModelSwitcher.APP", "Contents", "MacOS", "ModelSwitcher"), directory: root},
		{name: "mac bare executable", platform: "darwin", executable: filepath.Join(root, "bin", "ModelSwitcher"), directory: filepath.Join(root, "bin")},
		{name: "windows executable", platform: "windows", executable: filepath.Join(root, "portable.app", "ModelSwitcher.exe"), directory: filepath.Join(root, "portable.app")},
		{name: "linux executable", platform: "linux", executable: filepath.Join(root, "portable.app", "ModelSwitcher"), directory: filepath.Join(root, "portable.app")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			paths := profilePathsForExecutable(test.executable, test.platform, legacy, func(string) (string, bool) {
				t.Fatal("non-translocated executable should not invoke the original path resolver")
				return "", false
			}, func(dir string) bool { return dir == test.directory })
			want := filepath.Join(test.directory, "profiles.json")
			if paths.portable != want || paths.writable != want {
				t.Fatalf("paths = %+v, want %q", paths, want)
			}
		})
	}
}

func TestProfilePathCandidatesPreserveOrderAndRemoveDuplicates(t *testing.T) {
	root := t.TempDir()
	portable := filepath.Join(root, "download", "profiles.json")
	fallback := filepath.Join(root, "user-config", "profiles.json")
	paths := profilePaths{portable: portable, writable: fallback}
	want := []string{fallback, portable}
	if got := paths.candidates(filepath.Dir(portable), fallback); !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
	alias := filepath.Dir(portable) + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(filepath.Dir(portable))
	if got := paths.candidates(alias, fallback); !reflect.DeepEqual(got, want) {
		t.Fatalf("clean-path duplicate survived: got %q, want %q", got, want)
	}
	if got := (profilePaths{}).candidates("", ""); len(got) != 0 {
		t.Fatalf("empty configuration emitted candidates: %q", got)
	}
}

func readFirstProfileCandidateForTest(t *testing.T, candidates []string) (string, []byte) {
	t.Helper()
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		return path, data
	}
	t.Fatalf("no readable profile in candidates %q", candidates)
	return "", nil
}
