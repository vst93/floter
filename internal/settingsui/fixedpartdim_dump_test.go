package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The opt-in dump for P13-ar: the uninstall dialog's four component rows (the
// fixed program row on top, then the three the user chooses) rendered off the
// real page ground, both appearances. Run it with FLOTER_P13AR_TAG=before while
// the old `Opacity(0.6)` + inner `Disabled` + lit crown are re-injected to get
// the comparison pair.
func TestDumpP13ARFixedPart(t *testing.T) {
	if os.Getenv("FLOTER_P13AR_DUMP") == "" {
		t.Skip("set FLOTER_P13AR_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13AR_TAG")
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
		var draft UninstallComponents
		parts := uninstallParts(i18n.For("en").Settings, &draft)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(20).Gap(8).Width(420).Children(func() {
				for _, part := range parts {
					a.uninstallPartRow(c, part)
				}
			})
		}, 460, 360)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-ar-fixedpart-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(filepath.Clean(name))
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
