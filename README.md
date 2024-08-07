# Screenshot
Simple cross-platform pure Go screen shot library. (tested on linux&windows&osx)

## Changes:
* Changed: linux and freebsd source code is one file now
* Changed: linux (and freebsd) code refactored
* Changed: Connection to X in linux is separated from screenshoting (Now no memory leakage while screenshoting in a loop)

## My Changes
* Changed: use library github.com/jezek instead of github.com/BurntSushi

## Basic Usage
Import the package
```go
import (
    "github.com/fast-iq/screenshot"
)
```

```go
func main() {

	screenshoter := screenshot.New()

	img, err := screenshot.CaptureScreen()
	if err != nil {
		log.Fatal(err)
	}
}
```

## Dependencies
* **Windows** - None
* **Linux/FreeBSD** - https://github.com/jezek/xgb
* **OSX** - cgo (CoreGraphics,CoreFoundation, that should not be a problem)

## Examples
Look at `examples/` folder.
