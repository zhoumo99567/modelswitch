package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiJSONC(t *testing.T) {
	value, err := parseJSONObject([]byte(`{ // comment
 "url":"https://example.com/a,}", /* block */ "quoted":"a\"b", "array":[1,2,], "nested":{"ok":true,},
 }`))
	if err != nil {
		t.Fatal(err)
	}
	if rawString(value, "url") != "https://example.com/a,}" || rawString(value, "quoted") != `a"b` {
		t.Fatalf("strings: %v", value)
	}
	for _, text := range []string{"null", "[]", "/* unclosed", `{"x":1 "y":2}`} {
		if _, err := parseJSONObject([]byte(text)); err == nil {
			t.Fatalf("accepted %s", text)
		}
	}
}
func TestPiProfileObjectsPreserveSettingsAndCredentialCommand(t *testing.T) {
	models, err := parseJSONObject([]byte(`{"providers":{"custom":{"apiKey":"KEEP"}},"extra":true}`))
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := parseJSONObject([]byte(`{"theme":"dark","defaultProvider":"custom","extensions":["./a.ts"]}`))
	p := storedProfile{ProfileView: ProfileView{ID: "local-123", BaseURL: "http://localhost:1234/v1", SelectedModel: "model-a", Models: []Model{{ID: "model-b"}, {ID: "model-b"}}}, Secret: "encrypted"}
	providers, err := piProfileObjects(models, settings, p, "/tmp/App's Folder/model-switcher")
	if err != nil {
		t.Fatal(err)
	}
	var provider map[string]any
	_ = json.Unmarshal(providers[providerID], &provider)
	if provider["api"] != "openai-completions" || !strings.HasPrefix(provider["apiKey"].(string), "!") || strings.Contains(provider["apiKey"].(string), "encrypted") {
		t.Fatalf("provider: %v", provider)
	}
	if len(provider["models"].([]any)) != 2 || rawString(settings, "theme") != "dark" || rawString(settings, "defaultProvider") != providerID || len(providers["custom"]) == 0 {
		t.Fatal("lost settings or models")
	}
	entries := provider["models"].([]any)
	if entries[0].(map[string]any)["contextWindow"] != float64(defaultModelContextWindow) {
		t.Fatalf("default context window: %#v", entries[0])
	}
}
func TestPiActivateRestoreAndTargetIsolation(t *testing.T) {
	isolateSkillStore(t)
	root := piRoot()
	_ = os.MkdirAll(root, 0700)
	models := []byte(`{"providers":{"custom":{"api":"openai-completions","apiKey":"KEEP"}},"extra":true}`)
	settings := []byte(`{"defaultProvider":"custom","defaultModel":"before","theme":"dark","skills":["./elsewhere"]}`)
	_ = os.WriteFile(filepath.Join(root, "models.json"), models, 0600)
	_ = os.WriteFile(filepath.Join(root, "settings.json"), settings, 0600)
	_ = os.MkdirAll(filepath.Dir(configPath()), 0700)
	codex := []byte("model = \"codex-before\"\n")
	_ = os.WriteFile(configPath(), codex, 0600)
	p := storedProfile{ProfileView: ProfileView{ID: "local-pi", Name: "local", BaseURL: "http://localhost:1234/v1", SelectedModel: "after"}}
	store := storeFile{Target: "pi", Profiles: []storedProfile{p}}
	if err := writeStore(store); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	state, err := app.ActivateProfile(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Target != "pi" || state.ActiveProfileID != p.ID || state.ActiveModel != "after" || !state.CanRestore {
		t.Fatalf("state: %+v", state)
	}
	actualCodex, _ := os.ReadFile(configPath())
	if string(actualCodex) != string(codex) {
		t.Fatal("changed Codex config")
	}
	if root, target, err := targetSkillRoot(); err != nil || target != "pi" || root != filepath.Join(piRoot(), "skills") {
		t.Fatal("wrong skill target")
	}
	// Edits unrelated to model switching must survive restore.
	obj, _, _ := readPiObject(filepath.Join(root, "settings.json"))
	obj["theme"] = toRaw("light")
	data, _ := json.Marshal(obj)
	_ = os.WriteFile(filepath.Join(root, "settings.json"), data, 0600)
	state, err = app.ActivateOpenAI()
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveProvider != "custom" || state.ActiveModel != "before" || state.CanRestore {
		t.Fatalf("restored state: %+v", state)
	}
	restored, _, err := readPiObject(filepath.Join(root, "models.json"))
	original, _ := parseJSONObject(models)
	var restoredSemantic, originalSemantic any
	_ = json.Unmarshal(toRaw(restored), &restoredSemantic)
	_ = json.Unmarshal(toRaw(original), &originalSemantic)
	if err != nil || string(toRaw(restoredSemantic)) != string(toRaw(originalSemantic)) {
		t.Fatalf("provider not restored: %s", toRaw(restored))
	}
	restoredSettings, _, _ := readPiObject(filepath.Join(root, "settings.json"))
	if rawString(restoredSettings, "theme") != "light" || len(restoredSettings["skills"]) == 0 {
		t.Fatal("lost unrelated edit")
	}
	backups, err := os.ReadDir(filepath.Join(root, ".model-switcher-backups"))
	if err != nil || len(backups) < 2 {
		t.Fatal("missing backups")
	}
	state, err = app.SetTarget("chatgpt")
	if err != nil {
		t.Fatal(err)
	}
	if state.Target != "chatgpt" || state.ActiveModel != "codex-before" {
		t.Fatal("wrong ChatGPT state")
	}
	if _, err = app.SetTarget("bad"); err == nil {
		t.Fatal("invalid target")
	}
}

func TestPiConfigEditorRepairsInvalidJSONAndBacksUp(t *testing.T) {
	isolateSkillStore(t)
	root := piRoot()
	_ = os.MkdirAll(root, 0700)
	path := filepath.Join(root, "models.json")
	original := []byte("{ broken")
	_ = os.WriteFile(path, original, 0600)
	if err := writePiConfigText(`{"providers":{}}`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readPiObject(path); err != nil {
		t.Fatal(err)
	}
	backups, err := os.ReadDir(filepath.Join(root, ".model-switcher-backups"))
	if err != nil || len(backups) != 1 {
		t.Fatal("missing backup")
	}
	saved, err := os.ReadFile(filepath.Join(root, ".model-switcher-backups", backups[0].Name(), "models.json"))
	if err != nil || string(saved) != string(original) {
		t.Fatal("original not backed up")
	}
	if err := writePiConfigText(`{"providers":`); err == nil {
		t.Fatal("invalid edit accepted")
	}
}

func TestPiImageInputIsSavedPerModelAndAppliedAgain(t *testing.T) {
	isolateSkillStore(t)
	p := storedProfile{ProfileView: ProfileView{ID: "local-vision", Name: "Vision", BaseURL: "http://localhost:1234/v1", SelectedModel: "vision", Models: []Model{{ID: "vision", SupportsImages: true}, {ID: "text-only"}}}}
	store := storeFile{Target: "pi", Profiles: []storedProfile{p}}
	if err := writeStore(store); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	for i := 0; i < 2; i++ {
		if _, err := app.ActivateProfile(p.ID); err != nil {
			t.Fatal(err)
		}
	}
	models, _, err := readPiObject(filepath.Join(piRoot(), "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var providers map[string]struct {
		Models []struct {
			ID    string   `json:"id"`
			Input []string `json:"input"`
		} `json:"models"`
	}
	if err = json.Unmarshal(models["providers"], &providers); err != nil {
		t.Fatal(err)
	}
	entries := providers[providerID].Models
	if len(entries) != 2 || entries[0].ID != "vision" || strings.Join(entries[0].Input, ",") != "text,image" || strings.Join(entries[1].Input, ",") != "text" {
		t.Fatalf("inputs: %+v", entries)
	}
	state, err := app.LoadState()
	if err != nil || !state.Profiles[0].Models[0].SupportsImages || state.Profiles[0].Models[1].SupportsImages {
		t.Fatal("image capability not retained")
	}
	p.Models[0].SupportsImages = false
	store, err = readStore()
	if err != nil {
		t.Fatal(err)
	}
	store.Profiles[0] = p
	if err = writeStore(store); err != nil {
		t.Fatal(err)
	}
	if _, err = app.ActivateProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	models, _, _ = readPiObject(filepath.Join(piRoot(), "models.json"))
	_ = json.Unmarshal(models["providers"], &providers)
	if strings.Join(providers[providerID].Models[0].Input, ",") != "text" {
		t.Fatal("cannot disable image input")
	}
}
