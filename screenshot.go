// Package screenshot captures screenshot image as image.RGBA.
// Mac, Windows, Linux, FreeBSD, OpenBSD and NetBSD are supported.
package screenshot

import (
	"errors"
	"github.com/Fast-IQ/screenshot/win_cap"
	"github.com/lxn/win"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

// ErrUnsupported is returned when the platform or architecture used to compile the program
// does not support screenshot, e.g. if you're compiling without CGO on Darwin
var ErrUnsupported = errors.New("screenshot does not support your platform")

type ScreenCapturer interface {
	//	NumActiveDisplays() int
	Capture(x, y, width, height int) (*image.RGBA, error)
	GetDisplayBounds(displayIndex int) (image.Rectangle, error)
	GetAllDisplayBounds() ([]image.Rectangle, error)
}

// CaptureDisplay captures whole region of displayIndex'th display, starts at 0 for primary display.
func CaptureDisplay(displayIndex int) (*image.RGBA, error) {
	rect, err := GetDisplayBounds(displayIndex)
	if err != nil {
		return nil, err
	}
	img, err := CaptureRect(rect)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// CaptureRect captures specified region of desktop.
func CaptureRect(rect image.Rectangle) (*image.RGBA, error) {
	return Capture(rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
}

// NumActiveDisplays возвращает количество мониторов
func NumActiveDisplays() int {
	count := 0
	pinner := new(runtime.Pinner)
	pinner.Pin(&count)
	defer pinner.Unpin()

	callback := syscall.NewCallback(func(hMonitor win.HMONITOR, hdc win.HDC, lprc *win.RECT, dwData uintptr) uintptr {
		// Увеличиваем счётчик при каждом вызове callback'а
		if dwData != 0 {
			countPtr := (*int)(unsafe.Pointer(dwData))
			*countPtr++
		}
		return 1 // продолжить перечисление
	})

	// Передаём указатель на count через dwData
	win_cap.EnumDisplayMonitors(win.HDC(0), nil, callback, uintptr(unsafe.Pointer(&count)))

	return count
}

/*func createImage(rect image.Rectangle) (img *image.RGBA, e error) {
	img = nil
	e = errors.New("Cannot create image.RGBA ")

	defer func() {
		err := recover()
		if err == nil {
			e = nil
		}
	}()
	// image.NewRGBA may panic if rect is too large.
	img = image.NewRGBA(rect)

	return img, e
}*/
