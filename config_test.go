package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalConfigAddsAuthForPlaintextAPIKey(t *testing.T) {
	profile := storedProfile{ProfileView: ProfileView{ID: "local-1", Name: "Local", BaseURL: "http://127.0.0.1:1234/v1", APIKey: "plain-key", SelectedModel: "model-a"}}
	data, err := localConfig(nil, profile, "/tmp/model-switcher")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "[model_providers.model_switcher_local.auth]") || !strings.Contains(text, "--model-switcher-token") {
		t.Fatalf("plaintext key did not create auth command: %s", text)
	}
}

func TestLocalConfigPinsCredentialProfileFile(t *testing.T) {
	profile := storedProfile{ProfileView: ProfileView{ID: "local-1", Name: "Local", BaseURL: "http://127.0.0.1:1234/v1", APIKey: "dummy-key", SelectedModel: "model-a"}}
	path := filepath.Join(t.TempDir(), "下载 folder's", "profiles.json")
	data, err := localConfig(nil, profile, "/tmp/model-switcher-token", path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := parseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	providers := config["model_providers"].(map[string]any)
	auth := providers[providerID].(map[string]any)["auth"].(map[string]any)
	want := []any{"--model-switcher-token", profile.ID, "--profiles", path}
	if !reflect.DeepEqual(auth["args"], want) {
		t.Fatalf("credential args: %#v", auth["args"])
	}
	if strings.Contains(string(data), profile.APIKey) {
		t.Fatal("configuration should reference the profile file, not embed its key")
	}
}
