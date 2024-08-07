package main

import (
	"github.com/fast-iq/screenshot"
	"image/png"
	"log"
	"os"
)

func main() {

	screenshoter := screenshot.New()

	img, err := screenshoter.CaptureScreen()
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Create("./screenshot.png")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	err = png.Encode(file, img)
	if err != nil {
		log.Fatal(err)
	}

}
