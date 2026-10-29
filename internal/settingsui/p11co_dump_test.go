package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
)

// P11-co's zero-pixel evidence: the round changes only *where* the settings
// value is read from, never what it holds, so a frame drawn before and after
// must be identical byte for byte. Opt-in, like the other dumps: a test run
// should not litter /tmp.
func TestDumpP11COSettingsFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		for _, page := range []struct {
			name  string
			index int
		}{
			{"general", PageGeneral},
			{"integrations", PageIntegrations},
		} {
			a := New(settings.NewStore(settings.Default()), Actions{
				SetIntegrationEnabled: func(string, bool, func(error)) {},
				SetCommandEnabled:     func(string, string, bool) {},
			})
			a.Integrations = func() []Integration {
				out := make([]Integration, 0, 20)
				for i := 0; i < 20; i++ {
					commands := make([]Command, 0, 5)
					for j := 0; j < 5; j++ {
						commands = append(commands, Command{ID: "c", Name: "Command", Enabled: true, Available: true})
					}
					out = append(out, Integration{
						ID: "ext", Name: "Integration", Enabled: true, Running: true,
						Permissions: []string{"filesystem-read", "environment"},
						Enforced:    map[string]bool{"environment": true},
						Commands:    commands,
					})
				}
				return out
			}
			a.Page = page.index
			tt, _ := renderIn(t, a, 860, 900, appearance.dark)
			writePNG(t, filepath.Join("/tmp",
				"floter-P11-co-"+page.name+"-"+appearance.name+"-"+tag+".png"), tt)
		}
	}
}
