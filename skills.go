package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const maxSkillDownload = 64 << 20
const maxSkillFile = 10 << 20
const receiptFile = ".model-switcher-source.json"

var skillNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var commitPattern = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

type SkillMarket struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Prefix     string `json:"-"`
}

var skillMarkets = []SkillMarket{
	{"openai", "OpenAI Skills", "openai/skills", "skills/"},
	{"anthropic", "Anthropic Skills", "anthropics/skills", "skills/"},
	{"vercel", "Vercel Skills", "vercel-labs/agent-skills", "skills/"},
	{"openai-plugins", "OpenAI Plugins", "openai/plugins", "plugins/"},
	{"pi-skills", "pi-skills", "badlogic/pi-skills", ""},
}

type SkillInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Path        string `json:"path"`
	System      bool   `json:"system"`
	HasScripts  bool   `json:"hasScripts"`
	FileCount   int    `json:"fileCount"`
	SizeBytes   int64  `json:"sizeBytes"`
	ModifiedAt  string `json:"modifiedAt,omitempty"`
	Market      string `json:"market,omitempty"`
	Repository  string `json:"repository,omitempty"`
	Version     string `json:"version,omitempty"`
	Author      string `json:"author,omitempty"`
	License     string `json:"license,omitempty"`
	URL         string `json:"url,omitempty"`
	TrashID     string `json:"trashId,omitempty"`
	SourceID    string `json:"sourceId,omitempty"`
}
type SkillCatalogItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Directory   string `json:"directory"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Source      string `json:"source"`
	Market      string `json:"market"`
	Repository  string `json:"repository"`
	URL         string `json:"url"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	License     string `json:"license"`
	HasScripts  bool   `json:"hasScripts"`
	Installed   bool   `json:"installed"`
	Conflict    bool   `json:"conflict"`
	SkillPath   string `json:"skillPath"`
}
type SkillsState struct {
	Root    string             `json:"root"`
	Target  string             `json:"target"`
	Skills  []SkillInfo        `json:"skills"`
	Trash   []SkillInfo        `json:"trash"`
	Catalog []SkillCatalogItem `json:"catalog"`
	Markets []SkillMarket      `json:"markets"`
	Errors  []string           `json:"errors"`
}
type skillReceipt struct {
	ID         string `json:"id"`
	Market     string `json:"market"`
	Repository string `json:"repository"`
	Version    string `json:"version"`
	URL        string `json:"url"`
}
type skillMetadata struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	License     string         `yaml:"license"`
	Author      string         `yaml:"author"`
	Metadata    map[string]any `yaml:"metadata"`
}
type cachedCatalog struct {
	Items []SkillCatalogItem
	At    time.Time
}

var catalogCache = struct {
	sync.Mutex
	Entries map[string]cachedCatalog
}{Entries: map[string]cachedCatalog{}}

func skillRoot() string {
	if root := strings.TrimSpace(os.Getenv("CODEX_HOME")); root != "" {
		return filepath.Join(root, "skills")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "skills")
}
func targetSkillRoot() (string, string, error) {
	s, err := readStore()
	if err != nil {
		return "", "", err
	}
	target := selectedTarget(s)
	root, err := skillRootForTarget(target)
	return root, target, err
}

