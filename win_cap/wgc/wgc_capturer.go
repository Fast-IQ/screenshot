//go:build windows && amd64

package wgc

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"image"
	"syscall"
	"unsafe"
)

const (
	S_OK = uintptr(0)
)

var (
	// Windows.Graphics.Capture.dll
	libGraphicsCapture = windows.NewLazySystemDLL("win_cap.graphics.capture.dll")
	libGraphicsDirectX = windows.NewLazySystemDLL("win_cap.graphics.directx.dxd9helper.dll")

	// Методы из WinRT
	procCreateCaptureItemFromWindow = libGraphicsCapture.NewProc("CreateCaptureItemFromWindow")
	procCreateDirect3DDevice        = libGraphicsDirectX.NewProc("CreateDirect3DDevice")
	procCreateFramePool             = libGraphicsCapture.NewProc("CreateFramePool")
	procStartCapture                = libGraphicsCapture.NewProc("StartCapture")
	procTryGetNextFrame             = libGraphicsCapture.NewProc("TryGetNextFrame")
)

type (
	IInspectable = uintptr
	IUnknown     = uintptr
)

// WGCCapturer — реализация захвата через Windows Graphics Capture API
type WGCCapturer struct {
	device    uintptr // IDirect3DDevice
	framePool uintptr // IDirect3D11CaptureFramePool
	session   uintptr // IDirect3D11CaptureSession
	hwnd      windows.HWND
}

func NewWGCCapturer() (*WGCCapturer, error) {
	if !IsWindowsGraphicsCaptureSupported() {
		return nil, errors.New("WGC not supported on this OS version")
	}

	capturer := &WGCCapturer{
		hwnd: winGetDesktopWindow(),
	}

	// Создаем IGraphicsCaptureItem
	item, err := CreateCaptureItemFromWindow(capturer.hwnd)
	if err != nil {
		return nil, fmt.Errorf("failed to create capture item: %w", err)
	}

	// Создаем Direct3D устройство
	capturer.device, err = CreateDirect3DDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to create Direct3D device: %w", err)
	}

	// Создаем FramePool
	width, height := getScreenSize(capturer.hwnd)
	capturer.framePool, err = CreateFramePool(capturer.device, width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to create frame pool: %w", err)
	}

	// Создаем сессию захвата
	capturer.session, err = CreateCaptureSession(item, capturer.framePool)
	if err != nil {
		return nil, fmt.Errorf("failed to create capture session: %w", err)
	}

	err = StartCapture(capturer.session)

	return capturer, err
}

// Capture делает скриншот с помощью WGC и возвращает *image.RGBA
func (c *WGCCapturer) Capture(x, y, width, height int) (*image.RGBA, error) {
	frame, err := TryGetNextFrame(c.framePool)
	if err != nil {
		return nil, err
	}

	return ConvertFrameToImage(frame, width, height)
}

// GetDisplayBounds — пока возвращаем фиксированный размер
func (c *WGCCapturer) GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	w, h := getScreenSize(c.hwnd)
	return image.Rect(0, 0, w, h), nil
}

// GetAllDisplayBounds — заглушка поддержки нескольких мониторов
func (c *WGCCapturer) GetAllDisplayBounds() ([]image.Rectangle, error) {
	return []image.Rectangle{image.Rect(0, 0, 1920, 1080)}, nil
}

// --- Вспомогательные функции ---

// IsWindowsGraphicsCaptureSupported проверяет, доступен ли WGC
func IsWindowsGraphicsCaptureSupported() bool {
	major, minor := GetWindowsVersion()
	return major >= 10 && minor >= 0
}

// GetWindowsVersion возвращает версию ОС
func GetWindowsVersion() (major, minor uint32) {
	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetNtVersionNumbers")
	r1, r2, _ := proc.Call()
	major = uint32(r1)
	minor = uint32(r2)
	return
}

// winGetDesktopWindow получает HWND рабочего стола
func winGetDesktopWindow() windows.HWND {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("GetDesktopWindow")
	ret, _, _ := proc.Call()
	return windows.HWND(ret)
}

// getScreenSize получает размер экрана
func getScreenSize(hwnd windows.HWND) (width, height int) {
	var rc windows.Rect
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("GetClientRect")
	_, _, _ = proc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	return int(rc.Right - rc.Left), int(rc.Bottom - rc.Top)
}

