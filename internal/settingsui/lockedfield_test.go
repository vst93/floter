package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-cb · 锁定的字段：灯跟着状态一起熄。
//
// 上一轮（P13-ca）量到锁定后的字段还亮着半盏灯——冠线比面高 10 级（深）/ 2 级（浅），
// 底对侧那条暗线却被 0.5 那道淡吃掉了。当时改不动：冠线是 `fieldFace` 建元素时画的，
// mygo 的内阴影只追加不撤回（`ui/element.go:1162`）。这一轮把锁定状态带进了字段材质
// （`fieldFace` 的 `lit` 参数 → `DimFace`），所以本文件钉两件事：
//
//	A. 材质本身：同一个 `fieldFace`，`lit=false` 时冠线与面逐像素同色（下拉框那条链）。
//	B. 接线：**产品路径**（`configField`，不是 builder）在行忙时画出的字段必须没有那盏灯。
//
// B 是这一轮唯一的接线判据：把 `config.go` 里的 `!locked` 改成 `true`，它就会红。

// fieldEdge reads the field's top three rows in a render of the config form:
// the border, the lit-edge row under it, and the face. The column is clear of
// the label and of the placeholder, so the first row that is not the ground is
// the field's own top border.
func fieldEdge(t *testing.T, img *image.RGBA, ground ui.Color) (crown, face ui.Color) {
	t.Helper()
	const x = 250
	top := -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		if q := imagePixel(img, x, y); q != ground {
			top = y
			break
		}
	}
	if top < 0 || top+3 >= img.Bounds().Dy() {
		t.Fatalf("no field on the frame at x=%d: the column is all ground %v", x, ground)
	}
	return imagePixel(img, x, top+1), imagePixel(img, x, top+3)
}

// A. The drop-down on the same form: it shares `fieldFace`, so it must go unlit
// with the fields beside it — a locked form cannot have one control still saying
// "you can touch this".
func TestTheLockedSelectGoesUnlit(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, rest, locked := lockProbe(t, dark, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			selected := "alpha"
			e := a.configSelect(c, &selected, []string{"alpha", "beta"}, "mode", !isLocked)
			if isLocked {
				e = lockedControl(e)
			}
			return e
		})
		// The resting control really does wear a lit edge, otherwise this pin
		// proves nothing.
		if d := absDiff(litSum(rest.crown), litSum(rest.face)); d < 3 {
			t.Fatalf("the resting select (%s): its lit edge is %v and its face %v — the control has no light to put out",
				appearanceName(dark), rest.crown, rest.face)
		}
		if d := absDiff(litSum(locked.crown), litSum(locked.face)); d > 1 {
			t.Errorf("the locked select (%s): the lit edge is %v but the face is %v (%d levels apart) — a locked field still wears the mark of a surface the pointer can land on (docs/liquid-glass.md §3/§6.45)",
				appearanceName(dark), locked.crown, locked.face, d/3)
		}
	}
}

// B. The product path: a config form whose row is busy draws its field with the
// light out. Both renders are the real `configField`; the only difference is
// `a.toggleBusy`, i.e. the same `configLocked` predicate that gates
// `lockedControl`. The pin reads the field's own top edge off the frame: idle,
// it is brighter than the face; busy, it *is* the face.
//
// 这是这一轮接线的唯一判据：把 `config.go` 里的 `!locked` 写死成 `true`，
// 忙态那一格就重新亮起来，这条 pin 立刻红。
func TestTheConfigFormFieldGoesUnlitWhenTheRowIsBusy(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		edge := func(busy bool) (ui.Color, ui.Color) {
			ground := realPageGround(t, dark)
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			if busy {
				a.toggleBusy = map[string]bool{"x": true}
			}
			draft := &configDraft{values: map[string]any{}}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(24).Children(func() {
					a.extensionMetrics(func() {
						a.configField(c, en, Integration{ID: "x"}, draft, ConfigField{
							Key: "token", Label: "Token", Type: "text",
						})
					})
				})
			}, 480, 200)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			crown, face := fieldEdge(t, tt.Image(), ground)
			return crown, face
		}
		crown, face := edge(false)
		if litSum(crown) <= litSum(face) {
			t.Fatalf("the idle form (%s): the field's top edge %v is not brighter than its face %v — the pin cannot read what it is guarding",
				appearanceName(dark), crown, face)
		}
		crown, face = edge(true)
		if d := absDiff(litSum(crown), litSum(face)); d > 1 {
			t.Errorf("the busy form (%s): the field's top edge is %v but its face is %v (%d levels apart) — the row is locked, but its field is still lit (docs/liquid-glass.md §6.45)",
				appearanceName(dark), crown, face, d/3)
		}
	}
}

// The opt-in dump: the locked field's material, before and after.
// `FLOTER_PNG_TAG=before` keeps the lit edge on the locked field (the old
// material), so the pair can be compared without checking out the old tree.
func TestDumpP13CbLockedField(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	before := os.Getenv("FLOTER_PNG_TAG") == "before"
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(20).Width(320).Gap(10).Children(func() {
				for _, locked := range []bool{false, true} {
					locked := locked
					s := "hello"
					f := a.extensionFieldLit(c, &s, configFieldMetrics, !locked || before)
					if locked {
						f = lockedControl(f)
					}
				}
				for _, locked := range []bool{false, true} {
					locked := locked
					selected := "alpha"
					e := a.configSelect(c, &selected, []string{"alpha", "beta"}, "mode", !locked || before)
					if locked {
						e = lockedControl(e)
					}
				}
			})
		}, 400, 220)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tag := "after"
		if before {
			tag = "before"
		}
		name := filepath.Join("/tmp", "floter-P13-cb-lockedfield-"+map[bool]string{false: "light", true: "dark"}[dark]+"-"+tag+".png")
		file, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
		file.Close()
		t.Logf("wrote %s", name)
	}
}
