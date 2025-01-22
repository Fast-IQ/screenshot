//go:build windows

package screenshot

import (
	"fmt"
	"github.com/lxn/win"
	"image"
	"syscall"
	"unsafe"
)

var (
	libUser32, _               = syscall.LoadLibrary("user32.dll")
	modgdi32                   = syscall.NewLazyDLL("gdi32.dll")
	funcGetDesktopWindow, _    = syscall.GetProcAddress(syscall.Handle(libUser32), "GetDesktopWindow")
	funcEnumDisplayMonitors, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplayMonitors")
	funcGetMonitorInfo, _      = syscall.GetProcAddress(syscall.Handle(libUser32), "GetMonitorInfoW")
	funcEnumDisplaySettings, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplaySettingsW")
	procCreateDIBSection       = modgdi32.NewProc("CreateDIBSection")
)

func Capture(xZ, yZ, width, height int) (*image.RGBA, error) {
	hDC := win.GetDC(0)
	if hDC == 0 {
		return nil, fmt.Errorf("Could not Get primary display err:%d.\n", win.GetLastError())
	}
	defer win.ReleaseDC(0, hDC)

	hdcMemDC := win.CreateCompatibleDC(hDC)
	if hdcMemDC == 0 {
		return nil, fmt.Errorf("Could not Create Compatible DC err:%d.\n", win.GetLastError())
	}
	defer win.DeleteDC(hdcMemDC)

	//New
	// Get the client area for size calculation.
	hwnd := getDesktopWindow()
	var rcClient win.RECT
	win.GetClientRect(hwnd, &rcClient)

	bt := win.BITMAPINFO{}
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
	bt.BmiHeader = bi

	lpbitmap := unsafe.Pointer(uintptr(0))

	m_hBmp := CreateDIBSection(hdcMemDC, &bt, DIB_RGB_COLORS, &lpbitmap, 0, 0)
	if m_hBmp == 0 {
		return nil, fmt.Errorf("Could not Create DIB Section err:%d.\n", win.GetLastError())
	}
	if m_hBmp == InvalidParameter {
		return nil, fmt.Errorf("One or more of the input parameters is invalid while calling CreateDIBSection.\n")
	}
	defer win.DeleteObject(win.HGDIOBJ(m_hBmp))

	obj := win.SelectObject(hdcMemDC, win.HGDIOBJ(m_hBmp))
	if obj == 0 {
		return nil, fmt.Errorf("error occurred and the selected object is not a region err:%d.\n", win.GetLastError())
	}
	if obj == 0xffffffff { //GDI_ERROR
		return nil, fmt.Errorf("GDI_ERROR while calling SelectObject err:%d.\n", win.GetLastError())
	}
	defer win.DeleteObject(obj)

	if !win.BitBlt(hdcMemDC, 0, 0, int32(width), int32(height), hDC, int32(xZ), int32(yZ), SRCCOPY) {
		return nil, fmt.Errorf("BitBlt failed err:%d.", win.GetLastError())
	}

	/*	var slice []byte
		hdrp := (*reflect.SliceHeader)(unsafe.Pointer(&slice))
		hdrp.Data = uintptr(ptr)
		hdrp.Len = width * height * 4
		hdrp.Cap = width * height * 4

		imageBytes := make([]byte, len(slice))

		for i := 0; i < len(imageBytes); i += 4 {
			imageBytes[i], imageBytes[i+2], imageBytes[i+1], imageBytes[i+3] = slice[i+2], slice[i], slice[i+1], slice[i+3]
		}

		img := &image.RGBA{imageBytes, 4 * width, image.Rect(0, 0, width, height)}*/
	rect := image.Rect(0, 0, width, height)
	img, err := createImage(rect)
	if err != nil {
		return nil, err
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

func CreateDIBSection(hdc win.HDC, pbmi *win.BITMAPINFO, iUsage uint, ppvBits *unsafe.Pointer, hSection win.HANDLE, dwOffset uint) win.HBITMAP {
	ret, _, _ := procCreateDIBSection.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(pbmi)),
		uintptr(iUsage),
		uintptr(unsafe.Pointer(ppvBits)),
		uintptr(hSection),
		uintptr(dwOffset))

	return win.HBITMAP(ret)
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

const (
	HORZRES          = 8
	VERTRES          = 10
	BI_RGB           = 0
	InvalidParameter = 2
	DIB_RGB_COLORS   = 0
	SRCCOPY          = 0x00CC0020
)
