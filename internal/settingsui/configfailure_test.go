package settingsui

import (
	"errors"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// P13-em：失败那一次，这张卡上有**两处**告警。
//
// `configToolbar` 的状态词此前只有两档面：`dirty` → `ConfigUnsaved` + `Warning`，
// 否则 `ConfigSavedState` + `TextTertiary`。而失败时 `dirty` 仍为真（`finishConfigSave`
// 只在成功时 re-baseline），于是失败与「还没试过保存」是同一个面；同时字段下面那条
// 错误带子（`noticeLine`：`Control` 面 + `Warning` keyline + `alert` 图标 + `Warning`
// 句子）也在喊同一件事——**两处同色同义**，而带子才是能读的那一处（它有原文）。
//
// P13-el 已经把「状态词是状态，不是告警」写进合约（`docs/liquid-glass.md` §6.107）；
// 这一轮把同一条判据反过来用：词只在「它是唯一载体」时才是告警。失败时它不是，
// 所以词退回陈述档——**词一个字不改**（`ConfigUnsaved` 仍然为真，改词要新增 i18n 文案）。

// configFailureSentence is the message the failing write reports: it is what the
// error band prints, so the pin can find the band by the sentence itself.
const configFailureSentence = "the write was refused"

// configFailureFixture is the Integrations page with one open config form whose
// Save reports an error at once, so the failed frame can be read without waiting.
func configFailureFixture(t *testing.T, dark bool) (*App, *ui.Tester, theme.Tokens, i18n.Settings) {
	t.Helper()
	en := i18n.For("en").Settings
	integration := Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Config:       []ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		ConfigValues: map[string]any{"token": "abc"},
	}
	a := oneIntegrationApp(integration, Actions{
		SaveConfiguration: func(id string, values map[string]any, done func(error)) {
			done(errors.New(configFailureSentence))
		},
	})
	a.configOpen = map[string]bool{"dev.floter.tool": true}
	tt, tokens := renderIn(t, a, 900, 1400, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return a, tt, tokens, en
}

func TestTheFailedSaveKeepsOneAlarmOnTheCard(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		a, tt, tokens, en := configFailureFixture(t, appearance.dark)

		// Control group: dirty, never tried. The word is the *only* thing on the
		// card that says so, so it carries the alarm — the warning ink.
		a.draft(a.Integrations()[0]).values["token"] = "edited"
		tt.Frame()
		dirtyBox, ok := tt.Find(en.ConfigUnsaved)
		if !ok {
			t.Fatalf("[%v] the dirty form does not read %q: %q", appearance.name, en.ConfigUnsaved, tt.Texts())
		}
		face := boxFace(tt.Image(), dirtyBox)
		steps := map[string]ui.Color{
			"warning":  over(face, tokens.Theme.Warning),
			"muted":    over(face, tokens.Theme.TextMuted),
			"tertiary": over(face, tokens.TextTertiary),
		}
		if got, d := nearestStep(boxInk(tt.Image(), dirtyBox, face), steps); got != "warning" {
			t.Fatalf("[%v] a dirty form's word is painted nearest the %s step (%d levels off), "+
				"want the warning ink: with nothing else on the card saying so, this word is the alarm",
				appearance.name, got, d)
		}

		// Press Save; the write is refused at once.
		if err := tt.Click(en.ConfigSave); err != nil {
			t.Fatalf("[%v] press %q: %v", appearance.name, en.ConfigSave, err)
		}
		tt.Frame()
		if a.configError["dev.floter.tool"] == "" {
			t.Fatalf("[%v] the refused write left no error on the form", appearance.name)
		}

		// The word keeps its words and its box: the failed frame is the dirty
		// frame plus one band *below* the fields. A pin that let the word move
		// would hide a layout shift behind a colour change.
		failedBox, ok := tt.Find(en.ConfigUnsaved)
		if !ok {
			t.Fatalf("[%v] the failed form no longer reads %q: %q — the word must not change, "+
				"the band is what carries the failure", appearance.name, en.ConfigUnsaved, tt.Texts())
		}
		if failedBox != dirtyBox {
			t.Errorf("[%v] the status word moved from %v to %v when the band appeared: "+
				"the band sits below the fields, so nothing above it may move",
				appearance.name, dirtyBox, failedBox)
		}

		// One alarm per fact: the band has it, so the word steps back to the
		// statement ink — the same rung "Saved" and "Saving…" sit on.
		if got, d := nearestStep(boxInk(tt.Image(), failedBox, face), steps); got != "tertiary" {
			t.Errorf("[%v] the failed save's word is painted nearest the %s step (%d levels off), "+
				"want the tertiary step %v: the error band below the fields is already the alarm, "+
				"and this word repeating it in the same ink makes the card shout twice",
				appearance.name, got, d, steps["tertiary"])
		}

		// And the alarm is still there, in the band, in the warning ink: this pin
		// is about *where* the alarm lives, not about muting it.
		bandBox, ok := tt.Find(configFailureSentence)
		if !ok {
			t.Fatalf("[%v] the error band is gone: %q", appearance.name, tt.Texts())
		}
		bandFace := boxFace(tt.Image(), bandBox)
		bandSteps := map[string]ui.Color{
			"warning":  over(bandFace, tokens.Theme.Warning),
			"tertiary": over(bandFace, tokens.TextTertiary),
			"strong":   over(bandFace, tokens.TextStrong),
		}
		if got, d := nearestStep(boxInk(tt.Image(), bandBox, bandFace), bandSteps); got != "warning" {
			t.Errorf("[%v] the error band is painted nearest the %s step (%d levels off), "+
				"want the warning ink %v: the band is the one place the failure is allowed to shout",
				appearance.name, got, d, bandSteps["warning"])
		}
	}
}

// TestDumpP13EMConfigFailure is the opt-in picture: the toolbar's status word
// and the error band under the fields, in the dirty frame and the failed one.
// `FLOTER_P13EM_TAG=before|after` names the run.
func TestDumpP13EMConfigFailure(t *testing.T) {
	tag := os.Getenv("FLOTER_P13EM_TAG")
	if tag == "" {
		t.Skip("opt-in picture: set FLOTER_P13EM_TAG=before|after")
	}
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		a, tt, _, en := configFailureFixture(t, appearance.dark)
		a.draft(a.Integrations()[0]).values["token"] = "edited"
		tt.Frame()
		box, ok := tt.Find(en.ConfigUnsaved)
		if !ok {
			t.Fatalf("[%v] no status word: %q", appearance.name, tt.Texts())
		}
		crop := image.Rect(int(box.X)-10, int(box.Y)-8, int(box.X)+520, int(box.Y)+260)
		writeP13EMCrop(t, tt.Image(), crop, tag, appearance.name, "dirty")

		if err := tt.Click(en.ConfigSave); err != nil {
			t.Fatalf("[%v] press Save: %v", appearance.name, err)
		}
		tt.Frame()
		writeP13EMCrop(t, tt.Image(), crop, tag, appearance.name, "failed")
	}
}

func writeP13EMCrop(t *testing.T, img *image.RGBA, crop image.Rectangle, tag, appearance, state string) {
	t.Helper()
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	for y := crop.Min.Y; y < crop.Max.Y; y++ {
		for x := crop.Min.X; x < crop.Max.X; x++ {
			c := imagePixel(img, x, y)
			i := out.PixOffset(x-crop.Min.X, y-crop.Min.Y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	path := "/tmp/floter-P13-em-failure-" + tag + "-" + appearance + "-" + state + ".png"
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
