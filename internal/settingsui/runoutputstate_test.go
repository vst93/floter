package settingsui

import (
	"strings"
	"testing"
	"time"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func runOutputStateFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			a.extensionMetrics(func() { a.runOutput(c, i18n.For("en").Settings, Integration{ID: "runner"}) })
		})
	}, 720, 360)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func TestRunOutputNewResultStartsAtTop(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, success := range []bool{false, true} {
			name := "failure"
			if success {
				name = "success"
			}
			t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
				a, _ := runStateApp()
				old := strings.Repeat("Previous run line\n", 60)
				a.SetRunResult("runner", RunResult{Stdout: old, Success: true})
				a.setRunOutputOpen("runner", true)
				tt := runOutputStateFrame(t, a, dark)
				box := mustFind(t, tt, old)
				tt.Wheel(box.X+box.W/2, box.Y+box.H/2, 0, 300)
				tt.Advance(time.Second)
				tt.Frame()
				if a.runOutputScroll("runner").Y <= 0 {
					t.Fatal("fixture did not scroll")
				}
				a.runOutputScroll("other").Y = 37
				fresh := "NEW RUN FIRST LINE\n" + strings.Repeat("New result line\n", 60)
				a.SetRunResult("runner", RunResult{Stdout: fresh, Success: success})
				tt.Frame()
				tt.Frame()
				runStateDump(t, tt, "output-reset-"+name, dark)
				if got := a.runOutputScroll("runner").Y; got != 0 {
					t.Errorf("new output starts at stale Y=%v", got)
				}
				if got := a.runOutputScroll("other").Y; got != 37 {
					t.Errorf("other output jumped to %v", got)
				}
				if !a.outputOpen["runner"] {
					t.Error("new result collapsed an already open output")
				}
			})
		}
	}
}

func TestRunOutputRendersANSIAndCopiesPlainText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, _ := runStateApp()
			const raw = "\x1b[31mALERT\x1b[0m plain\x1b[2K"
			const plain = "ALERT plain"
			a.SetRunResult("runner", RunResult{Stdout: raw, Success: false})
			tt := runOutputStateFrame(t, a, dark)
			runStateDump(t, tt, "output-ansi", dark)
			box, ok := tt.Find(plain)
			if !ok {
				t.Fatalf("captured output exposes ANSI bytes: %q", tt.Texts())
			}
			red := 0
			img := tt.Image()
			for y := int(box.Y); y < int(box.Y+box.H); y++ {
				for x := int(box.X); x < int(box.X+box.W); x++ {
					p := imagePixel(img, x, y)
					if int(p.R) > int(p.G)+20 && int(p.R) > int(p.B)+20 {
						red++
					}
				}
			}
			if red == 0 {
				t.Error("ANSI red span has no red ink")
			}
			tt.ClickAt(box.X+3, box.Y+box.H/2)
			tt.Command("selectAll")
			tt.Command("copy")
			if got := tt.Clipboard(); got != plain {
				t.Errorf("output copy=%q, want %q", got, plain)
			}
			result, _ := a.LastRun("runner")
			if result.Text() != raw {
				t.Error("rendering rewrote captured output bytes")
			}
			// An empty stream and formatting-only output have no visible text;
			// both must keep the existing empty-state line and its geometry.
			for _, empty := range []string{"", "\x1b[31m\x1b[0m"} {
				a.SetRunResult("runner", RunResult{Stdout: empty, Success: false})
				tt.Frame()
				if !tt.HasText(i18n.For("en").Settings.RunNoOutput) {
					t.Errorf("empty output %q lost the placeholder", empty)
				}
			}
		})
	}
}
