//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func hiddenCommand(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

func findChatGPT() (string, error) {
	// Spotlight finds the app regardless of whether it is in /Applications or
	// the user's Applications folder. Keep common paths as a fallback when
	// Spotlight indexing is disabled.
	if out, err := hiddenCommand("mdfind", "kMDItemFSName == 'ChatGPT.app' && kMDItemContentType == 'com.apple.application-bundle'").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if path := strings.TrimSpace(line); path != "" && strings.HasSuffix(path, ".app") {
				return path, nil
			}
		}
	}
	home, _ := os.UserHomeDir()
	for _, path := range []string{"/Applications/ChatGPT.app", filepath.Join(home, "Applications", "ChatGPT.app")} {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("未检测到官方 ChatGPT 应用；请安装桌面应用，或在设置中填写 ChatGPT.app 路径")
}

func normalizeChatGPTTarget(target string) (string, error) {
	target = strings.TrimSpace(os.ExpandEnv(target))
	if target == "" {
		return "", nil
	}
	// Accept an application name such as ChatGPT for users who installed it
	// outside the usual Applications folders. `open -a` resolves it for us.
	if !strings.Contains(target, string(filepath.Separator)) && !strings.HasSuffix(target, ".app") {
		return target, nil
	}
	target = filepath.Clean(target)
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() || !strings.HasSuffix(target, ".app") {
		return "", errors.New("ChatGPT 路径不存在或不是 .app 应用")
	}
	return target, nil
}

func resolveChatGPTTarget(configured string) (string, error) {
	if configured != "" {
		return normalizeChatGPTTarget(configured)
	}
	return findChatGPT()
}

func isChatGPTRunning() bool {
	return hiddenCommand("pgrep", "-x", "ChatGPT").Run() == nil
}

func closeChatGPT() error {
	if !isChatGPTRunning() {
		return nil
	}
	if err := hiddenCommand("osascript", "-e", `tell application "ChatGPT" to quit`).Run(); err != nil {
		return errors.New("无法请求 ChatGPT 退出，请先手动关闭应用；配置尚未修改")
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if !isChatGPTRunning() {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("ChatGPT 尚未退出，请保存正在进行的任务并完全退出应用，再点击切换；配置尚未修改")
}

func launchChatGPT(target string) error {
	if strings.HasSuffix(target, ".app") || strings.Contains(target, string(filepath.Separator)) {
		return hiddenCommand("open", target).Start()
	}
	return hiddenCommand("open", "-a", target).Start()
}
func revealConfig(path string) error {
	return hiddenCommand("open", "-R", path).Start()
}
func openCodexDirectory(path string) error {
	return hiddenCommand("open", path).Start()
}
