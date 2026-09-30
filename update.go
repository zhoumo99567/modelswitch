package main

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var AppVersion = "0.1.0"

const defaultUpdateManifestURL = ""
const officialUpdateManifestLimit = 1 << 20

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
	if err = json.Unmarshal(data, &manifest); err != nil || strings.TrimSpace(manifest.Version) == "" {
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
		info.Status = "error"
		info.Message = err.Error()
		return info, err
	}
	info.Status = "applying"
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
	data, err := fetchURL(info.DownloadURL, 512<<20)
	if err != nil {
		return "", errors.New("下载更新包失败")
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(info.SHA256)) {
		return "", errors.New("更新包 SHA-256 校验失败")
	}
	if info.Signature != "" {
		if err = verifyUpdateSignature(data, info.Signature); err != nil {
			return "", err
		}
	}
	file, err := os.CreateTemp("", "model-switcher-update-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err = file.Write(data); err != nil {
		file.Close()
		os.Remove(path)
		return "", err
	}
	if err = file.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
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

func compareVersions(left, right string) int {
	parse := func(value string) []int {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		fields := strings.Split(strings.SplitN(value, "-", 2)[0], ".")
		result := make([]int, 3)
		for i := 0; i < len(fields) && i < len(result); i++ {
			result[i], _ = strconv.Atoi(fields[i])
		}
		return result
	}
	a, b := parse(left), parse(right)
	for i := range a {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

func launchUpdateHelper(artifact string) error {
	executable, err := os.Executable()
	if err != nil {
		return errors.New("无法定位当前程序")
	}
	helper, err := os.CreateTemp("", "model-switcher-updater-*.exe")
	if err != nil {
		return err
	}
	helperPath := helper.Name()
	data, err := os.ReadFile(executable)
	if err == nil {
		_, err = helper.Write(data)
	}
	if closeErr := helper.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(helperPath)
		return err
	}
	target := executable
	if runtime.GOOS == "darwin" {
		target = macOSBundlePath(executable)
	}
	cmd := exec.Command(helperPath, "--update-helper", target, artifact)
	if err = cmd.Start(); err != nil {
		os.Remove(helperPath)
		return errors.New("无法启动更新助手")
	}
	return nil
}

func runUpdateHelper(args []string) error {
	if len(args) != 2 {
		return errors.New("更新助手参数无效")
	}
	target, artifact := args[0], args[1]
	defer os.Remove(artifact)
	defer os.Remove(os.Args[0])
	if runtime.GOOS == "darwin" {
		return replaceMacOSBundle(target, artifact)
	}
	return replaceExecutable(target, artifact)
}

func replaceExecutable(target, artifact string) error {
	backup := target + ".backup"
	for i := 0; i < 120; i++ {
		if err := os.Rename(target, backup); err == nil {
			if err = os.Rename(artifact, target); err != nil {
				_ = os.Rename(backup, target)
				return errors.New("替换程序失败，已保留原版本")
			}
			cmd := exec.Command(target)
			if err = cmd.Start(); err != nil {
				_ = os.Remove(target)
				_ = os.Rename(backup, target)
				return errors.New("启动新版本失败，已恢复原版本")
			}
			_ = os.Remove(backup)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("等待程序退出超时，更新已取消")
}

func macOSBundlePath(executable string) string {
	if index := strings.Index(executable, ".app"+string(filepath.Separator)); index >= 0 {
		return executable[:index+len(".app")]
	}
	return executable
}

func replaceMacOSBundle(target, artifact string) error {
	stage, err := os.MkdirTemp("", "model-switcher-app-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	reader, err := zip.OpenReader(artifact)
	if err != nil {
		return errors.New("macOS 更新包不是有效的 ZIP")
	}
	defer reader.Close()
	for _, file := range reader.File {
		relative := filepath.Clean(filepath.FromSlash(file.Name))
		if relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("macOS 更新包包含非法路径")
		}
		destination := filepath.Join(stage, relative)
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(destination, 0755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		input, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		output, createErr := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if createErr == nil {
			_, createErr = io.Copy(output, input)
		}
		input.Close()
		output.Close()
		if createErr != nil {
			return createErr
		}
	}
	var bundle string
	entries, _ := os.ReadDir(stage)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			bundle = filepath.Join(stage, entry.Name())
			break
		}
	}
	if bundle == "" {
		return errors.New("macOS 更新包中没有 app")
	}
	backup := target + ".backup"
	if err = os.Rename(target, backup); err != nil {
		return errors.New("无法替换旧版 macOS 应用")
	}
	if err = os.Rename(bundle, target); err != nil {
		_ = os.Rename(backup, target)
		return errors.New("替换 macOS 应用失败")
	}
	if err = exec.Command("open", target).Start(); err != nil {
		_ = os.RemoveAll(target)
		_ = os.Rename(backup, target)
		return errors.New("启动新版本失败，已恢复原版本")
	}
	_ = os.RemoveAll(backup)
	return nil
}