func skillRootForTarget(target string) (string, error) {
	if target == "pi" {
		return filepath.Join(piRoot(), "skills"), nil
	}
	if target == "shared" {
		return sharedSkillsRoot(), nil
	}
	if target != "chatgpt" {
		return "", errors.New("技能目标无效")
	}
	return skillRoot(), nil
}
func (a *App) ListSkills() ([]SkillInfo, error) {
	root, _, err := targetSkillRoot()
	if err != nil {
		return nil, err
	}
	return listSkills(root)
}
func (a *App) LoadSkillsState() (SkillsState, error) { return a.SearchSkills("", "openai") }
func (a *App) ListSkillCatalog() ([]SkillCatalogItem, error) {
	s, err := a.SearchSkills("", "openai")
	if err == nil && len(s.Errors) > 0 {
		err = errors.New(strings.Join(s.Errors, "; "))
	}
	return s.Catalog, err
}
func (a *App) LoadLocalSkillsState() (SkillsState, error) {
	root, target, err := targetSkillRoot()
	if err != nil {
		return SkillsState{}, err
	}
	skills, err := listSkills(root)
	if err != nil {
		return SkillsState{}, err
	}
	trash, err := listTrash(root)
	if err != nil {
		return SkillsState{}, err
	}
	return SkillsState{Root: root, Target: target, Skills: skills, Trash: trash, Catalog: []SkillCatalogItem{}, Markets: skillMarkets, Errors: []string{}}, nil
}
func (a *App) SearchSkills(query, marketID string) (SkillsState, error) {
	state, err := a.LoadLocalSkillsState()
	if err != nil {
		return SkillsState{}, err
	}
	skills := state.Skills
	markets := []SkillMarket{}
	for _, m := range skillMarkets {
		if marketID == "all" || m.ID == marketID {
			markets = append(markets, m)
		}
	}
	if len(markets) == 0 {
		return SkillsState{}, errors.New("不支持的技能市场")
	}
	type result struct {
		items  []SkillCatalogItem
		err    error
		market SkillMarket
	}
	results := make(chan result, len(markets))
	for _, m := range markets {
		go func(m SkillMarket) { items, err := fetchMarketCatalog(m); results <- result{items, err, m} }(m)
	}
	installed := map[string]SkillInfo{}
	for _, s := range skills {
		installed[s.Name] = s
	}
	query = strings.ToLower(strings.TrimSpace(query))
	for range markets {
		r := <-results
		if r.err != nil {
			state.Errors = append(state.Errors, r.market.Name+": "+r.err.Error())
			continue
		}
		for _, item := range r.items {
			if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Directory+" "+item.Description+" "+item.Author+" "+item.Source+" "+item.Category), query) {
				continue
			}
			if local, ok := installed[item.Directory]; ok {
				item.Installed = local.SourceID == item.ID
				item.Conflict = !item.Installed
			}
			state.Catalog = append(state.Catalog, item)
		}
	}
	sort.Slice(state.Catalog, func(i, j int) bool { return state.Catalog[i].ID < state.Catalog[j].ID })
	sort.Strings(state.Errors)
	return state, nil
}
func parseSkillMetadata(data []byte) skillMetadata {
	result := skillMetadata{}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end >= 0 {
			_ = yaml.Unmarshal([]byte(content[4:4+end]), &result)
		}
	}
	if result.Author == "" {
		if author, ok := result.Metadata["author"].(string); ok {
			result.Author = author
		}
	}
	if result.Description == "" {
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "# ") {
				result.Description = strings.TrimSpace(line[2:])
				break
			}
		}
	}
	return result
}
func skillDescription(content string) string { return parseSkillMetadata([]byte(content)).Description }
func listSkills(root string) ([]SkillInfo, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []SkillInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []SkillInfo{}
	for _, e := range entries {
		if !e.IsDir() || (strings.HasPrefix(e.Name(), ".") && e.Name() != ".system") {
			continue
		}
		info, err := inspectSkill(e.Name(), filepath.Join(root, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取技能 %s: %w", e.Name(), err)
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}
func inspectSkill(name, dir string) (SkillInfo, error) {
	info := SkillInfo{Name: name, Title: name, Path: dir, System: name == ".system"}
	var newest time.Time
	err := filepath.Walk(dir, func(filePath string, stat os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if stat.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if stat.IsDir() {
			if stat.Name() == "scripts" {
				info.HasScripts = true
			}
			return nil
		}
		if relative, err := filepath.Rel(dir, filePath); err == nil && isSkillScript(filepath.ToSlash(relative)) {
			info.HasScripts = true
		}
		info.FileCount++
		info.SizeBytes += stat.Size()
		if stat.ModTime().After(newest) {
			newest = stat.ModTime()
		}
		return nil
	})
	if err != nil {
		return SkillInfo{}, err
	}
	if !newest.IsZero() {
		info.ModifiedAt = newest.Format(time.RFC3339)
	}
	if stat, err := os.Lstat(filepath.Join(dir, "SKILL.md")); err == nil && stat.Mode().IsRegular() && stat.Size() <= maxSkillFile {
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			return SkillInfo{}, err
		}
		m := parseSkillMetadata(data)
		if m.Name != "" {
			info.Title = m.Name
		}
		info.Description = m.Description
		info.Author = m.Author
		info.License = m.License
	}
	if stat, err := os.Lstat(filepath.Join(dir, receiptFile)); err == nil && stat.Mode().IsRegular() && stat.Size() <= 64<<10 {
		data, _ := os.ReadFile(filepath.Join(dir, receiptFile))
		var r skillReceipt
		if json.Unmarshal(data, &r) == nil {
			info.SourceID = r.ID
			info.Market = r.Market
			info.Repository = r.Repository
			info.Version = r.Version
			info.URL = r.URL
		}
	}
	return info, nil
}
func listTrash(root string) ([]SkillInfo, error) {
	if err := validateTrashRoot(filepath.Join(root, ".trash")); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, ".trash"))
	if errors.Is(err, os.ErrNotExist) {
		return []SkillInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []SkillInfo{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := trashOriginalName(e.Name())
		if !skillNamePattern.MatchString(name) {
			continue
		}
		s, err := inspectSkill(name, filepath.Join(root, ".trash", e.Name()))
		if err != nil {
			return nil, err
		}
		s.TrashID = e.Name()
		items = append(items, s)
	}
	return items, nil
}
func (a *App) DeleteSkill(name string) ([]SkillInfo, error) { return a.CleanSkills([]string{name}) }
func (a *App) CleanSkills(names []string) ([]SkillInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	root, _, err := targetSkillRoot()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, name := range names {
		if !skillNamePattern.MatchString(name) || name == ".system" || seen[name] {
			return nil, errors.New("技能名称无效或受保护")
		}
		seen[name] = true
		stat, err := os.Lstat(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("不能清理符号链接或非目录技能")
		}
	}
	trash := filepath.Join(root, ".trash")
	if err := validateTrashRoot(trash); err != nil {
		return nil, err
	}
	if len(names) > 0 {
		if err = os.MkdirAll(trash, 0700); err != nil {
			return nil, err
		}
	}
	moved := map[string]string{}
	for _, name := range names {
		dest := filepath.Join(trash, name+"--"+time.Now().Format("20060102-150405.000000000"))
		if err = os.Rename(filepath.Join(root, name), dest); err != nil {
			var rollback []string
			for old, to := range moved {
				if e := os.Rename(to, filepath.Join(root, old)); e != nil {
					rollback = append(rollback, e.Error())
				}
			}
			return nil, fmt.Errorf("清理失败: %v；回滚: %s", err, strings.Join(rollback, "; "))
		}
		moved[name] = dest
	}
	return listSkills(root)
}
func (a *App) RestoreSkill(trashID string) ([]SkillInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	root, _, err := targetSkillRoot()
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(trashID, "/\\:") || strings.HasPrefix(trashID, ".") {
		return nil, errors.New("回收区标识无效")
	}
	if err := validateTrashRoot(filepath.Join(root, ".trash")); err != nil {
		return nil, err
	}
	name := trashOriginalName(trashID)
	if name == "" || !skillNamePattern.MatchString(name) {
		return nil, errors.New("回收区标识无效")
	}
	from := filepath.Join(root, ".trash", trashID)
	stat, err := os.Lstat(from)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("回收项目不是目录")
	}
	to := filepath.Join(root, name)
	if _, err = os.Lstat(to); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("同名技能已存在，不能覆盖恢复")
	}
	if err = os.Rename(from, to); err != nil {
		return nil, err
	}
	return listSkills(root)
}
func findMarket(id string) (SkillMarket, bool) {
	for _, m := range skillMarkets {
		if m.ID == id {
			return m, true
		}
	}
	return SkillMarket{}, false
}

