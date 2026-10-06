//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

func piChatClipboardPaths() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Read file URLs directly from NSPasteboard; no Finder or shell interpolation.
	script := `ObjC.import("AppKit");
const items = $.NSPasteboard.generalPasteboard.pasteboardItems;
const files = [];
if (items) for (let i = 0; i < Math.min(Number(items.count), 9); i++) {
 const value = items.objectAtIndex(i).stringForType("public.file-url");
 if (value) { const url = $.NSURL.URLWithString(value); if (url.isFileURL) files.push(ObjC.unwrap(url.path)); }
}
JSON.stringify(files);`
	data, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", script).Output()
	if err != nil {
		return nil, err
	}
	var files []string
	err = json.Unmarshal(data, &files)
	return files, err
}
