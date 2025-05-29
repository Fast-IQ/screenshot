//go:build windows

package gdi

import (
	"errors"
	"fmt"
	"github.com/Fast-IQ/screenshot/win_cap"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"image"
	"runtime"
	"unsafe"
)

type GDICapturer struct {
	monitors []image.Rectangle
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
	defer runtime.KeepAlive(hwnd)

	hDC := win.GetDC(hwnd)
	if hDC == 0 {
		return nil, fmt.Errorf("failed to get device context")
	}
	defer win.ReleaseDC(hwnd, hDC)
	defer runtime.KeepAlive(hDC)

	screenDPI := GetDPI(hDC)
	scaledWidth := ScaleForDPI(width, screenDPI)
	scaledHeight := ScaleForDPI(height, screenDPI)

	if scaledWidth <= 0 || scaledHeight <= 0 {
		return nil, fmt.Errorf("invalid scaled size: %dx%d", scaledWidth, scaledHeight)
	}

	hdcMemDC := win.CreateCompatibleDC(hDC)
	if hdcMemDC == 0 {
		return nil, fmt.Errorf("failed to create memory DC")
	}
	defer win.DeleteDC(hdcMemDC)
	defer runtime.KeepAlive(hdcMemDC)

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
		return nil, fmt.Errorf("failed to create DIB section: %d", win.GetLastError())
	}
	defer win.DeleteObject(win.HGDIOBJ(mBmp))

	oldObj := win.SelectObject(hdcMemDC, win.HGDIOBJ(mBmp))
	if oldObj == 0 || oldObj == 0xffffffff {
		return nil, fmt.Errorf("SelectObject failed: %d", win.GetLastError())
	}
	defer win.SelectObject(hdcMemDC, oldObj)

	if ok := win.BitBlt(hdcMemDC, 0, 0, int32(scaledWidth), int32(scaledHeight), hDC,
		int32(ScaleForDPI(x, screenDPI)), int32(ScaleForDPI(y, screenDPI)), SRCCOPY); !ok {
		return nil, fmt.Errorf("bitblt failed")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if bits == nil {
		return nil, errors.New("bitmap data is nil")
	}

	bitmapData := unsafe.Slice((*byte)(bits), scaledWidth*scaledHeight*4)

	for dstY := 0; dstY < height; dstY++ {
		for dstX := 0; dstX < width; dstX++ {
			srcX := dstX * scaledWidth / width
			srcY := dstY * scaledHeight / height
			idxSrc := (srcY*scaledWidth + srcX) * 4
			idxDst := (dstY*width + dstX) * 4

			if idxSrc+3 >= len(bitmapData) {
				continue // избежание out-of-bounds
			}

			img.Pix[idxDst+0] = bitmapData[idxSrc+2]
			img.Pix[idxDst+1] = bitmapData[idxSrc+1]
			img.Pix[idxDst+2] = bitmapData[idxSrc+0]
			img.Pix[idxDst+3] = 255
		}
	}

	return img, nil
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
	if c.monitors == nil {
		pinner := new(runtime.Pinner)
		pinner.Pin(&c.monitors)
		defer pinner.Unpin()

		callback := windows.NewCallbackCDecl(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
			c.monitors = append(c.monitors, toImageRect(*lprcMonitor))
			runtime.KeepAlive(c.monitors)
			return 1
		})
		//defer callback.Release()

		success := win_cap.EnumDisplayMonitors(0, nil, callback, 0)
		if !success || len(c.monitors) == 0 {
			return nil, errors.New("no monitors found")
		}
	}
	return c.monitors, nil
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

// Вычисляем коэффициент масштабирования DPI
func ScaleForDPI(value int, dpi int) int {
	switch {
	case dpi < 96:
		return value
	case dpi == 96:
		return value
	default:
		return (value * dpi) / 96
	}
}
