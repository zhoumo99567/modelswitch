package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	window windowMemory
}
type Model struct {
	ID             string `json:"id"`
	OwnedBy        string `json:"owned_by,omitempty"`
	SupportsImages bool   `json:"supportsImages,omitempty"`
}
type ModelTestResult struct {
	Model    string `json:"model"`
	Reply    string `json:"reply"`
	Protocol string `json:"protocol"`
}
type ProfileInput struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	BaseURL       string  `json:"baseUrl"`
	APIKey        string  `json:"apiKey"`
	ClearAPIKey   bool    `json:"clearApiKey"`
	SelectedModel string  `json:"selectedModel"`
	Models        []Model `json:"models"`
}
type ProfileView struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	BaseURL       string  `json:"baseUrl"`
	APIKey        string  `json:"apiKey,omitempty"`
	HasAPIKey     bool    `json:"hasApiKey"`
	SelectedModel string  `json:"selectedModel"`
	Models        []Model `json:"models"`
}
type AppState struct {
	Target                string        `json:"target"`
	Version               string        `json:"version"`
	Profiles              []ProfileView `json:"profiles"`
	ActiveProfileID       string        `json:"activeProfileId"`
	ActiveProvider        string        `json:"activeProvider"`
	ActiveModel           string        `json:"activeModel"`
	ConfigPath            string        `json:"configPath"`
	ChatGPTRunning        bool          `json:"chatGptRunning"`
	CanRestore            bool          `json:"canRestore"`
	ChatGPTTarget         string        `json:"chatGptTarget"`
	ChatGPTResolvedTarget string        `json:"chatGptResolvedTarget"`
	ChatGPTTargetError    string        `json:"chatGptTargetError"`
}
type storedProfile struct {
	ProfileView
	Secret string `json:"secret,omitempty"`
}

// storedAPIKey returns the current plaintext key. New profiles store APIKey
// directly; Secret remains as a compatibility path for older installations.
func storedAPIKey(profile storedProfile) string {
	if profile.APIKey != "" {
		return profile.APIKey
	}
	if profile.Secret == "" {
		return ""
	}
	key, err := unprotectSecret(profile.Secret)
	if err != nil {
		return ""
	}
	return key
}

type baseline struct {
	Model    *string `json:"model"`
	Provider *string `json:"provider"`
}
type storeFile struct {
	Profiles          []storedProfile `json:"profiles"`
	Baseline          *baseline       `json:"baseline,omitempty"`
	ChatGPTTarget     string          `json:"chatGptTarget,omitempty"`
	Target            string          `json:"target,omitempty"`
	PiBaseline        *piBaseline     `json:"piBaseline,omitempty"`
	Workspaces        []Workspace     `json:"workspaces,omitempty"`
	ActiveWorkspaceID string          `json:"activeWorkspaceId,omitempty"`
}

