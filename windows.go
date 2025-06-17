//go:build windows && amd64
// +build windows,amd64

package screenshot

import (
	"github.com/Fast-IQ/screenshot/win_cap/gdi"
	"image"
)

var (
	currentCapturer ScreenCapturer
)

func init() {
	/*	var err error
		currentCapturer, err = wgc.NewWGCCapturer()
		if err != nil {
			currentCapturer = &gdi.GDICapturer{}
			slog.Info("[+] Using GDI-based screen capture")
			return
		} else {
			slog.Info("[+] Using Windows Graphics Capture API")
		}*/

	/*	if win_cap.IsWindowsGraphicsCaptureSupported() {
			currentCapturer, err = wgc.NewWGCCapturer()
			slog.Debug("[+] Using Windows Graphics Capture API")
		} else {
			currentCapturer = &gdi.GDICapturer{}
			slog.Debug("[+] Using GDI-based screen capture")
		}
	*/
	currentCapturer = &gdi.GDICapturer{}
	//currentCapturer, _ = wgc.NewWGCCapturer()

}

// Capture делает скриншот области экрана
func Capture(x, y, width, height int) (*image.RGBA, error) {
	return currentCapturer.Capture(x, y, width, height)
}

// GetDisplayBounds возвращает область отдельного монитора
func GetDisplayBounds(displayIndex int) (image.Rectangle, error) {
	return currentCapturer.GetDisplayBounds(displayIndex)
}

// GetAllDisplayBounds возвращает список всех мониторов
func GetAllDisplayBounds() ([]image.Rectangle, error) {
	return currentCapturer.GetAllDisplayBounds()
}
