package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// TestDumpP13EASessionRows is this round's visual evidence: the Sessions page
// with a running session, in the frame production draws in, light and dark.
// Set FLOTER_P13EA_DUMP=1 to write it; FLOTER_P13EA_TAG suffixes the file
// (before / after) so the two floor states can be read side by side.
func TestDumpP13EASessionRows(t *testing.T) {
	if os.Getenv("FLOTER_P13EA_DUMP") == "" {
		t.Skip("set FLOTER_P13EA_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13EA_TAG")
	if tag == "" {
		tag = "after"
	}
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		cfg := settings.Default()
		if dark {
			cfg.Theme = "dark"
		}
		a := New(settings.NewStore(cfg), Actions{
			CloseSessionAt: func(int) {},
			SelectSession:  func(int) {},
			NewSession:     func() {},
		})
		behind := productionSession()
		behind.ID, behind.Title, behind.Active, behind.Index = 2, "postgres", false, 1
		behind.Note, behind.Dir = "Running", "~/workspace/floter"
		a.Sessions = func() []Session { return []Session{productionSession(), behind} }
		tt := renderThemed(t, a, 720, 760)
		if err := tt.Click("Sessions"); err != nil {
			t.Fatal(err)
		}
		if _, ok := tt.Find(en.SessionsActive); !ok {
			t.Fatalf("the section hint is missing: %q", tt.Texts())
		}
		name := filepath.Join(os.TempDir(),
			fmt.Sprintf("floter-P13-ea-sessions-%s-%s.png", appearance, tag))
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
