package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Workspace is a project folder whose instructions and shared resources can be
// inspected without changing the active model target.
type Workspace struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	LastUsed string `json:"lastUsed,omitempty"`
}

type WorkspaceState struct {
	Projects []Workspace `json:"projects"`
	ActiveID string      `json:"activeId"`
}

type AgentDocument struct {
	ID         string `json:"id"`
	Target     string `json:"target"`
	Scope      string `json:"scope"`
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Loaded     bool   `json:"loaded"`
	Overridden bool   `json:"overridden"`
	Bytes      int64  `json:"bytes"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
}

type AgentConfigState struct {
	Workspace        Workspace       `json:"workspace"`
	Documents        []AgentDocument `json:"documents"`
	CodexHome        string          `json:"codexHome"`
	PiHome           string          `json:"piHome"`
	SharedSkillsRoot string          `json:"sharedSkillsRoot"`
	SharedSkills     []SkillInfo     `json:"sharedSkills"`
}

var agentDocumentNames = []string{"AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD"}

func workspaceState(s storeFile) WorkspaceState {
	projects := append([]Workspace(nil), s.Workspaces...)
	sort.Slice(projects, func(i, j int) bool { return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name) })
	return WorkspaceState{Projects: projects, ActiveID: s.ActiveWorkspaceID}
}

func workspaceByID(s storeFile, id string) (Workspace, bool) {
	for _, project := range s.Workspaces {
		if project.ID == id {
			return project, true
		}
	}
	return Workspace{}, false
}

func validateWorkspacePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return "", errors.New("请选择项目文件夹")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", errors.New("项目路径不存在或不是文件夹")
	}
	return filepath.Clean(abs), nil
}

func (a *App) LoadWorkspaceState() (WorkspaceState, error) {
	s, err := readStore()
	if err != nil {
		return WorkspaceState{}, err
	}
	return workspaceState(s), nil
}

func (a *App) ChooseWorkspaceDirectory(directory string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("目录选择需要在应用中使用")
	}
	initial, err := validateWorkspacePath(directory)
	if err != nil {
		initial, _ = os.UserHomeDir()
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择项目文件夹", DefaultDirectory: initial})
}

func (a *App) SaveWorkspace(id, name, directory string) (WorkspaceState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	path, err := validateWorkspacePath(directory)
	if err != nil {
		return WorkspaceState{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(path)
	}
	if len([]rune(name)) > 80 {
		return WorkspaceState{}, errors.New("项目名称不能超过 80 个字符")
	}
	s, err := readStore()
	if err != nil {
		return WorkspaceState{}, err
	}
	if id == "" {
		id = fmt.Sprintf("workspace-%d", time.Now().UnixNano())
	}
	found := false
	for i := range s.Workspaces {
		if s.Workspaces[i].ID == id {
			s.Workspaces[i].Name, s.Workspaces[i].Path, s.Workspaces[i].LastUsed = name, path, time.Now().Format(time.RFC3339)
			found = true
		}
	}
	if !found {
		s.Workspaces = append(s.Workspaces, Workspace{ID: id, Name: name, Path: path, LastUsed: time.Now().Format(time.RFC3339)})
	}
	s.ActiveWorkspaceID = id
	if err = writeStore(s); err != nil {
		return WorkspaceState{}, err
	}
	return workspaceState(s), nil
}

func (a *App) SelectWorkspace(id string) (WorkspaceState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := readStore()
	if err != nil {
		return WorkspaceState{}, err
	}
	project, ok := workspaceByID(s, id)
	if !ok {
		return WorkspaceState{}, errors.New("项目不存在，请刷新后重试")
	}
	project.LastUsed = time.Now().Format(time.RFC3339)
	for i := range s.Workspaces {
		if s.Workspaces[i].ID == id {
			s.Workspaces[i] = project
		}
	}
	s.ActiveWorkspaceID = id
	if err = writeStore(s); err != nil {
		return WorkspaceState{}, err
	}
	return workspaceState(s), nil
}

func (a *App) DeleteWorkspace(id string) (WorkspaceState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := readStore()
	if err != nil {
		return WorkspaceState{}, err
	}
	projects := make([]Workspace, 0, len(s.Workspaces))
	for _, project := range s.Workspaces {
		if project.ID != id {
			projects = append(projects, project)
		}
	}
	if len(projects) == len(s.Workspaces) {
		return WorkspaceState{}, errors.New("项目不存在")
	}
	s.Workspaces = projects
	if s.ActiveWorkspaceID == id {
		s.ActiveWorkspaceID = ""
	}
	if err = writeStore(s); err != nil {
		return WorkspaceState{}, err
	}
	return workspaceState(s), nil
}

func selectedWorkspace() (Workspace, error) {
	s, err := readStore()
	if err != nil {
		return Workspace{}, err
	}
	if s.ActiveWorkspaceID == "" {
		return Workspace{}, errors.New("请先选择项目工作区")
	}
	project, ok := workspaceByID(s, s.ActiveWorkspaceID)
	if !ok {
		return Workspace{}, errors.New("当前项目不存在，请重新选择工作区")
	}
	return project, nil
}

func codexHome() string {
	if root := strings.TrimSpace(os.Getenv("CODEX_HOME")); root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func sharedSkillsRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agents", "skills")
}

func documentID(path string) string { return filepath.Clean(path) }

func fileDocument(target, scope, kind, path string, loaded, overridden bool) AgentDocument {
	doc := AgentDocument{ID: documentID(path), Target: target, Scope: scope, Kind: kind, Path: path, Loaded: loaded, Overridden: overridden}
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		doc.Exists = true
		doc.Bytes = info.Size()
		doc.ModifiedAt = info.ModTime().Format(time.RFC3339)
	}
	return doc
}

func candidate(path string, names []string) string {
	for _, name := range names {
		p := filepath.Join(path, name)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return p
		}
	}
	return ""
}

func parentDirectories(path string) []string {
	result := []string{}
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		result = append(result, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func appendDocuments(result *[]AgentDocument, target, scope, directory string, names []string, loaded string) {
	selected := loaded != ""
	for _, name := range names {
		path := filepath.Join(directory, name)
		isLoaded := path == loaded
		if selected && !isLoaded {
			// Keep the file visible, but explain that it is shadowed by an override.
		}
		*result = append(*result, fileDocument(target, scope, name, path, isLoaded, selected && !isLoaded))
	}
}

func agentDocuments(project Workspace) []AgentDocument {
	documents := []AgentDocument{}
	codexGlobal := candidate(codexHome(), append([]string{"AGENTS.override.md"}, agentDocumentNames...))
	appendDocuments(&documents, "codex", "global", codexHome(), append([]string{"AGENTS.override.md"}, agentDocumentNames...), codexGlobal)
	piNames := append([]string{"AGENTS.override.md"}, agentDocumentNames...)
	piGlobal := candidate(piRoot(), piNames)
	appendDocuments(&documents, "pi", "global", piRoot(), piNames, piGlobal)
	if project.Path == "" {
		return documents
	}
	for _, directory := range parentDirectories(project.Path) {
		codex := candidate(directory, append([]string{"AGENTS.override.md"}, agentDocumentNames...))
		appendDocuments(&documents, "codex", "project", directory, append([]string{"AGENTS.override.md"}, agentDocumentNames...), codex)
		pi := candidate(directory, piNames)
		appendDocuments(&documents, "pi", "project", directory, piNames, pi)
	}
	return documents
}

func (a *App) LoadAgentConfig() (AgentConfigState, error) {
	project, err := selectedWorkspace()
	if err != nil {
		return AgentConfigState{}, err
	}
	shared, err := listSkills(sharedSkillsRoot())
	if err != nil {
		return AgentConfigState{}, err
	}
	return AgentConfigState{Workspace: project, Documents: agentDocuments(project), CodexHome: codexHome(), PiHome: piRoot(), SharedSkillsRoot: sharedSkillsRoot(), SharedSkills: shared}, nil
}

func allowedAgentDocument(path string) bool {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	validName := base == "AGENTS.override.md" || base == "AGENTS.md" || base == "AGENTS.MD" || base == "CLAUDE.md" || base == "CLAUDE.MD"
	if !validName {
		return false
	}
	project, err := selectedWorkspace()
	if err != nil {
		return false
	}
	roots := []string{codexHome(), piRoot(), filepath.Dir(sharedSkillsRoot())}
	if project.Path != "" {
		roots = append(roots, project.Path)
	}
	for _, root := range roots {
		if isChildPath(root, path) || filepath.Clean(root) == filepath.Dir(path) {
			return true
		}
	}
	return false
}

func (a *App) ReadAgentDocument(path string) (string, error) {
	if !allowedAgentDocument(path) {
		return "", errors.New("只能编辑已发现的代理指令文件")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

func (a *App) WriteAgentDocument(path, content string) error {
	if !allowedAgentDocument(path) {
		return errors.New("只能编辑已发现的代理指令文件")
	}
	if len(content) > 128<<10 {
		return errors.New("指令文件不能超过 128 KiB")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	backupDir := filepath.Join(filepath.Dir(dataPath()), "backups", "agent-docs")
	if old, err := os.ReadFile(path); err == nil {
		backup := filepath.Join(backupDir, time.Now().Format("20060102-150405.000000000")+"-"+filepath.Base(path))
		if err := atomicWrite(backup, old); err != nil {
			return fmt.Errorf("备份指令文件失败，未修改原文件: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicWrite(path, []byte(content))
}

func (a *App) LoadSharedSkillsState() (SkillsState, error) {
	root := sharedSkillsRoot()
	skills, err := listSkills(root)
	if err != nil {
		return SkillsState{}, err
	}
	trash, err := listTrash(root)
	if err != nil {
		return SkillsState{}, err
	}
	return SkillsState{Root: root, Target: "shared", Skills: skills, Trash: trash, Catalog: []SkillCatalogItem{}, Markets: skillMarkets, Errors: []string{}}, nil
}

func (a *App) OpenWorkspaceDirectory() error {
	project, err := selectedWorkspace()
	if err != nil {
		return err
	}
	return revealConfig(project.Path)
}

func (a *App) OpenSharedSkillsDirectory() error {
	root := sharedSkillsRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	return revealConfig(root)
}
