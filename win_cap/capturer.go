package win_cap

import (
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"unsafe"
)

var (
	ntdll  = windows.NewLazySystemDLL("ntdll.dll")
	user32 = windows.NewLazySystemDLL("user32.dll")

	funcEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")

	procRtlGetNtVersionNumbers = ntdll.NewProc("RtlGetNtVersionNumbers")
)

func EnumDisplayMonitors(hdc win.HDC, lprcClip *win.RECT, lpfnEnum uintptr, dwData uintptr) bool {
	ret, _, _ := funcEnumDisplayMonitors.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(lprcClip)),
		lpfnEnum,
		dwData,
	)
	return ret != 0
}

func GetWindowsVersion() (major, minor uint32) {
	if procRtlGetNtVersionNumbers.Find() == nil {
		r1, r2, _ := procRtlGetNtVersionNumbers.Call()
		major = uint32(r1)
		minor = uint32(r2)
	}
	return
}

func IsWindowsGraphicsCaptureSupported() bool {
	major, minor := GetWindowsVersion()
	return major >= 10 && minor >= 0
}

// IsWindows10OrGreater возвращает true, если версия Windows >= Windows 10
func IsWindows10OrGreater() bool {
	major, minor := GetWindowsVersion()
	return (major > 10) || (major == 10 && minor >= 0)
}

// IsWindows7OrLesser возвращает true, если версия Windows <= Windows 7
func IsWindows7OrLesser() bool {
	major, minor := GetWindowsVersion()
	return (major < 10) || (major == 6 && minor <= 1)
}
