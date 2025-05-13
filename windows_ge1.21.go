//go:build windows && go1.21

package screenshot

import (
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

func NumActiveDisplays() int {
	count := new(int)
	pinner := new(runtime.Pinner)
	pinner.Pin(count)
	defer pinner.Unpin()
	*count = 0
	ptr := unsafe.Pointer(count)
	enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(countupMonitorCallback), uintptr(ptr))
	return *count
}

/*
	func GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
		monitorCount := countMonitors()
		if displayIndex < 0 || displayIndex >= monitorCount {
			return image.Rectangle{}, fmt.Errorf("invalid display index: %d", displayIndex)
		}

		ctx := new(getMonitorBoundsContext)
		pinner := new(runtime.Pinner)
		pinner.Pin(ctx)
		defer pinner.Unpin()

		ctx.Index = displayIndex
		ctx.Count = 0

		ptr := unsafe.Pointer(ctx)
		enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(getMonitorBoundsCallback), uintptr(ptr))

		if ctx.Rect.Left == 0 && ctx.Rect.Top == 0 && ctx.Rect.Right == 0 && ctx.Rect.Bottom == 0 {
			return image.Rectangle{}, fmt.Errorf("failed to retrieve monitor bounds")
		}

		return image.Rect(
			int(ctx.Rect.Left), int(ctx.Rect.Top),
			int(ctx.Rect.Right), int(ctx.Rect.Bottom)), nil
	}
*/
func GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	var monitors []win.RECT
	callback := syscall.NewCallback(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		monitors = append(monitors, *lprcMonitor)
		return 1
	})

	if !enumDisplayMonitors(win.HDC(0), nil, callback, 0) {
		return image.Rectangle{}, fmt.Errorf("failed to enumerate monitors")
	}

	if displayIndex < 0 || displayIndex >= len(monitors) {
		return image.Rectangle{}, fmt.Errorf("invalid display index: %d", displayIndex)
	}

	rect := monitors[displayIndex]
	return image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom)), nil
}
