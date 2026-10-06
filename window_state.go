package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultWindowWidth  = 1120
	defaultWindowHeight = 760
	minWindowWidth      = 920
	minWindowHeight     = 640
)

// Coordinates use macOS points or Windows physical pixels, consistently with
// the platform's desktop work areas. This file is local to the current computer.
type windowBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type windowDisplay struct {
	Work    windowBounds
	Scale   float64
	Primary bool
	Current bool
}

type windowMemory struct {
	mu   sync.Mutex
	last *windowBounds
}

func windowStatePath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "ModelSwitcher", "window-state.json")
}

func readWindowBounds(path string) *windowBounds {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var bounds windowBounds
	if json.Unmarshal(data, &bounds) != nil {
		return nil
	}
	return &bounds
}

func scaledWindowSize(value int, scale float64) int {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return int(math.Round(float64(value) * scale))
}

func validWindowBounds(bounds windowBounds, displays []windowDisplay) bool {
	for _, display := range displays {
		work := display.Work
		// Subtract the size from the desktop edge, rather than adding unchecked
		// coordinates from disk. This also rejects oversized and malformed states.
		if bounds.Width >= min(scaledWindowSize(minWindowWidth, display.Scale), work.Width) &&
			bounds.Height >= min(scaledWindowSize(minWindowHeight, display.Scale), work.Height) &&
			bounds.Width > 0 && bounds.Height > 0 && bounds.Width <= work.Width && bounds.Height <= work.Height &&
			bounds.X >= work.X && bounds.Y >= work.Y &&
			bounds.X <= work.X+work.Width-bounds.Width && bounds.Y <= work.Y+work.Height-bounds.Height {
			return true
		}
	}
	return false
}

func preferredWindowDisplay(displays []windowDisplay) windowDisplay {
	for _, display := range displays {
		if display.Current {
			return display
		}
	}
	for _, display := range displays {
		if display.Primary {
			return display
		}
	}
	return displays[0]
}

func defaultWindowBounds(display windowDisplay) windowBounds {
	width := min(scaledWindowSize(defaultWindowWidth, display.Scale), display.Work.Width)
	height := min(scaledWindowSize(defaultWindowHeight, display.Scale), display.Work.Height)
	return windowBounds{X: display.Work.X + (display.Work.Width-width)/2, Y: display.Work.Y + (display.Work.Height-height)/2, Width: width, Height: height}
}

func (a *App) restoreWindow(ctx context.Context) {
	defer wailsruntime.WindowShow(ctx)
	_, displays, _, err := nativeWindowState()
	if err != nil || len(displays) == 0 {
		return
	}
	display := preferredWindowDisplay(displays)
	bounds := readWindowBounds(windowStatePath())
	if bounds == nil || !validWindowBounds(*bounds, displays) {
		fallback := defaultWindowBounds(display)
		bounds = &fallback
	} else {
		for _, candidate := range displays {
			if validWindowBounds(*bounds, []windowDisplay{candidate}) {
				display = candidate
				break
			}
		}
	}
	// Let the default window fit when a display's work area is smaller than the
	// usual minimum. Wails minimum sizes are logical units on both platforms.
	scale := display.Scale
	if scale <= 0 {
		scale = 1
	}
	wailsruntime.WindowSetMinSize(ctx, min(minWindowWidth, int(float64(display.Work.Width)/scale)), min(minWindowHeight, int(float64(display.Work.Height)/scale)))
	if nativeApplyWindowBounds(*bounds) == nil {
		a.window.remember(*bounds)
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.captureWindow()
			}
		}
	}()
}

func (m *windowMemory) remember(bounds windowBounds) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = &bounds
}

func (a *App) captureWindow() {
	bounds, _, normal, err := nativeWindowState()
	if err == nil && normal && bounds.Width > 0 && bounds.Height > 0 {
		a.window.remember(bounds)
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.cancelPiChat()
	a.ClosePiChatRuntime("")
	a.captureWindow()
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	if a.window.last != nil {
		if data, err := json.MarshalIndent(a.window.last, "", "  "); err == nil {
			if path := windowStatePath(); path != "" {
				if err := atomicWrite(path, data); err != nil {
					wailsruntime.LogErrorf(ctx, "无法保存窗口位置: %v", err)
				}
			}
		}
	}
	return false
}
