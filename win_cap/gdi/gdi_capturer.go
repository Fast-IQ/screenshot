//go:build windows

package gdi

import (
	"errors"
	"fmt"
	"github.com/Fast-IQ/screenshot/win_cap"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"image"
	"sync"
	"unsafe"
)

type GDICapturer struct {
	monitors []image.Rectangle
	dpiCache sync.Map
}

// === Подключаем Windows API функции ===

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	funcGetDesktopWindow = user32.NewProc("GetDesktopWindow")
	funcCreateDIBSection = gdi32.NewProc("CreateDIBSection")

	// Для DPI-aware начиная с Windows 10
	funcGetDpiForWindow = user32.NewProc("GetDpiForWindow")
)

// === Константы ===
const (
	HORZRES        = 8
	VERTRES        = 10
	BI_RGB         = 0
	DIB_RGB_COLORS = 0
	SRCCOPY        = 0x00CC0020
	LOGPIXELSX     = 88
)

func (c *GDICapturer) Capture(x, y, width, height int) (*image.RGBA, error) {
	hwnd := GetDesktopWindow()
	if hwnd == 0 {
		return nil, fmt.Errorf("failed to get desktop window")
	}

	hDC := win.GetDC(hwnd)
	if hDC == 0 {
		return nil, fmt.Errorf("failed to get device context")
	}
	defer win.ReleaseDC(hwnd, hDC)

	screenDPI := c.getCachedDPI(hDC)
	scaledWidth := ScaleForDPI(width, screenDPI)
	scaledHeight := ScaleForDPI(height, screenDPI)

	if scaledWidth <= 0 || scaledHeight <= 0 {
		return nil, fmt.Errorf("invalid scaled size: %dx%d", scaledWidth, scaledHeight)
	}

	hdcMemDC := win.CreateCompatibleDC(hDC)
	if hdcMemDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(hdcMemDC)

	pixel := win.GetDeviceCaps(hDC, win.BITSPIXEL)
	if pixel == 0 {
		return nil, fmt.Errorf("failed to get bits per pixel")
	}

	bt := win.BITMAPINFO{
		BmiHeader: win.BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
			BiWidth:       int32(scaledWidth),
			BiHeight:      -int32(scaledHeight),
			BiPlanes:      1,
			BiBitCount:    uint16(pixel),
			BiCompression: BI_RGB,
		},
	}

	var bits unsafe.Pointer
	mBmp := CreateDIBSection(hdcMemDC, &bt, DIB_RGB_COLORS, &bits, 0, 0)
	if mBmp == 0 {
		return nil, fmt.Errorf("failed to create DIB section")
	}
	defer win.DeleteObject(win.HGDIOBJ(mBmp))

	oldObj := win.SelectObject(hdcMemDC, win.HGDIOBJ(mBmp))
	if oldObj == 0 {
		return nil, fmt.Errorf("SelectObject failed")
	}
	defer win.SelectObject(hdcMemDC, oldObj)

	if !win.BitBlt(hdcMemDC, 0, 0, int32(scaledWidth), int32(scaledHeight), hDC,
		int32(ScaleForDPI(x, screenDPI)), int32(ScaleForDPI(y, screenDPI)), SRCCOPY) {
		return nil, fmt.Errorf("bitblt failed")
	}

	// Создаем копию данных перед возвратом
	dataSize := scaledWidth * scaledHeight * 4
	pixelData := unsafe.Slice((*byte)(bits), dataSize)

	// Копируем данные в новый буфер
	copiedData := make([]byte, dataSize)
	copy(copiedData, pixelData)

	return &image.RGBA{
		Pix:    copiedData,
		Stride: scaledWidth * 4,
		Rect:   image.Rect(0, 0, width, height),
	}, nil

}

func (c *GDICapturer) GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	bounds, err := c.GetAllDisplayBounds()
	if err != nil {
		return image.Rectangle{}, err
	}
	if displayIndex < 0 || displayIndex >= len(bounds) {
		return image.Rectangle{}, fmt.Errorf("invalid display index: %d", displayIndex)
	}
	return bounds[displayIndex], nil
}

func (c *GDICapturer) GetAllDisplayBounds() ([]image.Rectangle, error) {
	if c.monitors != nil {
		return c.monitors, nil
	}

	var monitors []image.Rectangle
	callback := func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		monitors = append(monitors, toImageRect(*lprcMonitor))
		return 1
	}

	if !win_cap.EnumDisplayMonitors(0, nil, windows.NewCallback(callback), 0) {
		return nil, errors.New("EnumDisplayMonitors failed")
	}

	c.monitors = monitors
	return monitors, nil
}

func GetDesktopWindow() win.HWND {
	ret, _, _ := funcGetDesktopWindow.Call()
	return win.HWND(ret)
}

func CreateDIBSection(hdc win.HDC, pbmi *win.BITMAPINFO, iUsage uint, ppvBits *unsafe.Pointer, hSection win.HANDLE, dwOffset uint) win.HBITMAP {
	ret, _, _ := funcCreateDIBSection.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(pbmi)),
		uintptr(iUsage),
		uintptr(unsafe.Pointer(ppvBits)),
		uintptr(hSection),
		uintptr(dwOffset),
	)
	return win.HBITMAP(ret)
}

func toImageRect(r win.RECT) image.Rectangle {
	if r.Right < r.Left || r.Bottom < r.Top {
		return image.Rect(0, 0, 0, 0)
	}
	return image.Rect(
		int(r.Left),
		int(r.Top),
		int(r.Right),
		int(r.Bottom),
	)
}

// === DPI-зависимые функции ===

// Проверяем, доступна ли функция GetDpiForWindow (Windows 10+)
func supportsPerMonitorDPI() bool {
	return funcGetDpiForWindow.Find() == nil
}

// Получаем DPI текущего контекста устройства
func GetDPI(hdc win.HDC) int {
	return int(win.GetDeviceCaps(hdc, LOGPIXELSX))
}

func (c *GDICapturer) getCachedDPI(hdc win.HDC) int {
	if dpi, ok := c.dpiCache.Load(hdc); ok {
		return dpi.(int)
	}

	dpi := GetDPI(hdc)
	c.dpiCache.Store(hdc, dpi)
	return dpi
}

// Вычисляем коэффициент масштабирования DPI
func ScaleForDPI(value int, dpi int) int {
	if dpi <= 96 {
		return value
	}
	return (value*dpi + 48) / 96 // Округление вместо усечения
}
