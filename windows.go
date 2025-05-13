//go:build windows

package screenshot

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var (
	libUser32               = syscall.NewLazyDLL("user32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	funcGetDesktopWindow    = libUser32.NewProc("GetDesktopWindow")
	funcEnumDisplayMonitors = libUser32.NewProc("EnumDisplayMonitors")
	funcGetMonitorInfo      = libUser32.NewProc("GetMonitorInfoW")
	funcEnumDisplaySettings = libUser32.NewProc("EnumDisplaySettingsW")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
)

func Capture(xZ, yZ, width, height int) (*image.RGBA, error) {
	hwnd := getDesktopWindow()
	if hwnd == 0 {
		return nil, fmt.Errorf("failed to get desktop window: %d", win.GetLastError())
	}

	hDC := win.GetDC(hwnd)
	if hDC == 0 {
		return nil, fmt.Errorf("failed to get device context: %d", win.GetLastError())
	}
	defer win.ReleaseDC(hwnd, hDC)

	hdcMemDC := win.CreateCompatibleDC(hDC)
	if hdcMemDC == 0 {
		return nil, fmt.Errorf("failed to create compatible DC: %d", win.GetLastError())
	}
	defer win.DeleteDC(hdcMemDC)

	pixel := win.GetDeviceCaps(hDC, win.BITSPIXEL)
	bt := win.BITMAPINFO{
		BmiHeader: win.BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
			BiWidth:       int32(width),
			BiHeight:      int32(-height),
			BiPlanes:      1,
			BiBitCount:    uint16(pixel),
			BiCompression: win.BI_RGB,
		},
	}

	var lpbitmap unsafe.Pointer
	m_hBmp := CreateDIBSection(hdcMemDC, &bt, DIB_RGB_COLORS, &lpbitmap, 0, 0)
	if m_hBmp == 0 {
		return nil, fmt.Errorf("failed to create DIB section: %d", win.GetLastError())
	}
	defer win.DeleteObject(win.HGDIOBJ(m_hBmp))

	obj := win.SelectObject(hdcMemDC, win.HGDIOBJ(m_hBmp))
	if obj == 0 || obj == 0xffffffff {
		return nil, fmt.Errorf("failed to select object: %d", win.GetLastError())
	}
	defer win.SelectObject(hdcMemDC, obj)

	if !win.BitBlt(hdcMemDC, 0, 0, int32(width), int32(height), hDC, int32(xZ), int32(yZ), SRCCOPY) {
		return nil, fmt.Errorf("bitblt failed: %d", win.GetLastError())
	}

	rect := image.Rect(0, 0, width, height)
	img, err := createImage(rect)
	if err != nil {
		return nil, err
	}

	if lpbitmap == nil {
		return nil, fmt.Errorf("failed to get bitmap data: %d", win.GetLastError())
	}
	bitmapData := unsafe.Slice((*byte)(lpbitmap), width*height*4)

	// Копируем данные в image.RGBA
	for i := 0; i < len(bitmapData); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bitmapData[i+2], bitmapData[i+1], bitmapData[i], 255
	}

	return img, nil
}

func getDesktopWindow() win.HWND {
	ret, _, _ := funcGetDesktopWindow.Call(0, 0, 0)
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
	ret, _, _ := funcEnumDisplayMonitors.Call(
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

func getMonitorBoundsCallback(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	// Проверяем корректность указателя
	if dwData == 0 {
		return uintptr(0) // Некорректный указатель
	}

	// Преобразуем dwData в контекст
	ctx, err := unsafeToContext(dwData)
	if err != nil {
		return uintptr(0)
	}
	pinner := new(runtime.Pinner)
	pinner.Pin(ctx)
	defer pinner.Unpin()

	// Проверяем индексацию
	if ctx.Count != ctx.Index {
		ctx.Count++
		return uintptr(1) // Продолжаем перечисление
	}

	// Получаем реальные размеры монитора
	if realSize := getMonitorRealSize(hMonitor); realSize != nil {
		ctx.Rect = *realSize
	} else if lprcMonitor != nil {
		ctx.Rect = *lprcMonitor
	} else {
		return uintptr(0) // Останавливаем перечисление при ошибке
	}

	// Останавливаем перечисление
	return uintptr(0)
}

func unsafeToContext(dwData uintptr) (*getMonitorBoundsContext, error) {
	if dwData == 0 {
		return nil, errors.New("invalid pointer")
	}
	return (*getMonitorBoundsContext)(unsafe.Pointer(dwData)), nil
}

type getMonitorBoundsContext struct {
	Index int
	Rect  win.RECT
	Count int
}

func getMonitors() []win.RECT {
	var monitors []win.RECT
	callback := syscall.NewCallback(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		monitors = append(monitors, *lprcMonitor)
		return 1
	})

	enumDisplayMonitors(0, nil, callback, 0)
	return monitors
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

	ret, _, _ := funcGetMonitorInfo.Call(uintptr(hMonitor), uintptr(unsafe.Pointer(&info)), 0)
	if ret == 0 {
		return nil
	}

	devMode := _DEVMODE{}
	devMode.DmSize = uint16(unsafe.Sizeof(devMode))

	ret, _, _ = funcEnumDisplaySettings.Call(
		uintptr(unsafe.Pointer(&info.DeviceName[0])),
		_ENUM_CURRENT_SETTINGS,
		uintptr(unsafe.Pointer(&devMode)),
	)
	if ret == 0 {
		return nil
	}

	if devMode.DmPelsWidth == 0 || devMode.DmPelsHeight == 0 {
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
	HORZRES        = 8
	VERTRES        = 10
	BI_RGB         = 0
	DIB_RGB_COLORS = 0
	SRCCOPY        = 0x00CC0020
)
