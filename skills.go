package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const officialMarketplaceURL = "https://raw.githubusercontent.com/openai/plugins/main/.agents/plugins/marketplace.json"
const officialCommitURL = "https://api.github.com/repos/openai/plugins/commits/main"
const maxSkillDownload = 64 << 20
const maxSkillFile = 10 << 20

var skillNamePattern = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	System      bool   `json:"system"`
	HasScripts  bool   `json:"hasScripts"`
	FileCount   int    `json:"fileCount"`
	SizeBytes   int64  `json:"sizeBytes"`
	ModifiedAt  string `json:"modifiedAt,omitempty"`
}

type SkillCatalogItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Source      string `json:"source"`
	URL         string `json:"url"`
	Installed   bool   `json:"installed"`
}

type SkillsState struct {
	Root    string             `json:"root"`
	Skills  []SkillInfo        `json:"skills"`
	Catalog []SkillCatalogItem `json:"catalog"`
}

type marketplace struct {
	Plugins []marketplacePlugin `json:"plugins"`
}
type marketplacePlugin struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Source   struct {
		Source string `json:"source"`
		Path   string `json:"path"`
		URL    string `json:"url"`
	} `json:"source"`
	Policy struct {
		Installation string   `json:"installation"`
		Products     []string `json:"products"`
	} `json:"policy"`
	Interface struct {
		DisplayName string `json:"displayName"`
	} `json:"interface"`
}

func skillRoot() string {
	if root := strings.TrimSpace(os.Getenv("CODEX_HOME")); root != "" {
		return filepath.Join(root, "skills")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "skills")
}

func (a *App) ListSkills() ([]SkillInfo, error) {
	return listSkills(skillRoot())
}

func (a *App) ListSkillCatalog() ([]SkillCatalogItem, error) {
	catalog, err := fetchSkillCatalog()
	if err != nil {
		return nil, err
	}
	installed, err := listSkills(skillRoot())
	if err != nil {
		return nil, err
	}
	installedSet := map[string]bool{}
	for _, item := range installed {
		installedSet[item.Name] = true
	}
	for i := range catalog {
		catalog[i].Installed = installedSet[catalog[i].ID]
	}
	return catalog, nil
}

func (a *App) LoadSkillsState() (SkillsState, error) {
	items, err := listSkills(skillRoot())
	if err != nil {
		return SkillsState{}, err
	}
	catalog, err := fetchSkillCatalog()
	if err != nil {
		return SkillsState{Root: skillRoot(), Skills: items, Catalog: []SkillCatalogItem{}}, nil
	}
	installed := map[string]bool{}
	for _, item := range items {
		installed[item.Name] = true
	}
	for i := range catalog {
		catalog[i].Installed = installed[catalog[i].ID]
	}
	return SkillsState{Root: skillRoot(), Skills: items, Catalog: catalog}, nil
}

func (a *App) InstallSkill(id string) ([]SkillInfo, error) {
	if !skillNamePattern.MatchString(id) {
		return nil, errors.New("技能标识不合法")
	}
	if !isOfficialCatalogSkill(id) {
		return nil, errors.New("只允许安装官方 Codex 技能目录中的项目")
	}
	data, err := downloadSkillArchive()
	if err != nil {
		return nil, err
	}
	if err = installPluginSkills(skillRoot(), id, data); err != nil {
		return nil, err
	}
	return listSkills(skillRoot())
}

func (a *App) DeleteSkill(name string) ([]SkillInfo, error) {
	if !skillNamePattern.MatchString(name) || name == ".system" {
		return nil, errors.New("该技能不能删除")
	}
	root := skillRoot()
	target := filepath.Join(root, name)
	if !isChildPath(root, target) {
		return nil, errors.New("技能路径无效")
	}
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("技能不存在")
	} else if err != nil {
		return nil, err
	}
	trash := filepath.Join(root, ".trash")
	if err := os.MkdirAll(trash, 0700); err != nil {
		return nil, err
	}
	backup := filepath.Join(trash, name+"-"+time.Now().Format("20060102-150405.000000000"))
	if err := os.Rename(target, backup); err != nil {
		return nil, fmt.Errorf("移入技能回收区失败: %w", err)
	}
	return listSkills(root)
}

func listSkills(root string) ([]SkillInfo, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]SkillInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || (strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".system") {
			continue
		}
		info, err := inspectSkill(entry.Name(), filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func inspectSkill(name, path string) (SkillInfo, error) {
	info := SkillInfo{Name: name, Path: path, System: name == ".system"}
	var newest time.Time
	err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			if strings.EqualFold(fileInfo.Name(), "scripts") {
				info.HasScripts = true
			}
			return nil
		}
		info.FileCount++
		info.SizeBytes += fileInfo.Size()
		if fileInfo.ModTime().After(newest) {
			newest = fileInfo.ModTime()
		}
		if strings.EqualFold(fileInfo.Name(), "SKILL.md") && info.Description == "" {
			if data, readErr := os.ReadFile(filePath); readErr == nil {
				info.Description = skillDescription(string(data))
			}
		}
		return nil
	})
	if err != nil {
		return SkillInfo{}, err
	}
	if !newest.IsZero() {
		info.ModifiedAt = newest.Format(time.RFC3339)
	}
	if info.Description == "" {
		info.Description = "本地技能"
	}
	return info, nil
}

