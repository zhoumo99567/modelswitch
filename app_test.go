package main

import "testing"

func TestStoredAPIKeyPrefersPlaintextAndHandlesMissingLegacySecret(t *testing.T) {
	if got := storedAPIKey(storedProfile{ProfileView: ProfileView{APIKey: "plain"}, Secret: "invalid"}); got != "plain" {
		t.Fatalf("plaintext API key was not preferred: %q", got)
	}
	if got := storedAPIKey(storedProfile{Secret: "invalid"}); got != "" {
		t.Fatalf("unreadable legacy secret should be empty, got %q", got)
	}
}