func NewApp() *App                         { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) LoadState() (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loadState()
}
func (a *App) loadState() (AppState, error) {
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	if selectedTarget(store) == "pi" {
		return a.loadPiState(store)
	}
	data, err := readConfig()
	if err != nil {
		return AppState{}, err
	}
	config, err := parseConfig(data)
	if err != nil {
		return AppState{}, err
	}
	provider := stringValue(config, "model_provider")
	if provider == "" {
		provider = "openai"
	}
	result := AppState{Target: "chatgpt", Version: AppVersion, Profiles: []ProfileView{}, ActiveProvider: provider, ActiveModel: stringValue(config, "model"), ConfigPath: configPath(), ChatGPTRunning: isChatGPTRunning(), CanRestore: store.Baseline != nil, ChatGPTTarget: store.ChatGPTTarget}
	result.ChatGPTResolvedTarget, err = resolveChatGPTTarget(store.ChatGPTTarget)
	if err != nil {
		result.ChatGPTTargetError = err.Error()
	}
	for _, p := range store.Profiles {
		v := p.ProfileView
		v.APIKey = storedAPIKey(p)
		v.HasAPIKey = v.APIKey != "" || p.Secret != ""
		if v.Models == nil {
			v.Models = []Model{}
		}
		result.Profiles = append(result.Profiles, v)
		if provider == providerID && configuredProfileID(config) == p.ID {
			result.ActiveProfileID = p.ID
		}
	}
	return result, nil
}
func (a *App) SaveProfile(input ProfileInput) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	base, err := normalizeBaseURL(input.BaseURL)
	if err != nil {
		return AppState{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return AppState{}, errors.New("请输入配置名称")
	}
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	id := input.ID
	if id == "" {
		id = fmt.Sprintf("local-%d", time.Now().UnixNano())
	}
	index := -1
	secret := ""
	apiKey := ""
	for i, p := range store.Profiles {
		if p.ID == id {
			index = i
			secret = p.Secret
			apiKey = p.APIKey
		}
	}
	if input.ID != "" && index < 0 {
		return AppState{}, errors.New("配置不存在，请刷新后重试")
	}
	if input.ClearAPIKey {
		secret = ""
		apiKey = ""
	} else if input.APIKey != "" {
		apiKey = input.APIKey
		secret = ""
	} else if apiKey == "" && secret != "" {
		// Migrate old encrypted values when this platform can still read them.
		if recovered := storedAPIKey(storedProfile{Secret: secret}); recovered != "" {
			apiKey = recovered
			secret = ""
		}
	}
	// Never carry an existing credential to an edited endpoint without re-entry.
	if index >= 0 && store.Profiles[index].BaseURL != base && input.APIKey == "" {
		secret = ""
		apiKey = ""
	}
	p := storedProfile{ProfileView: ProfileView{ID: id, Name: strings.TrimSpace(input.Name), BaseURL: base, APIKey: apiKey, SelectedModel: input.SelectedModel, Models: input.Models}, Secret: secret}
	if index < 0 {
		store.Profiles = append(store.Profiles, p)
	} else {
		store.Profiles[index] = p
	}
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	return a.loadState()
}
func (a *App) DeleteProfile(id string) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	if store.PiBaseline != nil && store.PiBaseline.ProfileID == id {
		return AppState{}, errors.New("请先恢复 pi agent 配置，再删除正在使用的配置")
	}
	data, err := readConfig()
	if err != nil {
		return AppState{}, err
	}
	conf, err := parseConfig(data)
	if err != nil {
		return AppState{}, err
	}
	if stringValue(conf, "model_provider") == providerID && configuredProfileID(conf) == id {
		return AppState{}, errors.New("请先切回 OpenAI，再删除正在使用的配置")
	}
	result := []storedProfile{}
	for _, p := range store.Profiles {
		if p.ID != id {
			result = append(result, p)
		}
	}
	store.Profiles = result
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	return a.loadState()
}
func (a *App) FetchModels(input ProfileInput) ([]Model, error) {
	base, err := normalizeBaseURL(input.BaseURL)
	if err != nil {
		return nil, err
	}
	key, err := credentialForInput(input, base)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	// No redirects: avoid forwarding a saved key to another destination.
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("无法连接模型服务，请检查地址、端口和服务是否启动")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型服务返回 HTTP %d；请检查地址和 API Key", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return nil, errors.New("读取模型列表失败")
	}
	if len(data) > 2<<20 {
		return nil, errors.New("模型列表超过 2 MB")
	}
	var decoded struct {
		Data []Model `json:"data"`
	}
	if err = json.Unmarshal(data, &decoded); err != nil {
		return nil, errors.New("服务未返回 OpenAI 格式的模型列表")
	}
	models := []Model{}
	seen := map[string]bool{}
	for _, m := range decoded.Data {
		if strings.TrimSpace(m.ID) != "" && !seen[m.ID] {
			models = append(models, m)
			seen[m.ID] = true
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

func (a *App) TestModel(input ProfileInput, target, message string) (ModelTestResult, error) {
	base, err := normalizeBaseURL(input.BaseURL)
	if err != nil {
		return ModelTestResult{}, err
	}
	model := strings.TrimSpace(input.SelectedModel)
	if model == "" {
		return ModelTestResult{}, errors.New("请先选择要测试的模型")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return ModelTestResult{}, errors.New("请输入测试内容")
	}
	if len([]rune(message)) > 4000 {
		return ModelTestResult{}, errors.New("测试内容不能超过 4000 个字符")
	}
	key, err := credentialForInput(input, base)
	if err != nil {
		return ModelTestResult{}, err
	}
	var endpoint string
	var payload any
	protocol := "responses"
	if target == "pi" {
		endpoint = base + "/chat/completions"
		protocol = "chat.completions"
		payload = struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			MaxTokens   int `json:"max_tokens"`
			Temperature int `json:"temperature"`
		}{Model: model, Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: message}}, MaxTokens: 512, Temperature: 0}
	} else if target == "chatgpt" {
		endpoint = base + "/responses"
		payload = struct {
			Model           string `json:"model"`
			Input           string `json:"input"`
			MaxOutputTokens int    `json:"max_output_tokens"`
		}{Model: model, Input: message, MaxOutputTokens: 512}
	} else {
		return ModelTestResult{}, errors.New("不支持的模型测试目标")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ModelTestResult{}, errors.New("生成模型测试请求失败")
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ModelTestResult{}, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return ModelTestResult{}, errors.New("无法连接模型，请检查地址、端口和服务是否启动")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return ModelTestResult{}, errors.New("读取模型测试响应失败")
	}
	if len(data) > 2<<20 {
		return ModelTestResult{}, errors.New("模型测试响应超过 2 MB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelTestResult{}, fmt.Errorf("模型测试失败：HTTP %d，请检查模型、地址和 API Key", resp.StatusCode)
	}
	reply, responseModel := modelTestText(data, target)
	if reply == "" {
		return ModelTestResult{}, errors.New("模型已返回响应，但没有可读文本")
	}
	if responseModel == "" {
		responseModel = model
	}
	return ModelTestResult{Model: responseModel, Reply: reply, Protocol: protocol}, nil
}