func skillDescription(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	frontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			frontmatter = !frontmatter
			continue
		}
		if frontmatter && strings.HasPrefix(trimmed, "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "description:")), "\"'")
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return "本地技能"
}

func fetchSkillCatalog() ([]SkillCatalogItem, error) {
	data, err := fetchURL(officialMarketplaceURL, 2<<20)
	if err != nil {
		return nil, errors.New("官方技能目录暂时不可用，请检查网络连接")
	}
	var source marketplace
	if err = json.Unmarshal(data, &source); err != nil {
		return nil, errors.New("官方技能目录格式无效")
	}
	result := []SkillCatalogItem{}
	for _, item := range source.Plugins {
		if item.Name == "" || item.Policy.Installation != "AVAILABLE" || !isCodexProduct(item.Policy.Products) {
			continue
		}
		if item.Source.Source != "local" || !strings.HasPrefix(item.Source.Path, "./plugins/") {
			continue
		}
		name := item.Interface.DisplayName
		if name == "" {
			name = item.Name
		}
		result = append(result, SkillCatalogItem{ID: item.Name, Name: name, Category: item.Category, Source: "OpenAI 官方目录", URL: "https://github.com/openai/plugins/tree/main/plugins/" + item.Name})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func isCodexProduct(products []string) bool {
	if len(products) == 0 {
		return true
	}
	for _, product := range products {
		if strings.EqualFold(product, "CODEX") {
			return true
		}
	}
	return false
}

func isOfficialCatalogSkill(id string) bool {
	catalog, err := fetchSkillCatalog()
	if err != nil {
		return false
	}
	for _, item := range catalog {
		if item.ID == id {
			return true
		}
	}
	return false
}

func downloadSkillArchive() ([]byte, error) {
	commitData, err := fetchURL(officialCommitURL, 128<<10)
	if err != nil {
		return nil, errors.New("无法确认官方技能版本")
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err = json.Unmarshal(commitData, &commit); err != nil || !regexp.MustCompile("^[a-fA-F0-9]{40}$").MatchString(commit.SHA) {
		return nil, errors.New("官方技能版本标识无效")
	}
	data, err := fetchURL("https://codeload.github.com/openai/plugins/zip/"+commit.SHA, maxSkillDownload)
	if err != nil {
		return nil, errors.New("下载官方技能包失败，请检查网络连接")
	}
	return data, nil
}

func fetchURL(rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ModelSwitcher")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("下载内容超过安全大小限制")
	}
	return data, nil
}

func installPluginSkills(root, pluginID string, archive []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return errors.New("官方技能包不是有效 ZIP")
	}
	prefix := ""
	marker := "/plugins/" + pluginID + "/skills/"
	for _, file := range reader.File {
		if index := strings.Index(file.Name, marker); index >= 0 {
			prefix = file.Name[:index+len(marker)]
			break
		}
	}
	if prefix == "" {
		return errors.New("官方项目中没有可安装的 skills 目录")
	}
	stage, err := os.MkdirTemp(root, ".skill-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	installedDirs := map[string]bool{}
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file.Name, prefix)
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("技能包包含非法路径")
		}
		parts := strings.Split(clean, string(filepath.Separator))
		if len(parts) < 2 || !skillNamePattern.MatchString(parts[0]) {
			continue
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return errors.New("技能包包含不支持的符号链接")
		}
		if file.UncompressedSize64 > maxSkillFile {
			return errors.New("技能文件超过安全大小限制")
		}
		destination := filepath.Join(stage, clean)
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(destination, 0700); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		contents, readErr := file.Open()
		if readErr != nil {
			return readErr
		}
		data, readErr := io.ReadAll(io.LimitReader(contents, maxSkillFile+1))
		contents.Close()
		if readErr != nil {
			return readErr
		}
		if int64(len(data)) > maxSkillFile {
			return errors.New("技能文件超过安全大小限制")
		}
		if err = os.WriteFile(destination, data, 0600); err != nil {
			return err
		}
		installedDirs[parts[0]] = true
	}
	for name := range installedDirs {
		if _, err = os.Stat(filepath.Join(stage, name, "SKILL.md")); err != nil {
			return fmt.Errorf("技能 %s 缺少 SKILL.md", name)
		}
		finalPath := filepath.Join(root, name)
		if _, statErr := os.Stat(finalPath); statErr == nil {
			return fmt.Errorf("技能 %s 已存在，请先删除旧版本", name)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
	}
	for name := range installedDirs {
		if err = os.Rename(filepath.Join(stage, name), filepath.Join(root, name)); err != nil {
			return fmt.Errorf("安装技能 %s 失败: %w", name, err)
		}
	}
	return nil
}

func hasZipPrefix(files []*zip.File, prefix string) bool {
	for _, file := range files {
		if strings.HasPrefix(file.Name, prefix) {
			return true
		}
	}
	return false
}

func isChildPath(root, target string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
