package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// RuntimeDocument is a user-editable agent configuration file.  The app only
// exposes files that belong to Codex/pi roots or to the selected workspace.
type RuntimeDocument struct {
	ID         string `json:"id"`
	Target     string `json:"target"`
	Scope      string `json:"scope"`
	Kind       string `json:"kind"`
	Format     string `json:"format"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Bytes      int64  `json:"bytes"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
}

// MemoryEntry describes one Markdown memory file.  Vault entries are editable;
// entries produced by an agent's own memory system are reported read-only.
type MemoryEntry struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Title      string   `json:"title,omitempty"`
	Scope      string   `json:"scope"`
	Origin     string   `json:"origin,omitempty"`
	Project    string   `json:"project,omitempty"`
	ProjectID  string   `json:"projectId,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	ReadOnly   bool     `json:"readOnly,omitempty"`
	Path       string   `json:"path"`
	Bytes      int64    `json:"bytes"`
	ModifiedAt string   `json:"modifiedAt,omitempty"`
}

type MCPServer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Target    string `json:"target"`
	Scope     string `json:"scope"`
	Path      string `json:"path"`
	Transport string `json:"transport,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Command   string `json:"command,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

type AdvancedState struct {
	Target           string            `json:"target"`
	CodexHome        string            `json:"codexHome"`
	PiHome           string            `json:"piHome"`
	MemoryVault      string            `json:"memoryVault"`
	CurrentProjectID string            `json:"currentProjectId,omitempty"`
	CodexMemoryRoot  string            `json:"codexMemoryRoot"`
	Documents        []RuntimeDocument `json:"documents"`
	Memories         []MemoryEntry     `json:"memories"`
	CodexMemories    []MemoryEntry     `json:"codexMemories"`
	MCPServers       []MCPServer       `json:"mcpServers"`
	Diagnostics      []string          `json:"diagnostics"`
}

func runtimeDocument(path, target, scope, kind, format string) RuntimeDocument {
	doc := RuntimeDocument{ID: filepath.Clean(path), Target: target, Scope: scope, Kind: kind, Format: format, Path: filepath.Clean(path)}
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		doc.Exists = true
		doc.Bytes = info.Size()
		doc.ModifiedAt = info.ModTime().Format(time.RFC3339)
	}
	return doc
}

func runtimeDocuments(project Workspace) []RuntimeDocument {
	docs := []RuntimeDocument{
		runtimeDocument(filepath.Join(codexHome(), "config.toml"), "codex", "global", "Codex config", "toml"),
		runtimeDocument(filepath.Join(codexHome(), "mcp.json"), "codex", "global", "Codex MCP", "json"),
		runtimeDocument(filepath.Join(piRoot(), "settings.json"), "pi", "global", "pi settings", "json"),
		runtimeDocument(filepath.Join(piRoot(), "models.json"), "pi", "global", "pi models", "json"),
		runtimeDocument(filepath.Join(piRoot(), "mcp.json"), "pi", "global", "pi MCP", "json"),
	}
	if project.Path != "" {
		docs = append(docs,
			runtimeDocument(filepath.Join(project.Path, ".pi", "settings.json"), "pi", "project", "project pi settings", "json"),
			runtimeDocument(filepath.Join(project.Path, ".pi", "mcp.json"), "pi", "project", "project pi MCP", "json"),
			runtimeDocument(filepath.Join(project.Path, ".codex", "config.toml"), "codex", "project", "project Codex config", "toml"),
		)
	}
	return docs
}

func runtimeDocumentAllowed(path string) bool {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	allowed := map[string]bool{"config.toml": true, "settings.json": true, "models.json": true, "mcp.json": true}
	if !allowed[base] {
		return false
	}
	roots := []string{codexHome(), piRoot()}
	if project, err := selectedWorkspace(); err == nil && project.Path != "" {
		roots = append(roots, filepath.Join(project.Path, ".codex"), filepath.Join(project.Path, ".pi"))
	}
	for _, root := range roots {
		if filepath.Clean(root) == filepath.Dir(path) {
			return true
		}
	}
	return false
}

func regularRuntimeFile(path string) bool {
	info, err := os.Lstat(filepath.Clean(path))
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

func readMCPJSON(path string, target, scope string) ([]MCPServer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []MCPServer{}, nil
	}
	if err != nil {
		return nil, err
	}
	obj, err := parseJSONObject(data)
	if err != nil {
		return nil, err
	}
	serversRaw := obj["servers"]
	if len(serversRaw) == 0 {
		serversRaw = obj["mcpServers"]
	}
	if len(serversRaw) == 0 {
		serversRaw = data
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversRaw, &servers); err != nil {
		return nil, err
	}
	return parseMCPServerMap(servers, path, target, scope), nil
}

