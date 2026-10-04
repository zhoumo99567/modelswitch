package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceStateWithoutWorkspacesReturnsJSONArray(t *testing.T) {
	// Existing profiles.json files predate the workspaces field. The sidebar
	// reads projects.length, so null would break the entire initial render.
	state := workspaceState(storeFile{})
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if string(result["projects"]) != "[]" {
		t.Fatalf("projects must be [], got %s", result["projects"])
	}
}

func TestValidateWorkspacePathAndInstructionDiscovery(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	child := filepath.Join(project, "internal")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("project rules"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := validateWorkspacePath(project)
	if err != nil || path != project {
		t.Fatalf("validateWorkspacePath() = %q, %v", path, err)
	}
	if _, err := validateWorkspacePath(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing workspace should fail validation")
	}
	dirs := parentDirectories(child)
	if len(dirs) < 2 || dirs[len(dirs)-1] != child || dirs[len(dirs)-2] != project {
		t.Fatalf("parentDirectories() = %#v", dirs)
	}
	loaded := candidate(project, append([]string{"AGENTS.override.md"}, agentDocumentNames...))
	if loaded != filepath.Join(project, "AGENTS.md") {
		t.Fatalf("candidate() = %q", loaded)
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.override.md"), []byte("override"), 0600); err != nil {
		t.Fatal(err)
	}
	if loaded = candidate(project, append([]string{"AGENTS.override.md"}, agentDocumentNames...)); loaded != filepath.Join(project, "AGENTS.override.md") {
		t.Fatalf("override candidate() = %q", loaded)
	}
}

func TestSkillRootForSharedTarget(t *testing.T) {
	root, err := skillRootForTarget("shared")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(root) != "skills" || filepath.Base(filepath.Dir(root)) != ".agents" {
		t.Fatalf("shared skill root = %q", root)
	}
	if _, err := skillRootForTarget("unknown"); err == nil {
		t.Fatal("unknown skill target should fail")
	}
}