type catalogFlight struct {
	Done  chan struct{}
	Items []SkillCatalogItem
	Err   error
}

var catalogFlights = struct {
	sync.Mutex
	Jobs map[string]*catalogFlight
}{Jobs: map[string]*catalogFlight{}}

func fetchMarketCatalog(m SkillMarket) ([]SkillCatalogItem, error) {
	catalogFlights.Lock()
	catalogCache.Lock()
	cached, ok := catalogCache.Entries[m.ID]
	catalogCache.Unlock()
	if ok && time.Since(cached.At) < 10*time.Minute {
		catalogFlights.Unlock()
		return append([]SkillCatalogItem(nil), cached.Items...), nil
	}
	if job, ok := catalogFlights.Jobs[m.ID]; ok {
		catalogFlights.Unlock()
		<-job.Done
		return append([]SkillCatalogItem(nil), job.Items...), job.Err
	}
	job := &catalogFlight{Done: make(chan struct{})}
	catalogFlights.Jobs[m.ID] = job
	catalogFlights.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	job.Items, job.Err = fetchMarketCatalogContext(ctx, m)
	catalogFlights.Lock()
	if job.Err == nil {
		catalogCache.Lock()
		catalogCache.Entries[m.ID] = cachedCatalog{job.Items, time.Now()}
		catalogCache.Unlock()
	}
	delete(catalogFlights.Jobs, m.ID)
	close(job.Done)
	catalogFlights.Unlock()
	return append([]SkillCatalogItem(nil), job.Items...), job.Err
}
func fetchMarketCatalogContext(ctx context.Context, m SkillMarket) ([]SkillCatalogItem, error) {
	data, err := fetchURLContext(ctx, "https://api.github.com/repos/"+m.Repository+"/git/ref/heads/main", 128<<10)
	if err != nil {
		return nil, err
	}
	var commit struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if json.Unmarshal(data, &commit) != nil || !commitPattern.MatchString(commit.Object.SHA) {
		return nil, errors.New("市场版本无效")
	}
	if m.ID == "anthropic" || m.ID == "openai-plugins" || m.ID == "pi-skills" {
		return fetchTreeCatalogContext(ctx, m, commit.Object.SHA)
	}
	archive, err := fetchURLContext(ctx, "https://codeload.github.com/"+m.Repository+"/zip/"+commit.Object.SHA, maxSkillDownload)
	if err != nil {
		return nil, err
	}
	return catalogFromArchive(m, commit.Object.SHA, archive)
}

