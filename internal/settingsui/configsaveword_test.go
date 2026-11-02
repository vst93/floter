package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// P13-el：配置工具栏的状态词是这张卡上唯一回答「我改的东西存住了没有」的地方。
//
// 它此前只有两档面：`dirty` → `ConfigUnsaved` + `Warning`，否则 `ConfigSavedState` +
// `TextTertiary`。而写入在飞的那几秒里 baseline 还没重设（`finishConfigSave` 才
// re-baseline），`configDirty` 仍为真——于是**按下 Save 之后**，这张卡上最响的
// 那一格说的还是「有未保存更改」，而卡片底部的药丸已经在说 "Saving…" 并且熄了灯。
// 同一件事两个控件说两句话，而且响的那一句是**按下之前**的那一句。
//
// 这一轮把在飞那一档补上：词复用 `copy.ConfigSaving`（不加新文案），墨落到与落地后
// 的 "Saved" **同一档**——于是「在飞 → 落地」是纯换词、零换墨，不跳色；而
// 「按下 → 在飞」把警报关掉，因为用户已经照做了。

// boxFace is the face behind a word: the most common pixel in its box. The card
// faces in this build are *translucent* washes (`tokens.Control` has alpha 18 in
// light and 26 in dark), so the only honest way to name the face under a word is
// to read it off the picture rather than to trust a token.
func boxFace(img *image.RGBA, r ui.Rect) ui.Color {
	counts := map[ui.Color]int{}
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := imagePixel(img, x, y)
			counts[c]++
		}
	}
	best, bestN := ui.Color{}, -1
	for c, n := range counts {
		if n > bestN {
			best, bestN = c, n
		}
	}
	return best
}

// boxInk is the word's own colour: the pixel in the box furthest from its face.
// Anti-aliasing never pushes a glyph *past* its ink, so the furthest pixel is the
// ink (this holds on both appearances, unlike "darkest", which only works on the
// light one).
func boxInk(img *image.RGBA, r ui.Rect, face ui.Color) ui.Color {
	best := face
	bestDelta := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := imagePixel(img, x, y)
			if d := channelDelta(c, face); d > bestDelta {
				best, bestDelta = c, d
			}
		}
	}
	return best
}

// nearestStep names which candidate ink a measured pixel is closest to, so a
// failure says what was painted rather than only a number.
func nearestStep(ink ui.Color, steps map[string]ui.Color) (string, int) {
	name, best := "", 1<<30
	for n, c := range steps {
		if d := channelDelta(ink, c); d < best {
			name, best = n, d
		}
	}
	return name, best
}

// configToolbarFixture is the Integrations page with one open config form whose
// Save hangs on a channel, so the write can be held in flight while the picture
// is read.
func configToolbarFixture(t *testing.T, dark bool) (*App, *ui.Tester, theme.Tokens, chan struct{}, i18n.Settings) {
	t.Helper()
	en := i18n.For("en").Settings
	release := make(chan struct{})
	integration := Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Config:       []ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		ConfigValues: map[string]any{"token": "abc"},
	}
	a := oneIntegrationApp(integration, Actions{
		SaveConfiguration: func(id string, values map[string]any, done func(error)) {
			go func() { <-release; done(nil) }()
		},
	})
	a.configOpen = map[string]bool{"dev.floter.tool": true}
	tt, tokens := renderIn(t, a, 900, 1400, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return a, tt, tokens, release, en
}

