package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/settings"
)

// The opt-in dump for P13-fv: the sessions page in its full state (22 rows) at
// the top, the middle and the end of the scroll, both appearances. Run with
// FLOTER_P13FV_TAG=before while the page body's scroll edges are handed
// `t.Background` again to get the comparison pair.
func TestDumpP13FVFullSessions(t *testing.T) {
	if os.Getenv("FLOTER_P13FV_DUMP") == "" {
		t.Skip("set FLOTER_P13FV_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13FV_TAG")
	if tag == "" {
		tag = "after"
	}
	const rows = 22
	base := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.Sessions = func() []Session {
			out := make([]Session, 0, rows)
			for i := 0; i < rows; i++ {
				out = append(out, Session{
					ID:      uint64(i + 1),
					Index:   i,
					Title:   fmt.Sprintf("zsh — floter %02d", i+1),
					Running: i%3 != 0,
					Active:  i == 0,
					Note:    "Running",
					Dir:     "/home/tar/workspace/floter",
					Size:    "80×24",
					Created: base.Add(-time.Duration(i) * time.Minute),
				})
			}
			return out
		}
		a.selectPage(PageSessions)
		tt, _ := renderIn(t, a, 720, 760, dark)
		for _, frac := range []float32{0, 0.5, 1} {
			a.Body.Y = a.Body.MaxY * frac
			tt.Frame()
			tt.Frame()
			name := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-fv-sessions-%s-%s-y%02d.png",
				appearance, tag, int(frac*100)))
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
}
