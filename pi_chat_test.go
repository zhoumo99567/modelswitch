package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupPiChat(t *testing.T, serverURL string) (*App, PiChatConfig, chan PiChatEvent) {
	t.Helper()
	isolateSkillStore(t)
	p := storedProfile{ProfileView: ProfileView{ID: "chat-test", Name: "Chat test", BaseURL: serverURL + "/v1", APIKey: "unit-test-secret", SelectedModel: "test-model"}}
	if err := writeStore(storeFile{Target: "pi", Profiles: []storedProfile{p}}); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	if _, err := app.ActivateProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	events := make(chan PiChatEvent, 64)
	app.chatEmit = func(event PiChatEvent) { events <- event }
	t.Cleanup(app.cancelPiChat)
	config, err := app.LoadPiChatConfig()
	if err != nil {
		t.Fatal(err)
	}
	return app, config, events
}

func awaitPiChatEvent(t *testing.T, events <-chan PiChatEvent) PiChatEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for chat event")
		return PiChatEvent{}
	}
}

func TestPiChatReadsQuotedCredentialCommands(t *testing.T) {
	isolateSkillStore(t)
	p := storedProfile{ProfileView: ProfileView{ID: "chat-test", Name: "Chat test", BaseURL: "http://127.0.0.1:1/v1", APIKey: "unit-test-secret", SelectedModel: "test-model"}}
	if err := writeStore(storeFile{Target: "pi", Profiles: []storedProfile{p}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp().ActivateProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(piRoot(), "models.json")
	models, _, err := readPiObject(path)
	if err != nil {
		t.Fatal(err)
	}
	var providers map[string]json.RawMessage
	if err := json.Unmarshal(models["providers"], &providers); err != nil {
		t.Fatal(err)
	}
	var provider map[string]json.RawMessage
	if err := json.Unmarshal(providers[providerID], &provider); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, command string
		valid         bool
	}{
		{"Windows", `!"C:\Program Files\model-switcher.exe" --model-switcher-token chat-test`, true},
		{"macOS", `!'/Applications/Model Switcher.app/Contents/MacOS/model-switcher' --model-switcher-token 'chat-test'`, true},
		{"wrong profile", `!'/app' --model-switcher-token 'other-profile'`, false},
		{"profile prefix", `!"app.exe" --model-switcher-token chat-test-other`, false},
		{"extra command", `!"app.exe" --model-switcher-token chat-test; echo unwanted`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider["apiKey"] = toRaw(tc.command)
			providers[providerID] = toRaw(provider)
			models["providers"] = toRaw(providers)
			if err := atomicWrite(path, toRaw(models)); err != nil {
				t.Fatal(err)
			}
			connection, err := loadPiChatConnection()
			if tc.valid {
				if err != nil {
					t.Fatalf("generated credential command rejected: %v", err)
				}
				if connection.apiKey != p.APIKey {
					t.Fatal("credential command did not resolve the saved profile")
				}
			} else if err == nil {
				t.Fatal("accepted a command for a different profile or extra arguments")
			}
		})
	}
}

func TestPiChatStreamsCurrentConfiguration(t *testing.T) {
	firstChunk := make(chan struct{})
	finish := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer unit-test-secret" {
			t.Errorf("incorrect endpoint or credentials")
		}
		data, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(data), "test-model") || !strings.Contains(string(data), "earlier reply") {
			t.Errorf("lost current model or conversation: %s", data)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n")
		w.(http.Flusher).Flush()
		close(firstChunk)
		select {
		case <-finish:
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	app, config, events := setupPiChat(t, server.URL)
	encoded, _ := json.Marshal(config)
	if strings.Contains(string(encoded), "unit-test-secret") || strings.Contains(string(encoded), "--model-switcher-token") {
		t.Fatal("credential material leaked to the webview")
	}
	body := `{"model":"test-model","stream":true,"messages":[{"role":"assistant","content":"earlier reply"},{"role":"user","content":"hello"}]}`
	if err := app.StartPiChat(PiChatRequest{ID: "turn-1", ConfigID: config.ID, Body: body}); err != nil {
		t.Fatal(err)
	}
	if event := awaitPiChatEvent(t, events); event.Type != "headers" || event.Status != 200 {
		t.Fatalf("headers: %#v", event)
	}
	if event := awaitPiChatEvent(t, events); event.Type != "chunk" || !strings.Contains(string(event.Data), "你好") {
		t.Fatalf("first live chunk: %#v", event)
	}
	<-firstChunk
	// The stream is still open: bytes must already have reached the webview.
	if err := app.StartPiChat(PiChatRequest{ID: "overlap", ConfigID: config.ID, Body: body}); err == nil {
		t.Fatal("accepted overlapping generation")
	}
	close(finish)
	for {
		event := awaitPiChatEvent(t, events)
		if event.Type == "error" {
			t.Fatal(event.Message)
		}
		if event.Type == "end" {
			break
		}
	}
}

func TestPiChatCancelAndRetry(t *testing.T) {
	started := make(chan struct{}, 2)
	cancelled := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		started <- struct{}{}
		<-r.Context().Done()
		cancelled <- struct{}{}
	}))
	defer server.Close()
	app, config, events := setupPiChat(t, server.URL)
	for _, id := range []string{"cancel-1", "cancel-2"} {
		if err := app.StartPiChat(PiChatRequest{ID: id, ConfigID: config.ID, Body: `{"model":"test-model","stream":true,"messages":[]}`}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("request never started")
		}
		app.CancelPiChat("another-request")
		app.CancelPiChat(id)
		for awaitPiChatEvent(t, events).Type != "error" {
		}
		select {
		case <-cancelled:
		case <-time.After(3 * time.Second):
			t.Fatal("cancel did not reach the HTTP request")
		}
	}
}

