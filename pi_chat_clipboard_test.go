package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiChatClipboardReadsFilesAndReportsUnreadablePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "说明.txt")
	if err := os.WriteFile(path, []byte("文件正文\n你好"), 0600); err != nil {
		t.Fatal(err)
	}
	result := readPiChatClipboardFiles([]string{path, dir, filepath.Join(dir, "missing.txt")})
	if len(result.Files) != 1 || result.Files[0].Name != "说明.txt" || string(result.Files[0].Data) != "文件正文\n你好" {
		t.Fatalf("unexpected files: %#v", result)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("missing errors: %#v", result.Errors)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), dir) {
		t.Fatal("clipboard response exposed an absolute file path")
	}
}

func TestPiChatClipboardEnforcesSizeAndCountLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxPiChatAttachmentBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPiChatClipboardFile(path); err == nil {
		t.Fatal("oversized file was accepted")
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 7<<20), 0600); err != nil {
		t.Fatal(err)
	}
	result := readPiChatClipboardFiles([]string{path, path})
	if len(result.Files) != 1 || len(result.Errors) != 1 {
		t.Fatal("total size limit not enforced")
	}
	if err := os.WriteFile(path, []byte("small"), 0600); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, maxPiChatAttachments+1)
	for index := range paths {
		paths[index] = path
	}
	result = readPiChatClipboardFiles(paths)
	if len(result.Files) != maxPiChatAttachments || len(result.Errors) != 1 {
		t.Fatal("count limit not enforced")
	}
}

func TestPiChatAcceptsImageSizedRequestsAndRejectsOversizedHistory(t *testing.T) {
	received := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		received <- len(body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	app, config, events := setupPiChat(t, server.URL)
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("A", 5<<20) + `"}}]}]}`
	if err := app.StartPiChat(PiChatRequest{ID: "image", ConfigID: config.ID, Body: body}); err != nil {
		t.Fatal(err)
	}
	for awaitPiChatEvent(t, events).Type != "end" {
	}
	if length := <-received; length != len(body) {
		t.Fatalf("image payload changed: %d", length)
	}
	if err := app.StartPiChat(PiChatRequest{ID: "too-large", ConfigID: config.ID, Body: strings.Repeat("x", (32<<20)+1)}); err == nil {
		t.Fatal("oversized conversation was accepted")
	}
}
