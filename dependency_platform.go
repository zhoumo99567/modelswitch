package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func platformNodeInstaller() (string, string) {
	id, method := "", "官方 Node.js 用户目录安装包"
	switch runtime.GOOS {
	case "windows":
		id = "winget"
	case "darwin":
		id = "brew"
	default:
		return "", method
	}
	if path, err := exec.LookPath(id); err == nil {
		return path, id
	}
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if runtime.GOOS == "windows" {
		dirs = []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps")}
	}
	for _, dir := range dirs {
		if path := toolInDirectory(dir, id); path != "" {
			return path, id
		}
	}
	return "", method
}

func platformNodeInstallArgs(platform string) []string {
	if platform == "windows" {
		return []string{"install", "--id", "OpenJS.NodeJS.LTS", "--exact", "--source", "winget", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity", "--silent"}
	}
	return []string{"install", "node@24"}
}

func installPlatformNode(ctx context.Context, progress func(string, int)) error {
	path, method := platformNodeInstaller()
	return installPlatformNodeUsing(ctx, path, method, runtime.GOOS, progress, installManagedNode)
}

func installPlatformNodeUsing(ctx context.Context, path, method, platform string, progress func(string, int), fallback func(context.Context, func(string, int)) error) error {
	if path == "" {
		return fallback(ctx, progress)
	}
	progress("正在通过 "+method+" 安装 Node.js 与 npm…", 15)
	cmd := dependencyCommand(ctx, path, platformNodeInstallArgs(platform)...)
	log := &limitedInstallOutput{}
	cmd.Stdout = log
	cmd.Stderr = log
	if method == "brew" {
		cmd.Env = append(cmd.Env, "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1")
	}
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil && detectDependency("node").Ready && detectDependency("npm").Ready {
		return nil
	}
	// Package managers may be unavailable on a fresh machine, require elevation,
	// or leave GUI processes with stale PATH. The per-user official distribution
	// remains usable immediately by the launcher and Pi runtime in all cases.
	progress(method+" 未能完成安装，正在改用官方用户目录安装包…", 10)
	if fallbackErr := fallback(ctx, progress); fallbackErr != nil {
		return fmt.Errorf("%s 安装未通过；用户目录安装也失败：%w", method, fallbackErr)
	}
	return nil
}
