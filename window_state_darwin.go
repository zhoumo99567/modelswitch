//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa
#include <Cocoa/Cocoa.h>
#include <stdlib.h>

typedef struct { int x,y,width,height; } MSWindowRect;
typedef struct { MSWindowRect work; int primary,current; } MSWindowDisplay;
typedef struct { MSWindowRect bounds; MSWindowDisplay *displays; int count,normal,ok; } MSWindowState;

static NSWindow *msMainWindow(void) {
 NSWindow *window=[NSApp mainWindow];
 if (!window) { for (NSWindow *candidate in [NSApp windows]) {
  if ([candidate.title isEqualToString:@"Model Switcher"]) { window=candidate; break; }
 } }
 return window;
}
static MSWindowRect msRect(NSRect frame, CGFloat top) {
 return (MSWindowRect){(int)round(frame.origin.x),(int)round(top-NSMaxY(frame)),(int)round(frame.size.width),(int)round(frame.size.height)};
}
static MSWindowState msReadWindow(void) {
 __block MSWindowState result={0};
 void (^read)(void)=^{
  NSArray<NSScreen *> *screens=[NSScreen screens];
  NSWindow *window=msMainWindow();
  if (!window || screens.count==0) return;
  CGFloat top=NSMaxY(screens[0].frame);
  result.count=(int)screens.count;
  result.displays=calloc(result.count,sizeof(MSWindowDisplay));
  if (!result.displays) { result.count=0; return; }
  for (int i=0;i<result.count;i++) {
   NSScreen *screen=screens[i];
   result.displays[i]=(MSWindowDisplay){msRect(screen.visibleFrame,top),i==0,screen==window.screen};
  }
  result.bounds=msRect(window.frame,top);
  result.normal=!window.isMiniaturized && !window.isZoomed && !(window.styleMask & NSWindowStyleMaskFullScreen);
  result.ok=1;
 };
 if ([NSThread isMainThread]) read(); else dispatch_sync(dispatch_get_main_queue(),read);
 return result;
}
static int msApplyWindow(MSWindowRect bounds) {
 __block int ok=0;
 void (^apply)(void)=^{
  NSArray<NSScreen *> *screens=[NSScreen screens];
  NSWindow *window=msMainWindow();
  if (!window || screens.count==0) return;
  CGFloat top=NSMaxY(screens[0].frame);
  [window setFrame:NSMakeRect(bounds.x,top-bounds.y-bounds.height,bounds.width,bounds.height) display:YES animate:NO];
  ok=1;
 };
 if ([NSThread isMainThread]) apply(); else dispatch_sync(dispatch_get_main_queue(),apply);
 return ok;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func nativeWindowState() (windowBounds, []windowDisplay, bool, error) {
	state := C.msReadWindow()
	if state.ok == 0 {
		return windowBounds{}, nil, false, errors.New("窗口尚未就绪")
	}
	defer C.free(unsafe.Pointer(state.displays))
	rect := func(r C.MSWindowRect) windowBounds {
		return windowBounds{X: int(r.x), Y: int(r.y), Width: int(r.width), Height: int(r.height)}
	}
	displays := make([]windowDisplay, 0, int(state.count))
	for _, display := range unsafe.Slice(state.displays, int(state.count)) {
		displays = append(displays, windowDisplay{Work: rect(display.work), Scale: 1, Primary: display.primary != 0, Current: display.current != 0})
	}
	return rect(state.bounds), displays, state.normal != 0, nil
}

func nativeApplyWindowBounds(bounds windowBounds) error {
	if C.msApplyWindow(C.MSWindowRect{x: C.int(bounds.X), y: C.int(bounds.Y), width: C.int(bounds.Width), height: C.int(bounds.Height)}) == 0 {
		return errors.New("无法恢复窗口位置")
	}
	return nil
}
