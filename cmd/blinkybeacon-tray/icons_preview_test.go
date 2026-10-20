package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// iconPreviewDirEnv names a directory to write icon previews into. Unset, the
// preview test skips: previews are for judging the icons by eye, not a check.
const iconPreviewDirEnv = "BB_ICON_PREVIEW_DIR"

// scaleUp enlarges an image k times, nearest-neighbour, so each pixel shows.
func scaleUp(src *image.RGBA, k int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := 0; y < b.Dy()*k; y++ {
		for x := 0; x < b.Dx()*k; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x/k, y/k))
		}
	}
	return dst
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("writing preview: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encoding %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing %s: %v", path, err)
	}
}

// TestIconPreviews_writtenWhenAsked writes every tray icon, unbound and with
// each candidate mark, for someone to look at:
//
//   - <name>-32.png and <name>-16.png at real size. The 16 is each 2x2 block
//     averaged, the closest this can show to a 100%-scaled Windows tray.
//   - <name>-32-x8.png and <name>-16-x8.png, both enlarged 8x so each pixel shows.
//   - sheet-light.png and sheet-dark.png: every icon on a light and on a dark
//     bar. Rows: unbound, sparkle, brain. Columns: disconnected, idle, spin,
//     flash. Each cell is the 32 at 8x beside the 16 at 8x, with both at real
//     size underneath.
func TestIconPreviews_writtenWhenAsked(t *testing.T) {
	dir := os.Getenv(iconPreviewDirEnv)
	if dir == "" {
		t.Skipf("set %s to a directory to write icon previews", iconPreviewDirEnv)
	}

	states := []struct {
		name string
		fill color.RGBA
	}{
		{"disconnected", colourDisconnected},
		{"idle", colourIdle},
		{"spin", colourSpin},
		{"flash", colourFlash},
	}
	rows := []struct {
		name string
		icon func(fill color.RGBA) []byte
	}{
		{"unbound", sirenIcon},
		{"sparkle", func(fill color.RGBA) []byte { return boundSirenIcon(fill, markSparkle) }},
		{"brain", func(fill color.RGBA) []byte { return boundSirenIcon(fill, markBrain) }},
	}

	const big, small, gap = 32 * 8, 16 * 8, 16
	cellW, cellH := big+gap+small+2*gap, big+gap+32+2*gap
	sheets := map[string]*image.RGBA{}
	for name, bg := range map[string]color.RGBA{
		"light": {R: 243, G: 243, B: 243, A: 255},
		"dark":  {R: 32, G: 32, B: 32, A: 255},
	} {
		s := image.NewRGBA(image.Rect(0, 0, len(states)*cellW, len(rows)*cellH))
		draw.Draw(s, s.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		sheets[name] = s
	}

	for ri, r := range rows {
		for si, st := range states {
			// The grey siren has no bound version: show it under each mark
			// anyway, so the sheet says so, but write its files only once.
			full := iconImage(t, r.icon(st.fill))
			if st.name == "disconnected" && r.name != "unbound" {
				full = iconImage(t, sirenIcon(st.fill))
			}
			half := halfSize(full)

			if st.name != "disconnected" || r.name == "unbound" {
				base := filepath.Join(dir, r.name+"-"+st.name)
				writePNG(t, base+"-32.png", full)
				writePNG(t, base+"-16.png", half)
				writePNG(t, base+"-32-x8.png", scaleUp(full, 8))
				writePNG(t, base+"-16-x8.png", scaleUp(half, 8))
			}

			ox, oy := si*cellW+gap, ri*cellH+gap
			for _, s := range sheets {
				for _, p := range []struct {
					img  *image.RGBA
					x, y int
				}{
					{scaleUp(full, 8), ox, oy},
					{scaleUp(half, 8), ox + big + gap, oy},
					{full, ox, oy + big + gap},
					{half, ox + big + gap, oy + big + gap},
				} {
					draw.Draw(s, p.img.Bounds().Add(image.Pt(p.x, p.y)), p.img, image.Point{}, draw.Over)
				}
			}
		}
	}
	for name, s := range sheets {
		writePNG(t, filepath.Join(dir, "sheet-"+name+".png"), s)
	}
}
