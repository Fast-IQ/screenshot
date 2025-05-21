package screenshot

import (
	"fmt"
	"github.com/Fast-IQ/screenshot/win_cap/gdi"
	"testing"

	"github.com/lxn/win"
)

var testCapturer *gdi.GDICapturer

func init() {
	testCapturer = &gdi.GDICapturer{}
}

func Test_GetDesktopWindow(t *testing.T) {
	hwnd := gdi.GetDesktopWindow()
	if hwnd == 0 {
		t.Error("GetDesktopWindow returned NULL")
	}
}

func Test_GetAllDisplayBounds(t *testing.T) {
	boundsList, err := testCapturer.GetAllDisplayBounds()
	if err != nil {
		t.Fatalf("GetAllDisplayBounds failed: %v", err)
	}
	if len(boundsList) < 1 {
		t.Error("Expected at least one display monitor")
	}
	for i, b := range boundsList {
		fmt.Printf("Monitor #%d: %v\n", i, b)
		if b.Dx() <= 0 || b.Dy() <= 0 {
			t.Errorf("Monitor #%d has invalid size: %dx%d", i, b.Dx(), b.Dy())
		}
	}
}

func Test_GetDisplayBounds_InvalidIndex(t *testing.T) {
	_, err := testCapturer.GetDisplayBounds(-1)
	if err == nil {
		t.Error("Expected error for negative index")
	}

	count := NumActiveDisplays()
	if count == 0 {
		t.Skip("No monitors found; skipping test")
	}

	_, err = testCapturer.GetDisplayBounds(count + 1)
	if err == nil {
		t.Errorf("Expected error for out-of-bounds index (index %d)", count+1)
	}
}

func Test_GetDisplayBounds_ValidIndices(t *testing.T) {
	count := NumActiveDisplays()
	if count == 0 {
		t.Skip("No monitors found; skipping test")
	}

	for i := 0; i < count; i++ {
		bounds, err := testCapturer.GetDisplayBounds(i)
		if err != nil {
			t.Errorf("GetDisplayBounds(%d) failed: %v", i, err)
		}
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			t.Errorf("Monitor #%d has invalid dimensions: %v", i, bounds)
		}
	}
}

func Test_Capture_Simple(t *testing.T) {
	count := NumActiveDisplays()
	if count == 0 {
		t.Skip("No displays found")
	}

	bounds, _ := testCapturer.GetDisplayBounds(0)
	img, err := testCapturer.Capture(
		bounds.Min.X,
		bounds.Min.Y,
		bounds.Dx(),
		bounds.Dy(),
	)

	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if img == nil {
		t.Fatal("Captured image is nil")
	}

	if img.Bounds().Dx() != bounds.Dx() || img.Bounds().Dy() != bounds.Dy() {
		t.Errorf("Image size mismatch: expected %dx%d, got %dx%d",
			bounds.Dx(), bounds.Dy(),
			img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func Test_NumActiveDisplays(t *testing.T) {
	count := NumActiveDisplays()
	if count <= 0 {
		t.Errorf("Expected at least one monitor, got %d", count)
	}
}

func Test_getDPI(t *testing.T) {
	hwnd := gdi.GetDesktopWindow()
	if hwnd == 0 {
		t.Skip("Cannot get desktop window handle")
	}
	hDC := win.GetDC(hwnd)
	if hDC == 0 {
		t.Skip("Cannot get device context")
	}
	defer win.ReleaseDC(hwnd, hDC)

	dpi := gdi.GetDPI(hDC)
	if dpi <= 0 {
		t.Errorf("Invalid DPI value: %d", dpi)
	} else {
		fmt.Printf("Detected DPI: %d\n", dpi)
	}
}

func Test_scaleForDPI(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		dpi      int
		expected int
	}{
		{"1920@96", 1920, 96, 1920},
		{"1920@144", 1920, 144, 2880},
		{"100@120", 100, 120, 125},
		{"0@any", 0, 144, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gdi.ScaleForDPI(tt.value, tt.dpi)
			if result != tt.expected {
				t.Errorf("scaleForDPI(%d, %d) = %d; want %d", tt.value, tt.dpi, result, tt.expected)
			}
		})
	}
}
