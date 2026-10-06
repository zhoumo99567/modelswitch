package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateAgentHomes points HOME, CODEX_HOME and PI_CODING_AGENT_DIR at
// throwaway folders so vault tests never touch the user's real memory.
func isolateAgentHomes(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi", "agent"))
	return home
}

func writeMemoryFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryProjectIDStablePerPath(t *testing.T) {
	if got := memoryProjectID(""); got != "" {
		t.Fatalf("memoryProjectID(\"\") = %q", got)
	}
	if got := memoryProjectID("   "); got != "" {
		t.Fatalf("memoryProjectID(blank) = %q", got)
	}
	first := memoryProjectID("/repo/a/modelswitch")
	if first == "" || !strings.HasPrefix(first, "modelswitch-") {
		t.Fatalf("memoryProjectID() = %q, want modelswitch-<hash>", first)
	}
	if again := memoryProjectID("/repo/a/modelswitch/"); again != first {
		t.Fatalf("same path must reuse one id: %q vs %q", first, again)
	}
	if other := memoryProjectID("/repo/b/modelswitch"); other == first {
		t.Fatalf("two checkouts must not share an id: %q", other)
	}
}

func TestMemoryVaultScopeAndWritePermission(t *testing.T) {
	home := isolateAgentHomes(t)

	global := filepath.Join(memoryVaultGlobalDir(), "preferences.md")
	if scope, projectID := memoryVaultClassify(global); scope != memoryScopeGlobal || projectID != "" {
		t.Fatalf("memoryVaultClassify(global) = %q, %q", scope, projectID)
	}
	if !memoryVaultAllowed(global) {
		t.Fatal("a note in the global folder must be editable")
	}

	index := filepath.Join(memoryVaultGlobalDir(), memoryIndexName)
	if scope, _ := memoryVaultClassify(index); scope != memoryScopeGlobal {
		t.Fatalf("INDEX.md lives in the global folder, got %q", scope)
	}
	if memoryVaultAllowed(index) {
		t.Fatal("INDEX.md is generated and must never be edited as a memory")
	}
	if memoryReadAllowed(index) {
		t.Fatal("INDEX.md is not a memory the app should open")
	}

	if memoryVaultAllowed(filepath.Join(memoryVaultGlobalDir(), "notes.txt")) {
		t.Fatal("only Markdown notes belong in the vault")
	}
	if memoryVaultAllowed(filepath.Join(memoryVaultGlobalDir(), "deep", "note.md")) {
		t.Fatal("nested folders must stay out of the vault")
	}
	if memoryVaultAllowed(filepath.Join(home, "note.md")) {
		t.Fatal("files outside the vault must be rejected")
	}

	projectID := memoryProjectID("/repo/alpha")
	projectNote := filepath.Join(memoryVaultProjectsDir(), projectID, "context.md")
	if scope, got := memoryVaultClassify(projectNote); scope != memoryScopeProject || got != projectID {
		t.Fatalf("memoryVaultClassify(project note) = %q, %q", scope, got)
	}
	if !memoryVaultAllowed(projectNote) {
		t.Fatal("a note in a project folder must be editable")
	}
	if memoryVaultAllowed(filepath.Join(memoryVaultProjectsDir(), projectID, "deep", "note.md")) {
		t.Fatal("nested project folders must stay out of the vault")
	}

	codexNote := filepath.Join(codexMemoryRoot(), "rollout-summary.md")
	if memoryVaultAllowed(codexNote) {
		t.Fatal("Codex memories must never be writable here")
	}
	if !memoryReadAllowed(codexNote) {
		t.Fatal("Codex memories must be readable")
	}
}

func TestParseMemoryFrontMatter(t *testing.T) {
	title, tags, project := parseMemoryFrontMatter("---\ntitle: 界面偏好\nproject: Alpha 项目\ntags: [ui, codex]\n---\n正文\n")
	if title != "界面偏好" {
		t.Fatalf("title = %q", title)
	}
	if project != "Alpha 项目" {
		t.Fatalf("project = %q", project)
	}
	if strings.Join(tags, ",") != "ui,codex" {
		t.Fatalf("tags = %#v", tags)
	}

	title, tags, project = parseMemoryFrontMatter("# 普通 Markdown\n\n没有 front matter。\n")
	if title != "" || project != "" || tags != nil {
		t.Fatalf("plain markdown parsed as front matter: %q, %#v, %q", title, tags, project)
	}
}

