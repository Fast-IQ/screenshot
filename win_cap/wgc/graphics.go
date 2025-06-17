package wgc

import (
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

type IDXGISurfaceVtbl struct {
	IUnknownVtbl // Наследует QueryInterface, AddRef, Release
	GetDesc      uintptr
	Map          uintptr
	Unmap        uintptr
}

type IDXGISurface struct {
	vtbl *IDXGISurfaceVtbl
}

type DXGI_SURFACE_DESC struct {
	Width      uint32
	Height     uint32
	Format     uint32
	SampleDesc struct {
		Count   uint32
		Quality uint32
	}
	BufferUsage uint32
	CPUAccess   uint32
	MiscFlags   uint32
}

type DXGI_MAPPED_RECT struct {
	Pitch int32
	PBits *byte
}

type IGraphicsCaptureItemInteropVtbl struct {
	IUnknownVtbl
	CreateForWindow  uintptr
	CreateForMonitor uintptr
}

// IGraphicsCaptureItemInterop структура с vtbl
type IGraphicsCaptureItemInterop struct {
	vtbl *IGraphicsCaptureItemInteropVtbl
}

const (
	DXGI_MAP_READ        = 1
	DXGI_CPU_ACCESS_READ = 0x1
)

var (
	IID_IDXGISurface = windows.GUID{
		Data1: 0xCAFC1EC3,
		Data2: 0x0482,
		Data3: 0x4A36,
		Data4: [8]byte{0xB3, 0x1C, 0x33, 0x28, 0x93, 0x5F, 0x7F, 0x9C},
	}
	IID_IGraphicsCaptureItemInterop = windows.GUID{
		0x3628E81B, 0x3CAC, 0x4C60, [8]byte{0xB7, 0xF4, 0x23, 0xCE, 0x07, 0x1A, 0x89, 0xE4}}
	CLSID_GraphicsCaptureItem = windows.GUID{
		0x79C3F95B, 0x31F7, 0x4EC2, [8]byte{0xA4, 0x64, 0x63, 0x9E, 0x3F, 0x4F, 0x35, 0xDE}}
	IID_IGraphicsCaptureItem       = windows.GUID{0x79C3F95B, 0x31F7, 0x4EC2, [8]byte{0xA4, 0x64, 0x63, 0x9E, 0x3F, 0x4F, 0x35, 0xDE}}
	IID_IGraphicsCaptureFramePool  = windows.GUID{0x3628E81B, 0x3CAC, 0x4C60, [8]byte{0xB7, 0xF4, 0x23, 0xCE, 0x07, 0x1A, 0x89, 0xE4}}
	IID_IGraphicsCaptureSession    = windows.GUID{0x814E42A9, 0xF70F, 0x4A7C, [8]byte{0xBB, 0x58, 0x55, 0xFA, 0x7E, 0xDE, 0x33, 0x21}}
	CLSID_GraphicsCaptureFramePool = windows.GUID{0xA87D1A94, 0x7117, 0x4E4D, [8]byte{0x9E, 0x95, 0x1B, 0x4F, 0x9F, 0x2D, 0x3D, 0x6F}}
	CLSID_GraphicsCaptureSession   = windows.GUID{0xEF65D4A2, 0x6F6D, 0x4A4C, [8]byte{0x87, 0x8D, 0x7D, 0x8F, 0x1F, 0x0F, 0x93, 0x0D}}
)

// Реализуем методы для IDXGISurface
func (s *IDXGISurface) GetDesc(desc *DXGI_SURFACE_DESC) uint32 {
	ret, _, _ := syscall.SyscallN(
		s.vtbl.GetDesc,
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(desc)),
	)
	return uint32(ret)
}

func (s *IDXGISurface) Map(lockedRect *DXGI_MAPPED_RECT, mapFlags uint32) uint32 {
	ret, _, _ := syscall.SyscallN(
		s.vtbl.Map,
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(lockedRect)),
		uintptr(mapFlags),
	)
	return uint32(ret)
}

func (s *IDXGISurface) Unmap() uint32 {
	ret, _, _ := syscall.SyscallN(
		s.vtbl.Unmap,
		uintptr(unsafe.Pointer(s)),
	)
	return uint32(ret)
}

// Методы для IGraphicsCaptureItemInterop
func (i *IGraphicsCaptureItemInterop) CreateForWindow(hwnd windows.HWND, riid *windows.GUID, item **IGraphicsCaptureItem) uint32 {
	ret, _, _ := syscall.SyscallN(
		i.vtbl.CreateForWindow,
		uintptr(unsafe.Pointer(i)),
		uintptr(hwnd),
		uintptr(unsafe.Pointer(riid)),
		uintptr(unsafe.Pointer(item)),
	)
	return uint32(ret)
}

func (i *IGraphicsCaptureItemInterop) Release() uint32 {
	ret, _, _ := syscall.SyscallN(
		i.vtbl.Release,
		uintptr(unsafe.Pointer(i)),
	)
	return uint32(ret)
}
