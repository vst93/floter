package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The About page's read-only facts wear the same ink order an approval record
// does: the name of the fact is a caption, the fact itself is the emphasis.
//
// `keyValue` draws the framework version, the link scheme and the settings
// file's path — three lines nothing on this page can change, and the only place
// in the app that says where the settings file actually lives. They were drawn
// with the plain settings row, whose emphasis step belongs to the **setting's
// name** (`rowTitle`: +1pt at 580, `TextStrong`) and which puts a value in the
// control slot at the quietest ink (`t.TextMuted` 500). Measured light 202 / 101
// and dark 198 / 94 — the same inverted hierarchy P13-ak fixed on the approval
// record (`docs/liquid-glass.md` §6.10), on the one card P13-ak did not cover.
//
// P13-an 重定基线（§6.11）：名字退到 caption / `TextSecondary`、值占 emphasis
// (`+1pt` / 580 / `TextStrong`)；盒子一个像素没动 —— 行距仍是 45px、卡片高度不变、
// 值仍可选（用户要能把路径复制走）。 These pins hold the resolved ink, not the
// requested token, because the contract is a claim about what a viewer reads.

// aboutFixture is the build identity the About page draws in these pins: every
// value is a distinct string, so each row's two boxes can be found by text
// alone (`tt.Find` matches the drawn glyphs).
func aboutFixture() About {
	return About{
		Name: "floter", Version: "1.2.3", Framework: "mygo 0.9",
		Scheme: "floter", SettingsPath: "/home/tar/.config/floter/settings.json",
		RepoURL: "https://github.com/vst93/floter",
	}
}

// aboutFacts are the card's three read-only rows in the order they are drawn.
func aboutFacts() [][2]string {
	en := i18n.For("en").Settings
	return [][2]string{
		{en.AboutFramework, "mygo 0.9"},
		{en.AboutScheme, "floter"},
		{en.AboutSettingsFile, "/home/tar/.config/floter/settings.json"},
	}
}

// aboutPage renders the About page of aboutFixture in one appearance.
func aboutPage(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := New(newStore(t), Actions{})
	a.About = aboutFixture()
	tt, _ := renderIn(t, a, 760, 1400, dark)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// inkRows is the vertical extent of the ink inside r: the first and last scan
// line whose pixels leave the row's own plane. It is what tells a *baseline*
// from a *box*: the caption is 11px and the value 15px, so their boxes can
// never be the same height, but their ink has to sit on one optical line.
func inkRows(tt *ui.Tester, r ui.Rect) (float32, float32) {
	img := tt.Image()
	// The plane is sampled at the far right of the box, where neither string
	// draws a glyph (both boxes are tight around their text).
	px := (int(r.Y)+int(r.H)/2)*img.Stride + (int(r.X+r.W)-1)*4
	plane := luma(img.Pix[px], img.Pix[px+1], img.Pix[px+2])
	top, bottom := float32(-1), float32(-1)
	for y := int(r.Y); y < int(r.Y+r.H) && y < img.Bounds().Dy(); y++ {
		for x := int(r.X); x < int(r.X+r.W) && x < img.Bounds().Dx(); x++ {
			i := y*img.Stride + x*4
			if d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - plane; d > 8 || d < -8 {
				if top < 0 {
					top = float32(y)
				}
				bottom = float32(y)
				break
			}
		}
	}
	return top, bottom
}

// TestTheAboutFactsPutTheValueInTheEmphasisRung pins the pair on every read-only
// row of the About card: the fact out-inks its name by a whole rung, the name
// stays on the quiet rung, the two share one optical line with the value on the
// right, and the rows keep the card's own 45px pitch (this round moved ink, not
// boxes).
func TestTheAboutFactsPutTheValueInTheEmphasisRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := aboutPage(t, dark)
		var last float32
		for i, fact := range aboutFacts() {
			name, value := mustFind(t, tt, fact[0]), mustFind(t, tt, fact[1])
			nameInk, valueInk := inkSpan(tt, name), inkSpan(tt, value)
			if valueInk-nameInk < 60 {
				t.Errorf("%s: %q resolves %d levels and its value %q only %d — the fact must "+
					"out-ink the name of the fact by a whole rung (§6.11)",
					appearance, fact[0], nameInk, fact[1], valueInk)
			}
			if nameInk > 150 {
				t.Errorf("%s: the fact's name %q resolves %d levels — on a fact sheet the name is "+
					"the caption over its value, not a setting's title (§6.11)",
					appearance, fact[0], nameInk)
			}
			nameTop, nameBottom := inkRows(tt, name)
			valueTop, valueBottom := inkRows(tt, value)
			if nameTop < 0 || valueTop < 0 {
				t.Fatalf("%s: %q / %q drew no ink inside their boxes", appearance, fact[0], fact[1])
			}
			// One optical line: the two ink bands overlap, and by most of the
			// shorter one. Comparing *centres* would punish a value that happens to
			// carry a descender (`mygo 0.9` does, `floter` does not), so the pin is
			// the overlap the eye actually reads.
			shorter := min(nameBottom-nameTop, valueBottom-valueTop)
			overlap := min(nameBottom, valueBottom) - max(nameTop, valueTop)
			if overlap < 0.6*shorter {
				t.Errorf("%s: %q's ink spans rows %v-%v and its value %q rows %v-%v — they overlap by "+
					"%.0f of the shorter band, so the pair reads as two lines, not one",
					appearance, fact[0], nameTop, nameBottom, fact[1], valueTop, valueBottom, overlap)
			}
			if value.X <= name.X {
				t.Errorf("%s: the value %q starts at x=%v, not right of its name at x=%v — the card "+
					"reads label-then-value on one line", appearance, fact[1], value.X, name.X)
			}
			if i > 0 {
				if pitch := name.Y - last; pitch < 44 || pitch > 46 {
					t.Errorf("%s: row %d starts %v below the row above it, not the card's own 45px "+
						"pitch — a one-line fact may not grow its row (§6.11)", appearance, i, pitch)
				}
			}
			last = name.Y
		}
	}
}

// TestDumpP13ANAboutFacts writes the frames this round measured, in both
// appearances, before and after the ink order was swapped.
func TestDumpP13ANAboutFacts(t *testing.T) {
	if os.Getenv("FLOTER_P13AN_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AN_DUMP=1 to write /tmp/floter-P13-an-*.png")
	}
	tag := os.Getenv("FLOTER_P13AN_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := aboutPage(t, dark)
		for _, fact := range aboutFacts() {
			name, value := mustFind(t, tt, fact[0]), mustFind(t, tt, fact[1])
			nameTop, nameBottom := inkRows(tt, name)
			valueTop, valueBottom := inkRows(tt, value)
			t.Logf("%s %q ink %d rows %v-%v | %q ink %d rows %v-%v",
				appearance, fact[0], inkSpan(tt, name), nameTop, nameBottom,
				fact[1], inkSpan(tt, value), valueTop, valueBottom)
		}
		writePNG(t, filepath.Join("/tmp", fmt.Sprintf("floter-P13-an-about-%s-%s.png", appearance, tag)), tt)
	}
}
