package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/apps"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestLauncherNavigationPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_NAV_PREVIEW")
	if dir == "" {
		t.Skip("set FLOTER_NAV_PREVIEW for visual evidence")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, held := range []bool{false, true} {
			a := testApp()
			a.SetApps([]apps.App{{Name: "Terminal", Path: "terminal"}, {Name: "Visual Studio Code", Path: "code"}, {Name: "Microsoft Edge", Path: "edge"}})
			a.SetRecent([]string{"terminal", "code", "edge"}, true)
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme, s.Language = appearance, "zh" }); err != nil {
				t.Fatal(err)
			}
			height := 300
			a.Actions.ResizeTo = func(h int) { height = h }
			tt := ui.NewTester(themedView(a), 720, height)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			name := "recent-"
			if held {
				tt.HoldModifiers(ui.Cmd)
				name = "super-"
			}
			tt.Frame()
			tt.SetSize(720, height)
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, name+appearance+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
