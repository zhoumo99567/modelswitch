package main

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowBoundsScreenChanges(t *testing.T) {
	primary := windowDisplay{Work: windowBounds{X: 0, Y: 25, Width: 1440, Height: 850}, Scale: 1, Primary: true}
	left := windowDisplay{Work: windowBounds{X: -1920, Y: -200, Width: 1920, Height: 1080}, Scale: 1}
	highDPI := windowDisplay{Work: windowBounds{X: 1440, Y: 40, Width: 2560, Height: 1400}, Scale: 1.5}
	tests := []struct {
		name     string
		bounds   windowBounds
		displays []windowDisplay
		valid    bool
	}{
		{"same display", windowBounds{100, 60, 1120, 760}, []windowDisplay{primary}, true},
		{"secondary negative coordinates", windowBounds{-1700, -120, 1120, 760}, []windowDisplay{primary, left}, true},
		{"secondary disconnected", windowBounds{-1700, -120, 1120, 760}, []windowDisplay{primary}, false},
		{"resolution shrunk", windowBounds{500, 60, 1120, 760}, []windowDisplay{primary}, false},
		{"title behind menu bar", windowBounds{100, 0, 1120, 760}, []windowDisplay{primary}, false},
		{"bottom behind dock", windowBounds{100, 130, 1120, 760}, []windowDisplay{primary}, false},
		{"smaller than minimum", windowBounds{100, 60, 500, 300}, []windowDisplay{primary}, false},
		{"zero size", windowBounds{}, []windowDisplay{primary}, false},
		{"corrupt extreme coordinates", windowBounds{math.MaxInt, 60, 1120, 760}, []windowDisplay{primary}, false},
		{"high DPI fits", windowBounds{1500, 60, 1680, 1140}, []windowDisplay{highDPI}, true},
		{"high DPI below logical minimum", windowBounds{1500, 60, 1000, 700}, []windowDisplay{highDPI}, false},
		{"unknown screens", windowBounds{100, 60, 1120, 760}, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validWindowBounds(test.bounds, test.displays); got != test.valid {
				t.Fatalf("valid=%v; want %v", got, test.valid)
			}
		})
	}
}

func TestDefaultWindowBoundsFitAndCenter(t *testing.T) {
	for _, display := range []windowDisplay{
		{Work: windowBounds{X: 0, Y: 25, Width: 1440, Height: 850}, Scale: 1},
		{Work: windowBounds{X: -1920, Y: -200, Width: 1920, Height: 1080}, Scale: 1},
		{Work: windowBounds{X: 0, Y: 25, Width: 800, Height: 550}, Scale: 1},
		{Work: windowBounds{X: 1440, Y: 40, Width: 2560, Height: 1400}, Scale: 1.5},
	} {
		bounds := defaultWindowBounds(display)
		if !validWindowBounds(bounds, []windowDisplay{display}) {
			t.Fatalf("default not visible: %+v", bounds)
		}
		if bounds.X != display.Work.X+(display.Work.Width-bounds.Width)/2 || bounds.Y != display.Work.Y+(display.Work.Height-bounds.Height)/2 {
			t.Fatalf("not centered: %+v", bounds)
		}
	}
}

func TestWindowStateMissingCorruptAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	if readWindowBounds(path) != nil {
		t.Fatal("missing file should use default")
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if readWindowBounds(path) != nil {
		t.Fatal("corrupt file should use default")
	}
	if err := atomicWrite(path, []byte(`{"x":-1200,"y":50,"width":1120,"height":760}`)); err != nil {
		t.Fatal(err)
	}
	got := readWindowBounds(path)
	if got == nil || *got != (windowBounds{-1200, 50, 1120, 760}) {
		t.Fatalf("round trip: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected permissions: %v", info.Mode())
	}
}
