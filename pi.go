package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type piBaseline struct {
	Root            string          `json:"root"`
	ProfileID       string          `json:"profileId"`
	Provider        json.RawMessage `json:"provider,omitempty"`
	DefaultProvider json.RawMessage `json:"defaultProvider,omitempty"`
	DefaultModel    json.RawMessage `json:"defaultModel,omitempty"`
}

func selectedTarget(s storeFile) string {
	if s.Target == "pi" {
		return "pi"
	}
	return "chatgpt"
}
func piRoot() string {
	root := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR"))
	home, _ := os.UserHomeDir()
	if root == "" {
		return filepath.Join(home, ".pi", "agent")
	}
	if root == "~" {
		return home
	}
	if strings.HasPrefix(root, "~/") || strings.HasPrefix(root, `~\`) {
		return filepath.Join(home, root[2:])
	}
	return root
}
func activeConfigPath() string {
	s, _ := readStore()
	if selectedTarget(s) == "pi" {
		return filepath.Join(piRoot(), "models.json")
	}
	return configPath()
}
func (a *App) SetTarget(target string) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if target != "chatgpt" && target != "pi" {
		return AppState{}, errors.New("不支持的应用")
	}
	s, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	previous := s.Target
	s.Target = target
	if err = writeStore(s); err != nil {
		return AppState{}, err
	}
	state, err := a.loadState()
	if err != nil {
		s.Target = previous
		if rollback := writeStore(s); rollback != nil {
			return AppState{}, fmt.Errorf("读取应用配置失败: %v；应用选择恢复失败: %v", err, rollback)
		}
		return AppState{}, err
	}
	return state, nil
}

// Strip JSONC comments and trailing commas while preserving strings and offsets.
func parseJSONObject(data []byte) (map[string]json.RawMessage, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	clean := append([]byte(nil), data...)
	quoted, escaped := false, false
	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == '/' && i+1 < len(clean) && clean[i+1] == '/' {
			for i < len(clean) && clean[i] != '\n' {
				clean[i] = ' '
				i++
			}
			i--
			continue
		}
		if c == '/' && i+1 < len(clean) && clean[i+1] == '*' {
			clean[i] = ' '
			clean[i+1] = ' '
			i += 2
			closed := false
			for i < len(clean) {
				if clean[i] == '*' && i+1 < len(clean) && clean[i+1] == '/' {
					clean[i] = ' '
					clean[i+1] = ' '
					i++
					closed = true
					break
				}
				if clean[i] != '\n' && clean[i] != '\r' {
					clean[i] = ' '
				}
				i++
			}
			if !closed {
				return nil, errors.New("JSONC 注释未闭合")
			}
		}
	}
	quoted = false
	escaped = false
	for i, c := range clean {
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
		}
		if c == ',' {
			j := i + 1
			for j < len(clean) && strings.ContainsRune(" \t\r\n", rune(clean[j])) {
				j++
			}
			if j < len(clean) && (clean[j] == '}' || clean[j] == ']') {
				clean[i] = ' '
			}
		}
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(clean, &value); err != nil {
		return nil, fmt.Errorf("pi 配置不是有效 JSON/JSONC: %w", err)
	}
	if value == nil {
		return nil, errors.New("pi 配置必须是 JSON 对象")
	}
	return value, nil
}
func readPiObject(path string) (map[string]json.RawMessage, []byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte("{}")
		err = nil
	}
	if err != nil {
		return nil, nil, err
	}
	obj, err := parseJSONObject(data)
	return obj, data, err
}
func rawString(obj map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(obj[key], &value)
	return value
}
func toRaw(v any) json.RawMessage { data, _ := json.Marshal(v); return data }
func (a *App) loadPiState(store storeFile) (AppState, error) {
	settings, _, err := readPiObject(filepath.Join(piRoot(), "settings.json"))
	if err != nil {
		return AppState{}, err
	}
	provider := rawString(settings, "defaultProvider")
	if provider == "" {
		provider = "automatic"
	}
	state := AppState{Target: "pi", Version: AppVersion, Profiles: []ProfileView{}, ActiveProvider: provider, ActiveModel: rawString(settings, "defaultModel"), ConfigPath: filepath.Join(piRoot(), "models.json"), ProfilePath: profileStorePath(), CanRestore: store.PiBaseline != nil}
	models, _, err := readPiObject(state.ConfigPath)
	if err != nil {
		return AppState{}, err
	}
	providers := map[string]json.RawMessage{}
	if len(models["providers"]) > 0 {
		if err = json.Unmarshal(models["providers"], &providers); err != nil || providers == nil {
			return AppState{}, errors.New("pi providers 必须是对象")
		}
	}
	var managed struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(providers[providerID], &managed)
	for _, p := range store.Profiles {
		v := p.ProfileView
		v.APIKey = storedAPIKey(p)
		v.HasAPIKey = v.APIKey != "" || p.Secret != ""
		if v.Models == nil {
			v.Models = []Model{}
		}
		v.Models = normalizeModels(v.Models)
		state.Profiles = append(state.Profiles, v)
		if provider == providerID && managed.Name == "ModelSwitcher / "+p.ID {
			state.ActiveProfileID = p.ID
		}
	}
	return state, nil
}
func shellTokenCommand(executable, id string, profileFile ...string) (string, error) {
	if !skillNamePattern.MatchString(id) {
		return "", errors.New("配置标识不合法")
	}
	if runtime.GOOS == "windows" {
		if strings.ContainsAny(executable, "\"%\r\n") {
			return "", errors.New("程序路径不支持凭据命令")
		}
		command := "!\"" + executable + "\" --model-switcher-token " + id
		if len(profileFile) > 0 {
			if strings.ContainsAny(profileFile[0], "\"%\r\n") {
				return "", errors.New("配置文件路径不支持凭据命令")
			}
			command += " --profiles \"" + profileFile[0] + "\""
		}
		return command, nil
	}
	command := "!'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "' --model-switcher-token '" + id + "'"
	if len(profileFile) > 0 {
		command += " --profiles '" + strings.ReplaceAll(profileFile[0], "'", "'\"'\"'") + "'"
	}
	return command, nil
}
func piProfileObjects(models, settings map[string]json.RawMessage, p storedProfile, helper string, profileFile ...string) (map[string]json.RawMessage, error) {
	providers := map[string]json.RawMessage{}
	if len(models["providers"]) > 0 {
		if err := json.Unmarshal(models["providers"], &providers); err != nil || providers == nil {
			return nil, errors.New("pi providers 必须是对象")
		}
	}
	key := "model-switcher-local"
	if p.Secret != "" || p.APIKey != "" {
		var err error
		key, err = shellTokenCommand(helper, p.ID, profileFile...)
		if err != nil {
			return nil, err
		}
	}
	entries := []map[string]any{}
	seen := map[string]bool{}
	for _, m := range append(p.Models, Model{ID: p.SelectedModel}) {
		if m.ID != "" && !seen[m.ID] {
			input := []string{"text"}
			if m.SupportsImages {
				input = append(input, "image")
			}
			entries = append(entries, map[string]any{"id": m.ID, "name": m.ID, "reasoning": false, "input": input, "contextWindow": normalizedModelContextWindow(m.ContextWindow), "maxTokens": 4096, "cost": map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}})
			seen[m.ID] = true
		}
	}
	providers[providerID] = toRaw(map[string]any{"name": "ModelSwitcher / " + p.ID, "baseUrl": p.BaseURL, "api": "openai-completions", "apiKey": key, "models": entries})
	models["providers"] = toRaw(providers)
	settings["defaultProvider"] = toRaw(providerID)
	settings["defaultModel"] = toRaw(p.SelectedModel)
	return providers, nil
}
func commitPiObjects(root string, models, settings map[string]json.RawMessage, oldModels, oldSettings []byte) error {
	backup := filepath.Join(root, ".model-switcher-backups", time.Now().Format("20060102-150405.000000000"))
	if err := atomicWrite(filepath.Join(backup, "models.json"), oldModels); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(backup, "settings.json"), oldSettings); err != nil {
		return err
	}
	nextModels, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}
	nextSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(root, "models.json"), nextModels); err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(root, "settings.json"), nextSettings); err != nil {
		rollback := atomicWrite(filepath.Join(root, "models.json"), oldModels)
		if rollback != nil {
			return fmt.Errorf("设置写入失败: %v；模型回滚失败: %v；备份: %s", err, rollback, backup)
		}
		return err
	}
	return nil
}
func (a *App) activatePiProfile(store storeFile, p storedProfile) (AppState, error) {
	root := piRoot()
	models, oldModels, err := readPiObject(filepath.Join(root, "models.json"))
	if err != nil {
		return AppState{}, err
	}
	settings, oldSettings, err := readPiObject(filepath.Join(root, "settings.json"))
	if err != nil {
		return AppState{}, err
	}
	if store.PiBaseline != nil && store.PiBaseline.Root != root {
		return AppState{}, errors.New("pi 配置目录已变化，请先恢复原目录")
	}
	if store.PiBaseline == nil {
		providers := map[string]json.RawMessage{}
		if len(models["providers"]) > 0 {
			if err = json.Unmarshal(models["providers"], &providers); err != nil || providers == nil {
				return AppState{}, errors.New("pi providers 必须是对象")
			}
		}
		store.PiBaseline = &piBaseline{Root: root, Provider: providers[providerID], DefaultProvider: settings["defaultProvider"], DefaultModel: settings["defaultModel"]}
	}
	helper := ""
	if p.Secret != "" && storedAPIKey(p) == "" && p.APIKey == "" {
		return AppState{}, errors.New("无法解密已保存的 Key，请重新填写")
	}
	if storedAPIKey(p) != "" {
		helper, err = credentialHelperPath()
		if err != nil {
			return AppState{}, err
		}
	}
	if _, err = piProfileObjects(models, settings, p, helper, dataPath()); err != nil {
		return AppState{}, err
	}
	store.PiBaseline.ProfileID = p.ID
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	if err = commitPiObjects(root, models, settings, oldModels, oldSettings); err != nil {
		return AppState{}, err
	}
	return a.loadPiState(store)
}
func restoreRaw(obj map[string]json.RawMessage, key string, value json.RawMessage) {
	if len(value) == 0 {
		delete(obj, key)
	} else {
		obj[key] = value
	}
}
func (a *App) restorePiProfile(store storeFile) (AppState, error) {
	b := store.PiBaseline
	if b == nil {
		return AppState{}, errors.New("尚未切换 pi 配置，无需恢复")
	}
	if b.Root != piRoot() {
		return AppState{}, errors.New("pi 配置目录已变化，请先恢复原目录")
	}
	models, oldModels, err := readPiObject(filepath.Join(b.Root, "models.json"))
	if err != nil {
		return AppState{}, err
	}
	settings, oldSettings, err := readPiObject(filepath.Join(b.Root, "settings.json"))
	if err != nil {
		return AppState{}, err
	}
	providers := map[string]json.RawMessage{}
	if len(models["providers"]) > 0 {
		if err = json.Unmarshal(models["providers"], &providers); err != nil || providers == nil {
			return AppState{}, errors.New("pi providers 必须是对象")
		}
	}
	restoreRaw(providers, providerID, b.Provider)
	models["providers"] = toRaw(providers)
	restoreRaw(settings, "defaultProvider", b.DefaultProvider)
	restoreRaw(settings, "defaultModel", b.DefaultModel)
	if err = commitPiObjects(b.Root, models, settings, oldModels, oldSettings); err != nil {
		return AppState{}, err
	}
	store.PiBaseline = nil
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	return a.loadPiState(store)
}
func writePiConfigText(content string) error {
	if _, err := parseJSONObject([]byte(content)); err != nil {
		return err
	}
	path := filepath.Join(piRoot(), "models.json")
	old, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		old = []byte("{}")
		err = nil
	}
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(piRoot(), ".model-switcher-backups", time.Now().Format("20060102-150405.000000000"), "models.json"), old); err != nil {
		return err
	}
	return atomicWrite(path, []byte(content))
}
