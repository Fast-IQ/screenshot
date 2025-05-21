package screenshot

import (
	"testing"
)

func TestCaptureRect(t *testing.T) {
	bounds, err := GetDisplayBounds(0)
	if err != nil {
		t.Error(err)
	}
	_, err = CaptureRect(bounds)
	if err != nil {
		t.Error(err)
	}
}

func BenchmarkCaptureRect(t *testing.B) {
	bounds, err := GetDisplayBounds(0)
	if err != nil {
		t.Error(err)
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_, err = CaptureRect(bounds)
		if err != nil {
			t.Error(err)
		}
	}
}
