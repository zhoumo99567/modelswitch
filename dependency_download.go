package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type nodeRelease struct {
	Version string          `json:"version"`
	LTS     json.RawMessage `json:"lts"`
	Files   []string        `json:"files"`
}

func selectNodeRelease(releases []nodeRelease, platform, arch string) (string, string, error) {
	if arch == "amd64" {
		arch = "x64"
	}
	if arch != "x64" && arch != "arm64" {
		return "", "", errors.New("此处理器架构暂不支持自动安装")
	}
	fileKey, suffix := "", ""
	switch platform {
	case "windows":
		fileKey = "win-" + arch + "-zip"
		suffix = "win-" + arch + ".zip"
	case "darwin":
		fileKey = "osx-" + arch + "-tar"
		suffix = "darwin-" + arch + ".tar.gz"
	default:
		return "", "", errors.New("不支持此操作系统")
	}
	for _, release := range releases {
		if len(release.LTS) == 0 || string(release.LTS) == "false" || string(release.LTS) == "null" || !nodeReleaseVersion.MatchString(release.Version) || !compatibleNodeVersion(release.Version) {
			continue
		}
		for _, file := range release.Files {
			if file == fileKey {
				return release.Version, "node-" + release.Version + "-" + suffix, nil
			}
		}
	}
	return "", "", errors.New("未找到兼容的 Node.js LTS 下载")
}

func dependencyHTTP(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ModelSwitcher/"+AppVersion)
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("下载服务器返回 HTTP %d", response.StatusCode)
	}
	return response, nil
}

func dependencyHTTPBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	response, err := dependencyHTTP(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = errors.New("下载内容超过大小限制")
	}
	return data, err
}

