//go:build windows && amd64

package wgc

import (
	"errors"
	"fmt"
	"github.com/Fast-IQ/screenshot/win_cap"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"image"
	"sync"
	"syscall"
	"unsafe"
)

const (
	S_OK                       = 0x00000000
	DXGI_FORMAT_B8G8R8A8_UNORM = 87
	D3D11_SDK_VERSION          = 7
)

var (
	libGraphicsCapture = windows.NewLazySystemDLL("windows.graphics.capture.dll")
	libD3D11           = windows.NewLazySystemDLL("d3d11.dll")
	libDXGI            = windows.NewLazySystemDLL("dxgi.dll")
	libOle32           = windows.NewLazySystemDLL("ole32.dll")

	procCreateCaptureItemForWindow = libGraphicsCapture.NewProc("CreateCaptureItemForWindow")
	procD3D11CreateDevice          = libD3D11.NewProc("D3D11CreateDevice")
	procCoCreateInstance           = libOle32.NewProc("CoCreateInstance")
)

// COM-интерфейсы с минимальным набором методов
type IUnknown struct {
	vtbl *IUnknownVtbl
}

type IUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

func (u *IUnknown) Release() uint32 {
	if u == nil || u.vtbl == nil {
		return S_OK
	}
	ret, _, _ := syscall.SyscallN(u.vtbl.Release, uintptr(unsafe.Pointer(u)))
	return uint32(ret)
}

type ID3D11Device struct {
	IUnknown
}

type ID3D11DeviceContext struct {
	IUnknown
}

type IGraphicsCaptureItem struct {
	IUnknown
}

type IGraphicsCaptureFramePool struct {
	IUnknown
}

type IGraphicsCaptureSession struct {
	IUnknown
}

type IGraphicsCaptureFrame struct {
	IUnknown
}

type WGCCapturer struct {
	device     *ID3D11Device
	context    *ID3D11DeviceContext
	item       *IGraphicsCaptureItem
	framePool  *IGraphicsCaptureFramePool
	session    *IGraphicsCaptureSession
	lastFrame  *IGraphicsCaptureFrame
	hwnd       windows.HWND
	dpiCache   sync.Map
	frameMutex sync.Mutex
}

func NewWGCCapturer() (*WGCCapturer, error) {
	// Проверяем поддержку WGC
	if !isWindowsGraphicsCaptureSupported() {
		return nil, errors.New("Windows Graphics Capture not supported")
	}

	// Инициализация COM
	if hr := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); hr != nil {
		return nil, fmt.Errorf("COM initialization failed: %v", hr)
	}

	c := &WGCCapturer{
		hwnd: getDesktopWindow(),
	}

	// Инициализация с откатом при ошибках
	var initErr error
	defer func() {
		if initErr != nil {
			c.Close()
		}
	}()

	if initErr = c.initD3D11(); initErr != nil {
		return nil, fmt.Errorf("D3D11 init failed: %w", initErr)
	}

	if initErr = c.createCaptureItem(); initErr != nil {
		return nil, fmt.Errorf("createCaptureItem failed: %w", initErr)
	}

	if initErr = c.createFramePool(); initErr != nil {
		return nil, fmt.Errorf("createFramePool failed: %w", initErr)
	}

	if initErr = c.startCapture(); initErr != nil {
		return nil, fmt.Errorf("startCapture failed: %w", initErr)
	}

	return c, nil
}

func (c *WGCCapturer) initD3D11() error {
	var device *ID3D11Device
	var context *ID3D11DeviceContext

	hr, _, _ := procD3D11CreateDevice.Call(
		0,
		1, // D3D_DRIVER_TYPE_HARDWARE
		0,
		0,
		0,
		0,
		D3D11_SDK_VERSION,
		uintptr(unsafe.Pointer(&device)),
		uintptr(unsafe.Pointer(&context)),
		0,
	)

	if hr != S_OK {
		return fmt.Errorf("D3D11CreateDevice failed: 0x%X", hr)
	}

	c.device = device
	c.context = context
	return nil
}

func (c *WGCCapturer) createCaptureItem() error {
	// Пытаемся получить IGraphicsCaptureItemInterop
	var interop *IGraphicsCaptureItemInterop
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_GraphicsCaptureItem)),
		0,
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IGraphicsCaptureItemInterop)),
		uintptr(unsafe.Pointer(&interop)),
	)
	if hr != S_OK {
		return fmt.Errorf("failed to create capture interop: 0x%X", hr)
	}
	defer interop.Release()

	// Создаем CaptureItem для окна
	var item *IGraphicsCaptureItem
	hr = uintptr(interop.CreateForWindow(c.hwnd, &IID_IGraphicsCaptureItem, &item))
	if hr != S_OK {
		return fmt.Errorf("failed to create capture item: 0x%X", hr)
	}

	c.item = item
	return nil
}

