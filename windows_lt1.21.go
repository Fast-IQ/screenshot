//go:build windows && !go1.21

package screenshot

import (
	"image"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

func NumActiveDisplays() int {
	var count int
	count = 0
	ptr := unsafe.Pointer(&count)
	enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(countupMonitorCallback), uintptr(ptr))
	return count
}

func GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	monitorCount := countMonitors()
	if displayIndex < 0 || displayIndex >= monitorCount {
		return image.Rectangle{}, fmt.Errorf("invalid display index: %d", displayIndex)
	}

	var ctx getMonitorBoundsContext
	ctx.Index = displayIndex
	ctx.Count = 0
	ptr := unsafe.Pointer(&ctx)
	enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(getMonitorBoundsCallback), uintptr(ptr))

	if ctx.Rect.Left == 0 && ctx.Rect.Top == 0 && ctx.Rect.Right == 0 && ctx.Rect.Bottom == 0 {
		return image.Rectangle{}, fmt.Errorf("failed to retrieve monitor bounds")
	}

	return image.Rect(
		int(ctx.Rect.Left), int(ctx.Rect.Top),
		int(ctx.Rect.Right), int(ctx.Rect.Bottom)), nil
}

func countMonitors() int {
	var count int
	callback := syscall.NewCallback(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		count++
		return 1
	})
	enumDisplayMonitors(win.HDC(0), nil, callback, 0)
	return count
}
