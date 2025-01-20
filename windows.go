//go:build windows

package screenshot

import (
	"errors"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"image"
	"syscall"
	"unsafe"
)

var (
	libUser32, _               = syscall.LoadLibrary("user32.dll")
	funcGetDesktopWindow, _    = syscall.GetProcAddress(syscall.Handle(libUser32), "GetDesktopWindow")
	funcEnumDisplayMonitors, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplayMonitors")
	funcGetMonitorInfo, _      = syscall.GetProcAddress(syscall.Handle(libUser32), "GetMonitorInfoW")
	funcEnumDisplaySettings, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplaySettingsW")
)

func Capture(x, y, width, height int) (*image.RGBA, error) {
	rect := image.Rect(0, 0, width, height)
	img, err := createImage(rect)
	if err != nil {
		return nil, err
	}

	hwnd := getDesktopWindow()
	hdcScreen := win.GetDC(0)
	hdcWindow := win.GetDC(hwnd)
	if hdcWindow == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer win.ReleaseDC(hwnd, hdcWindow)

	hdcMemDC := win.CreateCompatibleDC(hdcWindow)
	if hdcMemDC == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(hdcMemDC)

	//New
	// Get the client area for size calculation.
	var rcClient win.RECT
	win.GetClientRect(hwnd, &rcClient)

	// This is the best stretch mode.
	win.SetStretchBltMode(hdcWindow, win.HALFTONE)

	// The source DC is the entire screen, and the destination DC is the current window (HWND)
	if !win.StretchBlt(hdcWindow,
		0, 0,
		rcClient.Right, rcClient.Bottom,
		hdcScreen,
		0, 0,
		win.GetSystemMetrics(win.SM_CXSCREEN),
		win.GetSystemMetrics(win.SM_CYSCREEN),
		win.SRCCOPY) {
		err := windows.GetLastError()
		return nil, errors.Join(errors.New("StretchBlt has failed"), err)
	}

	// Create a compatible bitmap from the Window DC.
	hbmScreen := win.CreateCompatibleBitmap(hdcWindow,
		rcClient.Right-rcClient.Left,
		rcClient.Bottom-rcClient.Top)
	if hbmScreen == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(hbmScreen))

	// Select the compatible bitmap into the compatible memory DC.
	win.SelectObject(hdcMemDC, win.HGDIOBJ(hbmScreen))

	// Bit block transfer into our compatible memory DC.
	if !win.BitBlt(hdcMemDC,
		0, 0,
		rcClient.Right-rcClient.Left, rcClient.Bottom-rcClient.Top,
		hdcWindow,
		0, 0,
		win.SRCCOPY) {
		err := windows.GetLastError()
		return nil, errors.Join(errors.New("BitBlt failed"), err)
	}

	// Get the BITMAP from the HBITMAP.
	var bmpScreen win.BITMAP
	win.GetObject(win.HGDIOBJ(hbmScreen), unsafe.Sizeof(bmpScreen), unsafe.Pointer(&bmpScreen))

	var bi win.BITMAPINFOHEADER
	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = int32(width)
	bi.BiHeight = int32(-height)
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	bi.BiCompression = win.BI_RGB
	bi.BiSizeImage = 0
	bi.BiXPelsPerMeter = 0
	bi.BiYPelsPerMeter = 0
	bi.BiClrUsed = 0
	bi.BiClrImportant = 0

	// GetDIBits balks at using Go memory on some systems. The MSDN example uses
	// GlobalAlloc, so we'll do that too. See:
	// https://docs.microsoft.com/en-gb/windows/desktop/gdi/capturing-an-image
	//	bitmapDataSize := uintptr(((int64(width)*int64(bi.BiBitCount) + 31) / 32) * 4 * int64(height))
	dwBmpSize := uintptr(((int64(bmpScreen.BmWidth)*int64(bi.BiBitCount) + 31) / 32) * 4 * int64(bmpScreen.BmHeight))
	hDIB := win.GlobalAlloc(win.GMEM_MOVEABLE, dwBmpSize)
	defer win.GlobalFree(hDIB)
	lpbitmap := win.GlobalLock(hDIB)
	defer win.GlobalUnlock(hDIB)

	/*	old := win.SelectObject(hdcMemDC, win.HGDIOBJ(bitmap))
		if old == 0 {
			return nil, errors.New("SelectObject failed")
		}
		defer win.SelectObject(hdcMemDC, old)

		if x == width || y == height {
			return nil, errors.New("size failed (width or height are consistent)")
		}
		if !win.BitBlt(hdcMemDC, 0, 0, int32(width), int32(height), hdcWindow, int32(x), int32(y), win.SRCCOPY) {
			err := windows.GetLastError()
			return nil, errors.Join(errors.New("BitBlt failed"), err)
		}*/

	if win.GetDIBits(hdcWindow, hbmScreen,
		0,
		uint32(bmpScreen.BmHeight),
		(*uint8)(lpbitmap),
		(*win.BITMAPINFO)(unsafe.Pointer(&bi)),
		win.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	i := 0
	src := uintptr(lpbitmap)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v0 := *(*uint8)(unsafe.Pointer(src))
			v1 := *(*uint8)(unsafe.Pointer(src + 1))
			v2 := *(*uint8)(unsafe.Pointer(src + 2))

			// BGRA => RGBA, and set A to 255
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255

			i += 4
			src += 4
		}
	}

	return img, nil
}

