package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// wrapPNGInICO wraps PNG bytes in a minimal ICO container.
// Windows Vista+ supports PNG-in-ICO natively via LoadImageW.
func wrapPNGInICO(pngData []byte) []byte {
	const dataOffset = 22
	ico := make([]byte, dataOffset+len(pngData))
	binary.LittleEndian.PutUint16(ico[2:], 1)
	binary.LittleEndian.PutUint16(ico[4:], 1)
	binary.LittleEndian.PutUint16(ico[10:], 1)
	binary.LittleEndian.PutUint16(ico[12:], 32)
	binary.LittleEndian.PutUint32(ico[14:], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(ico[18:], uint32(dataOffset))
	copy(ico[dataOffset:], pngData)
	return ico
}

// sirenIcon renders a 32x32 siren/beacon shape filled with the given colour.
// The shape is two stacked half-circles (dome on top, base on bottom).
func sirenIcon(fill color.RGBA) []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx := float64(size) / 2
	// dome: upper half circle, radius 13, centred at y=16
	domeR := 13.0
	domeCY := 16.0
	// base bar: rect rows 18–22, x 6–26
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			// dome
			dist := math.Hypot(fx-cx, fy-domeCY)
			if dist <= domeR && fy <= domeCY {
				img.SetRGBA(x, y, fill)
				continue
			}
			// base bar
			if fy >= 18 && fy <= 22 && fx >= 6 && fx <= 26 {
				img.SetRGBA(x, y, fill)
			}
		}
	}

	// Encode to PNG then wrap in ICO
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return wrapPNGInICO(buf.Bytes())
}

var (
	// iconDisconnected — grey, beacon not found
	iconDisconnected = sirenIcon(color.RGBA{R: 100, G: 100, B: 100, A: 255})
	// iconIdle — dim blue-white, connected but not active
	iconIdle = sirenIcon(color.RGBA{R: 180, G: 210, B: 255, A: 255})
	// iconSpin — bright green, spinning
	iconSpin = sirenIcon(color.RGBA{R: 0, G: 220, B: 60, A: 255})
	// iconFlash — red-orange, flashing
	iconFlash = sirenIcon(color.RGBA{R: 255, G: 80, B: 20, A: 255})
)
