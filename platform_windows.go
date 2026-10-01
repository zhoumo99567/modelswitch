//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
func powershell(script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Output()
}
func findChatGPT() (string, error) {
	// Prefer an actual executable path for display and native file selection.
	// Auto-detected paths are resolved again so Store updates cannot stale them.
	scripts := []string{
		"[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; $packages=Get-AppxPackage -ErrorAction SilentlyContinue | Where-Object {$_.Name -like 'OpenAI.Codex*' -or $_.Name -like 'OpenAI.ChatGPT*'}; foreach($p in $packages){foreach($relative in @('app\\ChatGPT.exe','ChatGPT.exe')){$candidate=Join-Path $p.InstallLocation $relative; if(Test-Path -LiteralPath $candidate -PathType Leaf){$candidate; exit}}}",
		"[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; $p=Get-Process -Name ChatGPT -ErrorAction SilentlyContinue | Select-Object -First 1; if($p){$p.Path}",
		"[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; $a=Get-StartApps | Where-Object {$_.Name -match 'ChatGPT' -or $_.AppID -like 'OpenAI.Codex_*!*' -or $_.AppID -like 'OpenAI.ChatGPT*!*'} | Select-Object -First 1; if($a){$a.AppID}",
	}
	for _, script := range scripts {
		out, err := powershell(script)
		candidate := strings.TrimSpace(strings.TrimPrefix(string(out), "\ufeff"))
		if err == nil && candidate != "" {
			return candidate, nil
		}
	}
	return "", errors.New("未检测到官方 ChatGPT 应用；请安装桌面应用，或在设置中填写 ChatGPT.exe 路径")
}
func normalizeChatGPTTarget(target string) (string, error) {
	target = strings.TrimSpace(os.ExpandEnv(target))
	target = strings.Trim(target, "\"'")
	if target == "" {
		return "", nil
	}
	if strings.Contains(target, "!") {
		return target, nil
	}
	// Let users type ChatGPT.exe or ChatGPT instead of locating the protected
	// WindowsApps path manually.
	if !strings.ContainsAny(target, `\\/`) {
		if strings.EqualFold(target, "ChatGPT.exe") || strings.EqualFold(target, "ChatGPT") {
			return findChatGPT()
		}
		return "", errors.New("请输入 ChatGPT.exe 的完整路径，或点击“自动查找”")
	}
	target = filepath.Clean(target)
	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		candidate := filepath.Join(target, "app", "ChatGPT.exe")
		if appInfo, appErr := os.Stat(candidate); appErr == nil && !appInfo.IsDir() {
			return candidate, nil
		}
	}
	if err != nil || info.IsDir() {
		return "", errors.New("ChatGPT 路径不存在或不是可执行文件")
	}
	return target, nil
}
func resolveChatGPTTarget(configured string) (string, error) {
	if configured != "" {
		// Older builds stored the Store AppID. Keep using it for launching, but
		// show users the real executable path whenever the package exposes one.
		if strings.Contains(configured, "!") {
			if resolved, err := findChatGPT(); err == nil && !strings.Contains(resolved, "!") {
				return resolved, nil
			}
		}
		return normalizeChatGPTTarget(configured)
	}
	return findChatGPT()
}
func isChatGPTRunning() bool {
	out, err := hiddenCommand("tasklist.exe", "/FI", "IMAGENAME eq ChatGPT.exe", "/FO", "CSV", "/NH").Output()
	return err == nil && bytes.Contains(bytes.ToLower(out), []byte("\"chatgpt.exe\""))
}
func closeChatGPT() error {
	// The app asks this tool for its key, and force-killing the app orphans those
	// helper processes: they were still seen an hour later. Stop them first.
	const script = "$ErrorActionPreference='SilentlyContinue';" +
		"$apps=@(Get-Process -Name ChatGPT); foreach($p in $apps){if($p.MainWindowHandle -ne 0){[void]$p.CloseMainWindow()}};" +
		"$until=(Get-Date).AddSeconds(5); do { $left=@(Get-Process -Name ChatGPT); if($left.Count -eq 0){exit 0}; Start-Sleep -Milliseconds 250 } while((Get-Date) -lt $until);" +
		"$all=@(Get-CimInstance Win32_Process);" +
		"$helper=@($all | Where-Object { $_.Name -like 'ModelSwitcher*' -and $_.CommandLine -like '*--model-switcher-token*' });" +
		"foreach($h in $helper){Stop-Process -Id $h.ProcessId -Force};" +
		"foreach($p in @(Get-Process -Name ChatGPT)){Stop-Process -Id $p.Id -Force};" +
		"Start-Sleep -Milliseconds 500; if(@(Get-Process -Name ChatGPT).Count -eq 0){exit 0}; exit 2"
	if _, err := powershell(script); err != nil {
		return errors.New("ChatGPT 无法退出。请手动结束 ChatGPT.exe 后再点击切换；配置尚未修改")
	}
	return nil
}
func storeChatGPTAppID() string {
	const script = "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;" +
		"$p=Get-AppxPackage -ErrorAction SilentlyContinue | Where-Object {$_.Name -like 'OpenAI.Codex*' -or $_.Name -like 'OpenAI.ChatGPT*'} | Select-Object -First 1;" +
		"if($p){$a=Get-StartApps | Where-Object {$_.AppID -like ($p.PackageFamilyName+'!*')} | Select-Object -First 1; if($a){$a.AppID}}"
	out, err := powershell(script)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(string(out), "\ufeff"))
}

func insideWindowsApps(target string) bool {
	root := filepath.Join(os.Getenv("ProgramFiles"), "WindowsApps")
	if !strings.EqualFold(filepath.Base(root), "WindowsApps") {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(root)+string(filepath.Separator))
}

func launchChatGPT(target string) error {
	// Store builds have to be activated through their AUMID. Starting the packaged
	// executable directly is not a supported activation and frequently does nothing,
	// especially right after the previous instance was killed while switching.
	appID := ""
	if strings.Contains(target, "!") {
		appID = target
	} else if insideWindowsApps(target) {
		appID = storeChatGPTAppID()
	}
	launch := func() error {
		var cmd *exec.Cmd
		switch {
		case appID != "":
			cmd = hiddenCommand("explorer.exe", "shell:AppsFolder\\"+appID)
		case strings.HasSuffix(strings.ToLower(target), ".lnk"):
			cmd = hiddenCommand("explorer.exe", target)
		default:
			cmd = exec.Command(target)
		}
		return cmd.Start()
	}
	// Activation is dropped occasionally, so keep issuing it while waiting.
	var startErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if isChatGPTRunning() {
			return nil
		}
		if err := launch(); err != nil {
			startErr = err
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if isChatGPTRunning() {
				return nil
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	if startErr != nil {
		return fmt.Errorf("无法启动 ChatGPT：%w", startErr)
	}
	return errors.New("ChatGPT 启动超时，请手动打开应用")
}
func revealConfig(path string) error {
	return hiddenCommand("explorer.exe", "/select,"+path).Start()
}
func openCodexDirectory(path string) error {
	return hiddenCommand("explorer.exe", path).Start()
}
