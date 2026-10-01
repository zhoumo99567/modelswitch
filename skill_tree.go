package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type repositoryFile struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func fetchRepositoryTree(repository, sha string) ([]repositoryFile, error) {
	return fetchRepositoryTreeContext(context.Background(), repository, sha)
}
func fetchRepositoryTreeContext(ctx context.Context, repository, sha string) ([]repositoryFile, error) {
	data, err := fetchURLContext(ctx, "https://api.github.com/repos/"+repository+"/git/trees/"+sha+"?recursive=1", 8<<20)
	if err != nil {
		return nil, err
	}
	var tree struct {
		Tree      []repositoryFile `json:"tree"`
		Truncated bool             `json:"truncated"`
	}
	if err = json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	if tree.Truncated {
		return nil, errors.New("市场目录不完整，不能安装")
	}
	return tree.Tree, nil
}
func fetchTreeCatalogContext(ctx context.Context, m SkillMarket, sha string) ([]SkillCatalogItem, error) {
	tree, err := fetchRepositoryTreeContext(ctx, m.Repository, sha)
	if err != nil {
		return nil, err
	}
	docs := []repositoryFile{}
	for _, f := range tree {
		if (strings.HasPrefix(f.Path, m.Prefix) && strings.HasSuffix(f.Path, "/SKILL.md")) || (m.ID == "openai-plugins" && f.Path == ".agents/plugins/marketplace.json") {
			docs = append(docs, f)
		}
	}
	archive, err := downloadRepositoryFilesContext(ctx, m.Repository, sha, docs)
	if err != nil {
		return nil, err
	}
	items, err := catalogFromArchive(m, sha, archive)
	if err != nil {
		return nil, err
	}
	for i := range items {
		for _, f := range tree {
			prefix := items[i].SkillPath + "/"
			if strings.HasPrefix(f.Path, prefix) && isSkillScript(strings.TrimPrefix(f.Path, prefix)) {
				items[i].HasScripts = true
				break
			}
		}
	}
	return items, nil
}
func downloadTreeSkill(item SkillCatalogItem) ([]byte, error) {
	tree, err := fetchRepositoryTree(item.Repository, item.Version)
	if err != nil {
		return nil, err
	}
	files := []repositoryFile{}
	for _, f := range tree {
		if strings.HasPrefix(f.Path, item.SkillPath+"/") && f.Type != "tree" {
			files = append(files, f)
		}
	}
	return downloadRepositoryFiles(item.Repository, item.Version, files)
}
func downloadRepositoryFiles(repository, sha string, files []repositoryFile) ([]byte, error) {
	return downloadRepositoryFilesContext(context.Background(), repository, sha, files)
}
func downloadRepositoryFilesContext(ctx context.Context, repository, sha string, files []repositoryFile) ([]byte, error) {
	if len(files) == 0 || len(files) > 10000 {
		return nil, errors.New("技能文件数量无效")
	}
	var total int64
	for _, f := range files {
		if !safeArchivePath(f.Path) || f.Type != "blob" || (f.Mode != "100644" && f.Mode != "100755") {
			return nil, errors.New("市场包含不支持的路径、链接或子模块")
		}
		if f.Size < 0 || f.Size > maxSkillFile {
			return nil, errors.New("技能文件超过安全限制")
		}
		total += f.Size
		if total > maxSkillDownload {
			return nil, errors.New("技能总大小超过安全限制")
		}
	}
	type downloaded struct {
		File repositoryFile
		Data []byte
		Err  error
	}
	jobs := make(chan repositoryFile, len(files))
	results := make(chan downloaded, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	workers := 6
	if len(files) > 100 {
		workers = 16
	}
	if len(files) < workers {
		workers = len(files)
	}
	for i := 0; i < workers; i++ {
		go func() {
			for f := range jobs {
				raw := url.URL{Scheme: "https", Host: "raw.githubusercontent.com", Path: "/" + repository + "/" + sha + "/" + f.Path}
				data, err := fetchURLContext(ctx, raw.String(), maxSkillFile)
				results <- downloaded{f, data, err}
			}
		}()
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	total = 0
	var firstError error
	for range files {
		r := <-results
		if r.Err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("下载 %s: %w", r.File.Path, r.Err)
			}
			continue
		}
		total += int64(len(r.Data))
		if total > maxSkillDownload {
			firstError = errors.New("技能总大小超过安全限制")
			continue
		}
		if firstError != nil {
			continue
		}
		header := &zip.FileHeader{Name: "repository/" + r.File.Path, Method: zip.Deflate}
		mode := os.FileMode(0600)
		if r.File.Mode == "100755" {
			mode = 0700
		}
		header.SetMode(mode)
		file, err := writer.CreateHeader(header)
		if err == nil {
			_, err = file.Write(r.Data)
		}
		if err != nil {
			firstError = err
		}
	}
	err := writer.Close()
	if firstError != nil {
		return nil, firstError
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
