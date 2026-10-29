package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-cc · **列表底部那条带只有一行字。**
//
// The band is a fixed 30u `Absolute` row (`launcher.go:3049-3050`,
// `Height(t.Space(7.5))`) and the window only ever pays for that one band
// (`Geometry.Feedback` is a `bool`). The sentence in it is **user data**: the
// shell hands `Feedback`/`WarnFeedback` the sentences `i18n.Notifications`
// builds for install/uninstall completions, extension names included
// (`internal/shell/notices.go` → `panelFeedback`, `internal/shell/shell.go:851-968`).
// Without a one-line constraint a long name wrapped the sentence to two lines
// (measured at 720 wide, `{X:40 Y:404 W:664 H:44}`) and the second line landed
// on the list's lowest rows — `app-20`'s title was painted over, 54×15u of it,
// while the band itself had not grown by a pixel.
//
// The fix is the same constraint the band's other two sentences already wear
// (`heldHintRow`'s hint, `confirmPowerButton`'s labels): one line. Nothing
// about the height chain, the ink or the clock changes.

// bandSentence finds the run the band drew for one sentence. The headless
// tester reports a text run's own label, not the glyphs a `SingleLine()` cut
// leaves on screen (`ui/headless.go:461` matches the element's text), so the
// pin reads the run's **box** — one line is a box as tall as the band's other
// sentences, whatever the sentence says.
func bandSentence(t *testing.T, tt *ui.Tester, sentence string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(sentence)
	if !ok {
		t.Fatalf("the band drew no run for %q: %q", sentence, tt.Texts())
	}
	return r
}

// TestTheStatusBandKeepsItsSentenceOnOneLine is the band's one-line rule, in
// both shipped languages and both appearances. It measures against the band's
// own shortest sentence rather than a literal height: what the pin owns is
// 「一句话在带子里永远是同一个字盒」, not the body step's exact value.
func TestTheStatusBandKeepsItsSentenceOnOneLine(t *testing.T) {
	// A real extension name out of the reference's own vocabulary, long enough
	// that the sentence cannot fit the band at the launcher's fixed width.
	longName := strings.Repeat("very-long-extension-name-", 4)
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(lang+" "+appearance, func(t *testing.T) {
				a := statusBandApp()
				if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
					t.Fatal(err)
				}
				tt := render(t, a)
				tt.SetDark(dark)
				// A list that reaches past the window: the rows a wrapped
				// sentence was painted over are the ones at its bottom.
				tt.Type("app")
				tt.Frame()
				a.List.ScrollIntoView(len(statusBandNames) - 1)
				tt.Frame()
				tt.SetSize(InputWindowWidth, 488+30) // the window grows by the band's 30u

				// The band's one-line ruler: a sentence that certainly fits.
				short := i18n.NotificationsFor(lang).IntegrationInstalled("v")
				a.Feedback(short)
				tt.Frame()
				shortRect := bandSentence(t, tt, short)

				long := i18n.NotificationsFor(lang).IntegrationDisconnected(longName)
				a.Feedback(long)
				tt.Frame()
				longRect := bandSentence(t, tt, long)
				t.Logf("%s %s: short %+v, long %+v", lang, appearance, shortRect, longRect)

				// One line, and the same line box as the short sentence: the
				// band did not grow, and the sentence did not move up into the
				// list (on the old code this is 44u tall and 41u higher).
				if longRect.H != shortRect.H || longRect.Y != shortRect.Y {
					t.Errorf("the band's sentence box changed with the sentence's length: short %+v, long %+v"+
						" — the 30u band holds one line and cannot grow", shortRect, longRect)
				}
			})
		}
	}
}

// TestThePowerQuestionKeepsTheBandsLine is the same rule on the band's other
// sentence (the shutdown/restart confirmation, `power.go:150`). It is a
// **form** pin: the shipped question fits one line either way (en 56 bytes), so
// it is green before and after the fix — what it keeps is the band's "one
// sentence, one line" shape on the sentence that shares the row with two
// buttons. `bandHeight` is the band's own 30u (`height.go:44`,
// `launcherStatusUnits = 7.5` × 4); a wrapped question is two lines, ~44u.
func TestThePowerQuestionKeepsTheBandsLine(t *testing.T) {
	const bandHeight = 30
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			a := statusBandApp()
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
				t.Fatal(err)
			}
			tt := render(t, a)
			a.runPower(PowerShutdown)
			tt.SetSize(InputWindowWidth, 488+bandHeight)
			tt.Frame()

			msg := bandSentence(t, tt, a.powerMessage())
			label := bandSentence(t, tt, a.copy().PowerCancel)
			t.Logf("%s: question %+v, cancel %+v", lang, msg, label)
			if msg.H > bandHeight {
				t.Errorf("the question's box %+v is taller than the band (%du) — it wrapped", msg, bandHeight)
			}
			// The two buttons share the sentence's line: they must not be
			// pushed out of the band by it either.
			if label.Y+label.H > msg.Y+bandHeight || label.Y < msg.Y-bandHeight {
				t.Errorf("the cancel button %+v left the band the question's line %+v sits in", label, msg)
			}
		})
	}
}

// TestDumpTheStatusBandPNG writes the round's visual evidence: the band with a
// sentence longer than it is wide, in both appearances. Opt-in, as every dump
// in this package is (`FLOTER_PNG=1`, `FLOTER_PNG_SUFFIX` to keep a mutated
// build's pictures apart from the shipped one's).
func TestDumpTheStatusBandPNG(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	sentence := i18n.NotificationsFor("en").IntegrationDisconnected(strings.Repeat("very-long-extension-name-", 4))
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := statusBandApp()
		tt := render(t, a)
		tt.SetDark(dark)
		tt.Type("app")
		tt.Frame()
		a.List.ScrollIntoView(len(statusBandNames) - 1)
		tt.Frame()
		a.Feedback(sentence)
		tt.SetSize(InputWindowWidth, 488+30)
		tt.Frame()
		if !tt.HasText(sentence) {
			t.Fatalf("%s: the sentence is not drawn", appearance)
		}
		path := filepath.Join("/tmp", "floter-P11-cc-band-"+appearance+suffix+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}