func modelTestText(data []byte, target string) (string, string) {
	var decoded struct {
		Model      string `json:"model"`
		OutputText string `json:"output_text"`
		Choices    []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(data, &decoded) != nil {
		return "", ""
	}
	if target == "pi" && len(decoded.Choices) > 0 {
		return rawModelText(decoded.Choices[0].Message.Content), decoded.Model
	}
	if decoded.OutputText != "" {
		return decoded.OutputText, decoded.Model
	}
	var builder strings.Builder
	for _, item := range decoded.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String(), decoded.Model
}

func rawModelText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		if strings.TrimSpace(part.Text) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(part.Text)
	}
	return strings.TrimSpace(builder.String())
}
func credentialForInput(input ProfileInput, base string) (string, error) {
	if input.ClearAPIKey {
		return "", nil
	}
	if input.APIKey != "" {
		return input.APIKey, nil
	}
	store, err := readStore()
	if err != nil {
		return "", err
	}
	for _, p := range store.Profiles {
		if p.ID == input.ID && p.BaseURL == base {
			if key := storedAPIKey(p); key != "" {
				return key, nil
			}
			return "", nil
		}
	}
	return "", nil
}
func tokenForProfile(id string) (string, error) {
	store, err := readStore()
	if err != nil {
		return "", err
	}
	for _, p := range store.Profiles {
		if p.ID == id {
			if key := storedAPIKey(p); key != "" {
				return key, nil
			}
			return "", errors.New("找不到凭据")
		}
	}
	return "", errors.New("找不到凭据")
}
func (a *App) ActivateProfile(id string) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	var profile *storedProfile
	for i := range store.Profiles {
		if store.Profiles[i].ID == id {
			profile = &store.Profiles[i]
		}
	}
	if profile == nil || profile.SelectedModel == "" {
		return AppState{}, errors.New("请先保存配置并选择模型")
	}
	if selectedTarget(store) == "pi" {
		return a.activatePiProfile(store, *profile)
	}
	original, err := readConfig()
	if err != nil {
		return AppState{}, err
	}
	config, err := parseConfig(original)
	if err != nil {
		return AppState{}, err
	}
	if store.Baseline == nil {
		p := stringValue(config, "model_provider")
		if p != "" && p != "openai" {
			return AppState{}, errors.New("当前配置是其他提供方；请先在 Codex 中切回 OpenAI，再使用本工具")
		}
		store.Baseline = &baseline{Model: optionalString(config, "model"), Provider: optionalString(config, "model_provider")}
	}
	helper := ""
	if profile.Secret != "" && storedAPIKey(*profile) == "" && profile.APIKey == "" {
		return AppState{}, errors.New("无法解密已保存的 Key，请重新填写")
	}
	if storedAPIKey(*profile) != "" {
		helper, err = credentialHelperPath()
		if err != nil {
			return AppState{}, errors.New("无法定位当前程序，未修改配置")
		}
	}
	next, err := localConfig(original, *profile, helper)
	if err != nil {
		return AppState{}, err
	}
	chatGPTTarget, err := resolveChatGPTTarget(store.ChatGPTTarget)
	if err != nil {
		return AppState{}, err
	}
	// Persist the recovery baseline before changing the user's config.
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	if err = closeChatGPT(); err != nil {
		return AppState{}, err
	}
	if err = commitConfig(original, next); err != nil {
		return AppState{}, err
	}
	if err = launchChatGPT(chatGPTTarget); err != nil {
		return AppState{}, fmt.Errorf("配置已写入；自动启动失败（%v），请手动打开 ChatGPT", err)
	}
	return a.loadState()
}
func (a *App) ActivateOpenAI() (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	if selectedTarget(store) == "pi" {
		return a.restorePiProfile(store)
	}
	if store.Baseline == nil {
		return AppState{}, errors.New("尚未通过本工具切换，无需恢复")
	}
	original, err := readConfig()
	if err != nil {
		return AppState{}, err
	}
	next, err := restoreConfig(original, *store.Baseline)
	if err != nil {
		return AppState{}, err
	}
	chatGPTTarget, err := resolveChatGPTTarget(store.ChatGPTTarget)
	if err != nil {
		return AppState{}, err
	}
	if err = closeChatGPT(); err != nil {
		return AppState{}, err
	}
	if err = commitConfig(original, next); err != nil {
		return AppState{}, err
	}
	store.Baseline = nil
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	if err = launchChatGPT(chatGPTTarget); err != nil {
		return AppState{}, fmt.Errorf("已恢复 OpenAI 配置；自动启动失败（%v），请手动打开 ChatGPT", err)
	}
	return a.loadState()
}
func (a *App) SetChatGPTPath(target string) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	normalized, err := normalizeChatGPTTarget(target)
	if err != nil {
		return AppState{}, err
	}
	store, err := readStore()
	if err != nil {
		return AppState{}, err
	}
	store.ChatGPTTarget = normalized
	if err = writeStore(store); err != nil {
		return AppState{}, err
	}
	return a.loadState()
}