func TestListVaultMemoriesGroupsEveryProject(t *testing.T) {
	isolateAgentHomes(t)
	current := "/repo/alpha"
	currentID := memoryProjectID(current)
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "a.md"), "---\ntitle: 全局偏好\ntags: [ui]\n---\n内容\n")
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "b.md"), "无标题\n")
	writeMemoryFile(t, filepath.Join(memoryProjectDir(current), "context.md"), "---\nproject: Alpha 项目\n---\n内容\n")
	writeMemoryFile(t, filepath.Join(memoryVaultProjectsDir(), "beta-22222222", "context.md"), "内容\n")
	writeMemoryFile(t, filepath.Join(memoryVaultProjectsDir(), "beta-22222222", "deep", "ignored.md"), "内容\n")

	entries, err := listVaultMemories(current)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("listVaultMemories() returned %d entries: %#v", len(entries), entryPaths(entries))
	}
	if entries[0].Scope != memoryScopeGlobal || entries[1].Scope != memoryScopeGlobal {
		t.Fatalf("global notes must come first: %#v", entries)
	}
	if entries[0].Title != "全局偏好" || strings.Join(entries[0].Tags, ",") != "ui" {
		t.Fatalf("global entry = %#v", entries[0])
	}
	if entries[1].Title != "b" {
		t.Fatalf("missing front matter must fall back to the file name: %#v", entries[1])
	}
	currentMatches := 0
	for _, entry := range entries {
		if entry.Scope != memoryScopeProject {
			continue
		}
		if entry.ProjectID == currentID {
			currentMatches++
			if entry.Project != "Alpha 项目" {
				t.Fatalf("project label should use front matter: %#v", entry)
			}
		}
	}
	if currentMatches != 1 {
		t.Fatalf("current project folder must be listed once, got %d: %#v", currentMatches, entryPaths(entries))
	}
}

func TestMemoryVaultIndexTextAndRefresh(t *testing.T) {
	isolateAgentHomes(t)
	if err := refreshMemoryVaultIndex(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(memoryVaultRoot(), memoryIndexName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("INDEX.md must not appear before the vault exists, stat err = %v", err)
	}

	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "a.md"), "---\ntitle: 全局偏好\ntags: [ui]\n---\n内容\n")
	writeMemoryFile(t, filepath.Join(memoryProjectDir("/repo/alpha"), "context.md"), "---\nproject: Alpha 项目\n---\n内容\n")
	if err := refreshMemoryVaultIndex(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(memoryVaultRoot(), memoryIndexName))
	if err != nil {
		t.Fatal(err)
	}
	index := string(data)
	for _, want := range []string{"共享记忆索引", "## 全局（1）", "## 项目（1）", "### 项目：Alpha 项目", "`global/a.md`", "`projects/"} {
		if !strings.Contains(index, want) {
			t.Fatalf("INDEX.md missing %q:\n%s", want, index)
		}
	}
}

func TestCleanMemoryName(t *testing.T) {
	if got, err := cleanMemoryName("  界面偏好  "); err != nil || got != "界面偏好.md" {
		t.Fatalf("cleanMemoryName() = %q, %v", got, err)
	}
	if got, err := cleanMemoryName("notes.MD"); err != nil || got != "notes.MD" {
		t.Fatalf("existing extension must be kept: %q, %v", got, err)
	}
	for _, name := range []string{"", "   ", "a/b", "a\\b", strings.Repeat("x", 101)} {
		if got, err := cleanMemoryName(name); err == nil {
			t.Fatalf("cleanMemoryName(%q) = %q, want error", name, got)
		}
	}
}

