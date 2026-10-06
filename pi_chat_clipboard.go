package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

const maxPiChatAttachmentBytes = 8 << 20
const maxPiChatAttachmentTotal = 12 << 20
const maxPiChatAttachments = 8

type PiChatClipboardFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type PiChatClipboard struct {
	Files  []PiChatClipboardFile `json:"files"`
	Errors []string              `json:"errors"`
}

// Invoked only by the UI's paste gesture; paths come from the OS file clipboard.
func (a *App) ReadPiChatClipboardFiles() (PiChatClipboard, error) {
	paths, err := piChatClipboardPaths()
	if err != nil {
		return PiChatClipboard{}, err
	}
	return readPiChatClipboardFiles(paths), nil
}

func readPiChatClipboardFiles(paths []string) PiChatClipboard {
	result := PiChatClipboard{Files: []PiChatClipboardFile{}, Errors: []string{}}
	if len(paths) > maxPiChatAttachments {
		result.Errors = append(result.Errors, "每次最多粘贴 8 个文件")
		paths = paths[:maxPiChatAttachments]
	}
	total := 0
	for _, path := range paths {
		name := filepath.Base(path)
		data, err := readPiChatClipboardFile(path)
		if err != nil {
			result.Errors = append(result.Errors, name+": "+err.Error())
			continue
		}
		if total+len(data) > maxPiChatAttachmentTotal {
			result.Errors = append(result.Errors, name+": 附件总大小不能超过 12 MB")
			continue
		}
		total += len(data)
		kind := mime.TypeByExtension(filepath.Ext(name))
		if kind == "" {
			kind = http.DetectContentType(data)
		}
		result.Files = append(result.Files, PiChatClipboardFile{Name: name, Type: kind, Data: data})
	}
	return result
}

func readPiChatClipboardFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("只能粘贴普通文件")
	}
	if info.Size() > maxPiChatAttachmentBytes {
		return nil, fmt.Errorf("单个附件不能超过 8 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPiChatAttachmentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("无法读取文件")
	}
	if len(data) > maxPiChatAttachmentBytes {
		return nil, fmt.Errorf("单个附件不能超过 8 MB")
	}
	return data, nil
}
