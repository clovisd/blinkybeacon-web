package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func makeIconPNG(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

var (
	iconIdle  = makeIconPNG(color.RGBA{R: 120, G: 120, B: 120, A: 255}) // grey
	iconSpin  = makeIconPNG(color.RGBA{R: 255, G: 165, B: 0, A: 255})   // amber
	iconFlash = makeIconPNG(color.RGBA{R: 220, G: 50, B: 50, A: 255})   // red
)