func parseMCPServerMap(servers map[string]json.RawMessage, path, target, scope string) []MCPServer {
	result := make([]MCPServer, 0, len(servers))
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var raw struct {
			Command   string `json:"command"`
			URL       string `json:"url"`
			URL2      string `json:"serverUrl"`
			Transport string `json:"transport"`
			Enabled   *bool  `json:"enabled"`
		}
		_ = json.Unmarshal(servers[name], &raw)
		endpoint := raw.URL
		if endpoint == "" {
			endpoint = raw.URL2
		}
		result = append(result, MCPServer{ID: target + ":" + scope + ":" + name, Name: name, Target: target, Scope: scope, Path: path, Transport: raw.Transport, Endpoint: endpoint, Command: raw.Command, Enabled: raw.Enabled})
	}
	return result
}

func readMCPTOML(path, target, scope string) ([]MCPServer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []MCPServer{}, nil
	}
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := toml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	servers, _ := root["mcp_servers"].(map[string]any)
	result := []MCPServer{}
	for name, value := range servers {
		entry, _ := value.(map[string]any)
		server := MCPServer{ID: target + ":" + scope + ":" + name, Name: name, Target: target, Scope: scope, Path: path}
		server.Command, _ = entry["command"].(string)
		server.Endpoint, _ = entry["url"].(string)
		server.Transport, _ = entry["transport"].(string)
		if enabled, ok := entry["enabled"].(bool); ok {
			server.Enabled = &enabled
		}
		result = append(result, server)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func loadMCPServers(docs []RuntimeDocument) ([]MCPServer, []string) {
	servers, diagnostics := []MCPServer{}, []string{}
	for _, doc := range docs {
		isMCPDocument := strings.Contains(strings.ToLower(doc.Kind), "mcp") ||
			(doc.Target == "codex" && doc.Format == "toml" && strings.HasSuffix(doc.Path, "config.toml"))
		if !doc.Exists || !isMCPDocument {
			continue
		}
		var found []MCPServer
		var err error
		if doc.Format == "toml" {
			found, err = readMCPTOML(doc.Path, doc.Target, doc.Scope)
		} else {
			found, err = readMCPJSON(doc.Path, doc.Target, doc.Scope)
		}
		if err != nil {
			diagnostics = append(diagnostics, doc.Path+": "+err.Error())
			continue
		}
		servers = append(servers, found...)
	}
	return servers, diagnostics
}

func (a *App) LoadAdvancedState() (AdvancedState, error) {
	project, _ := selectedWorkspace()
	target := selectedTargetMust()
	docs := runtimeDocuments(project)
	runtimeTarget := "codex"
	if target == "pi" {
		runtimeTarget = "pi"
	}
	filteredDocs := make([]RuntimeDocument, 0, len(docs))
	for _, doc := range docs {
		if doc.Target == runtimeTarget {
			filteredDocs = append(filteredDocs, doc)
		}
	}
	docs = filteredDocs
	// The shared vault and Codex' own memory folder are listed for both agents so
	// one memory set works across projects regardless of the selected app.
	memories, err := listVaultMemories(project.Path)
	if err != nil {
		return AdvancedState{}, err
	}
	codexMemories, err := listCodexMemories()
	if err != nil {
		return AdvancedState{}, err
	}
	servers, diagnostics := loadMCPServers(docs)
	if err := ensurePiMemoryBridge(); err != nil {
		diagnostics = append(diagnostics, "pi 共享记忆自动加载配置失败: "+err.Error())
	}
	return AdvancedState{Target: target, CodexHome: codexHome(), PiHome: piRoot(), MemoryVault: memoryVaultRoot(), CurrentProjectID: memoryProjectID(project.Path), CodexMemoryRoot: codexMemoryRoot(), Documents: docs, Memories: memories, CodexMemories: codexMemories, MCPServers: servers, Diagnostics: diagnostics}, nil
}

func selectedTargetMust() string {
	s, err := readStore()
	if err != nil {
		return "chatgpt"
	}
	return selectedTarget(s)
}

func (a *App) ReadRuntimeDocument(path string) (string, error) {
	if !runtimeDocumentAllowed(path) {
		return "", errors.New("只能编辑已识别的代理配置文件")
	}
	if _, err := os.Stat(filepath.Clean(path)); err == nil && !regularRuntimeFile(path) {
		return "", errors.New("配置文件必须是普通文件")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

func validateRuntimeContent(path, content string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		var value map[string]any
		if err := toml.Unmarshal([]byte(content), &value); err != nil {
			return errors.New("TOML 格式无效，未写入")
		}
	case ".json":
		if _, err := parseJSONObject([]byte(content)); err != nil {
			return errors.New("JSON/JSONC 格式无效，未写入")
		}
	}
	return nil
}

func (a *App) WriteRuntimeDocument(path, content string) error {
	if !runtimeDocumentAllowed(path) {
		return errors.New("只能编辑已识别的代理配置文件")
	}
	if len(content) > 512<<10 {
		return errors.New("配置文件不能超过 512 KiB")
	}
	if err := validateRuntimeContent(path, content); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, statErr := os.Stat(path); statErr == nil && !regularRuntimeFile(path) {
		return errors.New("配置文件必须是普通文件")
	}
	original, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		original = nil
		err = nil
	}
	if err != nil {
		return err
	}
	backup := filepath.Join(filepath.Dir(dataPath()), "backups", "runtime", time.Now().Format("20060102-150405.000000000")+"-"+filepath.Base(path))
	if len(original) > 0 {
		if err := atomicWrite(backup, original); err != nil {
			return fmt.Errorf("备份配置失败，未写入: %w", err)
		}
	}
	return atomicWrite(path, []byte(content))
}

// ReadMemory returns any memory file the user may inspect: shared vault notes are
// editable, Codex-owned files are read-only.
func (a *App) ReadMemory(path string) (string, error) {
	path = filepath.Clean(path)
	if !memoryReadAllowed(path) {
		return "", errors.New("只能读取共享记忆或 Codex 记忆目录中的文件")
	}
	if !regularRuntimeFile(path) {
		return "", errors.New("记忆文件必须是普通文件")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// CreateMemory stores a new note in the shared vault.  scope is "global" or
// "project"; project notes live in a folder keyed by the workspace path so two
// projects never overwrite each other.
func (a *App) CreateMemory(name, content, scope string) (MemoryEntry, error) {
	cleaned, err := cleanMemoryName(name)
	if err != nil {
		return MemoryEntry{}, err
	}
	if len(content) > 256<<10 {
		return MemoryEntry{}, errors.New("记忆不能超过 256 KiB")
	}
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		scope = memoryScopeGlobal
	}
	projectID := ""
	directory := ""
	switch scope {
	case memoryScopeGlobal:
		directory = memoryVaultGlobalDir()
	case memoryScopeProject:
		project, workspaceErr := selectedWorkspace()
		if workspaceErr != nil || project.Path == "" {
			return MemoryEntry{}, errors.New("请先选择项目工作区，再创建项目记忆")
		}
		projectID = memoryProjectID(project.Path)
		directory = memoryProjectDir(project.Path)
	default:
		return MemoryEntry{}, errors.New("记忆范围无效")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ensureMemoryVaultDirs(directory); err != nil {
		return MemoryEntry{}, err
	}
	path := filepath.Join(directory, cleaned)
	if !memoryVaultAllowed(path) {
		return MemoryEntry{}, errors.New("共享记忆之外的文件不能创建")
	}
	if _, err := os.Stat(path); err == nil {
		return MemoryEntry{}, errors.New("同名记忆已存在")
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		return MemoryEntry{}, err
	}
	if err := refreshMemoryVaultIndex(); err != nil {
		return MemoryEntry{}, fmt.Errorf("记忆已保存，但索引或 pi 自动加载配置更新失败: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return MemoryEntry{}, err
	}
	title, tags, projectLabel := parseMemoryFrontMatter(content)
	if title == "" {
		title = strings.TrimSuffix(cleaned, filepath.Ext(cleaned))
	}
	if projectLabel == "" {
		projectLabel = projectID
	}
	return MemoryEntry{ID: path, Name: cleaned, Title: title, Scope: scope, Origin: "vault", Project: projectLabel, ProjectID: projectID, Tags: tags, Path: path, Bytes: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339)}, nil
}

func (a *App) WriteMemory(path, content string) error {
	path = filepath.Clean(path)
	if !memoryVaultAllowed(path) {
		return errors.New("共享记忆之外的文件不能修改")
	}
	if len(content) > 256<<10 {
		return errors.New("记忆不能超过 256 KiB")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !regularRuntimeFile(path) {
		return errors.New("记忆文件必须是普通文件")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := atomicWrite(memoryBackupPath(path, "writes"), original); err != nil {
		return fmt.Errorf("备份记忆失败，未写入: %w", err)
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		return err
	}
	if err := refreshMemoryVaultIndex(); err != nil {
		return fmt.Errorf("记忆已保存，但索引或 pi 自动加载配置更新失败: %w", err)
	}
	return nil
}

func (a *App) DeleteMemory(path string) error {
	path = filepath.Clean(path)
	if !memoryVaultAllowed(path) {
		return errors.New("共享记忆之外的文件不能删除")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !regularRuntimeFile(path) {
		return errors.New("记忆文件必须是普通文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := atomicWrite(memoryBackupPath(path, filepath.Join("writes", "trash")), data); err != nil {
		return fmt.Errorf("备份记忆失败，未删除: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if err := refreshMemoryVaultIndex(); err != nil {
		return fmt.Errorf("记忆已删除，但索引或 pi 自动加载配置更新失败: %w", err)
	}
	return nil
}

func (a *App) OpenAdvancedDirectory(kind string) error {
	var path string
	switch kind {
	case "codex":
		path = codexHome()
	case "pi":
		path = piRoot()
	case "memory":
		if err := ensureMemoryVaultDirs(); err != nil {
			return err
		}
		path = memoryVaultRoot()
	case "codex-memory":
		path = codexMemoryRoot()
	default:
		return errors.New("目录类型无效")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return openCodexDirectory(path)
}