// ChooseChatGPTPath opens the platform's native file picker. Cancellation must
// leave the existing manual/automatic selection unchanged.
func (a *App) ChooseChatGPTPath() (AppState, error) {
	a.mu.Lock()
	store, err := readStore()
	a.mu.Unlock()
	if err != nil {
		return AppState{}, err
	}
	options := wailsruntime.OpenDialogOptions{
		Title:   "选择 ChatGPT 应用",
		Filters: []wailsruntime.FileFilter{{DisplayName: "ChatGPT 应用 (*.exe; *.lnk)", Pattern: "*.exe;*.lnk"}},
	}
	if runtime.GOOS == "darwin" {
		options.Filters = []wailsruntime.FileFilter{{DisplayName: "macOS 应用 (*.app)", Pattern: "*.app"}}
		options.DefaultDirectory = "/Applications"
		options.TreatPackagesAsDirectories = false
	}
	if target, resolveErr := resolveChatGPTTarget(store.ChatGPTTarget); resolveErr == nil && filepath.IsAbs(target) {
		if info, statErr := os.Stat(filepath.Dir(target)); statErr == nil && info.IsDir() {
			options.DefaultDirectory = filepath.Dir(target)
		}
	}
	selected, err := wailsruntime.OpenFileDialog(a.ctx, options)
	if err != nil {
		return AppState{}, fmt.Errorf("无法打开文件选择窗口: %w", err)
	}
	if selected == "" {
		return a.LoadState()
	}
	return a.SetChatGPTPath(selected)
}

