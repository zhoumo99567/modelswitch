package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func skillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const testSkillDoc = "---\nname: demo\ndescription: >-\n  A multiline skill\n  description.\nlicense: MIT\nmetadata:\n  author: Team\n---\n# Demo\n"

func testCatalogItem() SkillCatalogItem {
	return SkillCatalogItem{ID: "vercel:skills/demo", Name: "demo", Directory: "demo", Market: "vercel", Repository: "vercel-labs/agent-skills", Version: strings.Repeat("a", 40), SkillPath: "skills/demo", URL: "https://github.com/vercel-labs/agent-skills"}
}
func isolateSkillStore(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "pi"))
	_ = os.Remove(dataPath())
	t.Cleanup(func() { _ = os.Remove(dataPath()) })
	return home
}
func TestCatalogMetadataAndCanonicalDirectory(t *testing.T) {
	m, _ := findMarket("vercel")
	data := skillArchive(t, map[string]string{"repo/skills/folder/SKILL.md": testSkillDoc, "repo/skills/folder/scripts/run.sh": "#!/bin/sh\n"})
	items, err := catalogFromArchive(m, strings.Repeat("a", 40), data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items: %v", items)
	}
	i := items[0]
	if i.Directory != "demo" || i.Description != "A multiline skill description." || i.Author != "Team" || i.License != "MIT" || !i.HasScripts || i.SkillPath != "skills/folder" {
		t.Fatalf("metadata: %+v", i)
	}
}
func TestInstallSkillProvenanceAndNoOverwrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	item := testCatalogItem()
	archive := skillArchive(t, map[string]string{"repo/skills/demo/SKILL.md": testSkillDoc, "repo/skills/demo/scripts/a.sh": "echo hello"})
	if err := installCatalogSkill(root, item, archive); err != nil {
		t.Fatal(err)
	}
	skills, err := listSkills(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].SourceID != item.ID || skills[0].Version != item.Version || !skills[0].HasScripts {
		t.Fatalf("skills: %+v", skills)
	}
	if err := installCatalogSkill(root, item, archive); err == nil {
		t.Fatal("overwrote existing skill")
	}
	if _, err := os.Stat(filepath.Join(root, "demo", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
func TestInstallRejectsUnsafeArchives(t *testing.T) {
	for _, rel := range []string{"../escape", "/absolute", `foo\..\escape`, "C:/escape", "scripts/../../escape", receiptFile} {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{"repo/skills/demo/SKILL.md": testSkillDoc, "repo/skills/demo/" + rel: "bad"}
			if err := installCatalogSkill(root, testCatalogItem(), skillArchive(t, files)); err == nil {
				t.Fatal("accepted unsafe path")
			}
			if _, err := os.Stat(filepath.Join(root, "demo")); !os.IsNotExist(err) {
				t.Fatal("partial install")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatal("staging not cleaned")
			}
		})
	}
	t.Run("missing metadata", func(t *testing.T) {
		err := installCatalogSkill(t.TempDir(), testCatalogItem(), skillArchive(t, map[string]string{"repo/skills/demo/SKILL.md": "# no frontmatter"}))
		if err == nil {
			t.Fatal("missing name accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		h := &zip.FileHeader{Name: "repo/skills/demo/SKILL.md"}
		h.SetMode(os.ModeSymlink | 0777)
		f, _ := w.CreateHeader(h)
		_, _ = f.Write([]byte("/etc/passwd"))
		_ = w.Close()
		if err := installCatalogSkill(t.TempDir(), testCatalogItem(), buf.Bytes()); err == nil {
			t.Fatal("accepted symlink")
		}
	})
	t.Run("oversized file", func(t *testing.T) {
		f := &zip.File{FileHeader: zip.FileHeader{UncompressedSize64: maxSkillFile + 1}}
		if _, err := readZipFile(f, maxSkillFile); err == nil {
			t.Fatal("accepted oversized file")
		}
	})
}
func TestSearchProvenanceAndCollision(t *testing.T) {
	isolateSkillStore(t)
	root := skillRoot()
	if err := os.MkdirAll(filepath.Join(root, "demo"), 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "demo", "SKILL.md"), []byte(testSkillDoc), 0600)
	catalogCache.Lock()
	previous := catalogCache.Entries
	catalogCache.Entries = map[string]cachedCatalog{}
	for _, m := range skillMarkets {
		catalogCache.Entries[m.ID] = cachedCatalog{Items: []SkillCatalogItem{}, At: time.Now()}
	}
	catalogCache.Entries["vercel"] = cachedCatalog{Items: []SkillCatalogItem{testCatalogItem()}, At: time.Now()}
	catalogCache.Unlock()
	t.Cleanup(func() { catalogCache.Lock(); catalogCache.Entries = previous; catalogCache.Unlock() })
	state, err := NewApp().SearchSkills("demo", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Catalog) != 1 || !state.Catalog[0].Conflict || state.Catalog[0].Installed {
		t.Fatalf("collision: %+v", state.Catalog)
	}
	receipt, _ := json.Marshal(skillReceipt{ID: testCatalogItem().ID})
	_ = os.WriteFile(filepath.Join(root, "demo", receiptFile), receipt, 0600)
	state, err = NewApp().SearchSkills("demo", "vercel")
	if err != nil || !state.Catalog[0].Installed || state.Catalog[0].Conflict {
		t.Fatalf("installed: %+v, %v", state, err)
	}
	if _, err = NewApp().SearchSkills("", "unknown"); err == nil {
		t.Fatal("accepted unknown market")
	}
}
func TestCleanAndRestoreSkills(t *testing.T) {
	isolateSkillStore(t)
	root := skillRoot()
	for _, name := range []string{"one", "two", ".system"} {
		_ = os.MkdirAll(filepath.Join(root, name), 0700)
		_ = os.WriteFile(filepath.Join(root, name, "SKILL.md"), []byte(testSkillDoc), 0600)
	}
	app := NewApp()
	if _, err := app.CleanSkills([]string{"one", ".system"}); err == nil {
		t.Fatal("cleaned protected skill")
	}
	if _, err := os.Stat(filepath.Join(root, "one")); err != nil {
		t.Fatal("partial clean before validation")
	}
	if _, err := app.CleanSkills([]string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	trash, err := listTrash(root)
	if err != nil || len(trash) != 2 {
		t.Fatalf("trash %v: %v", trash, err)
	}
	if _, err = app.RestoreSkill(trash[0].TrashID); err != nil {
		t.Fatal(err)
	}
	if _, err = app.RestoreSkill("../escape"); err == nil {
		t.Fatal("accepted traversal")
	}
	_ = os.MkdirAll(filepath.Join(root, trash[1].Name), 0700)
	if _, err = app.RestoreSkill(trash[1].TrashID); err == nil {
		t.Fatal("overwrote existing skill on restore")
	}
}
func TestRejectLinkedTrash(t *testing.T) {
	isolateSkillStore(t)
	root := skillRoot()
	_ = os.MkdirAll(filepath.Join(root, "demo"), 0700)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".trash")); err != nil {
		t.Skip(err)
	}
	if _, err := NewApp().DeleteSkill("demo"); err == nil {
		t.Fatal("used symlinked trash")
	}
	if _, err := listTrash(root); err == nil {
		t.Fatal("read symlinked trash")
	}
}
func TestLiveSkillMarkets(t *testing.T) {
	if os.Getenv("MODELSWITCHER_LIVE_TEST") != "1" {
		t.Skip("optional network integration")
	}
	for _, m := range skillMarkets {
		t.Run(m.ID, func(t *testing.T) {
			items, err := fetchMarketCatalog(m)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) == 0 {
				t.Fatal("empty market")
			}
			t.Logf("%s: %d skills; sample %s", m.Name, len(items), items[0].Name)
			if os.Getenv("MODELSWITCHER_LIVE_INSTALL_TEST") == "1" {
				item := items[0]
				archive, err := downloadTreeSkill(item)
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				if err = installCatalogSkill(root, item, archive); err != nil {
					t.Fatal(err)
				}
				installed, err := listSkills(root)
				if err != nil || len(installed) != 1 || installed[0].SourceID != item.ID {
					t.Fatalf("live install failed: %+v, %v", installed, err)
				}
				t.Logf("isolated install: %s (%d files)", installed[0].Name, installed[0].FileCount)
			}

		})
	}
}

