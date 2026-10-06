package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Shared memory vault.  Codex and pi agent keep their own memory formats, so
// ModelSwitcher maintains one portable Markdown vault that every agent can read.
// Layout: <vault>/global/<name>.md, <vault>/projects/<project-id>/<name>.md and
// an auto-generated <vault>/INDEX.md.
const memoryIndexName = "INDEX.md"

const (
	memoryScopeGlobal  = "global"
	memoryScopeProject = "project"
)

const memoryFrontMatterReadLimit = 4096

func memoryVaultRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agents", "memory")
}

func memoryVaultGlobalDir() string { return filepath.Join(memoryVaultRoot(), memoryScopeGlobal) }

func memoryVaultProjectsDir() string { return filepath.Join(memoryVaultRoot(), memoryScopeProject+"s") }

func codexMemoryRoot() string { return filepath.Join(codexHome(), "memories") }

var memorySlugInvalid = regexp.MustCompile(`[^a-z0-9._-]+`)

// memoryProjectID derives a stable, collision-free directory name for a project
// path so two checkouts with the same folder name keep separate memories.
func memoryProjectID(projectPath string) string {
	if strings.TrimSpace(projectPath) == "" {
		return ""
	}
	clean := filepath.Clean(projectPath)
	slug := strings.Trim(memorySlugInvalid.ReplaceAllString(strings.ToLower(filepath.Base(clean)), "-"), "-.")
	if slug == "" || slug == "." || slug == ".." {
		slug = "project"
	}
	sum := sha256.Sum256([]byte(clean))
	return slug + "-" + hex.EncodeToString(sum[:4])
}

func memoryProjectDir(projectPath string) string {
	id := memoryProjectID(projectPath)
	if id == "" {
		return ""
	}
	return filepath.Join(memoryVaultProjectsDir(), id)
}

// parseMemoryFrontMatter reads the optional leading YAML-ish block used to label
// a memory with a title, tags, or a human-friendly project name.
func parseMemoryFrontMatter(content string) (title string, tags []string, project string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return "", nil, ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "..." {
			break
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "title":
			title = value
		case "project":
			project = value
		case "tags":
			for _, tag := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' }) {
				tag = strings.Trim(tag, "[]\"'")
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
	}
	return title, tags, project
}

func readMemoryHead(path string, limit int) string {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return ""
	}
	defer file.Close()
	buffer := make([]byte, limit)
	read, err := io.ReadFull(file, buffer)
	if read <= 0 {
		return ""
	}
	return string(buffer[:read])
}

// memoryVaultClassify reports which vault scope a path belongs to.  Only files
// stored directly in the global folder or in one project folder are vault files.
func memoryVaultClassify(path string) (scope string, projectID string) {
	clean := filepath.Clean(path)
	globalDir := filepath.Clean(memoryVaultGlobalDir())
	if filepath.Dir(clean) == globalDir {
		return memoryScopeGlobal, ""
	}
	projectsDir := filepath.Clean(memoryVaultProjectsDir())
	if relative, err := filepath.Rel(projectsDir, clean); err == nil && !strings.HasPrefix(relative, "..") {
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return memoryScopeProject, parts[0]
		}
	}
	return "", ""
}

func memoryVaultAllowed(path string) bool {
	scope, _ := memoryVaultClassify(path)
	return scope != "" && strings.EqualFold(filepath.Ext(path), ".md") && filepath.Base(path) != memoryIndexName
}

func memoryReadOnlyAllowed(path string) bool {
	return isChildPath(codexMemoryRoot(), filepath.Clean(path))
}

func memoryReadAllowed(path string) bool {
	return memoryVaultAllowed(path) || memoryReadOnlyAllowed(path)
}

func vaultDirEntries(dir, scope, projectID string) ([]MemoryEntry, error) {
	items, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := []MemoryEntry{}
	for _, item := range items {
		if item.IsDir() || item.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(item.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, item.Name())
		info, err := item.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		title, tags, project := parseMemoryFrontMatter(readMemoryHead(path, memoryFrontMatterReadLimit))
		if title == "" {
			title = strings.TrimSuffix(item.Name(), filepath.Ext(item.Name()))
		}
		label := project
		if label == "" {
			label = projectID
		}
		result = append(result, MemoryEntry{ID: filepath.Clean(path), Name: item.Name(), Title: title, Scope: scope, Origin: "vault", Project: label, ProjectID: projectID, Tags: tags, Path: filepath.Clean(path), Bytes: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339)})
	}
	return result, nil
}

