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

// sirenImage draws a 32x32 siren/beacon shape filled with the given colour.
// The shape is two stacked half-circles (dome on top, base on bottom).
func sirenImage(fill color.RGBA) *image.RGBA {
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
	return img
}

// encodeIcon encodes to PNG, then into whatever container this platform's tray
// wants.
func encodeIcon(img *image.RGBA) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return trayIconBytes(buf.Bytes())
}

// sirenIcon renders the siren filled with the given colour, ready for the tray.
func sirenIcon(fill color.RGBA) []byte {
	return encodeIcon(sirenImage(fill))
}

// boundMark is what the tray icon adds while a dashboard line drives the
// light, so a glance at the tray says "this beacon is on automatic" without
// opening the menu. Two are drawn; boundMarkChoice picks the one that ships.
type boundMark int

const (
	// markSparkle is a violet four-point star, the sign much software now
	// uses for a smart or automatic feature. It is still a star at 16 px.
	markSparkle boundMark = iota
	// markBrain is a pink brain seen from the side. It reads as a brain at
	// 32 px; at 16 px the folds blur and it is a pink blob.
	markBrain
)

// boundMarkChoice is the mark the bound icons carry.
const boundMarkChoice = markSparkle

// markArt is a mark drawn as rows of pixels: a byte found in pal is painted in
// that colour, any other byte is left empty. Both marks sit in the bottom-right
// corner, the one space the siren leaves free, and never share an edge with
// it, so the two read as separate shapes. Both are solid shapes, nowhere
// thinner than two pixels, so the 16-pixel tray keeps a solid core of each.
type markArt struct {
	x0, y0 int
	pal    map[byte]color.RGBA
	rows   []string
}

var markArts = [...]markArt{
	markSparkle: {x0: 22, y0: 22, pal: map[byte]color.RGBA{
		'#': {R: 139, G: 92, B: 246, A: 255},
	}, rows: []string{
		"....##....",
		"....##....",
		"...####...",
		"..######..",
		"##########",
		"##########",
		"..######..",
		"...####...",
		"....##....",
		"....##....",
	}},
	markBrain: {x0: 18, y0: 23, pal: map[byte]color.RGBA{
		'#': {R: 236, G: 72, B: 153, A: 255}, // the brain
		'o': {R: 157, G: 23, B: 77, A: 255},  // its folds
	}, rows: []string{
		"....######....",
		"..###o###o##..",
		".##o##o##o###.",
		"##o###o###o###",
		"###oo##oo##o##",
		".##o##o####o#.",
		"...#####.####.",
		"........##....",
		"........##....",
	}},
}

// boundSirenIcon is the siren with mark m beside it. The siren's own pixels
// are untouched: the mark only fills space the siren leaves empty.
func boundSirenIcon(fill color.RGBA, m boundMark) []byte {
	img := sirenImage(fill)
	art := markArts[m]
	for j, row := range art.rows {
		for i := 0; i < len(row); i++ {
			if c, ok := art.pal[row[i]]; ok {
				img.SetRGBA(art.x0+i, art.y0+j, c)
			}
		}
	}
	return encodeIcon(img)
}

var (
	colourDisconnected = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	colourIdle         = color.RGBA{R: 180, G: 210, B: 255, A: 255}
	colourSpin         = color.RGBA{R: 255, G: 140, B: 0, A: 255}
	colourFlash        = color.RGBA{R: 220, G: 30, B: 30, A: 255}
)

var (
	// iconDisconnected — grey, beacon not found
	iconDisconnected = sirenIcon(colourDisconnected)
	// iconIdle — dim blue-white, connected but not active
	iconIdle = sirenIcon(colourIdle)
	// iconSpin — orange, spinning
	iconSpin = sirenIcon(colourSpin)
	// iconFlash — red, flashing
	iconFlash = sirenIcon(colourFlash)

	// The same three with the bound mark, while a dashboard line drives the
	// light. Disconnected has none: with no beacon there is no light to drive.
	iconIdleBound  = boundSirenIcon(colourIdle, boundMarkChoice)
	iconSpinBound  = boundSirenIcon(colourSpin, boundMarkChoice)
	iconFlashBound = boundSirenIcon(colourFlash, boundMarkChoice)
)