func (a *App) OpenConfigFolder() error {
	return revealConfig(activeConfigPath())
}
func (a *App) OpenCodexDirectory() error {
	return openCodexDirectory(filepath.Dir(activeConfigPath()))
}
func (a *App) ReadConfigText() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if activeConfigPath() != configPath() {
		data, err := os.ReadFile(activeConfigPath())
		if errors.Is(err, os.ErrNotExist) {
			return "{}", nil
		}
		return string(data), err
	}
	data, err := readConfig()
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func (a *App) WriteConfigText(content string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if activeConfigPath() != configPath() {
		return writePiConfigText(content)
	}
	if _, err := parseConfig([]byte(content)); err != nil {
		return errors.New("config.toml 不是有效 TOML，未写入")
	}
	original, err := readConfig()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(filepath.Dir(dataPath()), "backups")
	backup := filepath.Join(backupDir, time.Now().Format("20060102-150405.000000000")+".toml")
	if err = atomicWrite(backup, original); err != nil {
		return fmt.Errorf("备份失败，未修改 config.toml: %w", err)
	}
	if err = atomicWrite(configPath(), []byte(content)); err != nil {
		return fmt.Errorf("写入 config.toml 失败: %w", err)
	}
	return nil
}
func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("请输入完整的 http:// 或 https:// 服务地址，不要包含 Key、查询参数或片段")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	if strings.HasSuffix(u.Path, "/models") {
		u.Path = strings.TrimSuffix(u.Path, "/models")
	}
	if u.Path == "" {
		u.Path = "/v1"
	} // Preserve custom API prefixes.
	return strings.TrimRight(u.String(), "/"), nil
}
func dataPath() string {
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		// A macOS .app stores the executable inside Contents/MacOS. Put the
		// portable profile beside the app bundle instead of inside the bundle.
		if runtime.GOOS == "darwin" {
			if index := strings.Index(dir, ".app"+string(filepath.Separator)); index >= 0 {
				dir = filepath.Dir(dir[:index+len(".app")])
			}
		}
		return filepath.Join(dir, "profiles.json")
	}
	return legacyDataPath()
}

// credentialHelperPath returns a stable executable for Codex's auth.command.
// Wails development builds use a temporary *-dev.exe that is renamed or
// removed when the dev process restarts. Copying it to a fixed name keeps an
// already-written config usable across dev restarts and release updates.
func credentialHelperPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(executable)
	name := "model-switcher-token"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := strings.ToLower(filepath.Base(executable))
	if !strings.Contains(base, "-dev") && !strings.Contains(base, ".test") {
		return executable, nil
	}
	target := filepath.Join(dir, name)
	if filepath.Clean(executable) == filepath.Clean(target) {
		return target, nil
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(dir, ".model-switcher-token-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err = temp.Write(data); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err = temp.Chmod(0755); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err = temp.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(tempPath, target); err != nil {
		// Windows cannot rename over an existing destination. A currently
		// running helper remains usable, so keep it when replacement is locked.
		if removeErr := os.Remove(target); removeErr != nil {
			if _, statErr := os.Stat(target); statErr == nil {
				return target, nil
			}
			return "", err
		}
		if err = os.Rename(tempPath, target); err != nil {
			return "", err
		}
	}
	return target, nil
}
func legacyDataPath() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "ModelSwitcher", "profiles.json")
}
func configPath() string {
	if root := os.Getenv("CODEX_HOME"); root != "" {
		return filepath.Join(root, "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}
func readStore() (storeFile, error) {
	path := dataPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// Read the previous per-user location once as a migration source. The
		// next write will place the data beside the portable executable.
		if path != legacyDataPath() {
			data, err = os.ReadFile(legacyDataPath())
		}
		if errors.Is(err, os.ErrNotExist) {
			return storeFile{}, nil
		}
	}
	if err != nil {
		return storeFile{}, err
	}
	var s storeFile
	if err = json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("无法读取配置列表: %w", err)
	}
	return s, nil
}
func writeStore(s storeFile) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(dataPath(), data)
}
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".model-switcher-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err == nil {
		return nil
	}
	// Windows can reject Rename when the destination already exists.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(temp, path)
}

// Referenced here so non-Windows builds fail explicitly through platform stubs.
var _ *exec.Cmd
