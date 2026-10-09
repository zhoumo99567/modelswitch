//go:build darwin && cgo

package main

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <dlfcn.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>

static char *modelSwitcherOriginalPath(const char *path) {
	// This Security.framework function is exported but absent from public SDK
	// headers. Resolve it at runtime so unavailable symbols never stop launch.
	void *security = dlopen("/System/Library/Frameworks/Security.framework/Security", RTLD_LAZY | RTLD_LOCAL);
	if (security == NULL) return NULL;
	typedef CFURLRef (*OriginalPathFn)(CFURLRef, CFErrorRef *);
	OriginalPathFn originalPath = (OriginalPathFn)dlsym(security, "SecTranslocateCreateOriginalPathForURL");
	if (originalPath == NULL) { dlclose(security); return NULL; }
	CFURLRef translocated = CFURLCreateFromFileSystemRepresentation(kCFAllocatorDefault, (const UInt8 *)path, strlen(path), false);
	if (translocated == NULL) { dlclose(security); return NULL; }
	CFErrorRef error = NULL;
	CFURLRef original = originalPath(translocated, &error);
	if (error != NULL) CFRelease(error);
	CFRelease(translocated);
	if (original == NULL) { dlclose(security); return NULL; }
	UInt8 buffer[PATH_MAX];
	char *result = NULL;
	if (CFURLGetFileSystemRepresentation(original, true, buffer, sizeof(buffer))) {
		result = strdup((const char *)buffer);
	}
	CFRelease(original);
	dlclose(security);
	return result;
}
*/
import "C"

import "unsafe"

func originalExecutablePath(path string) (string, bool) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	original := C.modelSwitcherOriginalPath(cPath)
	if original == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(original))
	return C.GoString(original), true
}
