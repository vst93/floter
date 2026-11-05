package settingsui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/theme"
)

// P13-gy（`docs/liquid-glass.md` §6.175）：设置页那五面也走完那 120ms。
//
// 同族的 `pageButton`、侧栏行、`barButton` 早就走 120ms，而这五面（弹层的胶囊、
// 会话小格、杀掉会话的问句、快捷键录制钮、运行输出折叠行）在**指针落下的那一帧**
// 跳到答案（`f0 == f400`）。`internal/ci/colorstep_contract_test.go` 的账本把这
// 五处按文件钉死（删一处/加一处即红）；这里给出**动态**证据：弹层胶囊
// （`sheetActionButton`，`pluginsheet.go`）的 `Control → ControlHover` 必须从静息
// 面开始走，而不是一帧到站。
//
// 读法不假设任何几何：一张只有这枚胶囊的合成面，指针落下后的**第一帧**必须与静息
// 帧逐字节相同（一次跳过去的实现这里就红），推进 300ms 之后必须不同（不响应指针的
// 实现这里也红）。
func TestTheSheetPillStepsThePointer(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens, _ := liteTokens(t, dark)
		a := New(settings.NewStore(settings.Default()), Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(realPageGround(t, dark))
			ui.Column(c).Padding(20).Children(func() {
				a.sheetActionButton(c, "Run", false, func() {})
			})
		}, 320, 160)
		tt.Frame()
		rest := tt.Image()
		tt.Move(40, 34)
		tt.Frame()
		first := tt.Image()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		settled := tt.Image()
		t.Logf("%s/sheet pill: rest→f0 identical=%v, rest→settled identical=%v",
			appearance, bytes.Equal(rest.Pix, first.Pix), bytes.Equal(rest.Pix, settled.Pix))
		if !bytes.Equal(rest.Pix, first.Pix) {
			t.Errorf("%s: the sheet's pill answered the pointer in the frame it landed — "+
				"the answer must ride theme.ColorStep (%v), not arrive instantly", appearance, theme.ColorStep)
		}
		if bytes.Equal(rest.Pix, settled.Pix) {
			t.Errorf("%s: the sheet's pill never moved off its resting face — "+
				"either the pointer missed it or the face does not answer at all", appearance)
		}
	}
}

// stepSurface renders one widget alone on the settings page's ground and answers
// the same three frames for it: the resting face, the frame the pointer lands,
// and the settled face. The probe takes the widget's own padding band (four DIPs
// inside the left edge, two rows above the middle) — clear of glyphs, icons and
// the 1px lines, which is where the face's material is.
func stepSurface(t *testing.T, dark bool, build func(a *App, c *ui.Context) ui.Element) (rest, first, settled int) {
	t.Helper()
	tokens, _ := liteTokens(t, dark)
	a := New(settings.NewStore(settings.Default()), Actions{})
	ground := realPageGround(t, dark)
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			build(a, c).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 360, 180)
	tt.Frame()
	x, y := int(box.X)+4, int(box.Y+box.H/2)-2
	read := func() int { return litSum(pixelAt(tt, x, y)) }
	tt.Move(1, 1)
	tt.Frame()
	rest = read()
	tt.Move(float32(box.X+box.W/2), float32(box.Y+box.H/2))
	tt.Frame()
	first = read()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	return rest, first, read()
}

// TestTheSessionsAndShortcutsFacesStepThePointer is the same evidence on the
// three faces of this page that still answered a pointer in one frame: the
// sessions row's 28u icon button, the armed kill question (the warm pill), and
// the shortcut recorder. The census in `internal/ci` counts them per file; this
// pin is the dynamic half — the frame the pointer lands must still be the
// resting face, and 300ms later it must not be.
func TestTheSessionsAndShortcutsFacesStepThePointer(t *testing.T) {
	en := i18n.For("en").Settings
	builds := []struct {
		name  string
		build func(a *App, c *ui.Context) ui.Element
	}{
		{"session icon button", func(a *App, c *ui.Context) ui.Element {
			return boundSquare(c, func() {
				a.sessionIconButton(c, "play", en.SessionsResume, false, true, false, func() {})
			})
		}},
		{"kill question", func(a *App, c *ui.Context) ui.Element {
			// Drawn into the column it fills, not returned: the arm is part of
			// the sessions row, so the probe binds the column (`shortcutRejectedAlert`
			// is measured the same way).
			var wrap ui.Handle
			col := ui.Column(c).FillWidth().Bind(&wrap)
			col.Children(func() { a.sessionKillConfirm(c, en.SessionsKillArm, false, func() {}) })
			return col
		}},
		{"shortcut recorder", func(a *App, c *ui.Context) ui.Element {
			var wrap ui.Handle
			col := ui.Column(c).FillWidth().Bind(&wrap)
			col.Children(func() {
				a.shortcutRecorder(c, en.ShortcutRecord, shortcuts.Display(settings.ShortcutToggleWindow), func() {})
			})
			return col
		}},
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, b := range builds {
			rest, first, settled := stepSurface(t, dark, b.build)
			t.Logf("%s/%s: rest %d → f0 %d → settled %d", appearance, b.name, rest, first, settled)
			if first != rest {
				t.Errorf("%s: %s answered the pointer in the frame it landed (%d → %d, %+d) — "+
					"the answer must ride theme.ColorStep (%v), not arrive instantly",
					appearance, b.name, rest, first, first-rest, theme.ColorStep)
			}
			if settled == rest {
				t.Errorf("%s: %s never moved off its resting face (%d) — "+
					"either the pointer missed it or the face does not answer at all", appearance, b.name, rest)
			}
		}
	}
}
