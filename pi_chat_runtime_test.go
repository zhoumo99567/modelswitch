package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPiChatRuntimeLoadsGlobalSkillsAndExecutesExtensions(t *testing.T) {
	if _, _, err := resolvePiChatSDK(); err != nil {
		t.Skipf("installed Pi SDK required: %v", err)
	}
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, string(data))
		n := len(requests)
		mu.Unlock()
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer unit-test-secret" {
			t.Errorf("runtime lost applied connection: path=%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			_, _ = io.WriteString(w, `data: {"id":"probe","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"probe-1","type":"function","function":{"name":"runtime_probe","arguments":"{}"}}]},"finish_reason":null}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
		} else {
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"global runtime works\"},\"finish_reason\":\"stop\"}]}\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	app, config, _ := setupPiChat(t, server.URL)
	memory, err := app.CreateMemory("soul.md", "GLOBAL_MEMORY_ORIGINAL\n", "global")
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(piRoot(), "skills", "global-probe", "SKILL.md"), []byte("---\nname: global-probe\ndescription: A global skill loaded by the embedded conversation.\n---\nRead the global probe.\n")); err != nil {
		t.Fatal(err)
	}
	extension := `export default function(pi) {
 pi.on('session_start', (_, ctx) => ctx.ui.notify('global-session-start'));
 pi.on('before_agent_start', event => ({systemPrompt: event.systemPrompt + '\nGLOBAL_EXTENSION_PROMPT'}));
 pi.registerTool({name:'runtime_probe', label:'Global probe', description:'Returns the global extension result', parameters:{type:'object',properties:{}}, execute: async () => ({content:[{type:'text',text:'GLOBAL_TOOL_EXECUTED'}],details:{}})});
 pi.registerCommand('runtime-dialog', {description:'Confirm in the host UI', handler:async (_,ctx) => {const yes=await ctx.ui.confirm('Global extension','Continue?');ctx.ui.notify(yes ? 'DIALOG_CONFIRMED' : 'DIALOG_CANCELLED');}});
}`
	if err := atomicWrite(filepath.Join(piRoot(), "extensions", "global-probe.js"), []byte(extension)); err != nil {
		t.Fatal(err)
	}
	events := make(chan PiChatRuntimeEvent, 512)
	app.piRuntimeEmit = func(event PiChatRuntimeEvent) { events <- event }
	t.Cleanup(func() { app.ClosePiChatRuntime("") })
	info, err := app.OpenPiChatRuntime("runtime-test", config.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Skills) != 1 || info.Skills[0].Name != "global-probe" || !strings.Contains(strings.Join(info.Extensions, " "), "global-probe.js") || !strings.Contains(strings.Join(info.Tools, " "), "runtime_probe") {
		t.Fatalf("missing global resources: %#v", info)
	}
	if len(info.Diagnostics) != 0 {
		t.Fatalf("resource diagnostics: %v", info.Diagnostics)
	}
	send := func(value any) {
		t.Helper()
		data, _ := json.Marshal(value)
		if err := app.PiChatRuntimeCommand("runtime-test", string(data)); err != nil {
			t.Fatal(err)
		}
	}
	var reloadedInfo PiChatRuntimeInfo
	waitDone := func(id string, confirm bool) map[string]json.RawMessage {
		t.Helper()
		deadline := time.After(15 * time.Second)
		for {
			select {
			case event := <-events:
				var record map[string]json.RawMessage
				_ = json.Unmarshal(event.Event, &record)
				switch rawString(record, "type") {
				case "resources":
					_ = json.Unmarshal(record["info"], &reloadedInfo)
				case "closed", "failed", "fatal":
					t.Fatalf("runtime failure: %s", event.Event)
				case "ui":
					if rawString(record, "method") != "confirm" || !confirm {
						t.Fatalf("unexpected UI request: %s", event.Event)
					}
					send(map[string]any{"type": "ui_response", "id": rawString(record, "id"), "value": true})
				case "done":
					if rawString(record, "runID") == id {
						return record
					}
				}
			case <-deadline:
				t.Fatal("runtime prompt timed out")
			}
		}
	}
	prompt := func(id, text string, history any) {
		send(map[string]any{"type": "run", "id": id, "history": history, "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": text}}, "timestamp": 1}})
	}
	prompt("tools", "Use runtime_probe", []any{})
	result := waitDone("tools", false)
	if !strings.Contains(string(result["messages"]), "global runtime works") || !strings.Contains(string(result["messages"]), "GLOBAL_TOOL_EXECUTED") {
		t.Fatalf("extension tool did not execute: %s", result["messages"])
	}
	mu.Lock()
	if len(requests) != 2 || !strings.Contains(requests[0], "GLOBAL_EXTENSION_PROMPT") || !strings.Contains(requests[0], "GLOBAL_MEMORY_ORIGINAL") || !strings.Contains(requests[0], "global-probe") || !strings.Contains(requests[1], "GLOBAL_TOOL_EXECUTED") {
		t.Errorf("global resources not passed to model: %v", requests)
	}
	mu.Unlock()
	prompt("dialog", "/runtime-dialog", result["messages"])
	dialog := waitDone("dialog", true)
	if rawString(dialog, "disposition") != "handled" {
		t.Fatalf("extension command not handled: %v", dialog)
	}
	if err := app.WriteMemory(memory.Path, "GLOBAL_MEMORY_UPDATED\n"); err != nil {
		t.Fatal(err)
	}
	// Roll back the failed/current turn using actual SessionManager history, then
	// expand a skill command through the SDK rather than through UI text parsing.
	prompt("skill", "/skill:global-probe explain", []any{})
	waitDone("skill", false)
	mu.Lock()
	if len(requests) != 3 || !strings.Contains(requests[2], "Read the global probe.") || !strings.Contains(requests[2], "GLOBAL_MEMORY_UPDATED") || strings.Contains(requests[2], "GLOBAL_MEMORY_ORIGINAL") || strings.Contains(requests[2], "Use runtime_probe") {
		t.Error(fmt.Sprintf("skill expansion or history rollback failed: %v", requests))
	}
	mu.Unlock()
	// A new global skill is discovered without discarding this conversation.
	if err := atomicWrite(filepath.Join(piRoot(), "skills", "after-reload", "SKILL.md"), []byte("---\nname: after-reload\ndescription: Loaded after a runtime reload.\n---\nNew skill.\n")); err != nil {
		t.Fatal(err)
	}
	prompt("reload", "/reload", result["messages"])
	if result := waitDone("reload", false); rawString(result, "disposition") != "handled" {
		t.Fatalf("reload: %v", result)
	}
	if len(reloadedInfo.Skills) != 2 {
		t.Fatalf("reload did not discover the new global skill: %#v", reloadedInfo)
	}
}

