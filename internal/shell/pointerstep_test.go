package shell

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// P13-gy（`docs/liquid-glass.md` §6.175）：一个指针答案，一个速度。
//
// §5 第一行只给一个速度（`theme.ColorStep` = 120ms），但 shell 这一侧有五个面
// 在**指针落下的那一帧**直接跳到答案：两个插件窗口的条子键（捕获输出页的
// Rerun/Close、便条的 Close）与浮层里的两枚控件（toast 的动作 chip 与 ✕）。
// 同族的设置页小格、启动器小格都在 120ms 里走完，同一个指针动作在一个应用里
// 两种速度。
//
// 这一族 pin 的读法是三帧：指针不在时读一次（静息），指针落下后的**第一帧**
// 再读一次，然后推进 300ms 读第三次（落定）。
//
//	① 第一帧必须还是静息面——一次跳过去的实现（`f0 == f400`）在这里变红；
//	② 落定帧必须**不等于**静息面——不响应指针的实现（`f0 == f400 == rest`）
//	   也在这里变红，所以 ① 不会被一个「什么都不动」的破面蒙过去。
//
// 条子键与 ✕ 读的是键盒内部的像素（左 padding 带、盒中线那一行，字与图标之外）；
// toast 的两枚读的是**同一行上盒内与盒外的差**，因为 toast 的浮面自带一条纵向
// alpha 斜坡（最旧的一条会淡），绝对亮度不是面，而斜坡在同一 y 的两侧相同。

// pointerFrames drives the pointer onto (x, y) and answers the probe in the three
// frames the family needs: rest (pointer away), the frame it lands, and settled.
func pointerFrames(tt *ui.Tester, probe func(*image.RGBA) int, x, y int) (rest, first, settled int) {
	tt.Move(0, 0)
	tt.Frame()
	rest = probe(tt.Image())
	tt.Move(float32(x), float32(y))
	tt.Frame()
	first = probe(tt.Image())
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	settled = probe(tt.Image())
	return rest, first, settled
}

// assertOneStep states both halves of §5's first line.
func assertOneStep(t *testing.T, appearance, name string, rest, first, settled int) {
	t.Helper()
	t.Logf("%s/%s: rest %d → f0 %d → settled %d", appearance, name, rest, first, settled)
	if first != rest {
		t.Errorf("%s: %s answered the pointer in the frame it landed (%d → %d, %+d) — "+
			"the answer must ride theme.ColorStep (%v), not arrive instantly",
			appearance, name, rest, first, first-rest, theme.ColorStep)
	}
	if settled == rest {
		t.Errorf("%s: %s never moved off its resting face (%d) — "+
			"either the pointer never landed on it or the face does not answer at all",
			appearance, name, rest)
	}
}

// barKeyProbe reads the key's own face a few DIPs inside its left edge, on the
// middle row: clear of the word's strokes and of the two 1px lines.
func barKeyProbe(tt *ui.Tester, box ui.Rect) func(*image.RGBA) int {
	x := int(box.X) + 4
	y := int(box.Y+box.H/2) - 2
	return func(img *image.RGBA) int { return luma(shellPixelAt(img, x, y)) }
}

// TestTheWindowBarKeysStepThePointer: the capture window's Rerun and Close keys,
// and the note's Close key — the same bar, the same `barButton` builder.
func TestTheWindowBarKeysStepThePointer(t *testing.T) {
	bar := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		detached, _ := barDetached(t, dark, false)
		for _, label := range []string{bar.OutputWindowRerun, bar.OutputWindowClose} {
			box := barKeyBox(t, detached, label)
			rest, first, settled := pointerFrames(detached, barKeyProbe(detached, box), int(box.X+box.W/2), int(box.Y+box.H/2))
			assertOneStep(t, appearance, "capture window / "+label, rest, first, settled)
		}
		note := barPinned(t, dark)
		box := barKeyBox(t, note, bar.OutputWindowClose)
		rest, first, settled := pointerFrames(note, barKeyProbe(note, box), int(box.X+box.W/2), int(box.Y+box.H/2))
		assertOneStep(t, appearance, "note / "+bar.OutputWindowClose, rest, first, settled)
	}
}

// toastProbe reads a toast control as the difference between a pixel inside its
// box and a pixel on the pane a few DIPs to its left, on the same row: the pane's
// own vertical ramp cancels, the control's material does not.
func toastProbe(tt *ui.Tester, box ui.Rect) func(*image.RGBA) int {
	in := int(box.X) + 3
	out := int(box.X) - 3
	y := int(box.Y+box.H/2) - 2
	return func(img *image.RGBA) int { return luma(shellPixelAt(img, in, y)) - luma(shellPixelAt(img, out, y)) }
}

// TestTheToastControlsStepThePointer: the floating stack's action chip and its
// ✕ — both on the same pane, both answering a pointer in one frame before.
func TestTheToastControlsStepThePointer(t *testing.T) {
	dismissCopy := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := toastTester(t, dark)
		if !tt.HasText("Where am I") {
			t.Fatalf("%s: the toast is not drawn: %q", appearance, tt.Texts())
		}
		if dismiss, ok := tt.Find(dismissCopy.DismissNotice); ok {
			rest, first, settled := pointerFrames(tt, toastProbe(tt, dismiss),
				int(dismiss.X+dismiss.W/2), int(dismiss.Y+dismiss.H/2))
			assertOneStep(t, appearance, "toast / dismiss", rest, first, settled)
		} else {
			t.Fatalf("%s: the toast has no dismiss key: %q", appearance, tt.Texts())
		}
	}

	// The action chip needs a toast that carries one; the fixture above has none.
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := newApp(t)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
		tt.Frame()
		a.noticeMu.Lock()
		a.notices = append(a.notices, ui.Toast{
			ID: "floter-notice", Title: "Plugin page failed", Type: "error",
			Action: "Retry", OnAction: func() {}, Timeout: 30 * time.Second,
		})
		a.noticeMu.Unlock()
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		chip, ok := tt.Find("Retry")
		if !ok {
			t.Fatalf("%s: the toast has no action chip: %q", appearance, tt.Texts())
		}
		rest, first, settled := pointerFrames(tt, toastProbe(tt, chip),
			int(chip.X+chip.W/2), int(chip.Y+chip.H/2))
		assertOneStep(t, appearance, "toast / action", rest, first, settled)
	}
}

// TestDumpP13GYPointerStep writes the settled frame of each surface this round
// moved: the capture window's bar and the note's bar (siblings, same builder), and
// the floating stack's dismiss key. The frame is the one after the 120ms step has
// run out, so it is the face a reader sees — the round is a timing change, and a
// still cannot show a duration; the three readings in the log above are the
// evidence, this is the picture of the state it settles on.
func TestDumpP13GYPointerStep(t *testing.T) {
	bar := i18n.For("en").Launcher
	dismissCopy := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for name, tt := range map[string]*ui.Tester{
			"capture": func() *ui.Tester { d, _ := barDetached(t, dark, false); return d }(),
			"note":    barPinned(t, dark),
			"toast":   toastTester(t, dark),
		} {
			var box ui.Rect
			if name == "toast" {
				b, ok := tt.Find(dismissCopy.DismissNotice)
				if !ok {
					t.Fatalf("%s: no dismiss key", appearance)
				}
				box = b
			} else {
				box = barKeyBox(t, tt, bar.OutputWindowClose)
			}
			tt.Move(float32(box.X+box.W/2), float32(box.Y+box.H/2))
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P13-gy-"+name+"-"+appearance+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