type skillTestTransport func(*http.Request) (*http.Response, error)

func (f skillTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPartialMarketFailure(t *testing.T) {
	isolateSkillStore(t)
	catalogCache.Lock()
	previous := catalogCache.Entries
	catalogCache.Entries = map[string]cachedCatalog{}
	for _, m := range skillMarkets {
		if m.ID != "anthropic" {
			catalogCache.Entries[m.ID] = cachedCatalog{Items: []SkillCatalogItem{}, At: time.Now()}
		}
	}
	catalogCache.Entries["vercel"] = cachedCatalog{Items: []SkillCatalogItem{testCatalogItem()}, At: time.Now()}
	catalogCache.Unlock()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = skillTestTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		catalogCache.Lock()
		catalogCache.Entries = previous
		catalogCache.Unlock()
	})
	state, err := NewApp().SearchSkills("", "all")
	if err != nil || len(state.Errors) != 1 || !strings.Contains(state.Errors[0], "Anthropic Skills") || len(state.Catalog) != 1 {
		t.Fatalf("partial results: %+v; %v", state, err)
	}
}
func TestLegacyTrashName(t *testing.T) {
	if name := trashOriginalName("example-skill-20260930-123456.123456789"); name != "example-skill" {
		t.Fatal(name)
	}
	if name := trashOriginalName("example--20260930-123456.123456789"); name != "example" {
		t.Fatal(name)
	}
}