func (c *WGCCapturer) createFramePool() error {
	var framePool *IGraphicsCaptureFramePool
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_GraphicsCaptureFramePool)),
		0,
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IGraphicsCaptureFramePool)),
		uintptr(unsafe.Pointer(&framePool)),
	)

	if hr != S_OK {
		return fmt.Errorf("createFramePool failed: 0x%X", hr)
	}

	c.framePool = framePool
	return nil
}

func (c *WGCCapturer) startCapture() error {
	var session *IGraphicsCaptureSession
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_GraphicsCaptureSession)),
		0,
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IGraphicsCaptureSession)),
		uintptr(unsafe.Pointer(&session)),
	)

	if hr != S_OK {
		return fmt.Errorf("startCapture failed: 0x%X", hr)
	}

	c.session = session
	return nil
}

func (c *WGCCapturer) getCachedDPI(hwnd windows.HWND) int {
	if dpi, ok := c.dpiCache.Load(hwnd); ok {
		return dpi.(int)
	}

	dpi := getDPIForWindow(hwnd)
	c.dpiCache.Store(hwnd, dpi)
	return dpi
}

func (c *WGCCapturer) Capture(x, y, width, height int) (*image.RGBA, error) {
	if c == nil {
		return nil, errors.New("WGCCapturer is nil")
	}

	c.frameMutex.Lock()
	defer c.frameMutex.Unlock()

	dpi := c.getCachedDPI(c.hwnd)
	scaledWidth := scaleForDPI(width, dpi)
	scaledHeight := scaleForDPI(height, dpi)

	frame, err := c.getFrame()
	if err != nil {
		return nil, fmt.Errorf("getFrame failed: %w", err)
	}
	defer frame.Release()

	img, err := c.frameToImage(frame, scaledWidth, scaledHeight)
	if err != nil {
		return nil, fmt.Errorf("frameToImage failed: %w", err)
	}

	// Обрезаем изображение до запрошенных координат и размеров
	scaledX := scaleForDPI(x, dpi)
	scaledY := scaleForDPI(y, dpi)
	subImg := img.SubImage(image.Rect(scaledX, scaledY, scaledX+scaledWidth, scaledY+scaledHeight))

	// Создаем копию, чтобы избежать проблем с памятью
	bounds := subImg.Bounds()
	result := image.NewRGBA(bounds)
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			result.Set(px, py, subImg.At(px, py))
		}
	}

	return result, nil
}

func (c *WGCCapturer) GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	bounds, err := c.GetAllDisplayBounds()
	if err != nil {
		return image.Rectangle{}, err
	}
	if displayIndex < 0 || displayIndex >= len(bounds) {
		return image.Rectangle{}, fmt.Errorf("invalid display index: %d", displayIndex)
	}
	return bounds[displayIndex], nil
}

func (c *WGCCapturer) GetAllDisplayBounds() ([]image.Rectangle, error) {
	var monitors []image.Rectangle
	callback := func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		monitors = append(monitors, toImageRect(*lprcMonitor))
		return 1
	}

	if !win_cap.EnumDisplayMonitors(0, nil, windows.NewCallback(callback), 0) {
		return nil, errors.New("EnumDisplayMonitors failed")
	}

	return monitors, nil
}