func TestPiChatRuntimeRecoversEmptyAssistantAfterTool(t *testing.T) {
	if _, _, err := resolvePiChatSDK(); err != nil {
		t.Skipf("installed Pi SDK required: %v", err)
	}
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		n := requests
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		switch n {
		case 1:
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"recovery-call\",\"type\":\"function\",\"function\":{\"name\":\"recovery_probe\",\"arguments\":\"{}\"}}]},\"finish_reason\":null}]}\n\n")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
		case 2:
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":\"stop\"}]}\n\n")
		default:
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"recovered response\"},\"finish_reason\":\"stop\"}]}\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	app, config, _ := setupPiChat(t, server.URL)
	if err := atomicWrite(filepath.Join(piRoot(), "extensions", "recovery-probe.js"), []byte(`export default function(pi) { pi.registerTool({name:'recovery_probe', label:'Recovery probe', description:'Returns a recovery result', parameters:{type:'object',properties:{}}, execute: async () => ({content:[{type:'text',text:'TOOL_RESULT'}],details:{}})}); }`)); err != nil {
		t.Fatal(err)
	}
	events := make(chan PiChatRuntimeEvent, 1024)
	app.piRuntimeEmit = func(event PiChatRuntimeEvent) { events <- event }
	t.Cleanup(func() { app.ClosePiChatRuntime("") })
	if _, err := app.OpenPiChatRuntime("recovery", config.ID); err != nil {
		t.Fatal(err)
	}
	command, _ := json.Marshal(map[string]any{"type": "run", "id": "recover-run", "history": []any{}, "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "recover this task"}}, "timestamp": 1}})
	if err := app.PiChatRuntimeCommand("recovery", string(command)); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(20 * time.Second)
	for {
		select {
		case packet := <-events:
			var record map[string]json.RawMessage
			_ = json.Unmarshal(packet.Event, &record)
			if rawString(record, "type") != "done" {
				continue
			}
			var outcome struct {
				Reason           string `json:"reason"`
				RecoveryAttempts int    `json:"recoveryAttempts"`
			}
			_ = json.Unmarshal(record["outcome"], &outcome)
			if outcome.RecoveryAttempts < 1 || outcome.Reason != "completed" || !strings.Contains(string(record["messages"]), "recovered response") {
				t.Fatalf("recovery outcome: %s", packet.Event)
			}
			mu.Lock()
			defer mu.Unlock()
			if requests < 3 {
				t.Fatalf("expected a recovery model request, got %d", requests)
			}
			return
		case <-deadline:
			t.Fatal("recovery run timed out")
		}
	}
}

func TestPiChatRuntimeStreamsImagesAndAcknowledgesAbort(t *testing.T) {
	if _, _, err := resolvePiChatSDK(); err != nil {
		t.Skipf("installed Pi SDK required: %v", err)
	}
	requestBodies := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		requestBodies <- string(data)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	app, _, _ := setupPiChat(t, server.URL)
	defer app.ClosePiChatRuntime("")
	models, _, _ := readPiObject(filepath.Join(piRoot(), "models.json"))
	var providers map[string]map[string]json.RawMessage
	_ = json.Unmarshal(models["providers"], &providers)
	var entries []map[string]json.RawMessage
	_ = json.Unmarshal(providers[providerID]["models"], &entries)
	entries[0]["input"] = toRaw([]string{"text", "image"})
	providers[providerID]["models"] = toRaw(entries)
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
	t.Cleanup(func() { app.ClosePiChatRuntime("") })
	if _, err := app.OpenPiChatRuntime("images", config.ID); err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	pngData := base64.StdEncoding.EncodeToString(imageData.Bytes())
	for turn := 0; turn < 2; turn++ {
		command := fmt.Sprintf(`{"type":"run","id":"image-%d","history":[],"message":{"role":"user","content":[{"type":"text","text":"image prompt"},{"type":"image","mimeType":"image/png","data":"%s"}],"timestamp":1}}`, turn, pngData)
		if err := app.PiChatRuntimeCommand("images", command); err != nil {
			t.Fatal(err)
		}
		select {
		case body := <-requestBodies:
			if !strings.Contains(body, "data:image/png;base64,") || !strings.Contains(body, "image prompt") {
				t.Fatalf("lost image content: %s", body)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("image request timed out")
		}
		deadline := time.After(10 * time.Second)
		streamed, aborted := false, false
		for !aborted {
			select {
			case packet := <-events:
				var record map[string]json.RawMessage
				_ = json.Unmarshal(packet.Event, &record)
				if rawString(record, "type") == "event" && strings.Contains(string(record["event"]), `"type":"message_update"`) && !streamed {
					streamed = true
					if err := app.PiChatRuntimeCommand("images", `{"type":"abort"}`); err != nil {
						t.Fatal(err)
					}
				}
				if rawString(record, "type") == "done" {
					if !strings.Contains(string(record["messages"]), `"stopReason":"aborted"`) {
						t.Fatalf("missing SDK abort: %s", record["messages"])
					}
					aborted = true
				}
				if rawString(record, "type") == "failed" || rawString(record, "type") == "closed" {
					t.Fatalf("runtime failure: %s", packet.Event)
				}
			case <-deadline:
				t.Fatal("stream/abort acknowledgement timed out")
			}
		}
		if !streamed {
			t.Fatal("no live assistant event before cancellation")
		}
	}
}

func TestPiChatRuntimeStartupDialogDoesNotBlockIPC(t *testing.T) {
	if _, _, err := resolvePiChatSDK(); err != nil {
		t.Skipf("installed Pi SDK required: %v", err)
	}
	app, config, _ := setupPiChat(t, "http://127.0.0.1:1")
	defer app.ClosePiChatRuntime("")
	if err := atomicWrite(filepath.Join(piRoot(), "extensions", "startup-dialog.js"), []byte(`export default function(pi) { pi.on('session_start', async (_,ctx) => { if(!await ctx.ui.confirm('Startup', 'Continue?')) throw new Error('Startup not confirmed'); }); }`)); err != nil {
		t.Fatal(err)
	}
	events := make(chan PiChatRuntimeEvent, 512)
	app.piRuntimeEmit = func(event PiChatRuntimeEvent) { events <- event }
	ready := make(chan error, 1)
	go func() { _, err := app.OpenPiChatRuntime("startup-ui", config.ID); ready <- err }()
	deadline := time.After(15 * time.Second)
	confirmed := false
	for {
		select {
		case packet := <-events:
			var record map[string]json.RawMessage
			_ = json.Unmarshal(packet.Event, &record)
			if rawString(record, "type") == "ui" {
				data, _ := json.Marshal(map[string]any{"type": "ui_response", "id": rawString(record, "id"), "value": true})
				if err := app.PiChatRuntimeCommand("startup-ui", string(data)); err != nil {
					t.Fatal(err)
				}
				confirmed = true
			}
		case err := <-ready:
			if err != nil {
				t.Fatal(err)
			}
			if !confirmed {
				t.Fatal("startup dialog was skipped")
			}
			return
		case <-deadline:
			t.Fatal("startup UI blocked runtime initialization")
		}
	}
}
