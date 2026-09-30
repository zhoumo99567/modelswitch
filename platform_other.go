//go:build !windows && !darwin

package main

import (
	"errors"
	"os/exec"
)

func hiddenCommand(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }
func findChatGPT() (string, error)                        { return "", errors.New("此版本仅支持 Windows 和 macOS") }
func normalizeChatGPTTarget(string) (string, error) {
	return "", errors.New("此版本仅支持 Windows 和 macOS")
}
func resolveChatGPTTarget(string) (string, error) {
	return "", errors.New("此版本仅支持 Windows 和 macOS")
}
func isChatGPTRunning() bool     { return false }
func closeChatGPT() error        { return errors.New("此版本仅支持 Windows 和 macOS") }
func launchChatGPT(string) error { return errors.New("此版本仅支持 Windows 和 macOS") }
func revealConfig(string) error  { return errors.New("此版本仅支持 Windows 和 macOS") }
func openCodexDirectory(string) error {
	return errors.New("此版本仅支持 Windows 和 macOS")
}
