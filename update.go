package main

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var AppVersion = "0.1.0"

// A variable allows release builds in forks to inject their own repository URL.
var defaultUpdateManifestURL = "https://github.com/zhoumo99567/modelswitch/releases/latest/download/latest.json"

const officialUpdateManifestLimit = 1 << 20

var updateVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type UpdateArtifact struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature,omitempty"`
}

type UpdateManifest struct {
	Version   string         `json:"version"`
	Notes     string         `json:"notes,omitempty"`
	Published string         `json:"published_at,omitempty"`
	Windows   UpdateArtifact `json:"windows"`
	MacOS     UpdateArtifact `json:"macos"`
}

type UpdateInfo struct {
	Status          string `json:"status"`
	Message         string `json:"message"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	Notes           string `json:"notes,omitempty"`
	Published       string `json:"published,omitempty"`
	DownloadURL     string `json:"downloadUrl,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	Signature       string `json:"signature,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

func updateManifestURL() string {
	if value := strings.TrimSpace(os.Getenv("MODELSWITCHER_UPDATE_URL")); value != "" {
		return value
	}
	return defaultUpdateManifestURL
}

func (a *App) CheckForUpdate() (UpdateInfo, error) {
	result := UpdateInfo{Status: "unconfigured", Message: "更新源未配置，已跳过检查。", CurrentVersion: AppVersion}
	manifestURL := updateManifestURL()
	if manifestURL == "" {
		return result, nil
	}
	data, err := fetchURL(manifestURL, officialUpdateManifestLimit)
	if err != nil {
		return UpdateInfo{Status: "error", Message: "无法读取更新清单。", CurrentVersion: AppVersion}, err
	}
	var manifest UpdateManifest
	if err = json.Unmarshal(data, &manifest); err != nil || !validUpdateVersion(manifest.Version) {
		return UpdateInfo{Status: "error", Message: "更新清单格式无效。", CurrentVersion: AppVersion}, errors.New("更新清单格式无效")
	}
	artifact := manifest.Windows
	if runtime.GOOS == "darwin" {
		artifact = manifest.MacOS
	}
	if artifact.URL == "" || artifact.SHA256 == "" {
		return UpdateInfo{Status: "error", Message: "当前平台没有可用更新包。", CurrentVersion: AppVersion}, errors.New("当前平台没有可用更新包")
	}
	result = UpdateInfo{
		Status: "up-to-date", Message: "当前已经是最新版本。", CurrentVersion: AppVersion,
		LatestVersion: manifest.Version, Notes: manifest.Notes, Published: manifest.Published,
		DownloadURL: artifact.URL, SHA256: strings.ToLower(strings.TrimSpace(artifact.SHA256)), Signature: artifact.Signature,
	}
	if compareVersions(manifest.Version, AppVersion) > 0 {
		result.Status = "available"
		result.Message = "发现新版本 " + manifest.Version + "。"
		result.UpdateAvailable = true
	}
	return result, nil
}

func (a *App) StartUpdate() (UpdateInfo, error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.updateStarted {
		return UpdateInfo{Status: "applying", Message: "更新正在安装，程序即将重启。", CurrentVersion: AppVersion}, nil
	}
	info, err := a.CheckForUpdate()
	if err != nil {
		return info, err
	}
	if !info.UpdateAvailable {
		return info, nil
	}
	artifact, err := downloadUpdate(info)
	if err != nil {
		info.Status = "error"
		info.Message = err.Error()
		return info, err
	}
	if err = launchUpdateHelper(artifact); err != nil {
		_ = os.Remove(artifact)
		info.Status = "error"
		info.Message = err.Error()
		return info, err
	}
	info.Status = "applying"
	a.updateStarted = true
	info.Message = "更新包已校验，程序即将重启。"
	go func() {
		time.Sleep(300 * time.Millisecond)
		if a.ctx != nil {
			wailsruntime.Quit(a.ctx)
		}
	}()
	return info, nil
}

func downloadUpdate(info UpdateInfo) (string, error) {
	expected, err := hex.DecodeString(strings.TrimSpace(info.SHA256))
	if err != nil || len(expected) != sha256.Size {
		return "", errors.New("更新包 SHA-256 格式无效")
	}
	response, err := dependencyHTTP(context.Background(), info.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("下载更新包失败: %w", err)
	}
	defer response.Body.Close()
	file, err := os.CreateTemp("", "model-switcher-update-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	hash := sha256.New()
	const limit = 512 << 20
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", fmt.Errorf("下载更新包失败: %w", err)
	}
	if size > limit {
		return "", errors.New("更新包超过大小限制")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), strings.TrimSpace(info.SHA256)) {
		return "", errors.New("更新包 SHA-256 校验失败")
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	if info.Signature != "" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		if err = verifyUpdateSignature(data, info.Signature); err != nil {
			return "", err
		}
	}
	keep = true
	return path, nil
}

