//go:build windows

package main

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var windowUser32 = windows.NewLazySystemDLL("user32.dll")

type windowNativeRect struct{ Left, Top, Right, Bottom int32 }
type windowMonitorInfo struct {
	Size          uint32
	Monitor, Work windowNativeRect
	Flags         uint32
}

// Windows retains callback thunks for the life of the process. Reuse these
// callbacks rather than allocating new thunks on each geometry sample.
var windowNativeMu sync.Mutex
var enumeratedWindow uintptr
var enumeratedDisplays []windowDisplay
var enumeratedCurrent uintptr
var windowGetDPI = windows.NewLazySystemDLL("shcore.dll").NewProc("GetDpiForMonitor")
var windowEnumCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	var pid uint32
	windowUser32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != uint32(os.Getpid()) {
		return 1
	}
	var title [128]uint16
	windowUser32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	if windows.UTF16ToString(title[:]) == "Model Switcher" {
		enumeratedWindow = hwnd
		return 0
	}
	return 1
})
var windowMonitorCallback = syscall.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
	info := windowMonitorInfo{Size: uint32(unsafe.Sizeof(windowMonitorInfo{}))}
	ok, _, _ := windowUser32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 1
	}
	scale := 1.0
	if windowGetDPI.Find() == nil {
		var x, y uint32
		result, _, _ := windowGetDPI.Call(monitor, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)))
		if result == 0 && x > 0 {
			scale = float64(x) / 96
		}
	}
	work := info.Work
	enumeratedDisplays = append(enumeratedDisplays, windowDisplay{Work: windowBounds{X: int(work.Left), Y: int(work.Top), Width: int(work.Right - work.Left), Height: int(work.Bottom - work.Top)}, Scale: scale, Primary: info.Flags&1 != 0, Current: monitor == enumeratedCurrent})
	return 1
})

func mainWindowHandle() uintptr {
	enumeratedWindow = 0
	windowUser32.NewProc("EnumWindows").Call(windowEnumCallback, 0)
	return enumeratedWindow
}

func nativeWindowState() (windowBounds, []windowDisplay, bool, error) {
	windowNativeMu.Lock()
	defer windowNativeMu.Unlock()
	hwnd := mainWindowHandle()
	if hwnd == 0 {
		return windowBounds{}, nil, false, errors.New("窗口尚未就绪")
	}
	var rect windowNativeRect
	ok, _, err := windowUser32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return windowBounds{}, nil, false, err
	}
	bounds := windowBounds{X: int(rect.Left), Y: int(rect.Top), Width: int(rect.Right - rect.Left), Height: int(rect.Bottom - rect.Top)}
	enumeratedCurrent, _, _ = windowUser32.NewProc("MonitorFromWindow").Call(hwnd, 2)
	enumeratedDisplays = nil
	ok, _, err = windowUser32.NewProc("EnumDisplayMonitors").Call(0, 0, windowMonitorCallback, 0)
	if ok == 0 {
		return bounds, nil, false, err
	}
	displays := enumeratedDisplays
	if len(displays) == 0 {
		return bounds, nil, false, errors.New("无法读取屏幕可用区域")
	}

	zoomed, _, _ := windowUser32.NewProc("IsZoomed").Call(hwnd)
	minimized, _, _ := windowUser32.NewProc("IsIconic").Call(hwnd)
	return bounds, displays, zoomed == 0 && minimized == 0, nil
}

func nativeApplyWindowBounds(bounds windowBounds) error {
	windowNativeMu.Lock()
	defer windowNativeMu.Unlock()
	hwnd := mainWindowHandle()
	if hwnd == 0 {
		return errors.New("窗口尚未就绪")
	}
	// Both GetWindowRect and monitor work areas use physical desktop coordinates.
	ok, _, err := windowUser32.NewProc("SetWindowPos").Call(hwnd, 0, uintptr(bounds.X), uintptr(bounds.Y), uintptr(bounds.Width), uintptr(bounds.Height), 0x0004|0x0010)
	if ok == 0 {
		return err
	}
	return nil
}
