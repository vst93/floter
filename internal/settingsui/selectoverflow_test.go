package settingsui

import (
	"strings"
	"testing"

	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCustomUnknownActionKeepsArrowInsideField(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			value := "plugin:unavailable-extension-with-a-long-action-name"
			*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: value}}
			a.Store.Update(func(s *settings.Settings) { s.Language = "en"; s.UIScale = "large" })
			a.Page = PageShortcuts
			tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 1000)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			refusalDump(t, tt, "unknown-action-arrow", dark)
			picker := mustFind(t, tt, a.copy().ShortcutsAction)
			text := mustFind(t, tt, value)
			t.Logf("trigger=%+v text=%+v", picker, text)
			if text.X+text.W > picker.X+picker.W-33 {
				t.Error("long action consumes dropdown arrow space")
			}
			if picker.X+picker.W > 684.8 {
				t.Error("long action exceeds row content edge")
			}
			ownershipChoose(t, tt, a.copy().ShortcutsAction, true)
			if !a.customDraftOpen || len(*entries) != 0 {
				t.Fatal("unknown action cannot recover through dropdown")
			}
		})
	}
}

func TestPluginLongOptionStaysInsidePopup(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			long := "工作目录：" + strings.Repeat("项目文件夹/", 24)
			sheet := selectValueSheet("files", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: long}})
			tt := pluginIdentityFrame(t, dark, &sheet)
			ownershipClick(t, tt, "文件")
			tt.Key(0, ui.KeyEnd)
			tt.Frame()
			refusalDump(t, tt, "long-option-popup", dark)
			text := mustFind(t, tt, long)
			t.Logf("option text=%+v", text)
			// The option's highlight must finish its right corner inside the
			// viewport. The old popup used the window width before positioning;
			// its blue fill ran through the last drawable column (the root
			// clips the final raster column itself).
			img := tt.Image()
			y := int(text.Y + text.H/2)
			if img.RGBAAt(img.Bounds().Max.X-2, y) == img.RGBAAt(int(text.X)-3, y) {
				t.Error("long option highlight is cut off at the window edge")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["mode"] != "apps" {
				t.Fatal("clipped option lost its value")
			}
		})
	}
}