func verifyUpdateSignature(data []byte, encoded string) error {
	keyText := strings.TrimSpace(os.Getenv("MODELSWITCHER_UPDATE_PUBLIC_KEY"))
	if keyText == "" {
		return errors.New("更新清单包含签名，但程序未配置公钥")
	}
	publicKey, err := base64.StdEncoding.DecodeString(keyText)
	if err != nil {
		publicKey, err = hex.DecodeString(keyText)
	}
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return errors.New("更新公钥格式无效")
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), data, signature) {
		return errors.New("更新包签名校验失败")
	}
	return nil
}

func validUpdateVersion(value string) bool {
	return updateVersionPattern.MatchString(strings.TrimPrefix(strings.TrimSpace(value), "v"))
}

func compareVersions(left, right string) int {
	parts := func(value string) []string {
		value = strings.SplitN(strings.TrimPrefix(strings.TrimSpace(value), "v"), "+", 2)[0]
		return strings.SplitN(value, "-", 2)
	}
	if !validUpdateVersion(left) || !validUpdateVersion(right) {
		return 0
	}
	a, b := parts(left), parts(right)
	numeric := func(x, y string) int {
		if len(x) != len(y) {
			if len(x) > len(y) {
				return 1
			}
			return -1
		}
		return strings.Compare(x, y)
	}
	x, y := strings.Split(a[0], "."), strings.Split(b[0], ".")
	for i := range x {
		if cmp := numeric(x[i], y[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a) == 1 && len(b) == 1 {
		return 0
	}
	if len(a) == 1 {
		return 1
	}
	if len(b) == 1 {
		return -1
	}
	x, y = strings.Split(a[1], "."), strings.Split(b[1], ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		xNumber := strings.Trim(x[i], "0123456789") == ""
		yNumber := strings.Trim(y[i], "0123456789") == ""
		if xNumber != yNumber {
			if xNumber {
				return -1
			}
			return 1
		}
		cmp := strings.Compare(x[i], y[i])
		if xNumber {
			cmp = numeric(x[i], y[i])
		}
		if cmp != 0 {
			return cmp
		}
	}
	if len(x) > len(y) {
		return 1
	}
	if len(x) < len(y) {
		return -1
	}
	return 0
}

func launchUpdateHelper(artifact string) error {
	executable, err := os.Executable()
	if err != nil {
		return errors.New("无法定位当前程序")
	}
	helperPath, err := stageUpdateExecutable(executable, os.TempDir(), "model-switcher-updater-*.exe")
	if err != nil {
		return err
	}
	target := executable
	if runtime.GOOS == "darwin" {
		target = macOSBundlePath(executable)
	}
	log, err := os.OpenFile(target+".update.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_ = os.Remove(helperPath)
		return fmt.Errorf("无法创建更新日志: %w", err)
	}
	defer log.Close()
	cmd := hiddenCommand(helperPath, "--update-helper", target, artifact, strconv.Itoa(os.Getpid()))
	cmd.Dir = filepath.Dir(target)
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		os.Remove(helperPath)
		return errors.New("无法启动更新助手")
	}
	go cmd.Wait()
	return nil
}

func runUpdateHelper(args []string) error {
	if len(args) != 2 && len(args) != 3 {
		return errors.New("更新助手参数无效")
	}
	target, artifact := args[0], args[1]
	defer os.Remove(artifact)
	defer cleanupUpdateHelper(os.Args[0])
	if len(args) == 3 {
		pid, err := strconv.Atoi(args[2])
		if err != nil || pid <= 0 || pid == os.Getpid() {
			return errors.New("更新助手进程参数无效")
		}
		if err = waitForUpdateParent(pid, time.Minute); err != nil {
			return err
		}
	}
	if runtime.GOOS == "darwin" {
		return replaceMacOSBundle(target, artifact)
	}
	return replaceExecutable(target, artifact)
}

func replaceExecutable(target, artifact string) error {
	// Staging beside the destination makes the final rename work across volumes.
	staged, err := stageUpdateExecutable(artifact, filepath.Dir(target), ".model-switcher-update-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	return replaceUpdateTarget(target, staged, func(path string) error {
		// The updater helper is hidden, but the restarted GUI must be visible.
		cmd := exec.Command(path)
		cmd.Dir = filepath.Dir(path)
		if err := cmd.Start(); err != nil {
			return err
		}
		go cmd.Wait()
		return nil
	})
}