// listVaultMemories returns the whole vault: global notes plus every project
// folder, so memories stay visible across projects instead of per workspace.
func listVaultMemories(projectPath string) ([]MemoryEntry, error) {
	directories := []struct {
		dir       string
		scope     string
		projectID string
	}{
		{memoryVaultGlobalDir(), memoryScopeGlobal, ""},
	}
	seen := map[string]bool{filepath.Clean(memoryVaultGlobalDir()): true}
	if current := memoryProjectDir(projectPath); current != "" && !seen[filepath.Clean(current)] {
		seen[filepath.Clean(current)] = true
		directories = append(directories, struct {
			dir       string
			scope     string
			projectID string
		}{current, memoryScopeProject, memoryProjectID(projectPath)})
	}
	if items, err := os.ReadDir(memoryVaultProjectsDir()); err == nil {
		for _, item := range items {
			if !item.IsDir() {
				continue
			}
			dir := filepath.Join(memoryVaultProjectsDir(), item.Name())
			if seen[filepath.Clean(dir)] {
				continue
			}
			seen[filepath.Clean(dir)] = true
			directories = append(directories, struct {
				dir       string
				scope     string
				projectID string
			}{dir, memoryScopeProject, item.Name()})
		}
	}
	entries := []MemoryEntry{}
	for _, directory := range directories {
		found, err := vaultDirEntries(directory.dir, directory.scope, directory.projectID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, found...)
	}
	if len(entries) > 512 {
		return nil, errors.New("共享记忆文件超过 512 个，请先清理后再加载")
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Scope != entries[j].Scope {
			return entries[i].Scope == memoryScopeGlobal
		}
		if entries[i].ProjectID != entries[j].ProjectID {
			return strings.ToLower(entries[i].ProjectID) < strings.ToLower(entries[j].ProjectID)
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// listCodexMemories lists Codex' own memory folder.  Codex rewrites and prunes
// that folder freely, so the app only ever reads it.
func listCodexMemories() ([]MemoryEntry, error) {
	root := filepath.Clean(codexMemoryRoot())
	entries := []MemoryEntry{}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return entries, nil
	} else if err != nil {
		return nil, err
	}
	err := filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || item.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(item.Name()), ".md") {
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		name := filepath.Base(path)
		if relative, err := filepath.Rel(root, path); err == nil && relative != "" {
			name = filepath.ToSlash(relative)
		}
		title, tags, _ := parseMemoryFrontMatter(readMemoryHead(path, memoryFrontMatterReadLimit))
		if title == "" {
			title = strings.TrimSuffix(name, filepath.Ext(name))
		}
		entries = append(entries, MemoryEntry{ID: filepath.Clean(path), Name: name, Title: title, Scope: "native", Origin: "codex", Tags: tags, Path: filepath.Clean(path), Bytes: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339), ReadOnly: true})
		if len(entries) > 512 {
			return errors.New("Codex 记忆文件超过 512 个，请先清理后再加载")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path) })
	return entries, nil
}

func memoryVaultIndexText(entries []MemoryEntry) string {
	var builder strings.Builder
	builder.WriteString("# 共享记忆索引\n\n")
	builder.WriteString("> 由 ModelSwitcher 自动生成，请勿手动编辑。\n")
	builder.WriteString(fmt.Sprintf("> 生成时间：%s\n", time.Now().Format(time.RFC3339)))
	if len(entries) == 0 {
		builder.WriteString("\n暂无记忆文件。\n")
		return builder.String()
	}
	groups := []struct {
		title string
		match func(MemoryEntry) bool
	}{
		{"全局", func(entry MemoryEntry) bool { return entry.Scope == memoryScopeGlobal }},
		{"项目", func(entry MemoryEntry) bool { return entry.Scope == memoryScopeProject }},
	}
	for _, group := range groups {
		selected := []MemoryEntry{}
		for _, entry := range entries {
			if group.match(entry) {
				selected = append(selected, entry)
			}
		}
		if len(selected) == 0 {
			continue
		}
		builder.WriteString(fmt.Sprintf("\n## %s（%d）\n\n", group.title, len(selected)))
		currentProject := ""
		for _, entry := range selected {
			if entry.Scope == memoryScopeProject && entry.ProjectID != currentProject {
				currentProject = entry.ProjectID
				builder.WriteString(fmt.Sprintf("\n### 项目：%s\n\n", entry.Project))
			}
			relative, err := filepath.Rel(memoryVaultRoot(), entry.Path)
			if err != nil {
				relative = entry.Name
			}
			line := fmt.Sprintf("- `%s` — %s", filepath.ToSlash(relative), entry.Title)
			if len(entry.Tags) > 0 {
				line += "（标签：" + strings.Join(entry.Tags, ", ") + "）"
			}
			builder.WriteString(line + "\n")
		}
	}
	return builder.String()
}

// refreshMemoryVaultIndex rebuilds INDEX.md so other agents can discover the
// vault without an integration.  It is a no-op before the vault exists.
func refreshMemoryVaultIndex() error {
	root := filepath.Clean(memoryVaultRoot())
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	entries, err := listVaultMemories("")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(root, memoryIndexName), []byte(memoryVaultIndexText(entries)))
}

func ensureMemoryVaultDirs(paths ...string) error {
	if err := os.MkdirAll(memoryVaultGlobalDir(), 0700); err != nil {
		return err
	}
	for _, path := range paths {
		if path != "" {
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
		}
	}
	return nil
}

func memoryBackupPath(path string, folder string) string {
	return filepath.Join(filepath.Dir(dataPath()), "backups", "memories", folder, time.Now().Format("20060102-150405.000000000")+"-"+filepath.Base(path))
}

func cleanMemoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("请输入记忆名称")
	}
	if len([]rune(name)) > 100 || strings.ContainsAny(name, "/\\\x00\r\n") {
		return "", errors.New("记忆名称不合法")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return name, nil
}