// Some markets keep helpers alongside SKILL.md rather than in scripts/.
func isSkillScript(relative string) bool {
	if strings.HasPrefix(relative, "scripts/") {
		return true
	}
	switch strings.ToLower(path.Ext(relative)) {
	case ".js", ".mjs", ".cjs", ".ts", ".py", ".sh", ".bash", ".ps1":
		return true
	}
	return false
}

func catalogFromArchive(m SkillMarket, sha string, archive []byte) ([]SkillCatalogItem, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	if len(reader.File) > 50000 {
		return nil, errors.New("市场文件过多")
	}
	allowed := map[string]string{}
	if m.ID == "openai-plugins" {
		for _, f := range reader.File {
			if strings.HasSuffix(f.Name, "/.agents/plugins/marketplace.json") {
				data, err := readZipFile(f, 2<<20)
				if err != nil {
					return nil, err
				}
				var marketplace struct {
					Plugins []struct {
						Name     string `json:"name"`
						Category string `json:"category"`
						Policy   struct {
							Installation string   `json:"installation"`
							Products     []string `json:"products"`
						} `json:"policy"`
						Source struct {
							Source string `json:"source"`
							Path   string `json:"path"`
						} `json:"source"`
					} `json:"plugins"`
				}
				if err = json.Unmarshal(data, &marketplace); err != nil {
					return nil, err
				}
				for _, p := range marketplace.Plugins {
					codex := len(p.Policy.Products) == 0
					for _, product := range p.Policy.Products {
						if strings.EqualFold(product, "CODEX") {
							codex = true
						}
					}
					if codex && p.Policy.Installation == "AVAILABLE" && p.Source.Source == "local" && p.Source.Path == "./plugins/"+p.Name {
						allowed[p.Name] = p.Category
					}
				}
				break
			}
		}
	}
	items := []SkillCatalogItem{}
	for _, f := range reader.File {
		_, relative, found := strings.Cut(f.Name, "/")
		if !found || !strings.HasPrefix(relative, m.Prefix) || !strings.HasSuffix(relative, "/SKILL.md") {
			continue
		}
		dir := path.Dir(relative)
		parts := strings.Split(dir, "/")
		category := "Agent Skills"
		if m.ID == "openai-plugins" {
			if len(parts) < 4 || parts[2] != "skills" {
				continue
			}
			var ok bool
			category, ok = allowed[parts[1]]
			if !ok {
				continue
			}
		} else if m.ID == "openai" && !strings.HasPrefix(relative, "skills/.curated/") {
			continue
		}
		directory := path.Base(dir)
		if !skillNamePattern.MatchString(directory) || !safeArchivePath(dir) {
			continue
		}
		data, err := readZipFile(f, maxSkillFile)
		if err != nil {
			return nil, err
		}
		metadata := parseSkillMetadata(data)
		if skillNamePattern.MatchString(metadata.Name) {
			directory = metadata.Name
		}
		if metadata.Name == "" || metadata.Description == "" {
			continue
		}
		scripts := false
		prefix := strings.TrimSuffix(f.Name, "SKILL.md")
		for _, other := range reader.File {
			if strings.HasPrefix(other.Name, prefix) && isSkillScript(strings.TrimPrefix(other.Name, prefix)) {
				scripts = true
				break
			}
		}
		items = append(items, SkillCatalogItem{ID: m.ID + ":" + dir, Name: metadata.Name, Directory: directory, Description: metadata.Description, Category: category, Source: m.Name, Market: m.ID, Repository: m.Repository, URL: "https://github.com/" + m.Repository + "/tree/" + sha + "/" + dir, Version: sha, Author: metadata.Author, License: metadata.License, HasScripts: scripts, SkillPath: dir})
	}
	return items, nil
}
func (a *App) InstallSkill(id string) ([]SkillInfo, error) {
	s, err := readStore()
	if err != nil {
		return nil, err
	}
	return a.InstallSkillTo(id, selectedTarget(s))
}

