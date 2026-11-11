package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settingsui"
)

// No surface wears a ring of the card's own face around its sheet.
//
// The user's report on the settings page was 「四周有和背景不同的颜色内边」:
// a band of the card's translucent face showing between the window's rounded
// edge and the darker `Sunken` body — an inner border no desktop app draws.
// The same band was on the search page, where it read as 「搜索框顶部有一条
// 灰线」 (the 4u above the field row), and the same shape had already been
// reported once on Windows as 「窗口周边外圈还有一圈额外的底色」.
//
// The cause was one decision in this file: the shell padded the column that
// holds the surface (2.5u a side for the settings, 4u/2u above and below for
// the launcher) and painted the card's face behind it. The card *is* the
// window, so that padding can only ever be a ring. Each surface now pays its
// own inset inside its own sheet, and this pin reads the consequence off the
// rendered frame: the first pixel row and column inside the card's edge carry
// the surface's sheet, not the frame's face.
//
// The frame is translucent (the window shows the desktop through it), so the
// reading is taken on a composited copy: a test frame has no desktop behind it.
func TestNoSurfaceWearsARingOfTheCardsFace(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		for _, surface := range []struct {
			name  string
			surf  Surface
			page  int
			width int
		}{
			{"launcher", SurfaceLauncher, 0, 720},
			{"settings/general", SurfaceSettings, settingsui.PageGeneral, 720},
			{"settings/sessions", SurfaceSettings, settingsui.PageSessions, 720},
			{"settings/shortcuts", SurfaceSettings, settingsui.PageShortcuts, 720},
			{"settings/integrations", SurfaceSettings, settingsui.PageIntegrations, 720},
			{"settings/about", SurfaceSettings, settingsui.PageAbout, 720},
		} {
			t.Run(appearance.name+"/"+surface.name, func(t *testing.T) {
				tt := redesignFrame(appearance.name, "en", 47, surface.surf, surface.page, surface.width, 580)
				img := tt.Image()
				// The surface's sheet is the *same* token the launcher's panel
				// paints (`theme.Tokens.Sunken`), so the reference for "this pixel
				// is the sheet, not the frame's face" is the panel's own value read
				// off a surface that has no ring at all. The card's face is a
				// different colour, and the band the report is about is that colour:
				// a 1px stroke and the lit face's ramp are inside a few levels of
				// the sheet, while the ring was tens.
				sheet := sheetFace(t, appearance.name)
				w, h := img.Rect.Dx(), img.Rect.Dy()
				// The reading is taken just inside the card's stroke, on the middle
				// 60% of each edge so the rounded corners stay out of it. The bottom
				// edge is deliberately not read: the settings body's scroll fade and
				// its horizontal scrollbar both live there, and both are bands the
				// design draws on purpose. The ring the report is about was on all
				// four sides, so three sides and the crown are enough to see it.
				const inset = 3
				type edge struct {
					what string
					at   func(i int) ui.Color
					last int
				}
				edges := []edge{
					{"top", func(i int) ui.Color { return pixelOf(img, i, inset) }, w},
					{"left", func(i int) ui.Color { return pixelOf(img, inset, i) }, h},
					{"right", func(i int) ui.Color { return pixelOf(img, w-1-inset, i) }, h},
				}
				for _, e := range edges {
					// The bottom edge is read on its upper 60% too: the settings
					// body's scroll fade lives on the last rows and is not this ring.
					lo, hi := e.last*2/10, e.last*8/10
					for i := lo; i < hi; i += 2 {
						if delta := colourDelta(e.at(i), sheet); delta > 6 {
							t.Errorf("%s: the pixel %d px in from the card's %s edge is %v, the surface's sheet is %v "+
								"(%d levels apart): the surface wears a band of the card's face again",
								surface.name, inset, e.what, e.at(i), sheet, delta)
							break
						}
					}
				}
			})
		}
	}
}

// sheetFace is the colour the surface's own sheet paints: the launcher's panel
// token (`theme.Tokens.Sunken`), which every non-terminal surface shares. It is
// read from the launcher's own frame, well inside its sheet, so the pin
// compares against a pixel rather than against a token the ring's own colour
// could also be derived from.
func sheetFace(t *testing.T, appearance string) ui.Color {
	t.Helper()
	tt := redesignFrame(appearance, "en", 47, SurfaceLauncher, 0, 720, 580)
	img := tt.Image()
	return pixelOf(img, img.Rect.Dx()/2, img.Rect.Dy()/2)
}

// pixelOf reads one pixel as a colour.
func pixelOf(img *image.RGBA, x, y int) ui.Color {
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// colourDelta is the largest per-channel difference of two colours.
func colourDelta(a, b ui.Color) int {
	d := 0
	for _, pair := range [][2]byte{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		diff := int(pair[0]) - int(pair[1])
		if diff < 0 {
			diff = -diff
		}
		if diff > d {
			d = diff
		}
	}
	return d
}