// CreateCaptureItemFromWindow создает IGraphicsCaptureItem из HWND
func CreateCaptureItemFromWindow(hwnd windows.HWND) (uintptr, error) {
	var item uintptr
	iid := windows.GUID{} // IID_IInspectable
	hr, _, _ := procCreateCaptureItemFromWindow.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&iid)),
		uintptr(unsafe.Pointer(&item)),
	)

	if hr != S_OK {
		return 0, fmt.Errorf("failed to create capture item: 0x%x", hr)
	}
	return item, nil
}

// CreateDirect3DDevice создает IDirect3DDevice
func CreateDirect3DDevice() (uintptr, error) {
	var device uintptr
	hr, _, _ := procCreateDirect3DDevice.Call(0, uintptr(unsafe.Pointer(&device)))
	if hr != S_OK {
		return 0, fmt.Errorf("failed to create D3D device: 0x%x", hr)
	}
	return device, nil
}

// CreateFramePool создает FramePool для захвата кадров
func CreateFramePool(device uintptr, width, height int) (uintptr, error) {
	var framePool uintptr
	hr, _, _ := procCreateFramePool.Call(
		device,
		uintptr(width),
		uintptr(height),
		0, // pixelFormat
		uintptr(unsafe.Pointer(&framePool)),
	)

	if hr != S_OK {
		return 0, fmt.Errorf("failed to create frame pool: 0x%x", hr)
	}
	return framePool, nil
}

// CreateCaptureSession запускает сессию захвата
func CreateCaptureSession(item uintptr, framePool uintptr) (uintptr, error) {
	var session uintptr
	hr, _, _ := procStartCapture.Call(
		item,
		framePool,
		uintptr(unsafe.Pointer(&session)),
	)

	if hr != S_OK {
		return 0, fmt.Errorf("failed to start capture session: 0x%x", hr)
	}
	return session, nil
}

type IGraphicsCaptureSessionVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	StartCapture   uintptr
}

// StartCapture запускает сессию
func StartCapture(session uintptr) error {
	if session == 0 {
		return errors.New("invalid session pointer")
	}

	// Получаем vtable из указателя COM-объекта
	vtbl := (*IGraphicsCaptureSessionVtbl)(unsafe.Pointer(session))

	// Вызываем StartCapture через vtable
	// StartCapture: HRESULT StartCapture()
	//ret, _, _ := syscall.Syscall(vtbl.StartCapture, 1, session)
	ret, _, _ := syscall.SyscallN(vtbl.StartCapture, session)

	if ret != S_OK {
		return fmt.Errorf("failed to start capture session: 0x%x", ret)
	}
	return nil
}

type IDirect3D11CaptureFramePoolVtbl struct {
	QueryInterface  uintptr
	AddRef          uintptr
	Release         uintptr
	TryGetNextFrame uintptr
}

// TryGetNextFrame получает следующий кадр
func TryGetNextFrame(framePool uintptr) (uintptr, error) {
	if framePool == 0 {
		return 0, errors.New("invalid framePool pointer")
	}

	vtbl := (*IDirect3D11CaptureFramePoolVtbl)(unsafe.Pointer(framePool))
	if vtbl.TryGetNextFrame == 0 {
		return 0, errors.New("TryGetNextFrame method is NULL in vtable")
	}

	var frame uintptr

	// Вызываем метод через vtable
	ret, _, _ := syscall.SyscallN(vtbl.TryGetNextFrame,
		framePool,
		uintptr(unsafe.Pointer(&frame)),
		0)
	/*	ret, _, _ := syscall.Syscall(vtbl.TryGetNextFrame, 2,
		framePool,
		uintptr(unsafe.Pointer(&frame)),
		0)
	*/
	if ret != 0 {
		return 0, fmt.Errorf("TryGetNextFrame failed with HRESULT: 0x%x", ret)
	}

	if frame == 0 {
		return 0, errors.New("received null frame from TryGetNextFrame")
	}

	return frame, nil
}

// ConvertFrameToImage конвертирует кадр в *image.RGBA
func ConvertFrameToImage(frame uintptr, width, height int) (*image.RGBA, error) {
	surface := getSurfaceFromFrame(frame)
	if surface == 0 {
		return nil, errors.New("surface is nil")
	}

	// Получаем данные текстуры (упрощённая реализация)
	data, w, h := getTextureData(surface)
	if data == nil {
		return nil, errors.New("texture data is nil")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, data)
	return img, nil
}

// Заглушки — их нужно реализовать через DXGI/Direct3D
func getSurfaceFromFrame(frame uintptr) uintptr {
	// Это упрощённая реализация — в реальности это методы WinRT
	return 0xdeadbeef // пример
}

func getTextureData(surface uintptr) ([]byte, int, int) {
	// Пример данных с текстуры
	return make([]byte, 1920*1080*4), 1920, 1080
}