func (c *WGCCapturer) getFrame() (*IGraphicsCaptureFrame, error) {
	if c == nil || c.framePool == nil || c.framePool.vtbl == nil {
		return nil, errors.New("WGCCapturer or framePool not initialized")
	}

	var frame *IGraphicsCaptureFrame
	hr, _, _ := syscall.SyscallN(
		c.framePool.vtbl.QueryInterface,
		uintptr(unsafe.Pointer(c.framePool)),
		uintptr(unsafe.Pointer(&IID_IGraphicsCaptureFramePool)),
		uintptr(unsafe.Pointer(&frame)),
	)

	if hr != S_OK {
		return nil, fmt.Errorf("TryGetNextFrame failed: 0x%X", hr)
	}

	if c.lastFrame != nil {
		c.lastFrame.Release()
	}
	c.lastFrame = frame

	return frame, nil
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

func (c *WGCCapturer) frameToImage(frame *IGraphicsCaptureFrame, width, height int) (*image.RGBA, error) {
	var surface *IDXGISurface
	hr, _, _ := syscall.SyscallN(
		frame.vtbl.QueryInterface,
		uintptr(unsafe.Pointer(frame)),
		uintptr(unsafe.Pointer(&IID_IDXGISurface)),
		uintptr(unsafe.Pointer(&surface)),
	)
	if hr != S_OK {
		return nil, fmt.Errorf("getSurface failed: 0x%X", hr)
	}
	defer func() {
		if surface != nil {
			// Вызываем Release через IUnknownVtbl
			syscall.SyscallN(
				surface.vtbl.Release,
				uintptr(unsafe.Pointer(surface)),
			)
		}
	}()

	var desc DXGI_SURFACE_DESC
	if hr := surface.GetDesc(&desc); hr != S_OK {
		return nil, fmt.Errorf("GetDesc failed: 0x%X", hr)
	}

	var mappedRect DXGI_MAPPED_RECT
	if hr := surface.Map(&mappedRect, DXGI_MAP_READ); hr != S_OK {
		return nil, fmt.Errorf("Map failed: 0x%X", hr)
	}
	defer surface.Unmap()

	img := image.NewRGBA(image.Rect(0, 0, int(desc.Width), int(desc.Height)))
	bytesPerPixel := 4
	src := unsafe.Slice(mappedRect.PBits, int(mappedRect.Pitch)*int(desc.Height))

	for y := 0; y < int(desc.Height); y++ {
		srcRow := src[y*int(mappedRect.Pitch):]
		dstRow := img.Pix[y*img.Stride:]
		copy(dstRow[:img.Rect.Dx()*bytesPerPixel],
			srcRow[:img.Rect.Dx()*bytesPerPixel])
	}

	return img, nil
}

func (c *WGCCapturer) Close() error {
	// Освобождаем ресурсы в правильном порядке
	var errs []error

	if c.session != nil {
		if hr := c.session.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("session release failed: 0x%X", hr))
		}
		c.session = nil
	}

	if c.lastFrame != nil {
		if hr := c.lastFrame.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("lastFrame release failed: 0x%X", hr))
		}
		c.lastFrame = nil
	}

	if c.framePool != nil {
		if hr := c.framePool.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("framePool release failed: 0x%X", hr))
		}
		c.framePool = nil
	}

	if c.item != nil {
		if hr := c.item.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("item release failed: 0x%X", hr))
		}
		c.item = nil
	}

	if c.context != nil {
		if hr := c.context.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("context release failed: 0x%X", hr))
		}
		c.context = nil
	}

	if c.device != nil {
		if hr := c.device.Release(); hr != S_OK {
			errs = append(errs, fmt.Errorf("device release failed: 0x%X", hr))
		}
		c.device = nil
	}

	windows.CoUninitialize()

	if len(errs) > 0 {
		return fmt.Errorf("errors while closing: %v", errs)
	}
	return nil
}

func getDesktopWindow() windows.HWND {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("GetDesktopWindow")
	ret, _, _ := proc.Call()
	return windows.HWND(ret)
}

func getDPIForWindow(hwnd windows.HWND) int {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("GetDpiForWindow")
	dpi, _, _ := proc.Call(uintptr(hwnd))
	return int(dpi)
}

func scaleForDPI(value int, dpi int) int {
	if dpi <= 96 {
		return value
	}
	return (value*dpi + 48) / 96 // С округлением
}

func isWindowsGraphicsCaptureSupported() bool {
	// Проверяем наличие DLL
	lib := windows.NewLazySystemDLL("windows.graphics.capture.dll")
	if err := lib.Load(); err != nil {
		return false
	}

	// Проверяем наличие функций
	if proc := lib.NewProc("CreateCaptureItemForWindow"); proc.Find() != nil {
		return false
	}

	// Проверяем версию Windows
	major, minor, build := getWindowsVersion()
	return major > 10 || (major == 10 && minor >= 0 && build >= 17763)
}

func getWindowsVersion() (major, minor, build uint32) {
	// Пробуем через kernel32.GetVersion
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if proc := kernel32.NewProc("GetVersion"); proc.Find() == nil {
		v, _, _ := proc.Call()
		major = uint32(v & 0xFF)
		minor = uint32((v >> 8) & 0xFF)
		build = uint32((v >> 16) & 0xFFFF)
		return
	}

	// Fallback для старых версий Windows
	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetNtVersionNumbers")
	proc.Call(
		uintptr(unsafe.Pointer(&major)),
		uintptr(unsafe.Pointer(&minor)),
		uintptr(unsafe.Pointer(&build)),
	)
	build &= 0xFFFF
	return
}
