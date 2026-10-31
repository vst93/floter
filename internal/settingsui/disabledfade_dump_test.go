package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The opt-in dump: the four disabled faces (icon key, config Save, About's
// update CTA, the sessions kill pill) above their enabled twins, both
// appearances. Run with FLOTER_P13AQ_TAG=before while the four fades are
// re-injected to get the comparison pair.
func TestDumpP13AQDisabledFade(t *testing.T) {
	if os.Getenv("FLOTER_P13AQ_DUMP") == "" {
		t.Skip("set FLOTER_P13AQ_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13AQ_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(20).Gap(8).Width(360).Children(func() {
				for _, disabled := range []bool{true, false} {
					disabled := disabled
					a.extensionIconButton(c, "settings", "Configure", disabled)
					wrapControl(c, func() { _ = a.extensionPrimaryButton(c, "Save", disabled) })
					wrapControl(c, func() { a.updateButton(c, "Download", disabled, nil) })
					wrapControl(c, func() { a.sessionKillConfirm(c, "Close", disabled, nil) })
				}
			})
		}, 400, 420)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		name := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-aq-disabled-%s-%s.png", appearance, tag))
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