func (a *App) InstallSkillTo(id, target string) ([]SkillInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if target == "active" {
		store, err := readStore()
		if err != nil {
			return nil, err
		}
		target = selectedTarget(store)
	}
	if target != "chatgpt" && target != "pi" && target != "shared" {
		return nil, errors.New("技能安装目标无效")
	}
	marketID, _, ok := strings.Cut(id, ":")
	m, found := findMarket(marketID)
	if !ok || !found {
		return nil, errors.New("技能必须来自支持的市场")
	}
	catalog, err := fetchMarketCatalog(m)
	if err != nil {
		return nil, err
	}
	var item *SkillCatalogItem
	for i := range catalog {
		if catalog[i].ID == id {
			item = &catalog[i]
			break
		}
	}
	if item == nil {
		return nil, errors.New("技能不在市场目录中")
	}
	root, err := skillRootForTarget(target)
	if err != nil {
		return nil, err
	}
	archive, err := downloadTreeSkill(*item)
	if err != nil {
		return nil, err
	}
	if err = installCatalogSkill(root, *item, archive); err != nil {
		return nil, err
	}
	return listSkills(root)
}
func safeArchivePath(relative string) bool {
	return relative != "" && !strings.ContainsAny(relative, "\\:\x00") && !strings.HasPrefix(relative, "/") && path.Clean(relative) == strings.TrimSuffix(relative, "/") && relative != ".." && !strings.HasPrefix(relative, "../")
}
func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	if file.Mode()&os.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
		return nil, errors.New("技能包包含不支持的链接或特殊文件")
	}
	if file.UncompressedSize64 > uint64(limit) {
		return nil, errors.New("技能文件超过安全大小限制")
	}
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("技能文件超过安全大小限制")
	}
	return data, nil
}
func installCatalogSkill(root string, item SkillCatalogItem, archive []byte) error {
	if !skillNamePattern.MatchString(item.Directory) || !safeArchivePath(item.SkillPath) {
		return errors.New("技能路径无效")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	final := filepath.Join(root, item.Directory)
	if _, err := os.Lstat(final); !errors.Is(err, os.ErrNotExist) {
		return errors.New("同名技能已存在，不能覆盖安装")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return err
	}
	if len(reader.File) > 50000 {
		return errors.New("技能包文件过多")
	}
	stage, err := os.MkdirTemp(root, ".skill-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	var total int64
	count := 0
	seen := map[string]bool{}
	for _, f := range reader.File {
		_, relative, ok := strings.Cut(f.Name, "/")
		if !ok || !strings.HasPrefix(relative, item.SkillPath+"/") {
			continue
		}
		rel := strings.TrimPrefix(relative, item.SkillPath+"/")
		if rel == "" {
			continue
		}
		if !safeArchivePath(rel) {
			return errors.New("技能包包含非法路径")
		}
		dest := filepath.Join(stage, filepath.FromSlash(rel))
		if !isChildPath(stage, dest) {
			return errors.New("技能包路径越界")
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("技能包包含符号链接")
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return err
			}
			continue
		}
		key := strings.ToLower(path.Clean(rel))
		if seen[key] || key == receiptFile {
			return errors.New("技能包包含重复或保留文件")
		}
		seen[key] = true
		count++
		if f.UncompressedSize64 > maxSkillFile {
			return errors.New("技能文件超过安全大小限制")
		}
		total += int64(f.UncompressedSize64)
		if count > 10000 || total > maxSkillDownload {
			return errors.New("技能解压内容超过安全限制")
		}
		data, err := readZipFile(f, maxSkillFile)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if f.Mode()&0111 != 0 {
			mode = 0700
		}
		if err = os.WriteFile(dest, data, mode); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(filepath.Join(stage, "SKILL.md"))
	if err != nil {
		return errors.New("技能缺少 SKILL.md")
	}
	metadata := parseSkillMetadata(data)
	if metadata.Name == "" || metadata.Description == "" {
		return errors.New("SKILL.md 缺少名称或描述")
	}
	receipt, _ := json.MarshalIndent(skillReceipt{item.ID, item.Market, item.Repository, item.Version, item.URL}, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, receiptFile), receipt, 0600); err != nil {
		return err
	}
	if _, err = os.Lstat(final); !errors.Is(err, os.ErrNotExist) {
		return errors.New("同名技能已存在")
	}
	return os.Rename(stage, final)
}
func fetchURL(rawURL string, limit int64) ([]byte, error) {
	return fetchURLContext(context.Background(), rawURL, limit)
}
func fetchURLContext(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ModelSwitcher")
	timeout := 45 * time.Second
	if strings.HasPrefix(rawURL, "https://codeload.github.com/") {
		timeout = 90 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d（网络或 GitHub 请求额度限制）", resp.StatusCode)
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

func validateTrashRoot(dir string) error {
	stat, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		return errors.New("技能回收区不能是符号链接或文件")
	}
	return nil
}

var legacyTrashPattern = regexp.MustCompile(`^(.*)-[0-9]{8}-[0-9]{6}\.[0-9]{9}$`)

func trashOriginalName(id string) string {
	if i := strings.LastIndex(id, "--"); i > 0 {
		return id[:i]
	}
	if match := legacyTrashPattern.FindStringSubmatch(id); len(match) == 2 {
		return match[1]
	}
	return ""
}
