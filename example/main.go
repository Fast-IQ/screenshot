package main

import (
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"runtime"

	"github.com/Fast-IQ/screenshot"
)

// save *image.RGBA to filePath with PNG format.
func save(img *image.RGBA, filePath string) {
	// Проверка на nil
	if img == nil {
		panic("nil image provided")
	}

	// Проверка корректности размеров
	if img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		panic("invalid image dimensions")
	}

	// Проверка соответствия размера данных
	expectedLength := img.Rect.Dx() * img.Rect.Dy() * 4
	if len(img.Pix) < expectedLength {
		panic("image data length doesn't match dimensions")
	}

	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %v\n", err)
		}
	}()

	enc := png.Encoder{
		CompressionLevel: png.BestCompression,
	}

	/*	// Сохраняем копию изображения на случай, если оригинал станет недействительным
		imgCopy := &image.RGBA{
			Pix:    make([]byte, len(img.Pix)),
			Stride: img.Stride,
			Rect:   img.Rect,
		}
		copy(imgCopy.Pix, img.Pix)*/

	err = enc.Encode(file, img)
	if err != nil {
		panic(err)
	}
}

func main() {
	// Убедимся, что поток привязан к OS thread для Windows API
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	slog.SetLogLoggerLevel(slog.LevelDebug)

	// Capture each displays.
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		panic("Active display not found")
	}

	var all image.Rectangle = image.Rect(0, 0, 0, 0)

	// Создаем папку example, если ее нет
	if err := os.Mkdir("example", 0755); err != nil && !os.IsExist(err) {
		panic(err)
	}

	for i := 0; i < n; i++ {
		bounds, err := screenshot.GetDisplayBounds(i)
		if err != nil {
			panic(fmt.Sprintf("Bounds uncorrected for display %d: %v", i, err))
		}
		all = bounds.Union(all)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			panic(fmt.Sprintf("Capture failed for display %d: %v", i, err))
		}

		fileName := fmt.Sprintf("example/%d_%dx%d.png", i, bounds.Dx(), bounds.Dy())
		save(img, fileName)

		fmt.Printf("#%d : %v \"%s\"\n", i, bounds, fileName)
	}
	// Capture all desktop region into an image.
	if all.Empty() {
		panic("no valid display area found")
	}

	fmt.Printf("Capturing full area: %v\n", all)
	img, err := screenshot.Capture(all.Min.X, all.Min.Y, all.Dx(), all.Dy())
	if err != nil {
		panic(fmt.Sprintf("Full capture failed: %v", err))
	}
	save(img, "example/all.png")

}
