package launcher

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// P11-az · **列表底部那条带（toast / 电源确认）要占列表的位置，不要浮在列表上。**
//
// The window already grows by the band's own 30u when the band is docked
// (`Geometry.Feedback` → `height.go`'s `feedbackBand`), and the list is the
// panel's `Grow(1)`, so the list grew by the same 30u — but the band is an
// `Absolute()` layer over the list's bottom edge, so the 30u the window paid
// for is exactly the 30u the band covers: the list's last visible row hides
// under the band and cannot be seen or clicked (measured before the fix at
// 720×488: the lowest row ended at `y=443` inside the band's `y≈410..440`).
// The fix takes the band's height back out of the list's content
// (`statusBand` in the list's bottom padding), which is what the reference's
// in-flow `.launcher-feedback` (`main:src/styles/launcher.css:693`) does by
// being a sibling of the results rather than a layer over them.

// statusBandTools is the fixture's own catalog: twenty names, one shared word
// (`app`) so one query matches every row and the list overflows the window.
func statusBandTools() []apps.App {
	var list []apps.App
	for i := 1; i <= 20; i++ {
		n := fmt.Sprintf("app-%02d", i)
		list = append(list, apps.App{Name: n, Path: "/usr/bin/" + n})
	}
	return list
}

// statusBandNames is the same catalog's names, for reading the frame.
var statusBandNames = func() []string {
	var names []string
	for _, app := range statusBandTools() {
		names = append(names, app.Name)
	}
	return names
}()

func statusBandApp() *App {
	a := testApp()
	a.SetTools(statusBandTools(), true)
	return a
}

// lowestRowBottom is the bottom edge of the lowest result row the frame drew.
// Only the fixture's own tool names count: the footer's row, the field's hint
// and the band's sentence are not rows of the list.
func lowestRowBottom(t *testing.T, tt *ui.Tester, names []string) (string, float32) {
	t.Helper()
	best, bestName := float32(-1), ""
	for _, n := range names {
		r, ok := tt.Find(n)
		if !ok || r.H <= 0 || r.W <= 0 {
			continue
		}
		if bottom := r.Y + r.H; bottom > best {
			best, bestName = bottom, n
		}
	}
	return bestName, best
}

func TestTheStatusBandTakesItsSpaceFromTheList(t *testing.T) {
	a := statusBandApp()
	tt := render(t, a)
	tt.Type("app")
	tt.Frame()
	// The window is the shell's, not the app's: the shell sizes it to
	// `Geometry.Height()`, and that height grows by the band's own 30u when the
	// band is docked (`height.go`'s `feedbackBand`, `Geometry.Feedback`). The
	// fixture does what the shell does: it grows the window by exactly that
	// band, then asks whether the list's last row is still under it.
	const w, h = 720, 488
	const bandPixels = 30 // 7.5u × 4 at the shipped step

	// Scroll to the end: that is the row the band could hide. (At the top the
	// rows pass under the band by construction — a floating band over a
	// scrolling list, the same geometry as the pinned footer.)
	a.List.ScrollIntoView(len(statusBandNames) - 1)
	tt.Frame()
	if name, _ := lowestRowBottom(t, tt, statusBandNames); name != statusBandNames[len(statusBandNames)-1] {
		t.Fatalf("fixture does not reach the list's bottom edge: the lowest row is %q", name)
	}

	a.runPower(PowerShutdown)
	tt.SetSize(w, h+bandPixels)
	tt.Frame()
	msg, ok := tt.Find(a.powerMessage())
	if !ok {
		t.Fatalf("the confirmation did not show: %q", tt.Texts())
	}
	name, bottom := lowestRowBottom(t, tt, statusBandNames)
	if name == "" {
		t.Fatalf("the band ate every row: %q", tt.Texts())
	}
	t.Logf("lowest row %q bottom=%.1f, band sentence top=%.1f", name, bottom, msg.Y)
	if bottom > msg.Y {
		t.Errorf("the list's lowest row %q ends at y=%.1f, inside the band whose sentence starts at y=%.1f"+
			" — the band covers the list instead of taking its space", name, bottom, msg.Y)
	}
}

