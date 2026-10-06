//go:build windows

package main

import (
	"errors"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var piClipboardUser32 = windows.NewLazySystemDLL("user32.dll")
var piClipboardShell32 = windows.NewLazySystemDLL("shell32.dll")

func piChatClipboardPaths() ([]string, error) {
	const cfHDROP = 15
	available, _, _ := piClipboardUser32.NewProc("IsClipboardFormatAvailable").Call(cfHDROP)
	if available == 0 {
		return nil, nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	open := piClipboardUser32.NewProc("OpenClipboard")
	var opened uintptr
	for attempt := 0; attempt < 3; attempt++ {
		opened, _, _ = open.Call(0)
		if opened != 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if opened == 0 {
		return nil, errors.New("剪贴板正被其他程序使用，请再次粘贴")
	}
	defer piClipboardUser32.NewProc("CloseClipboard").Call()
	handle, _, _ := piClipboardUser32.NewProc("GetClipboardData").Call(cfHDROP)
	if handle == 0 {
		return nil, errors.New("无法读取剪贴板文件")
	}
	query := piClipboardShell32.NewProc("DragQueryFileW")
	count, _, _ := query.Call(handle, 0xffffffff, 0, 0)
	paths := []string{}
	// One extra item lets the reader report a batch exceeding the limit.
	if count > maxPiChatAttachments+1 {
		count = maxPiChatAttachments + 1
	}
	for index := uintptr(0); index < count; index++ {
		length, _, _ := query.Call(handle, index, 0, 0)
		if length == 0 || length > 32768 {
			continue
		}
		buffer := make([]uint16, length+1)
		copied, _, _ := query.Call(handle, index, uintptr(unsafe.Pointer(&buffer[0])), length+1)
		if copied > 0 {
			paths = append(paths, windows.UTF16ToString(buffer))
		}
	}
	return paths, nil
}
