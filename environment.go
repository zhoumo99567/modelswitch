package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const minimumNodeVersion = "22.19.0"

type DependencyInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Ready     bool   `json:"ready"`
	Managed   bool   `json:"managed"`
	Error     string `json:"error"`
}

type EnvironmentState struct {
	Platform          string           `json:"platform"`
	Architecture      string           `json:"architecture"`
	InstallDirectory  string           `json:"installDirectory"`
	NodeInstallMethod string           `json:"nodeInstallMethod"`
	Tools             []DependencyInfo `json:"tools"`
}

type DependencyInstallState struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Progress int    `json:"progress"`
	Error    string `json:"error"`
}

func managedToolsRoot() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "ModelSwitcher", "tools")
}

func managedNodeDirectory() string {
	root := managedToolsRoot()
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "node-version"))
	version := strings.TrimSpace(string(data))
	if err != nil || !managedNodeVersion.MatchString(version) {
		return ""
	}
	return filepath.Join(root, "runtimes", version)
}

func managedNodeBin() string {
	dir := managedNodeDirectory()
	if dir == "" {
		return ""
	}
	if runtime.GOOS != "windows" {
		return filepath.Join(dir, "bin")
	}
	return dir
}

func managedNpmPrefix() string {
	if root := managedToolsRoot(); root != "" {
		return filepath.Join(root, "npm")
	}
	return ""
}

func managedNpmBin() string {
	prefix := managedNpmPrefix()
	if prefix == "" {
		return ""
	}
	if runtime.GOOS != "windows" {
		return filepath.Join(prefix, "bin")
	}
	return prefix
}

func toolInDirectory(dir, id string) string {
	if dir == "" {
		return ""
	}
	names := []string{id}
	if runtime.GOOS == "windows" {
		names = []string{id + ".exe", id + ".cmd", id + ".bat"}
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
			return path
		}
	}
	return ""
}

func loginShellTool(id string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	out, err := exec.CommandContext(ctx, shell, "-lic", "command -v "+id).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	path := strings.TrimSpace(lines[len(lines)-1])
	if filepath.IsAbs(path) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path
		}
	}
	return ""
}

func resolveNode() (string, error) {
	if path := toolInDirectory(managedNodeBin(), "node"); path != "" {
		return path, nil
	}
	// Versioned Homebrew formulae are keg-only, so their bin directory may not
	// appear in Finder's PATH even immediately after a successful install.
	if runtime.GOOS == "darwin" {
		for _, dir := range []string{"/opt/homebrew/opt/node@24/bin", "/usr/local/opt/node@24/bin"} {
			if path := toolInDirectory(dir, "node"); path != "" {
				return path, nil
			}
		}
	}
	if path, err := exec.LookPath("node"); err == nil {
		return filepath.Abs(path)
	}
	home, _ := os.UserHomeDir()
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, ".local", "bin")}
	if runtime.GOOS == "windows" {
		dirs = []string{filepath.Join(os.Getenv("ProgramFiles"), "nodejs"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "nodejs")}
	}
	for _, dir := range dirs {
		if path := toolInDirectory(dir, "node"); path != "" {
			return path, nil
		}
	}
	if path := loginShellTool("node"); path != "" {
		return path, nil
	}
	return "", errors.New("未找到 Node.js")
}

// Invoke npm's JavaScript entry directly, avoiding Windows .cmd/.ps1 policy issues.
func resolveNpmScript(node string) (string, error) {
	roots := []string{filepath.Dir(node)}
	if real, err := filepath.EvalSymlinks(node); err == nil {
		roots = append(roots, filepath.Dir(real))
	}
	for _, name := range []string{"npm", "npm.cmd"} {
		if path, err := exec.LookPath(name); err == nil {
			roots = append(roots, filepath.Dir(path))
			if real, err := filepath.EvalSymlinks(path); err == nil {
				roots = append(roots, filepath.Dir(real))
			}
		}
	}
	if path := loginShellTool("npm"); path != "" {
		roots = append(roots, filepath.Dir(path))
		if real, err := filepath.EvalSymlinks(path); err == nil {
			roots = append(roots, filepath.Dir(real))
		}
	}
	for _, root := range roots {
		for _, path := range []string{filepath.Join(root, "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(root, "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(root, "npm-cli.js")} {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return filepath.Clean(path), nil
			}
		}
	}
	return "", errors.New("未找到 npm，安装 Node.js 可同时安装 npm")
}

