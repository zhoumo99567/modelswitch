package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTestModelUsesChatCompletionsForPi(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization header")
		}
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "local-model" || len(request.Messages) != 1 || request.Messages[0].Role != "user" || request.Messages[0].Content != "你好，请介绍一下你自己。" {
			t.Fatalf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"local-model","choices":[{"message":{"content":"连接测试成功。"}}]}`))
	}))
	defer server.Close()

	result, err := (&App{}).TestModel(ProfileInput{BaseURL: server.URL + "/v1", APIKey: "test-key", SelectedModel: "local-model"}, "pi", "你好，请介绍一下你自己。")
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != "chat.completions" || result.Model != "local-model" || result.Reply != "连接测试成功。" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTestModelUsesResponsesForCodex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		var request struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "codex-model" || request.Input != "请用一句话介绍你自己。" {
			t.Fatalf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"codex-model","output":[{"content":[{"type":"output_text","text":"responses 正常。"}]}]}`))
	}))
	defer server.Close()

	result, err := (&App{}).TestModel(ProfileInput{BaseURL: server.URL + "/v1", SelectedModel: "codex-model"}, "chatgpt", "请用一句话介绍你自己。")
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != "responses" || result.Model != "codex-model" || result.Reply != "responses 正常。" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
