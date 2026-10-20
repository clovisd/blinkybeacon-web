package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

// sirenIconV080 is the siren exactly as dashboard-v0.8.0 drew it, kept here so
// the unbound icons can be held to it byte for byte. Do not "tidy" it: it is a
// record, not code to share.
func sirenIconV080(fill color.RGBA) []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx := float64(size) / 2
	domeR := 13.0
	domeCY := 16.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			dist := math.Hypot(fx-cx, fy-domeCY)
			if dist <= domeR && fy <= domeCY {
				img.SetRGBA(x, y, fill)
				continue
			}
			if fy >= 18 && fy <= 22 && fx >= 6 && fx <= 26 {
				img.SetRGBA(x, y, fill)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return trayIconBytes(buf.Bytes())
}

func TestUnboundIcons_areByteIdenticalToV080(t *testing.T) {
	// An unbound tray must look exactly as it did before the bound mark
	// existed: same shape, same four colours, same bytes.
	for _, c := range []struct {
		name string
		got  []byte
		fill color.RGBA
	}{
		{"disconnected", iconDisconnected, color.RGBA{R: 100, G: 100, B: 100, A: 255}},
		{"idle", iconIdle, color.RGBA{R: 180, G: 210, B: 255, A: 255}},
		{"spin", iconSpin, color.RGBA{R: 255, G: 140, B: 0, A: 255}},
		{"flash", iconFlash, color.RGBA{R: 220, G: 30, B: 30, A: 255}},
	} {
		if !bytes.Equal(c.got, sirenIconV080(c.fill)) {
			t.Errorf("the unbound %s icon differs from v0.8.0's", c.name)
		}
	}
}

// iconImage decodes an icon back to pixels, whatever container this platform's
// tray wraps it in (see trayIconBytes).
func iconImage(t *testing.T, b []byte) *image.RGBA {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("\x89PNG")) {
		b = b[22:] // wrapPNGInICO's header and its one directory entry
	}
	src, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decoding icon: %v", err)
	}
	img := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			img.Set(x, y, src.At(x, y))
		}
	}
	return img
}

// halfSize approximates the 16-pixel icon a 100%-scaled Windows tray shows:
// each 2x2 block averaged (image.RGBA is premultiplied, so a plain mean is
// right). Windows' own scaler may differ in detail.
func halfSize(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
	for y := 0; y < b.Dy()/2; y++ {
		for x := 0; x < b.Dx()/2; x++ {
			var r, g, bl, a int
			for _, p := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				c := src.RGBAAt(2*x+p[0], 2*y+p[1])
				r, g, bl, a = r+int(c.R), g+int(c.G), bl+int(c.B), a+int(c.A)
			}
			dst.SetRGBA(x, y, color.RGBA{uint8((r + 2) / 4), uint8((g + 2) / 4), uint8((bl + 2) / 4), uint8((a + 2) / 4)})
		}
	}
	return dst
}

var allBoundMarks = []boundMark{markSparkle, markBrain}

var boundStates = []struct {
	name string
	fill color.RGBA
}{
	{"idle", colourIdle},
	{"spin", colourSpin},
	{"flash", colourFlash},
}

func TestBoundIcons_keepTheSirenAndAddAMarkBesideIt(t *testing.T) {
	// The base colours stay: every pixel of the unbound siren is still there,
	// in its colour, and the mark only fills space the siren left empty —
	// never touching it, so the two read as separate shapes.
	for _, m := range allBoundMarks {
		for _, s := range boundStates {
			plain := iconImage(t, sirenIcon(s.fill))
			bound := iconImage(t, boundSirenIcon(s.fill, m))

			added := 0
			for y := 0; y < 32; y++ {
				for x := 0; x < 32; x++ {
					p, b := plain.RGBAAt(x, y), bound.RGBAAt(x, y)
					if p.A != 0 {
						if b != p {
							t.Fatalf("mark %d, %s: siren pixel (%d,%d) is %v, want %v", m, s.name, x, y, b, p)
						}
						continue
					}
					if b.A == 0 {
						continue
					}
					added++
					for _, n := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
						nx, ny := x+n[0], y+n[1]
						if nx >= 0 && nx < 32 && ny >= 0 && ny < 32 && plain.RGBAAt(nx, ny).A != 0 {
							t.Fatalf("mark %d, %s: mark pixel (%d,%d) touches the siren at (%d,%d)", m, s.name, x, y, nx, ny)
						}
					}
				}
			}
			if added == 0 {
				t.Errorf("mark %d, %s: the bound icon adds no pixels to the siren", m, s.name)
			}
		}
	}
}

func TestBoundIcons_markSurvivesTheSixteenPixelTray(t *testing.T) {
	// Windows at 100% shows the 32-pixel icon at 16. A mark drawn in hairlines
	// would average away to a faint smudge; require a solid core.
	for _, m := range allBoundMarks {
		for _, s := range boundStates {
			plain := halfSize(iconImage(t, sirenIcon(s.fill)))
			bound := halfSize(iconImage(t, boundSirenIcon(s.fill, m)))
			solid := 0
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if plain.RGBAAt(x, y).A == 0 && bound.RGBAAt(x, y).A >= 192 {
						solid++
					}
				}
			}
			if solid < 6 {
				t.Errorf("mark %d, %s: %d solid mark pixels at 16 px, want at least 6", m, s.name, solid)
			}
		}
	}
}