func stageUpdateExecutable(source, directory, pattern string) (string, error) {
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	path, keep := file.Name(), false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if _, err = io.Copy(file, input); err != nil {
		return "", err
	}
	if err = file.Chmod(0755); err != nil {
		return "", err
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

// The same transaction protects the Windows executable and the complete macOS
// bundle. A failed replacement or launch restores the old target, and a failed
// rollback reports the retained backup path instead of claiming success.
func replaceUpdateTarget(target, staged string, restart func(string) error) error {
	if !filepath.IsAbs(target) || !filepath.IsAbs(staged) || !isChildPath(filepath.Dir(target), staged) || filepath.Clean(target) == filepath.Clean(staged) {
		return errors.New("更新替换路径无效")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return errors.New("更新目标必须是程序文件或应用目录")
	}
	backupFile, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".backup-*")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	if err = backupFile.Close(); err != nil {
		return err
	}
	if err = os.Remove(backup); err != nil {
		return err
	}
	for i := 0; ; i++ {
		err = os.Rename(target, backup)
		if err == nil {
			break
		}
		if i >= 119 {
			return fmt.Errorf("无法替换旧版本，更新已取消: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	rollback := func(cause error) error {
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("更新失败 (%v)，无法恢复旧版本，备份保留在 %s: %w", cause, backup, err)
		}
		if err := os.Rename(backup, target); err != nil {
			return fmt.Errorf("更新失败 (%v)，无法恢复旧版本，备份保留在 %s: %w", cause, backup, err)
		}
		if err := restart(target); err != nil {
			return fmt.Errorf("更新失败，已恢复原版本，但无法重启 (%v): %w", err, cause)
		}
		return fmt.Errorf("更新失败，已恢复原版本: %w", cause)
	}
	if err = os.Rename(staged, target); err != nil {
		return rollback(err)
	}
	if err = restart(target); err != nil {
		return rollback(err)
	}
	if err = os.RemoveAll(backup); err != nil {
		fmt.Fprintf(os.Stderr, "新版本已启动，旧版备份保留在 %s: %v\n", backup, err)
	}
	return nil
}

func waitForUpdateParent(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		running, err := updateProcessRunning(pid)
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("等待程序退出超时，更新已取消")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func macOSBundlePath(executable string) string {
	if index := strings.Index(executable, ".app"+string(filepath.Separator)); index >= 0 {
		return executable[:index+len(".app")]
	}
	return executable
}

func replaceMacOSBundle(target, artifact string) error {
	stage, err := os.MkdirTemp(filepath.Dir(target), ".model-switcher-app-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	bundle, err := prepareMacOSUpdateBundle(artifact, stage)
	if err != nil {
		return err
	}
	return replaceUpdateTarget(target, bundle, func(path string) error {
		return exec.Command("/usr/bin/open", "-n", path).Run()
	})
}

func prepareMacOSUpdateBundle(artifact, stage string) (string, error) {
	name, err := validateMacOSUpdateArchive(artifact)
	if err != nil {
		return "", err
	}
	// ditto preserves bundle permissions, symbolic links and macOS metadata.
	if output, err := exec.Command("/usr/bin/ditto", "-x", "-k", artifact, stage).CombinedOutput(); err != nil {
		return "", fmt.Errorf("解压 macOS 更新包失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	bundle := filepath.Join(stage, name)
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	output, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleExecutable", plist).Output()
	if err != nil {
		return "", errors.New("macOS 更新包缺少有效的应用信息")
	}
	binName := strings.TrimSpace(string(output))
	if binName == "" || filepath.Base(binName) != binName {
		return "", errors.New("macOS 更新包的执行文件路径无效")
	}
	bin, err := os.Stat(filepath.Join(bundle, "Contents", "MacOS", binName))
	if err != nil || !bin.Mode().IsRegular() || bin.Mode().Perm()&0111 == 0 {
		return "", errors.New("macOS 更新包缺少可执行程序")
	}
	return bundle, nil
}

func validateMacOSUpdateArchive(artifact string) (string, error) {
	reader, err := zip.OpenReader(artifact)
	if err != nil {
		return "", errors.New("macOS 更新包不是有效的 ZIP")
	}
	defer reader.Close()
	var bundle string
	var size uint64
	links := make(map[string]string)
	paths := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		relative := filepath.Clean(filepath.FromSlash(name))
		if name == "" || strings.Contains(name, "\\") || strings.Contains(name, ":") || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.ToSlash(relative) != name {
			return "", errors.New("macOS 更新包包含非法路径")
		}
		if file.UncompressedSize64 > 1<<30 || size > 1<<30-file.UncompressedSize64 {
			return "", errors.New("macOS 更新包解压内容过大")
		}
		size += file.UncompressedSize64
		root := strings.Split(name, "/")[0]
		if root != "__MACOSX" {
			if !strings.HasSuffix(root, ".app") || (bundle != "" && bundle != root) {
				return "", errors.New("macOS 更新包必须包含唯一的 app")
			}
			bundle = root
		}
		paths = append(paths, relative)
		if file.Mode()&os.ModeSymlink != 0 {
			input, err := file.Open()
			if err != nil {
				return "", err
			}
			data, err := io.ReadAll(io.LimitReader(input, 4097))
			input.Close()
			link := string(data)
			resolved := filepath.Clean(filepath.Join(filepath.Dir(relative), filepath.FromSlash(link)))
			if err != nil || len(data) > 4096 || link == "" || strings.ContainsAny(link, "\\:\x00") || strings.HasPrefix(link, "/") || filepath.IsAbs(filepath.FromSlash(link)) || !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
				return "", errors.New("macOS 更新包包含非法链接")
			}
			links[relative] = link
		} else if !file.FileInfo().IsDir() && !file.Mode().IsRegular() {
			return "", errors.New("macOS 更新包包含不支持的文件")
		}
	}
	for _, path := range paths {
		for parent := filepath.Dir(path); parent != "."; parent = filepath.Dir(parent) {
			if _, ok := links[parent]; ok {
				return "", errors.New("macOS 更新包试图写入链接目录")
			}
		}
	}
	// Resolve link chains component by component: '..' after an intermediate
	// symlink must not escape the bundle even when its lexical path looks safe.
	for name := range links {
		remaining := strings.Split(filepath.ToSlash(name), "/")
		var resolved []string
		expansions := 0
		for len(remaining) > 0 {
			part := remaining[0]
			remaining = remaining[1:]
			switch part {
			case "", ".":
				continue
			case "..":
				if len(resolved) <= 1 {
					return "", errors.New("macOS 更新包链接超出应用目录")
				}
				resolved = resolved[:len(resolved)-1]
			default:
				resolved = append(resolved, part)
				if link, ok := links[filepath.FromSlash(strings.Join(resolved, "/"))]; ok {
					expansions++
					if expansions > 40 {
						return "", errors.New("macOS 更新包包含循环链接")
					}
					resolved = resolved[:len(resolved)-1]
					remaining = append(strings.Split(link, "/"), remaining...)
				}
			}
		}
	}
	if bundle == "" {
		return "", errors.New("macOS 更新包中没有 app")
	}
	return bundle, nil
}
