//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
#include <unistd.h>

// displayContainingOwnWindow returns the 0-based index (in CGGetActiveDisplayList
// order — the same order `screencapture -D` and kbinani use) of the display that
// contains the centre of this process's largest on-screen window, or -1 when it
// can't be determined.
//
// We resolve the display natively rather than from Wails' WindowGetPosition,
// which on macOS reports the window position RELATIVE to its current screen's
// visible frame, not in the global virtual-desktop space the display bounds use —
// so matching it against the bounds always lands on the main display.
static int displayContainingOwnWindow() {
    int myPid = (int)getpid();
    CFArrayRef windows = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID);
    if (windows == NULL) {
        return -1;
    }

    double bestArea = 0;
    CGRect bestBounds = CGRectNull;
    int found = 0;
    CFIndex n = CFArrayGetCount(windows);
    for (CFIndex i = 0; i < n; i++) {
        CFDictionaryRef info = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);
        CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(info, kCGWindowOwnerPID);
        if (pidRef == NULL) {
            continue;
        }
        int pid = 0;
        CFNumberGetValue(pidRef, kCFNumberIntType, &pid);
        if (pid != myPid) {
            continue;
        }
        CFDictionaryRef boundsDict = (CFDictionaryRef)CFDictionaryGetValue(info, kCGWindowBounds);
        if (boundsDict == NULL) {
            continue;
        }
        CGRect r;
        if (!CGRectMakeWithDictionaryRepresentation(boundsDict, &r)) {
            continue;
        }
        // Pick the largest window owned by us (skip tiny helper/status windows).
        double area = r.size.width * r.size.height;
        if (area > bestArea) {
            bestArea = area;
            bestBounds = r;
            found = 1;
        }
    }
    CFRelease(windows);
    if (!found) {
        return -1;
    }

    CGPoint center = CGPointMake(CGRectGetMidX(bestBounds), CGRectGetMidY(bestBounds));

    CGDirectDisplayID active[16];
    uint32_t count = 0;
    if (CGGetActiveDisplayList(16, active, &count) != kCGErrorSuccess) {
        return -1;
    }
    for (uint32_t i = 0; i < count; i++) {
        if (CGRectContainsPoint(CGDisplayBounds(active[i]), center)) {
            return (int)i;
        }
    }
    return -1;
}
*/
import "C"

// DisplayContainingWindow reports the display index showing the app window, found
// via the OS's native window geometry. The bool is false when it can't be
// determined, so the caller can fall back. On macOS this uses the CoreGraphics
// window list (global coordinates); see displayContainingOwnWindow for why the
// generic Wails-position path is wrong here.
func DisplayContainingWindow() (int, bool) {
	idx := int(C.displayContainingOwnWindow())
	if idx < 0 {
		return 0, false
	}
	return idx, true
}
