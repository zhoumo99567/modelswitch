package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const piChatEventName = "pi-chat-stream"

// Only public model metadata is sent to the webview. Credentials stay in Go.
type PiChatConfig struct {
	ID          string          `json:"id"`
	ProfileID   string          `json:"profileId"`
	ProfileName string          `json:"profileName"`
	BaseURL     string          `json:"baseUrl"`
	Model       json.RawMessage `json:"model"`
}

type PiChatRequest struct {
	ID       string `json:"id"`
	ConfigID string `json:"configId"`
	Body     string `json:"body"`
}

type PiChatEvent struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Data        []byte `json:"data,omitempty"`
	Message     string `json:"message,omitempty"`
}

type piChatConnection struct {
	config  PiChatConfig
	apiKey  string
	headers map[string]string
	api     string
}

func (a *App) LoadPiChatConfig() (PiChatConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, err := loadPiChatConnection()
	return c.config, err
}

func loadPiChatConnection() (piChatConnection, error) {
	c := piChatConnection{headers: map[string]string{}}
	store, err := readStore()
	if err != nil {
		return c, err
	}
	if selectedTarget(store) != "pi" {
		return c, errors.New("对话仅在 pi agent 下可用")
	}
	settings, _, err := readPiObject(filepath.Join(piRoot(), "settings.json"))
	if err != nil {
		return c, err
	}
	models, _, err := readPiObject(filepath.Join(piRoot(), "models.json"))
	if err != nil {
		return c, err
	}
	var providers map[string]json.RawMessage
	if err = json.Unmarshal(models["providers"], &providers); err != nil || rawString(settings, "defaultProvider") != providerID {
		return c, errors.New("请先在模型设置中应用一个配置到 pi agent")
	}
	var provider map[string]json.RawMessage
	if err = json.Unmarshal(providers[providerID], &provider); err != nil {
		return c, errors.New("当前 pi 模型服务配置无效，请重新应用配置")
	}
	var profile *storedProfile
	for i := range store.Profiles {
		if rawString(provider, "name") == "ModelSwitcher / "+store.Profiles[i].ID {
			profile = &store.Profiles[i]
			break
		}
	}
	if profile == nil {
		return c, errors.New("请先在模型设置中应用一个配置到 pi agent")
	}
	base, err := normalizeBaseURL(rawString(provider, "baseUrl"))
	if err != nil {
		return c, err
	}
	modelID := rawString(settings, "defaultModel")
	var entries []map[string]json.RawMessage
	if err = json.Unmarshal(provider["models"], &entries); err != nil {
		return c, errors.New("当前 pi 模型列表无效")
	}
	var model map[string]json.RawMessage
	for _, entry := range entries {
		if modelID != "" && rawString(entry, "id") == modelID {
			model = entry
			break
		}
	}
	if model == nil {
		return c, errors.New("当前 pi 默认模型不在模型列表中，请重新应用配置")
	}
	c.api = rawString(model, "api")
	if c.api == "" {
		c.api = rawString(provider, "api")
	}
	if c.api == "" {
		c.api = "openai-completions"
	}
	if c.api != "openai-completions" && c.api != "openai-responses" {
		return c, fmt.Errorf("对话暂不支持 %s 协议", c.api)
	}
	key := rawString(provider, "apiKey")
	if strings.HasPrefix(key, "!") {
		// Resolve our own credential command directly; never execute config text.
		if !modelSwitcherTokenCommand(key, profile.ID, dataPath(), profileStorePath()) {
			return c, errors.New("对话不执行自定义凭据命令，请重新应用配置")
		}
		c.apiKey = storedAPIKey(*profile)
		if c.apiKey == "" {
			return c, errors.New("无法读取已保存的 Key，请重新填写并应用配置")
		}
	} else {
		c.apiKey = piChatEnvValue(key)
	}
	for _, source := range []map[string]json.RawMessage{provider, model} {
		var headers map[string]string
		if len(source["headers"]) > 0 {
			if err = json.Unmarshal(source["headers"], &headers); err != nil {
				return c, errors.New("pi 请求头必须是字符串对象")
			}
			for name, value := range headers {
				if strings.HasPrefix(value, "!") {
					return c, errors.New("对话不执行自定义请求头命令")
				}
				c.headers[name] = piChatEnvValue(value)
			}
		}
	}
	// Include credentials in the digest so a key change invalidates old sessions.
	fingerprint, _ := json.Marshal([]any{piRoot(), settings, provider, c.apiKey, c.headers})
	c.config = PiChatConfig{ID: fmt.Sprintf("%x", sha256.Sum256(fingerprint)), ProfileID: profile.ID, ProfileName: profile.Name, BaseURL: base}
	normalizePiModelMetadata(model)
	if piModelRawMissingOrNull(model["name"]) {
		model["name"] = toRaw(modelID)
	}
	model["provider"] = toRaw(providerID)
	model["api"] = toRaw(c.api)
	model["baseUrl"] = toRaw(base)
	delete(model, "headers")
	delete(model, "apiKey")
	c.config.Model = toRaw(model)
	return c, nil
}

