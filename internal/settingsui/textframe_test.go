package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-aj · **没有两段字压在同一块像素上，也没有一段字跑出窗口** —— 设置页。
//
// The settings surface is a two-column frame inside a scrolling body: a 148u
// sidebar, a hairline, and a body whose header scrolls with the content. Every
// page in it is a stack of sections, rows and trailing controls, so a row that
// overruns its column is the way this surface goes wrong — and it goes wrong
// silently, because `Tester.Texts` reports runs that are clipped as happily as
// runs that are painted.
//
// The rule is the launcher's (`internal/launcher/textframe_test.go`), read the
// same way: two boxes that overlap by more than one DIP in both axes are two
// words on top of each other, unless one box contains the other (a button's
// label sits inside its button). A box that leaves the window was never
// visible.
func auditSettingsFrame(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	img := tt.Image()
	if img == nil {
		t.Fatalf("%s: no frame", tag)
	}
	w, h := float32(img.Rect.Dx()), float32(img.Rect.Dy())
	seen := map[string]bool{}
	type box struct {
		s string
		r ui.Rect
	}
	var boxes []box
	for _, s := range tt.Texts() {
		if seen[s] {
			continue
		}
		seen[s] = true
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 || r.H <= 0 {
			continue
		}
		if r.W >= w-1 {
			continue
		}
		boxes = append(boxes, box{s, r})
	}
	for _, b := range boxes {
		if b.r.X < -0.5 || b.r.Y < -0.5 || b.r.X+b.r.W > w+0.5 || b.r.Y+b.r.H > h+0.5 {
			t.Errorf("%s: %q leaves the window: %+v (window %.0fx%.0f)", tag, b.s, b.r, w, h)
		}
	}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if enclosesBox(a.r, b.r) || enclosesBox(b.r, a.r) {
				continue
			}
			ox := minBox(a.r.X+a.r.W, b.r.X+b.r.W) - maxBox(a.r.X, b.r.X)
			oy := minBox(a.r.Y+a.r.H, b.r.Y+b.r.H) - maxBox(a.r.Y, b.r.Y)
			if ox > 1 && oy > 1 {
				t.Errorf("%s: %q %+v is painted over %q %+v (overlap %.1fx%.1f)", tag, a.s, a.r, b.s, b.r, ox, oy)
			}
		}
	}
}

func enclosesBox(outer, inner ui.Rect) bool {
	return outer.X <= inner.X+0.5 && outer.Y <= inner.Y+0.5 &&
		outer.X+outer.W >= inner.X+inner.W-0.5 && outer.Y+outer.H >= inner.Y+inner.H-0.5
}

func minBox(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxBox(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// All five pages, in both appearances, at the shipped size and at the two
// window sizes the reference's surface has to survive: the default card and
// the smallest one the settings window opens at.
func TestTheSettingsPagesPaintNoTextOverText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range [][2]int{{860, 900}, {720, 560}} {
			for page := 0; page < pageCount; page++ {
				store := settings.NewStore(settings.Default())
				if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
					t.Fatal(err)
				}
				a := New(store, Actions{})
				a.Integrations = func() []Integration {
					return []Integration{{
						ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true, Running: true,
						RuntimeAvailable: true, Version: "1.0.0", Publisher: "vst",
						Description: "A tool that does one thing.",
					}}
				}
				a.selectPage(page)
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					a.View(c)
				}, size[0], size[1])
				tt.SetDark(dark)
				tt.Frame()
				tt.Frame()
				auditSettingsFrame(t, fmt.Sprintf("%s %s %dx%d", appearance, PageName(page), size[0], size[1]), tt)
			}
		}
	}
}