// TestOnlyOneSentenceWinsTheBand: the band is one rectangle, and
// `Geometry.Feedback` is a bool — the window only ever pays for one band. Two
// sentences there were two runs on one block of pixels (measured before the
// fix: `{X:40 Y:462 W:488 H:22}` and `{X:40 Y:462 W:664 H:22}`, the question's
// buttons painted over the toast's words). The blocking question wins; the
// toast keeps its words and its clock has not started.
func TestOnlyOneSentenceWinsTheBand(t *testing.T) {
	a := statusBandApp()
	tt := render(t, a)
	tt.Type("app")
	tt.Frame()

	const toast = "Could not open the system terminal"
	a.Feedback(toast)
	a.runPower(PowerShutdown)
	tt.Frame()

	if !tt.HasText(a.powerMessage()) {
		t.Errorf("the power question lost the band: %q", tt.Texts())
	}
	if tt.HasText(toast) {
		t.Errorf("the toast shares the band with the question: %q", tt.Texts())
	}
	// 为什么这里不用整帧的「零压字」审计（`auditLauncherFrame`）：带子是**不透明**的
	// 一条 30u 浮带，列表的行照旧从它下面滚过去（和固定在底部的页脚同一种几何；
	// 参考里带子在流里时也只是把行**裁**在它上沿）。审计只看盒子、不看谁盖谁，
	// 于是会把「行在带子底下」报成压字——那不是本轮的缺陷。本轮要钉的是
	// **带子内部只有一个句子**，上面两条断言就是它。
}

// TestTheToastComesBackAfterTheQuestion is the other half of the rule: yielding
// the band is not swallowing the message. Once the question is answered the
// toast shows, and its clock starts from the moment it is actually drawn.
func TestTheToastComesBackAfterTheQuestion(t *testing.T) {
	a := statusBandApp()
	tt := render(t, a)
	tt.Type("app")
	tt.Frame()

	const toast = "Could not open the system terminal"
	a.Feedback(toast)
	a.runPower(PowerShutdown)
	tt.Frame()
	if tt.HasText(toast) {
		t.Fatalf("the toast drew while the question was up: %q", tt.Texts())
	}
	a.cancelPower()
	tt.Frame()
	if !tt.HasText(toast) {
		t.Errorf("the toast was swallowed by the question: %q", tt.Texts())
	}
}

// P11-az visual evidence: the docked band over the list, in both appearances,
// written to /tmp so the report can point at a file. Opt-in (`FLOTER_PNG=1`).
func TestDumpP11AZStatusBandPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-az-band-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := statusBandApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := render(t, a)
		tt.SetDark(dark)
		tt.Type("app")
		tt.Frame()
		a.runPower(PowerShutdown)
		// The shell grows the window by the band's own 30u; the dump shows what
		// the user's window looks like, not a fixed test frame.
		if img := tt.Image(); img != nil {
			tt.SetSize(img.Rect.Dx(), img.Rect.Dy()+30)
		}
		tt.Frame()
		if msg, ok := tt.Find(a.powerMessage()); ok {
			img := tt.Image()
			if img != nil {
				x := int(msg.X) + 4
				t.Logf("%s: band sentence y=%.0f..%.0f; above the band %v, inside it %v",
					appearance, msg.Y, msg.Y+msg.H,
					img.RGBAAt(x, int(msg.Y)-4), img.RGBAAt(x, int(msg.Y+msg.H/2)))
				// The lowest ink in the list's own band (above the band's top
				// edge): the last row's text must end there, not under the band.
				lowest := -1
				for y := 100; y < int(msg.Y)-4; y++ {
					for px := 50; px < 660; px++ {
						c := img.RGBAAt(px, y)
						if int(c.R)*299+int(c.G)*587+int(c.B)*114 < 30000 {
							lowest = y
							break
						}
					}
				}
				t.Logf("%s: the list's lowest ink is y=%d, the band's top edge is y=%d", appearance, lowest, int(msg.Y)-4)
			}
		}
		path := filepath.Join("/tmp", "floter-P11-az-band-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s", path)
	}
}