func TestPluginCatalogEnforcesCodexPolicy(t *testing.T) {
	m, _ := findMarket("openai-plugins")
	manifest := `{"plugins":[{"name":"allowed","category":"tools","source":{"source":"local","path":"./plugins/allowed"},"policy":{"installation":"AVAILABLE","products":["CODEX"]}},{"name":"denied","source":{"source":"local","path":"./plugins/denied"},"policy":{"installation":"AVAILABLE","products":["CHATGPT"]}}]}`
	archive := skillArchive(t, map[string]string{"repo/.agents/plugins/marketplace.json": manifest, "repo/plugins/allowed/skills/demo/SKILL.md": testSkillDoc, "repo/plugins/denied/skills/other/SKILL.md": strings.Replace(testSkillDoc, "name: demo", "name: other", 1)})
	items, err := catalogFromArchive(m, strings.Repeat("a", 40), archive)
	if err != nil || len(items) != 1 || items[0].SkillPath != "plugins/allowed/skills/demo" {
		t.Fatalf("policy: %+v; %v", items, err)
	}
}

func TestLocalSkillsStateOfflineAndTargetIsolation(t *testing.T) {
	isolateSkillStore(t)
	oldTransport := http.DefaultTransport
	requests := 0
	http.DefaultTransport = skillTestTransport(func(*http.Request) (*http.Response, error) { requests++; return nil, errors.New("offline") })
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	app := NewApp()
	for _, target := range []string{"chatgpt", "pi"} {
		if err := writeStore(storeFile{Target: target}); err != nil {
			t.Fatal(err)
		}
		root, gotTarget, err := targetSkillRoot()
		if err != nil || gotTarget != target {
			t.Fatalf("target: %s, %v", gotTarget, err)
		}
		for _, name := range []string{target + "-installed", target + "-removed"} {
			dir := filepath.Join(root, name)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(testSkillDoc), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := app.CleanSkills([]string{target + "-removed"}); err != nil {
			t.Fatal(err)
		}
		state, err := app.LoadLocalSkillsState()
		if err != nil {
			t.Fatal(err)
		}
		if state.Target != target || state.Root != root || len(state.Skills) != 1 || state.Skills[0].Name != target+"-installed" || len(state.Trash) != 1 || state.Trash[0].Name != target+"-removed" || len(state.Catalog) != 0 || len(state.Errors) != 0 {
			t.Fatalf("local state: %+v", state)
		}
	}
	if requests != 0 {
		t.Fatalf("local skills required network: %d requests", requests)
	}
}

func TestPiSkillsRootCatalogAndInstall(t *testing.T) {
	market, ok := findMarket("pi-skills")
	if !ok || market.Repository != "badlogic/pi-skills" {
		t.Fatal("pi-skills market not configured")
	}
	archive := skillArchive(t, map[string]string{
		"repo/brave-search/SKILL.md":     testSkillDoc,
		"repo/brave-search/search.js":    "// helper script",
		"repo/brave-search/package.json": `{"name":"brave-search"}`,
		"repo/README.md":                 "pi-skills repository",
	})
	items, err := catalogFromArchive(market, strings.Repeat("a", 40), archive)
	if err != nil || len(items) != 1 {
		t.Fatalf("root catalog: %+v, %v", items, err)
	}
	item := items[0]
	if item.ID != "pi-skills:brave-search" || item.SkillPath != "brave-search" || !item.HasScripts {
		t.Fatalf("metadata: %+v", item)
	}
	root := filepath.Join(t.TempDir(), "skills")
	if err := installCatalogSkill(root, item, archive); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, item.Directory, "search.js")); err != nil {
		t.Fatal("root helper missing:", err)
	}
	if _, err := os.Stat(filepath.Join(root, item.Directory, "README.md")); !os.IsNotExist(err) {
		t.Fatal("repository root file was installed")
	}
	installed, err := listSkills(root)
	if err != nil || len(installed) != 1 || installed[0].SourceID != item.ID || !installed[0].HasScripts {
		t.Fatalf("provenance: %+v, %v", installed, err)
	}
}