func TestPiChatRejectsStaleOrWrongConfiguration(t *testing.T) {
	app, config, _ := setupPiChat(t, "http://127.0.0.1:1")
	body := `{"model":"test-model","stream":true,"messages":[]}`
	for _, input := range []PiChatRequest{
		{ID: "stale", ConfigID: "old-config", Body: body},
		{ID: "wrong-model", ConfigID: config.ID, Body: `{"model":"other-model","stream":true}`},
		{ID: "non-stream", ConfigID: config.ID, Body: `{"model":"test-model","stream":false}`},
	} {
		if err := app.StartPiChat(input); err == nil {
			t.Fatalf("accepted invalid request %s", input.ID)
		}
	}
	path := filepath.Join(piRoot(), "models.json")
	data, _ := os.ReadFile(path)
	data = []byte(strings.ReplaceAll(string(data), "test-model", "new-model"))
	_ = os.WriteFile(path, data, 0600)
	if _, err := app.LoadPiChatConfig(); err == nil {
		t.Fatal("accepted missing default model")
	}
	if _, err := app.SetTarget("chatgpt"); err != nil {
		t.Fatal(err)
	}
	if err := app.StartPiChat(PiChatRequest{ID: "wrong-target", ConfigID: config.ID, Body: body}); err == nil {
		t.Fatal("accepted chat outside pi target")
	}
}

func TestPiChatRedactsProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid key unit-test-secret", http.StatusUnauthorized)
	}))
	defer server.Close()
	app, config, events := setupPiChat(t, server.URL)
	if err := app.StartPiChat(PiChatRequest{ID: "error", ConfigID: config.ID, Body: `{"model":"test-model","stream":true,"messages":[]}`}); err != nil {
		t.Fatal(err)
	}
	event := awaitPiChatEvent(t, events)
	if event.Type != "error" || !strings.Contains(event.Message, "401") || strings.Contains(event.Message, "unit-test-secret") {
		t.Fatalf("unexpected or unredacted error: %#v", event)
	}
}

func TestPiChatReadsActualModelAndEnvironmentCredentials(t *testing.T) {
	app, original, _ := setupPiChat(t, "http://127.0.0.1:1")
	path := filepath.Join(piRoot(), "models.json")
	models, _, _ := readPiObject(path)
	var providers map[string]json.RawMessage
	_ = json.Unmarshal(models["providers"], &providers)
	var provider map[string]json.RawMessage
	_ = json.Unmarshal(providers[providerID], &provider)
	provider["apiKey"] = toRaw("PI_CHAT_UNIT_KEY")
	provider["api"] = toRaw("openai-responses")
	provider["models"] = toRaw([]map[string]any{{"id": "edited-model", "reasoning": true, "contextWindow": 64000, "maxTokens": 8192, "input": []string{"text", "image"}}})
	providers[providerID] = toRaw(provider)
	models["providers"] = toRaw(providers)
	if err := atomicWrite(path, toRaw(models)); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(piRoot(), "settings.json")
	settings, _, _ := readPiObject(settingsPath)
	settings["defaultModel"] = toRaw("edited-model")
	if err := atomicWrite(settingsPath, toRaw(settings)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CHAT_UNIT_KEY", "env-secret-one")
	connection, err := loadPiChatConnection()
	if err != nil {
		t.Fatal(err)
	}
	var model map[string]any
	_ = json.Unmarshal(connection.config.Model, &model)
	if model["id"] != "edited-model" || model["reasoning"] != true || model["maxTokens"] != float64(8192) || connection.api != "openai-responses" || connection.apiKey != "env-secret-one" {
		t.Fatalf("did not read actual files and env: %#v", model)
	}
	if original.ID == connection.config.ID {
		t.Fatal("model edit did not invalidate the old config")
	}
	t.Setenv("PI_CHAT_UNIT_KEY", "env-secret-two")
	updated, err := app.LoadPiChatConfig()
	if err != nil || updated.ID == connection.config.ID {
		t.Fatal("credential change did not invalidate the old config")
	}
}

func TestNormalizePiModelMetadataFillsMissingCost(t *testing.T) {
	model := map[string]json.RawMessage{
		"id":   toRaw("local-model"),
		"cost": json.RawMessage("null"),
	}
	normalizePiModelMetadata(model)
	var cost map[string]json.RawMessage
	if err := json.Unmarshal(model["cost"], &cost); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"input", "output", "cacheRead", "cacheWrite"} {
		if got := string(cost[name]); got != "0" {
			t.Fatalf("cost.%s = %s, want 0", name, got)
		}
	}
	if rawString(model, "name") != "local-model" || len(model["input"]) == 0 {
		t.Fatalf("metadata defaults missing: %#v", model)
	}
}

func TestPiChatResponsesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("wrong Responses path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")
	}))
	defer server.Close()
	app, _, events := setupPiChat(t, server.URL)
	path := filepath.Join(piRoot(), "models.json")
	data, _ := os.ReadFile(path)
	if err := atomicWrite(path, []byte(strings.ReplaceAll(string(data), "openai-completions", "openai-responses"))); err != nil {
		t.Fatal(err)
	}
	config, err := app.LoadPiChatConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.StartPiChat(PiChatRequest{ID: "responses", ConfigID: config.ID, Body: `{"model":"test-model","stream":true,"input":[]}`}); err != nil {
		t.Fatal(err)
	}
	sawChunk := false
	for {
		event := awaitPiChatEvent(t, events)
		if event.Type == "error" {
			t.Fatal(event.Message)
		}
		if event.Type == "chunk" {
			sawChunk = strings.Contains(string(event.Data), "response.completed")
		}
		if event.Type == "end" {
			break
		}
	}
	if !sawChunk {
		t.Fatal("Responses SSE was not delivered")
	}
}
