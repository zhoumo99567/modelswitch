//go:build darwin

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
)

const keychainService = "ModelSwitcher API Key"

func protectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	account := hex.EncodeToString(buf)
	cmd := exec.Command("security", "add-generic-password", "-U", "-s", keychainService, "-a", account, "-w", value)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", errors.New(strings.TrimSpace(string(output)))
	}
	return "keychain:" + account, nil
}

func unprotectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "keychain:") || len(value) <= len("keychain:") {
		return "", errors.New("凭据未使用 macOS 钥匙串保存，请重新填写")
	}
	account := strings.TrimPrefix(value, "keychain:")
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-a", account, "-w").Output()
	if err != nil {
		return "", errors.New("无法从 macOS 钥匙串读取凭据，请重新填写")
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}