func TestTheConfigToolbarStopsAccusingWhileTheWriteIsOut(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		a, tt, tokens, release, en := configToolbarFixture(t, appearance.dark)

		// Dirty the draft so the Save is live, and read the toolbar's word in the
		// state the user sees *before* pressing it: "Unsaved changes", in the
		// warning ink — the loudest step on the card. That is right there: the
		// draft really is unsaved and there is a live button to fix it.
		a.draft(a.Integrations()[0]).values["token"] = "edited"
		tt.Frame()
		dirtyBox, ok := tt.Find(en.ConfigUnsaved)
		if !ok {
			t.Fatalf("[%v] the dirty form does not read %q: %q", appearance.name, en.ConfigUnsaved, tt.Texts())
		}
		face := boxFace(tt.Image(), dirtyBox)
		dirtyInk := boxInk(tt.Image(), dirtyBox, face)
		steps := map[string]ui.Color{
			"warning":  over(face, tokens.Theme.Warning),
			"muted":    over(face, tokens.Theme.TextMuted),
			"tertiary": over(face, tokens.TextTertiary),
		}
		if got, d := nearestStep(dirtyInk, steps); got != "warning" {
			t.Fatalf("[%v] a dirty form's word is painted %v — nearest the %s step %v (%d levels off), "+
				"want the warning ink %v: the pre-press state is the loud one", appearance.name,
				dirtyInk, got, steps[got], d, steps["warning"])
		}

		// Press it. The write is out: the status word must stop saying the change
		// is pending, because it is no longer pending — it is being written.
		if err := tt.Click(en.ConfigSave); err != nil {
			t.Fatalf("[%v] press %q: %v", appearance.name, en.ConfigSave, err)
		}
		tt.Frame()
		if !a.configSaveBusy["dev.floter.tool"] {
			t.Fatalf("[%v] the write is not in flight after the press", appearance.name)
		}
		if tt.HasText(en.ConfigUnsaved) {
			t.Errorf("[%v] the toolbar still reads %q while the write is out: the form accuses the user "+
				"of an unsaved change in the same frame its Save says %q",
				appearance.name, en.ConfigUnsaved, en.ConfigSaving)
		}

		// The word keeps its row box — this round changes which word and which
		// ink, not the geometry — so the box read off the dirty frame is the box
		// to read now.
		flightInk := boxInk(tt.Image(), dirtyBox, face)
		if got, d := nearestStep(flightInk, steps); got != "tertiary" {
			t.Errorf("[%v] the in-flight word is painted %v — nearest the %s step %v (%d levels off), "+
				"want the tertiary step %v: while the write is out nothing is wrong, so the loudest ink "+
				"on the card must not be this word", appearance.name, flightInk, got, steps[got], d,
				steps["tertiary"])
		}

		// Let it land. The word becomes "Saved" and the ink must not move at all:
		// landing is a word swap, not a colour jump — if the in-flight ink were a
		// step of its own, every save would flash between two greys.
		close(release)
		deadline := time.Now().Add(2 * time.Second)
		for a.configSaveBusy["dev.floter.tool"] && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
			tt.Frame()
		}
		if a.configSaveBusy["dev.floter.tool"] {
			t.Fatalf("[%v] the Save stayed in flight after the write landed", appearance.name)
		}
		tt.Frame()
		savedBox, ok := tt.Find(en.ConfigSavedState)
		if !ok {
			t.Fatalf("[%v] the landed form does not read %q: %q", appearance.name, en.ConfigSavedState, tt.Texts())
		}
		savedInk := boxInk(tt.Image(), savedBox, boxFace(tt.Image(), savedBox))
		for _, ch := range []struct {
			name     string
			inFlight int
			saved    int
		}{
			{"R", int(flightInk.R), int(savedInk.R)},
			{"G", int(flightInk.G), int(savedInk.G)},
			{"B", int(flightInk.B), int(savedInk.B)},
		} {
			if d := ch.inFlight - ch.saved; d > 3 || d < -3 {
				t.Errorf("[%v] the in-flight word is painted %v and the landed word %v (channel %s differs "+
					"by %d): landing must be a word swap in one ink, not a colour jump",
					appearance.name, flightInk, savedInk, ch.name, d)
			}
		}
	}
}

// TestDumpP13ELConfigStatus is the opt-in picture: the toolbar row, cropped to
// the status word and the two icon-buttons beside it, in all three states.
// `FLOTER_P13EL_TAG=before|after` names the run.
func TestDumpP13ELConfigStatus(t *testing.T) {
	tag := os.Getenv("FLOTER_P13EL_TAG")
	if tag == "" {
		t.Skip("opt-in picture: set FLOTER_P13EL_TAG=before|after")
	}
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		a, tt, _, release, en := configToolbarFixture(t, appearance.dark)
		a.draft(a.Integrations()[0]).values["token"] = "edited"
		tt.Frame()
		box, ok := tt.Find(en.ConfigUnsaved)
		if !ok {
			t.Fatalf("[%v] no status word: %q", appearance.name, tt.Texts())
		}
		crop := image.Rect(int(box.X)-10, int(box.Y)-8, int(box.X)+250, int(box.Y+box.H)+8)
		writeP13ELCrop(t, tt.Image(), crop, tag, appearance.name, "dirty")

		if err := tt.Click(en.ConfigSave); err != nil {
			t.Fatalf("[%v] press Save: %v", appearance.name, err)
		}
		tt.Frame()
		writeP13ELCrop(t, tt.Image(), crop, tag, appearance.name, "inflight")

		close(release)
		deadline := time.Now().Add(2 * time.Second)
		for a.configSaveBusy["dev.floter.tool"] && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
			tt.Frame()
		}
		tt.Frame()
		writeP13ELCrop(t, tt.Image(), crop, tag, appearance.name, "saved")
	}
}

func writeP13ELCrop(t *testing.T, img *image.RGBA, crop image.Rectangle, tag, appearance, state string) {
	t.Helper()
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	for y := crop.Min.Y; y < crop.Max.Y; y++ {
		for x := crop.Min.X; x < crop.Max.X; x++ {
			c := imagePixel(img, x, y)
			i := out.PixOffset(x-crop.Min.X, y-crop.Min.Y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	path := "/tmp/floter-P13-el-configstatus-" + tag + "-" + appearance + "-" + state + ".png"
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	t.Logf("wrote %s (%dx%d)", path, out.Rect.Dx(), out.Rect.Dy())
}
