package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type profilePaths struct {
	portable string
	writable string
}

func currentProfilePaths() profilePaths {
	executable, err := os.Executable()
	legacy := legacyDataPath()
	if err != nil {
		return profilePaths{writable: legacy}
	}
	return profilePathsForExecutable(executable, runtime.GOOS, legacy, originalExecutablePath, writableDirectory)
}

func profilePathsForExecutable(executable, platform, legacy string, resolve func(string) (string, bool), canWrite func(string) bool) profilePaths {
	if platform == "darwin" && translocatedPath(executable) {
		original, ok := resolve(executable)
		if !ok || !filepath.IsAbs(original) || translocatedPath(original) {
			return profilePaths{writable: legacy}
		}
		executable = original
	}
	dir := filepath.Dir(executable)
	if platform == "darwin" {
		// Walk from the executable outward, choosing the nearest bundle even
		// when an ancestor folder happens to end in .app as well.
		for parent := dir; parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if strings.EqualFold(filepath.Ext(parent), ".app") {
				dir = filepath.Dir(parent)
				break
			}
		}
	}
	portable := filepath.Join(dir, "profiles.json")
	if canWrite(dir) {
		return profilePaths{portable: portable, writable: portable}
	}
	// Keep read-only portable installs independent. A global user profile must
	// not shadow profiles bundled beside a different downloaded app.
	digest := sha256.Sum256([]byte(portable))
	fallback := filepath.Join(filepath.Dir(legacy), "portable", fmt.Sprintf("%x", digest[:8]), "profiles.json")
	return profilePaths{portable: portable, writable: fallback}
}

func (paths profilePaths) candidates(cwd, legacy string) []string {
	ordered := []string{paths.writable, paths.portable}
	if cwd != "" {
		ordered = append(ordered, filepath.Join(cwd, "profiles.json"))
	}
	ordered = append(ordered, legacy)
	result := []string{}
	seen := map[string]bool{}
	for _, path := range ordered {
		if path != "" && !seen[filepath.Clean(path)] {
			result = append(result, path)
			seen[filepath.Clean(path)] = true
		}
	}
	return result
}

func translocatedPath(path string) bool {
	return strings.Contains(strings.ToLower(filepath.ToSlash(path)), "/apptranslocation/")
}

func credentialHelperDestination(executable, platform, legacy string) string {
	base := strings.ToLower(filepath.Base(executable))
	dir := filepath.Dir(executable)
	stableCopy := strings.Contains(base, "-dev") || strings.Contains(base, ".test")
	if platform == "darwin" && translocatedPath(executable) {
		// Never mutate the signed bundle or retain its temporary mount path in
		// a credential command. The helper receives the profile file explicitly.
		stableCopy = true
		dir = filepath.Dir(legacy)
	}
	if !stableCopy {
		return executable
	}
	name := "model-switcher-token"
	if platform == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func writableDirectory(dir string) bool {
	file, err := os.CreateTemp(dir, ".model-switcher-write-check-*")
	if err != nil {
		return false
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}