func dependencyPathPrefix(path string) string {
	parts := []string{}
	nodeDir := ""
	if node, err := resolveNode(); err == nil {
		nodeDir = filepath.Dir(node)
	}
	for _, dir := range []string{nodeDir, filepath.Dir(path), managedNpmBin()} {
		if dir != "" && dir != "." {
			parts = append(parts, dir)
		}
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func dependencyEnvironment(path string) []string {
	result := []string{}
	for _, entry := range os.Environ() {
		if !strings.EqualFold(strings.SplitN(entry, "=", 2)[0], "PATH") {
			result = append(result, entry)
		}
	}
	return append(result, "PATH="+dependencyPathPrefix(path)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func dependencyCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" && (strings.EqualFold(filepath.Ext(path), ".cmd") || strings.EqualFold(filepath.Ext(path), ".bat")) {
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
		command := "& " + quote(path)
		for _, arg := range args {
			command += " " + quote(arg)
		}
		command += "; exit $LASTEXITCODE"
		units := utf16.Encode([]rune(command))
		data := make([]byte, len(units)*2)
		for i, unit := range units {
			binary.LittleEndian.PutUint16(data[i*2:], unit)
		}
		args = []string{"-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(data)}
		path = "powershell.exe"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = hiddenCommand(path).SysProcAttr
	cmd.Env = dependencyEnvironment(path)
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

var nodeReleaseVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var managedNodeVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$`)

func compatibleNodeVersion(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	major, e1 := strconv.Atoi(parts[0])
	minor, e2 := strconv.Atoi(parts[1])
	_, e3 := strconv.Atoi(parts[2])
	return e1 == nil && e2 == nil && e3 == nil && (major > 22 || major == 22 && minor >= 19)
}

func detectDependency(id string) DependencyInfo {
	names := map[string]string{"node": "Node.js", "npm": "npm", "pi": "pi agent", "codex": "Codex CLI"}
	info := DependencyInfo{ID: id, Name: names[id]}
	var err error
	args := []string{"--version"}
	command := ""
	if id == "node" || id == "npm" {
		command, err = resolveNode()
		info.Path = command
		if err == nil && id == "npm" {
			info.Path, err = resolveNpmScript(command)
			args = []string{info.Path, "--version"}
		}
	} else {
		info.Path, err = resolveCLI(id)
		command = info.Path
	}
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Installed = true
	if rel, err := filepath.Rel(managedToolsRoot(), info.Path); err == nil {
		info.Managed = rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := dependencyCommand(ctx, command, args...).Output()
	if err != nil {
		info.Error = "无法运行版本检测，请安装或修复依赖"
		return info
	}
	info.Version = strings.TrimSpace(string(out))
	if index := strings.IndexByte(info.Version, '\n'); index >= 0 {
		info.Version = strings.TrimSpace(info.Version[:index])
	}
	info.Ready = info.Version != ""
	if id == "node" && !compatibleNodeVersion(info.Version) {
		info.Ready = false
		info.Error = "需要 Node.js " + minimumNodeVersion + " 或更高版本"
	}
	return info
}

func (a *App) DetectEnvironment() EnvironmentState {
	state := EnvironmentState{Platform: runtime.GOOS, Architecture: runtime.GOARCH, InstallDirectory: managedToolsRoot(), Tools: make([]DependencyInfo, 4)}
	_, state.NodeInstallMethod = platformNodeInstaller()
	var wg sync.WaitGroup
	for i, id := range []string{"node", "npm", "pi", "codex"} {
		wg.Add(1)
		go func(i int, id string) { defer wg.Done(); state.Tools[i] = detectDependency(id) }(i, id)
	}
	wg.Wait()
	return state
}

func (a *App) GetDependencyInstallState() DependencyInstallState {
	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
	return a.dependencyJob
}

func (a *App) dependencyProgress(message string, progress int) {
	a.dependencyMu.Lock()
	defer a.dependencyMu.Unlock()
	a.dependencyJob.Message = message
	a.dependencyJob.Progress = progress
}

func (a *App) StartDependencyInstall(id string) (DependencyInstallState, error) {
	if id != "node" && id != "npm" && id != "pi" && id != "codex" {
		return DependencyInstallState{}, errors.New("不支持的依赖")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return DependencyInstallState{}, errors.New("依赖安装仅支持 Windows 和 macOS")
	}
	if managedToolsRoot() == "" {
		return DependencyInstallState{}, errors.New("无法确定用户安装目录")
	}
	a.dependencyMu.Lock()
	if a.dependencyJob.Status == "running" {
		a.dependencyMu.Unlock()
		return DependencyInstallState{}, errors.New("已有安装任务正在运行")
	}
	a.dependencyJob = DependencyInstallState{ID: id, Status: "running", Message: "正在检查安装依赖…", Progress: 5}
	job := a.dependencyJob
	a.dependencyMu.Unlock()
	go func() {
		parent := a.ctx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
		defer cancel()
		err := installDependency(ctx, id, a.dependencyProgress)
		a.dependencyMu.Lock()
		defer a.dependencyMu.Unlock()
		if err != nil {
			a.dependencyJob.Status = "error"
			a.dependencyJob.Error = err.Error()
			a.dependencyJob.Message = "安装失败，可重试"
		} else {
			a.dependencyJob.Status = "success"
			a.dependencyJob.Progress = 100
			a.dependencyJob.Message = "安装完成，已通过检测"
		}
	}()
	return job, nil
}

func installDependency(ctx context.Context, id string, progress func(string, int)) error {
	return installDependencyWithNodeInstaller(ctx, id, progress, installPlatformNode)
}

func installDependencyWithNodeInstaller(ctx context.Context, id string, progress func(string, int), installNode func(context.Context, func(string, int)) error) error {
	nodeInfo := detectDependency("node")
	npmInfo := detectDependency("npm")
	if id == "node" || id == "npm" || !nodeInfo.Ready || !npmInfo.Ready {
		if err := installNode(ctx, progress); err != nil {
			return fmt.Errorf("安装 Node.js/npm 失败：%w", err)
		}
	}
	if id == "pi" || id == "codex" {
		node, err := resolveNode()
		if err != nil {
			return err
		}
		npm, err := resolveNpmScript(node)
		if err != nil {
			return err
		}
		prefix := managedNpmPrefix()
		if err = os.MkdirAll(prefix, 0700); err != nil {
			return err
		}
		pkg := "@openai/codex@latest"
		if id == "pi" {
			pkg = "@earendil-works/pi-coding-agent@1"
		}
		progress("正在下载并安装 "+id+"…", 75)
		install := func(force bool) (error, string) {
			args := []string{npm, "install", "--global", "--prefix", prefix, "--registry=https://registry.npmjs.org", "--no-audit", "--no-fund"}
			if force {
				// The managed prefix can contain a stale bin/pi or bin/codex from
				// an interrupted install. npm otherwise refuses to replace it with
				// EEXIST, while --force is safe here because this directory is owned
				// by Model Switcher rather than a system-wide npm prefix.
				args = append(args, "--force")
			}
			args = append(args, pkg)
			cmd := dependencyCommand(ctx, node, args...)
			cmd.Dir = prefix
			log := &limitedInstallOutput{}
			cmd.Stdout = log
			cmd.Stderr = log
			return cmd.Run(), log.String()
		}
		var output string
		if err, output = install(false); err != nil && strings.Contains(output, "EEXIST") {
			progress("检测到已有的 "+id+" CLI 文件，正在修复安装…", 82)
			err, output = install(true)
		}
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("安装超时或已取消，请重试：%w", ctx.Err())
			}
			return fmt.Errorf("npm 安装失败：%s", output)
		}
	}
	progress("正在验证安装结果…", 95)
	if info := detectDependency(id); !info.Ready {
		return fmt.Errorf("安装后检测未通过：%s", info.Error)
	}
	if id == "node" {
		if info := detectDependency("npm"); !info.Ready {
			return errors.New("Node.js 已安装，但 npm 检测未通过")
		}
	}
	if id == "pi" {
		if _, _, err := resolvePiChatSDK(); err != nil {
			return err
		}
	}
	return nil
}

// Keep bounded stderr/stdout for actionable errors without retaining entire npm logs.
type limitedInstallOutput struct {
	mu   sync.Mutex
	data []byte
}

func (w *limitedInstallOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	w.data = append(w.data, p...)
	if len(w.data) > 8192 {
		w.data = w.data[len(w.data)-8192:]
	}
	return n, nil
}
func (w *limitedInstallOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.TrimSpace(string(w.data))
}
