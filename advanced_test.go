package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMCPJSONSupportsCommonShapes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"servers":{"docs":{"command":"npx","transport":"stdio"},"remote":{"url":"https://example.test/mcp","enabled":false}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	servers, err := readMCPJSON(path, "pi", "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || servers[0].Name != "docs" || servers[1].Endpoint != "https://example.test/mcp" || servers[1].Enabled == nil || *servers[1].Enabled {
		t.Fatalf("unexpected MCP servers: %#v", servers)
	}
}

func TestReadMCPTOML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	content := "[mcp_servers.docs]\ncommand = \"npx\"\nargs = [\"-y\"]\n\n[mcp_servers.remote]\nurl = \"https://example.test/mcp\"\nenabled = false\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	servers, err := readMCPTOML(path, "codex", "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || servers[0].Name != "docs" || servers[0].Command != "npx" || servers[1].Endpoint != "https://example.test/mcp" {
		t.Fatalf("unexpected TOML MCP servers: %#v", servers)
	}
}

func TestLoadMCPServersIncludesCodexConfigTOML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte("[mcp_servers.docs]\ncommand = \"npx\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	servers, diagnostics := loadMCPServers([]RuntimeDocument{{Target: "codex", Scope: "global", Kind: "Codex config", Format: "toml", Path: path, Exists: true}})
	if len(diagnostics) != 0 || len(servers) != 1 || servers[0].Name != "docs" {
		t.Fatalf("unexpected Codex MCP result: servers=%#v diagnostics=%#v", servers, diagnostics)
	}
}

func TestValidateRuntimeContent(t *testing.T) {
	if err := validateRuntimeContent("settings.json", `{"defaultThinkingLevel":"high",}`); err != nil {
		t.Fatalf("JSONC should be accepted: %v", err)
	}
	if err := validateRuntimeContent("config.toml", "model = \"ok\"\n"); err != nil {
		t.Fatalf("TOML should be accepted: %v", err)
	}
	if err := validateRuntimeContent("settings.json", "not json"); err == nil {
		t.Fatal("invalid JSON should fail")
	}
}