func modelSwitcherTokenCommand(value, profileID string, profileFiles ...string) bool {
	if !strings.HasPrefix(value, "!") || profileID == "" {
		return false
	}
	for _, suffix := range []string{
		"--model-switcher-token " + profileID,
		"--model-switcher-token '" + profileID + "'",
		"--model-switcher-token \"" + profileID + "\"",
	} {
		if strings.HasSuffix(value, suffix) {
			return true
		}
		for _, path := range profileFiles {
			if path == "" {
				continue
			}
			for _, quoted := range []string{"'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'", "\"" + path + "\""} {
				if strings.HasSuffix(value, suffix+" --profiles "+quoted) {
					return true
				}
			}
		}
	}
	return false
}

func piChatEnvValue(value string) string {
	if resolved, ok := os.LookupEnv(value); ok {
		return resolved
	}
	return value
}

func piModelRawMissingOrNull(value json.RawMessage) bool {
	value = bytes.TrimSpace(value)
	return len(value) == 0 || bytes.Equal(value, []byte("null"))
}

// Pi's cost calculator expects a cost object even for local models. Older
// profiles and some /v1/models responses omit it or write it as null; keep the
// model usable with those configurations instead of failing after the provider
// has already returned text.
func normalizePiModelMetadata(model map[string]json.RawMessage) {
	defaults := map[string]any{
		"name":          rawString(model, "id"),
		"reasoning":     false,
		"input":         []string{"text"},
		"cost":          map[string]int{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
		"contextWindow": defaultModelContextWindow,
		"maxTokens":     4096,
	}
	for name, value := range defaults {
		if piModelRawMissingOrNull(model[name]) {
			model[name] = toRaw(value)
		}
	}
	var cost map[string]json.RawMessage
	if err := json.Unmarshal(model["cost"], &cost); err != nil || cost == nil {
		model["cost"] = toRaw(defaults["cost"])
		return
	}
	for _, name := range []string{"input", "output", "cacheRead", "cacheWrite"} {
		if piModelRawMissingOrNull(cost[name]) {
			cost[name] = toRaw(0)
		}
	}
	model["cost"] = toRaw(cost)
}

// Reserve the request before returning, then send bytes through Wails events.
// Windows' asset server buffers HTTP responses, so it cannot carry a live SSE stream.
func (a *App) StartPiChat(input PiChatRequest) error {
	if input.ID == "" || len(input.ID) > 128 {
		return errors.New("聊天请求无效")
	}
	if len(input.Body) > 32<<20 {
		return errors.New("对话及图片总大小超过 32 MB，请减少附件或开启新对话")
	}
	a.mu.Lock()
	c, err := loadPiChatConnection()
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if input.ConfigID != c.config.ID {
		return errors.New("当前配置已变化，请刷新聊天后重新发送")
	}
	var body map[string]json.RawMessage
	if err = json.Unmarshal([]byte(input.Body), &body); err != nil || body == nil || rawString(body, "model") == "" {
		return errors.New("聊天请求格式无效")
	}
	var model struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(c.config.Model, &model)
	if rawString(body, "model") != model.ID || string(body["stream"]) != "true" {
		return errors.New("聊天请求必须使用当前模型及流式输出")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	a.chatMu.Lock()
	if a.chatID != "" {
		a.chatMu.Unlock()
		cancel()
		return errors.New("上一条回复尚未结束，请先停止生成")
	}
	a.chatID, a.chatCancel = input.ID, cancel
	a.chatMu.Unlock()
	go func() {
		err := a.streamPiChat(ctx, input, c)
		cancel()
		a.chatMu.Lock()
		a.chatID, a.chatCancel = "", nil
		a.chatMu.Unlock()
		if err != nil {
			message := err.Error()
			if c.apiKey != "" {
				message = strings.ReplaceAll(message, c.apiKey, "[redacted]")
			}
			a.emitPiChat(PiChatEvent{ID: input.ID, Type: "error", Message: message})
		} else {
			a.emitPiChat(PiChatEvent{ID: input.ID, Type: "end"})
		}
	}()
	return nil
}

func (a *App) CancelPiChat(id string) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.chatID == id && a.chatCancel != nil {
		a.chatCancel()
	}
}

func (a *App) cancelPiChat() {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.chatCancel != nil {
		a.chatCancel()
	}
}

func (a *App) emitPiChat(event PiChatEvent) {
	if a.chatEmit != nil {
		a.chatEmit(event)
	} else if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, piChatEventName, event)
	}
}

func (a *App) streamPiChat(ctx context.Context, input PiChatRequest, c piChatConnection) error {
	path := "/chat/completions"
	if c.api == "openai-responses" {
		path = "/responses"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+path, bytes.NewBufferString(input.Body))
	if err != nil {
		return err
	}
	for name, value := range c.headers {
		req.Header.Set(name, value)
	}
	if c.apiKey != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return fmt.Errorf("模型服务返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	a.emitPiChat(PiChatEvent{ID: input.ID, Type: "headers", Status: response.StatusCode, ContentType: response.Header.Get("Content-Type")})
	buffer := make([]byte, 8<<10)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			a.emitPiChat(PiChatEvent{ID: input.ID, Type: "chunk", Data: append([]byte(nil), buffer[:n]...)})
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}