func releaseChecksum(data []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != filename {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err == nil && len(decoded) == sha256.Size {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", errors.New("官方清单中没有此安装包的 SHA-256")
}

func installManagedNode(ctx context.Context, progress func(string, int)) error {
	progress("正在获取 Node.js LTS 版本…", 10)
	data, err := dependencyHTTPBytes(ctx, "https://nodejs.org/dist/index.json", 2<<20)
	if err != nil {
		return err
	}
	var releases []nodeRelease
	if err = json.Unmarshal(data, &releases); err != nil {
		return err
	}
	version, filename, err := selectNodeRelease(releases, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	baseURL := "https://nodejs.org/dist/" + version + "/"
	checksums, err := dependencyHTTPBytes(ctx, baseURL+"SHASUMS256.txt", 1<<20)
	if err != nil {
		return err
	}
	expected, err := releaseChecksum(checksums, filename)
	if err != nil {
		return err
	}
	root := managedToolsRoot()
	if err = os.MkdirAll(filepath.Join(root, "runtimes"), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Join(root, "runtimes"), ".install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, filename)
	progress("正在下载 Node.js "+version+"…", 20)
	if err = downloadNodeArchive(ctx, baseURL+filename, archive, expected, progress); err != nil {
		return err
	}
	progress("正在解压 Node.js 与 npm…", 65)
	unpacked := filepath.Join(stage, "unpacked")
	archiveRoot := strings.TrimSuffix(strings.TrimSuffix(filename, ".zip"), ".tar.gz")
	if err = extractNodeArchive(archive, unpacked, archiveRoot); err != nil {
		return err
	}
	bin := unpacked
	if runtime.GOOS != "windows" {
		bin = filepath.Join(bin, "bin")
	}
	node := toolInDirectory(bin, "node")
	if node == "" {
		return errors.New("安装包中缺少 Node.js")
	}
	if _, err = resolveNpmScript(node); err != nil {
		return err
	}
	out, err := dependencyCommand(ctx, node, "--version").Output()
	if err != nil || !compatibleNodeVersion(string(out)) {
		return errors.New("下载的 Node.js 无法运行或版本不兼容")
	}
	// Versioned directories keep running CLI/chat processes intact during repairs.
	destination := filepath.Join(root, "runtimes", version)
	if _, err = os.Stat(destination); err == nil {
		// A prior interrupted/invalid installation must be repairable without
		// deleting an executable that could still be in use.
		destination, err = os.MkdirTemp(filepath.Join(root, "runtimes"), version+"-")
		if err != nil {
			return err
		}
		if err = os.Remove(destination); err != nil {
			return err
		}
	}
	if err = os.Rename(unpacked, destination); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(root, "node-version"), []byte(filepath.Base(destination)))
}

func downloadNodeArchive(ctx context.Context, url, destination, expected string, progress func(string, int)) error {
	response, err := dependencyHTTP(ctx, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	buffer := make([]byte, 64<<10)
	var total int64
	last := 20
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			total += int64(n)
			if total > 256<<20 {
				return errors.New("Node.js 下载超过大小限制")
			}
			if _, err = writer.Write(buffer[:n]); err != nil {
				return err
			}
			if response.ContentLength > 0 {
				percent := 20 + int(total*40/response.ContentLength)
				if percent > last {
					last = percent
					progress(fmt.Sprintf("正在下载 Node.js… %d%%", total*100/response.ContentLength), percent)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	progress("正在校验安装包 SHA-256…", 62)
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("安装包 SHA-256 校验失败，请重新下载")
	}
	return file.Close()
}

// Official archives are extracted into a new staging directory. Reject traversal
// and links escaping it before creating anything at those paths.
func nodeArchivePath(root, name, archiveRoot string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", errors.New("安装包包含不安全路径")
	}
	parts := strings.Split(name, "/")
	if len(parts) == 0 || parts[0] != archiveRoot {
		return "", errors.New("安装包根目录不匹配")
	}
	for _, part := range parts {
		if part == ".." {
			return "", errors.New("安装包包含不安全路径")
		}
	}
	return filepath.Join(root, filepath.FromSlash(strings.Join(parts[1:], "/"))), nil
}

func writeNodeArchiveFile(path string, mode os.FileMode, reader io.Reader, size int64) error {
	if size < 0 || size > 256<<20 {
		return errors.New("安装包文件过大")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()&0755)
	if err != nil {
		return err
	}
	_, err = io.CopyN(file, reader, size)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func extractNodeArchive(archive, root, archiveRoot string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	var total int64
	if strings.HasSuffix(archive, ".zip") {
		reader, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, entry := range reader.File {
			path, err := nodeArchivePath(root, entry.Name, archiveRoot)
			if err != nil {
				return err
			}
			if entry.FileInfo().IsDir() {
				if err = os.MkdirAll(path, 0700); err != nil {
					return err
				}
				continue
			}
			if entry.Mode()&os.ModeSymlink != 0 {
				return errors.New("ZIP 安装包包含不支持的链接")
			}
			total += int64(entry.UncompressedSize64)
			if total > 1<<30 {
				return errors.New("安装包解压内容过大")
			}
			file, err := entry.Open()
			if err != nil {
				return err
			}
			err = writeNodeArchiveFile(path, entry.Mode(), file, int64(entry.UncompressedSize64))
			file.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	type link struct{ path, target string }
	links := []link{}
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path, err := nodeArchivePath(root, entry.Name, archiveRoot)
		if err != nil {
			return err
		}
		switch entry.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, 0700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += entry.Size
			if total > 1<<30 {
				return errors.New("安装包解压内容过大")
			}
			if err = writeNodeArchiveFile(path, os.FileMode(entry.Mode), reader, entry.Size); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(entry.Linkname) {
				return errors.New("安装包链接指向安装目录之外")
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(path), entry.Linkname))
			rel, err := filepath.Rel(root, target)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return errors.New("安装包链接指向安装目录之外")
			}
			links = append(links, link{path, entry.Linkname})
		default:
			return errors.New("安装包包含不支持的文件类型")
		}
	}
	for _, link := range links {
		if err = os.MkdirAll(filepath.Dir(link.path), 0700); err != nil {
			return err
		}
		if err = os.Symlink(link.target, link.path); err != nil {
			return err
		}
	}
	return nil
}
