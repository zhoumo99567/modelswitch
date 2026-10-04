package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type CLIInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}
type CLIState struct {
	Tools     []CLIInfo `json:"tools"`
	Directory string    `json:"directory"`
}

func cliName(id string) (string, error) {
	switch id {
	case "pi":
		return "pi CLI", nil
	case "codex":
		return "Codex CLI", nil
	default:
		return "", errors.New("不支持的 CLI")
	}
}
func resolveCLI(id string) (string, error) {
	if _, err := cliName(id); err != nil {
		return "", err
	}
	if file, err := exec.LookPath(id); err == nil {
		return filepath.Abs(file)
	}
	home, _ := os.UserHomeDir()
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, ".local", "bin"), filepath.Join(home, ".npm-global", "bin"), filepath.Join(home, ".bun", "bin")}
	if runtime.GOOS == "windows" {
		dirs = []string{filepath.Join(os.Getenv("APPDATA"), "npm"), filepath.Join(home, ".bun", "bin")}
	}
	for _, dir := range dirs {
		names := []string{id}
		if runtime.GOOS == "windows" {
			names = []string{id + ".exe", id + ".cmd", id + ".bat"}
		}
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
				return candidate, nil
			}
		}
	}
	// Finder launches apps with a reduced PATH; the login shell knows version-manager installs.
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/zsh"
		}
		out, err := exec.CommandContext(ctx, shell, "-lic", "command -v "+id).Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			candidate := strings.TrimSpace(lines[len(lines)-1])
			if filepath.IsAbs(candidate) {
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
					return candidate, nil
				}
			}
		}
		if id == "codex" {
			for _, bundle := range []string{"/Applications/ChatGPT.app", filepath.Join(home, "Applications", "ChatGPT.app")} {
				candidate := filepath.Join(bundle, "Contents", "Resources", "codex-cli", "CodexCLI.app", "Contents", "MacOS", "codex")
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
					return candidate, nil
				}
			}
		}
	}
	return "", fmt.Errorf("未找到 %s，请先安装并确保命令在 PATH 中。", id)
}
func (a *App) LoadCLIState() CLIState {
	home, _ := os.UserHomeDir()
	state := CLIState{Directory: home, Tools: []CLIInfo{}}
	target := "chatgpt"
	if store, err := readStore(); err == nil {
		target = selectedTarget(store)
	}
	ids := []string{"codex"}
	if target == "pi" {
		ids = []string{"pi"}
	}
	for _, id := range ids {
		name, _ := cliName(id)
		path, err := resolveCLI(id)
		info := CLIInfo{ID: id, Name: name, Command: id, Installed: err == nil, Path: path}
		if err != nil {
			info.Error = err.Error()
		}
		state.Tools = append(state.Tools, info)
	}
	return state
}
func cliDirectory(directory string) (string, error) {
	if directory == "" {
		directory, _ = os.UserHomeDir()
	}
	if strings.ContainsAny(directory, "\x00\r\n") {
		return "", errors.New("工作目录包含不支持的字符")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return "", errors.New("工作目录不存在或不是文件夹")
	}
	return directory, nil
}
func (a *App) ChooseCLIDirectory(directory string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("目录选择需要在应用中使用")
	}
	initial, err := cliDirectory(directory)
	if err != nil {
		initial, _ = os.UserHomeDir()
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择 CLI 工作目录", DefaultDirectory: initial})
}
func (a *App) LaunchCLI(id, directory string) error {
	path, err := resolveCLI(id)
	if err != nil {
		return err
	}
	directory, err = cliDirectory(directory)
	if err != nil {
		return err
	}
	if err = launchCLIInTerminal(path, directory); err != nil {
		return fmt.Errorf("启动 CLI 失败：%w", err)
	}
	return nil
}
func quoteCLIShell(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func cliShellScript(path, directory string) string {
	return "export PATH=" + quoteCLIShell(filepath.Dir(path)) + ":\"$PATH\"; cd -- " + quoteCLIShell(directory) + " && " + quoteCLIShell(path)
}
