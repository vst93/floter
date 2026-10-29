package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The settings pages' *transient* faces in the same ruler as their resting ones.
//
// `TestTheSettingsPagesPaintNoTextOverText` walks the five pages at two
// appearances, two languages and two window sizes, but every page it draws is
// resting: no action in flight, no refused recording, no open recorder, no
// empty integration list. Those faces are drawn by the same row builders with
// different words in the same boxes — a busy pill gains a spinner, the refusal
// is a full-width sentence above the groups and a second one under its row, the
// recorder's pane lands where the resting key-cap was — so "the resting page is
// clean" does not cover them. P11-bu's rule for a surface that was never
// measured: measure it, then make it a pin; a probe that runs once and is
// deleted is a claim, not a guard.
//
// Each arm is *inline* by construction: a busy action pill, a refused
// recording's sentence, an open recorder, an empty list. The floating ones (the
// save-failed banner, the plugin sheet) belong to a different rule — the banner
// sits over the page it reports on by design, so folding it in here would teach
// this ruler to accept text over text.
//
// Every arm also proves it induced something: it must change pixels against the
// same page's resting frame, and (where the state has a sentence of its own)
// that sentence must be on the frame. Without that, a rename of one field would
// leave a t.Run that draws the resting page and passes.
func TestTheSettingsPagesTransientFacesPaintNoTextOverText(t *testing.T) {
	type state struct {
		name string
		page int
		// setup puts the page into the state; nil is the resting frame the arm
		// is compared against.
		setup func(a *App)
		// want names the copy field the state itself puts on the page, empty
		// when the state's own picture is the proof (the busy pill).
		want string
	}
	states := []state{
		{
			name: "busy re-check",
			page: PageIntegrations,
			setup: func(a *App) {
				// The row whose re-check is running: the pill swaps its glyph for
				// the turning LoaderCircle and keeps its word
				// (`busyActionButton`), so the word is the resting pill's too.
				a.repairBusy = map[string]bool{"dev.floter.broken": true}
			},
		},
		{
			name: "refused recording",
			page: PageShortcuts,
			setup: func(a *App) {
				a.rejectedShortcut = settings.ShortcutToggleWindow
			},
			want: "ShortcutRejected",
		},
		{
			name: "open recorder",
			page: PageShortcuts,
			setup: func(a *App) {
				a.recording, a.ShortcutID = true, settings.ShortcutToggleWindow
			},
			want: "ShortcutRecording",
		},
		{
			name: "no integrations",
			page: PageIntegrations,
			setup: func(a *App) {
				a.Integrations = func() []Integration { return nil }
			},
			want: "IntegrationsEmpty",
		},
	}
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range [][2]int{{860, 900}, {720, 560}} {
				for _, st := range states {
					tag := fmt.Sprintf("%s %s %s %s %dx%d", lang, appearance, st.name, PageName(st.page), size[0], size[1])
					t.Run(tag, func(t *testing.T) {
						resting := drawSettingsState(t, lang, appearance, dark, st.page, nil, size[0], size[1])
						tt := drawSettingsState(t, lang, appearance, dark, st.page, st.setup, size[0], size[1])
						auditSettingsFrame(t, tag, tt)
						if diff := changedPixels(resting.Image(), tt.Image()); diff == 0 {
							t.Errorf("%s: the state painted the resting page byte for byte; the arm exercises nothing", tag)
						}
						if st.want != "" {
							sentence := copyField(lang, st.want)
							if sentence == "" {
								t.Fatalf("%s: no copy for %q", tag, st.want)
							}
							if !tt.HasText(sentence) {
								t.Errorf("%s: the state's own sentence %q is not on the frame: %v", tag, sentence, tt.Texts())
							}
						}
						if os.Getenv("FLOTER_PNG") != "" {
							dumpInlineFrame(t, tag, tt)
						}
					})
				}
			}
		}
	}
}

// drawSettingsState draws one settings page, optionally put into a transient
// state first: the resting frame (setup nil) is what each arm is measured
// against.
func drawSettingsState(t *testing.T, lang, appearance string, dark bool, page int, setup func(a *App), w, h int) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance; s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{
		// The broken row's own entry ("Re-check") is only drawn when the page
		// has somewhere to send the press, so a harness without it measures a
		// page that never shows the control this round is about.
		RepairIntegration: func(string, func(error)) {},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.broken", Name: "Broken Tool", Enabled: true,
			Broken: true, FailureCode: "missing", FailureDetail: "/opt/tool",
			Description: "A tool whose runtime went missing.",
			Version:     "1.0.0", Publisher: "vst",
			Permissions: []string{"network"}, Enforced: map[string]bool{},
		}}
	}
	a.selectPage(page)
	if setup != nil {
		setup(a)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}

// copyField resolves the copy names the arms above ask for, so a renamed
// sentence is a compile-time-ish failure here rather than a silently missing
// assertion.
func copyField(lang, name string) string {
	c := i18n.For(lang).Settings
	switch name {
	case "ShortcutRejected":
		return c.ShortcutRejected
	case "ShortcutRecording":
		return c.ShortcutRecording
	case "IntegrationsEmpty":
		return c.IntegrationsEmpty
	}
	return ""
}

// changedPixels counts the pixels two frames of the same size disagree on.
func changedPixels(a, b *image.RGBA) int {
	if a == nil || b == nil || a.Rect != b.Rect {
		return -1
	}
	n := 0
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			i := y*a.Stride + x*4
			j := y*b.Stride + x*4
			if a.Pix[i] != b.Pix[j] || a.Pix[i+1] != b.Pix[j+1] ||
				a.Pix[i+2] != b.Pix[j+2] || a.Pix[i+3] != b.Pix[j+3] {
				n++
			}
		}
	}
	return n
}

// dumpInlineFrame writes one arm's frame when FLOTER_PNG is set: the round's
// visual evidence, not a pass of its own.
func dumpInlineFrame(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	name := "/tmp/floter-P11-bx-inline-" + slug(tag) + ".png"
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// slug turns a subtest name into a file name: the dump's tags carry spaces and
// 'x' separators, neither of which belong in a /tmp path.
func slug(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ' || r == '-':
			out = append(out, '-')
		}
	}
	return string(out)
}
