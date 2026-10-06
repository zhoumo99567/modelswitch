package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPiChatRuntimeUsesAppliedResponsesModel(t *testing.T) {
	if _, _, err := resolvePiChatSDK(); err != nil {
		t.Skipf("installed Pi SDK required: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer unit-test-secret" {
			t.Errorf("incorrect Responses connection")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"test-model"`) {
			t.Errorf("lost applied model")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(event map[string]any) {
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
		}
		item := map[string]any{"id": "msg-test", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "responses runtime works", "annotations": []any{}}}}
		send(map[string]any{"type": "response.created", "response": map[string]any{"id": "resp-test", "model": "test-model", "status": "in_progress", "output": []any{}}})
		send(map[string]any{"type": "response.output_item.added", "output_index": 0, "item": map[string]any{"id": "msg-test", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}})
		send(map[string]any{"type": "response.content_part.added", "item_id": "msg-test", "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
		send(map[string]any{"type": "response.output_text.delta", "item_id": "msg-test", "output_index": 0, "content_index": 0, "delta": "responses runtime works"})
		send(map[string]any{"type": "response.output_text.done", "item_id": "msg-test", "output_index": 0, "content_index": 0, "text": "responses runtime works"})
		send(map[string]any{"type": "response.output_item.done", "output_index": 0, "item": item})
		send(map[string]any{"type": "response.completed", "response": map[string]any{"id": "resp-test", "model": "test-model", "status": "completed", "output": []any{item}, "usage": map[string]any{"input_tokens": 5, "output_tokens": 5, "total_tokens": 10, "input_tokens_details": map[string]any{"cached_tokens": 0}, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}}})
	}))
	defer server.Close()
	app, _, _ := setupPiChat(t, server.URL)
	defer app.ClosePiChatRuntime("")
	models, _, _ := readPiObject(filepath.Join(piRoot(), "models.json"))
	var providers map[string]map[string]json.RawMessage
	_ = json.Unmarshal(models["providers"], &providers)
	providers[providerID]["api"] = toRaw("openai-responses")
	models["providers"] = toRaw(providers)
	data, _ := json.Marshal(models)
	if err := atomicWrite(filepath.Join(piRoot(), "models.json"), data); err != nil {
		t.Fatal(err)
	}
	config, err := app.LoadPiChatConfig()
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan PiChatRuntimeEvent, 512)
	app.piRuntimeEmit = func(event PiChatRuntimeEvent) { events <- event }
	if _, err := app.OpenPiChatRuntime("responses", config.ID); err != nil {
		t.Fatal(err)
	}
	if err := app.PiChatRuntimeCommand("responses", `{"type":"run","id":"response","history":[],"message":{"role":"user","content":[{"type":"text","text":"hello"}],"timestamp":1}}`); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case packet := <-events:
			var record map[string]json.RawMessage
			_ = json.Unmarshal(packet.Event, &record)
			if rawString(record, "type") == "done" {
				if !strings.Contains(string(record["messages"]), "responses runtime works") || !strings.Contains(string(record["messages"]), `"stopReason":"stop"`) {
					t.Fatalf("Responses result: %s", packet.Event)
				}
				return
			}
			if rawString(record, "type") == "failed" || rawString(record, "type") == "closed" {
				t.Fatalf("Responses failure: %s", packet.Event)
			}
		case <-deadline:
			t.Fatal("Responses runtime timed out")
		}
	}
}
