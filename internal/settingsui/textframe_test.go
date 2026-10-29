package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/frameaudit"
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
// P11-by：整宽箱子也下场（见 `internal/frameaudit` 的包注释），规则本身只留一份。
func auditSettingsFrame(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	frameaudit.Audit(t, tag, tt, nil)
}

// All five pages, in both appearances and both shipped languages, at the
// shipped size and at the two window sizes the reference's surface has to
// survive: the default card and the smallest one the settings window opens at.
//
// P11-ba: the audit used to run in English only. The user reads the Chinese
// surface, and the Chinese copy puts different-length words in the same rows,
// so the same rule has to hold on both. Measured clean on both before this
// change; the arms below keep it that way.
func TestTheSettingsPagesPaintNoTextOverText(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range [][2]int{{860, 900}, {720, 560}} {
				t.Run(fmt.Sprintf("%s %s %dx%d", lang, appearance, size[0], size[1]), func(t *testing.T) {
					for page := 0; page < pageCount; page++ {
						store := settings.NewStore(settings.Default())
						if err := store.Update(func(s *settings.Settings) { s.Theme = appearance; s.Language = lang }); err != nil {
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
						auditSettingsFrame(t, fmt.Sprintf("%s %s %s %dx%d", appearance, lang, PageName(page), size[0], size[1]), tt)
					}
				})
			}
		}
	}
}
