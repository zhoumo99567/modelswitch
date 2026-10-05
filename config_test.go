package main

import (
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