func TestListCodexMemoriesIsReadOnly(t *testing.T) {
	isolateAgentHomes(t)
	entries, err := listCodexMemories()
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing Codex memory folder = %#v, %v", entries, err)
	}
	writeMemoryFile(t, filepath.Join(codexMemoryRoot(), "summary.md"), "摘要\n")
	writeMemoryFile(t, filepath.Join(codexMemoryRoot(), "archive", "old.md"), "旧记忆\n")

	entries, err = listCodexMemories()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("listCodexMemories() = %#v", entryPaths(entries))
	}
	if entries[0].Name != "archive/old.md" {
		t.Fatalf("nested memories must show their relative name: %#v", entries[0])
	}
	for _, entry := range entries {
		if !entry.ReadOnly || entry.Origin != "codex" || entry.Scope != "native" {
			t.Fatalf("Codex memories must be read-only native entries: %#v", entry)
		}
	}
}

func findPromptDocument(docs []AgentDocument, target, scope, kind, directory string) (AgentDocument, bool) {
	for _, doc := range docs {
		if doc.Target == target && doc.Scope == scope && doc.Kind == kind && filepath.Dir(filepath.Clean(doc.Path)) == filepath.Clean(directory) {
			return doc, true
		}
	}
	return AgentDocument{}, false
}

func TestAgentDocumentsSurfacePiPromptFiles(t *testing.T) {
	isolateAgentHomes(t)
	project := t.TempDir()
	writeMemoryFile(t, filepath.Join(piRoot(), "APPEND_SYSTEM.md"), "共享记忆规则\n")
	writeMemoryFile(t, filepath.Join(project, ".pi", "APPEND_SYSTEM.md"), "项目记忆规则\n")

	documents := agentDocuments(Workspace{Path: project})
	global, ok := findPromptDocument(documents, "pi", "global", "APPEND_SYSTEM.md", piRoot())
	if !ok || !global.Exists || !global.Loaded {
		t.Fatalf("pi global APPEND_SYSTEM.md = %#v, %v", global, ok)
	}
	if name, ok := findPromptDocument(documents, "pi", "global", "SYSTEM.md", piRoot()); !ok || name.Exists {
		t.Fatalf("missing SYSTEM.md must still be listed: %#v, %v", name, ok)
	}
	local, ok := findPromptDocument(documents, "pi", "project", "APPEND_SYSTEM.md", filepath.Join(project, ".pi"))
	if !ok || !local.Exists || !local.Loaded {
		t.Fatalf("pi project APPEND_SYSTEM.md = %#v, %v", local, ok)
	}
}

func TestAllowedAgentDocumentAcceptsPromptFilesOnlyInAgentFolders(t *testing.T) {
	isolateAgentHomes(t)
	project := t.TempDir()
	selectWorkspaceForStore(t, project)
	if !allowedAgentDocument(filepath.Join(piRoot(), "SYSTEM.md")) {
		t.Fatal("pi global prompt files must be editable")
	}
	if !allowedAgentDocument(filepath.Join(project, ".pi", "APPEND_SYSTEM.md")) {
		t.Fatal("prompt files in a project's .pi folder should be accepted when that project is selected")
	}
	if allowedAgentDocument(filepath.Join(project, "SYSTEM.md")) {
		t.Fatal("a stray SYSTEM.md in the project root is not a pi prompt file")
	}
	if allowedAgentDocument(filepath.Join(piRoot(), "MEMORY.md")) {
		t.Fatal("only known pi prompt files are editable")
	}
}

// selectWorkspaceForStore selects a workspace in the portable store, which is
// how the existing tests drive the code paths that need an active project.
func selectWorkspaceForStore(t *testing.T, path string) {
	t.Helper()
	original, err := readStore()
	if err != nil {
		t.Fatalf("read the test store: %v", err)
	}
	storePath := dataPath()
	_, statErr := os.Stat(storePath)
	updated := original
	updated.Workspaces = append(append([]Workspace{}, original.Workspaces...), Workspace{ID: "memory-vault-test", Name: filepath.Base(path), Path: path})
	updated.ActiveWorkspaceID = "memory-vault-test"
	if err := writeStore(updated); err != nil {
		t.Fatalf("write the test store: %v", err)
	}
	t.Cleanup(func() {
		if errors.Is(statErr, os.ErrNotExist) {
			_ = os.Remove(storePath)
			return
		}
		if err := writeStore(original); err != nil {
			t.Errorf("restore the test store: %v", err)
		}
	})
}

func entryPaths(entries []MemoryEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Scope+"/"+entry.ProjectID+"/"+entry.Name)
	}
	return paths
}