func getDesktopWindow() win.HWND {
	ret, _, _ := syscall.SyscallN(funcGetDesktopWindow, 0, 0, 0)
	return win.HWND(ret)
}

func enumDisplayMonitors(hdc win.HDC, lprcClip *win.RECT, lpfnEnum uintptr, dwData uintptr) bool {
	ret, _, _ := syscall.SyscallN(funcEnumDisplayMonitors,
		uintptr(hdc),
		uintptr(unsafe.Pointer(lprcClip)),
		lpfnEnum,
		dwData,
		0,
		0)
	return int(ret) != 0
}

func countupMonitorCallback(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var count *int
	count = (*int)(unsafe.Pointer(dwData))
	*count = *count + 1
	return uintptr(1)
}

type getMonitorBoundsContext struct {
	Index int
	Rect  win.RECT
	Count int
}

func getMonitorBoundsCallback(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var ctx *getMonitorBoundsContext
	ctx = (*getMonitorBoundsContext)(unsafe.Pointer(dwData))
	if ctx.Count != ctx.Index {
		ctx.Count = ctx.Count + 1
		return uintptr(1)
	}

	if realSize := getMonitorRealSize(hMonitor); realSize != nil {
		ctx.Rect = *realSize
	} else {
		ctx.Rect = *lprcMonitor
	}

	return uintptr(0)
}

type _MONITORINFOEX struct {
	win.MONITORINFO
	DeviceName [win.CCHDEVICENAME]uint16
}

const _ENUM_CURRENT_SETTINGS = 0xFFFFFFFF

type _DEVMODE struct {
	_            [68]byte
	DmSize       uint16
	_            [6]byte
	DmPosition   win.POINT
	_            [86]byte
	DmPelsWidth  uint32
	DmPelsHeight uint32
	_            [40]byte
}

// getMonitorRealSize makes a call to GetMonitorInfo
// to obtain the device name for the monitor handle
// provided to the method.
//
// With the device name, EnumDisplaySettings is called to
// obtain the current configuration for the monitor, this
// information includes the real resolution of the monitor
// rather than the scaled version based on DPI.
//
// If either handle calls fail, it will return a nil
// allowing the calling method to use the bounds information
// returned by EnumDisplayMonitors which may be affected
// by DPI.
func getMonitorRealSize(hMonitor win.HMONITOR) *win.RECT {
	info := _MONITORINFOEX{}
	info.CbSize = uint32(unsafe.Sizeof(info))

	//	ret, _, _ := syscall.Syscall( funcGetMonitorInfo, 2, uintptr(hMonitor), uintptr(unsafe.Pointer(&info)), 0)
	ret, _, _ := syscall.SyscallN(funcGetMonitorInfo, uintptr(hMonitor), uintptr(unsafe.Pointer(&info)), 0)
	if ret == 0 {
		return nil
	}

	devMode := _DEVMODE{}
	devMode.DmSize = uint16(unsafe.Sizeof(devMode))

	if ret, _, _ := syscall.SyscallN(funcEnumDisplaySettings, uintptr(unsafe.Pointer(&info.DeviceName[0])), _ENUM_CURRENT_SETTINGS, uintptr(unsafe.Pointer(&devMode))); ret == 0 {
		return nil
	}

	return &win.RECT{
		Left:   devMode.DmPosition.X,
		Right:  devMode.DmPosition.X + int32(devMode.DmPelsWidth),
		Top:    devMode.DmPosition.Y,
		Bottom: devMode.DmPosition.Y + int32(devMode.DmPelsHeight),
	}
}
