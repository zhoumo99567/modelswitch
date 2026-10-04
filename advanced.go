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

type MemoryEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
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
	Target      string            `json:"target"`
	CodexHome   string            `json:"codexHome"`
	PiHome      string            `json:"piHome"`
	MemoryRoot  string            `json:"memoryRoot"`
	Documents   []RuntimeDocument `json:"documents"`
	Memories    []MemoryEntry     `json:"memories"`
	MCPServers  []MCPServer       `json:"mcpServers"`
	Diagnostics []string          `json:"diagnostics"`
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

func memoryRoots(project Workspace) [][2]string {
	roots := [][2]string{{"global", filepath.Join(codexHome(), "memories")}}
	if project.Path != "" {
		roots = append(roots, [2]string{"project", filepath.Join(project.Path, ".codex", "memories")})
	}
	return roots
}

func listMemoryEntries(project Workspace) ([]MemoryEntry, error) {
	entries := []MemoryEntry{}
	for _, root := range memoryRoots(project) {
		if _, err := os.Stat(root[1]); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := filepath.WalkDir(root[1], func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != root[1] && filepath.Base(path) == ".trash" {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			entries = append(entries, MemoryEntry{ID: filepath.Clean(path), Name: entry.Name(), Scope: root[0], Path: filepath.Clean(path), Bytes: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339)})
			if len(entries) > 512 {
				return errors.New("记忆文件数量超过 512 个，请先清理后再加载")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path) })
	return entries, nil
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

func memoryPathAllowed(path string) bool {
	path = filepath.Clean(path)
	for _, root := range memoryRoots(func() Workspace { p, _ := selectedWorkspace(); return p }()) {
		if isChildPath(root[1], path) && strings.EqualFold(filepath.Ext(path), ".md") {
			return true
		}
	}
	return false
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
	memories := []MemoryEntry{}
	if target == "chatgpt" {
		var err error
		memories, err = listMemoryEntries(project)
		if err != nil {
			return AdvancedState{}, err
		}
	}
	servers, diagnostics := loadMCPServers(docs)
	memoryRoot := ""
	if target == "chatgpt" {
		memoryRoot = filepath.Join(codexHome(), "memories")
	}
	return AdvancedState{Target: target, CodexHome: codexHome(), PiHome: piRoot(), MemoryRoot: memoryRoot, Documents: docs, Memories: memories, MCPServers: servers, Diagnostics: diagnostics}, nil
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

func (a *App) ReadMemory(path string) (string, error) {
	if !memoryPathAllowed(path) {
		return "", errors.New("只能读取已识别的记忆文件")
	}
	if !regularRuntimeFile(path) {
		return "", errors.New("记忆文件必须是普通文件")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

func (a *App) CreateMemory(name, content string) (MemoryEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return MemoryEntry{}, errors.New("请输入记忆名称")
	}
	if len([]rune(name)) > 100 || strings.ContainsAny(name, `/\\\x00\r\n`) {
		return MemoryEntry{}, errors.New("记忆名称不合法")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	root := filepath.Join(codexHome(), "memories")
	path := filepath.Join(root, name)
	if _, err := os.Stat(path); err == nil {
		return MemoryEntry{}, errors.New("同名记忆已存在")
	}
	if len(content) > 256<<10 {
		return MemoryEntry{}, errors.New("记忆不能超过 256 KiB")
	}
	if err := atomicWrite(path, []byte(content)); err != nil {
		return MemoryEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return MemoryEntry{}, err
	}
	return MemoryEntry{ID: path, Name: name, Scope: "global", Path: path, Bytes: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339)}, nil
}

func (a *App) WriteMemory(path, content string) error {
	if !memoryPathAllowed(path) {
		return errors.New("只能编辑已识别的记忆文件")
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
	backup := filepath.Join(filepath.Dir(dataPath()), "backups", "memories", time.Now().Format("20060102-150405.000000000")+"-"+filepath.Base(path))
	if err := atomicWrite(backup, original); err != nil {
		return fmt.Errorf("备份记忆失败，未写入: %w", err)
	}
	return atomicWrite(path, []byte(content))
}

func (a *App) DeleteMemory(path string) error {
	if !memoryPathAllowed(path) {
		return errors.New("只能删除已识别的记忆文件")
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
	trash := filepath.Join(filepath.Dir(dataPath()), "backups", "memories", "trash", time.Now().Format("20060102-150405.000000000")+"-"+filepath.Base(path))
	if err := atomicWrite(trash, data); err != nil {
		return fmt.Errorf("备份记忆失败，未删除: %w", err)
	}
	return os.Remove(path)
}

func (a *App) OpenAdvancedDirectory(kind string) error {
	var path string
	switch kind {
	case "codex":
		path = codexHome()
	case "pi":
		path = piRoot()
	case "memory":
		path = filepath.Join(codexHome(), "memories")
	default:
		return errors.New("目录类型无效")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return openCodexDirectory(path)
}
