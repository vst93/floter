package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-k · 「toast 顶 14 行的纵向渐变」是产品缺陷还是 dump 台的黑底？
//
// P13-j 的「仍待」第 2 条记下过一个观察：toast 的 `Float` 面半透明，pane 顶 14 行
// 是一条 241→202 的纵向渐变（浅色档），怀疑是 E2 的两层影从面里透出来。本轮把它量
// 到底，结论是**两条都不是**：
//
//  1. **不是自己的影**：mygo 的 `Shadow` 只画在 cast 矩形**外面**（`paint.go:1218`
//     的注释与 `scene.Op.Cast` 字段），同一个元素上的影进不到自己的面里。隔离渲染
//     （一个 `Float` 面 + `theme.ElevThrow`）实测面内是平的：浅色 253 一整片，影只
//     出现在盒外的行上。
//  2. **也不是兄弟的影**：两张 toast 叠起来时，上面那张的 throw 落在 8px 的缝里
//     （缝里的行 100~107，背景 137），到下面那张的第一个内行已经衰减到 8 级以内。
//
// 那个渐变是**背景透过玻璃**：设置页内容区顶部有一条 16px 的 `glass.ScrollEdge`
// （`settingsui.go` 的 `body`），它回放的是**不透明的**窗口底色（浅色 `RGB(243,244,247)`
// 的 85%），而 dump 台的窗口后面没有桌面（黑），于是框体合成出来是 137、那条带子是
// 228 —— 91 级的亮带。toast 的 `Float` 面（可见合成 0.77）把它透出来，就是 241→202。
//
// 真机上框体后面是桌面，浅色档框体本身就在 240 上下，同一条带子只有 4~6 级；只有
// 「深色壁纸 + 浅色外观 + 低透明度」这一种组合会让它重新亮起来。那是**滚动边缘回放
// 不透明底色**的问题（需要拍板：插件 API 只接受一个不透明 `Background`，回放什么由它
// 自己乘 0.85，回放不了「框体的半透明面」），不是 toast 的材质问题。
//
// 所以本轮不产出像素断言，只产出这把尺子：把 1 张 / 2 张 toast 的每一行读数写进日志，
// 并把帧写成 PNG，下一轮要动滚动边缘时先拿它对比。
//
// Opt-in：`FLOTER_PNG=1 go test -run TestDumpP13KToastStack ./internal/shell/`。
func TestDumpP13KToastStack(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the stack sheets")
	}
	for _, appearance := range chipAppearances {
		appearance := appearance
		t.Run(appearance.name, func(t *testing.T) {
			for _, count := range []int{1, 2} {
				img, bare := toastStackFrame(t, appearance.dark, count)
				name := "/tmp/floter-P13-k-toast-stack-" + appearance.name + "-" + itoa(count) + ".png"
				f, err := os.Create(name)
				if err != nil {
					t.Fatalf("create %s: %v", name, err)
				}
				if err := png.Encode(f, img); err != nil {
					f.Close()
					t.Fatalf("encode %s: %v", name, err)
				}
				if err := f.Close(); err != nil {
					t.Fatalf("close %s: %v", name, err)
				}
				t.Logf("[%s n=%d] wrote %s", appearance.name, count, name)
				// The profile: the pane's rows, the bare backdrop's rows (the same
				// frame with no toast), and the gap between two panes.
				for _, x := range []int{370, 620} {
					t.Logf("[%s n=%d] x=%d bare:%s", appearance.name, count, x, toastColumn(bare, x, 55, 120))
					t.Logf("[%s n=%d] x=%d toast:%s", appearance.name, count, x, toastColumn(img, x, 55, 120))
				}
			}
		})
	}
}

// TestTheToastFaceOnlyAttenuatesItsBackdrop is the pin the measurement above
// leaves behind, and the one that tells H1/H2 from H3 on any frame: a pane that
// is glass **follows** its backdrop, attenuated — where the backdrop brightens
// the pane brightens, and by no more than the backdrop did. A shadow bleeding
// through the pane (H1/H2) inverts that: the pane would darken while the
// backdrop underneath is flat or brightening.
//
// The pair of readings is the same surface rendered twice — with one toast and
// with none — so the backdrop is the app's own, not an assumption about the
// tester's floor. Rows are read inside the pane's face: two rows below its top
// rim and two above its bottom rim, in the empty column right of the sentence.
func TestTheToastFaceOnlyAttenuatesItsBackdrop(t *testing.T) {
	for _, appearance := range chipAppearances {
		appearance := appearance
		t.Run(appearance.name, func(t *testing.T) {
			img, bare := toastStackFrame(t, appearance.dark, 1)
			label, ok := toastLabel(t, appearance.dark)
			if !ok {
				t.Fatalf("[%s] the toast is not drawn", appearance.name)
			}
			x := int(label.X+label.W/2) + 90
			y0, y1 := int(label.Y)-2, int(label.Y+label.H)+2
			for y := y0; y < y1; y++ {
				paneNow, paneBefore := chipPixel(img, x, y), chipPixel(img, x, y+1)
				backNow, backBefore := chipPixel(bare, x, y), chipPixel(bare, x, y+1)
				dPane := int(paneNow.R) - int(paneBefore.R)
				dBack := int(backNow.R) - int(backBefore.R)
				if absInt(dBack) < 2 {
					continue
				}
				if (dPane > 0) != (dBack > 0) && dPane != 0 {
					t.Fatalf("[%s] y=%d: the pane went %+d while its backdrop went %+d — the pane is not following "+
						"its backdrop, something is painted inside it (a shadow through the face)", appearance.name, y, dPane, dBack)
				}
				if absInt(dPane) > absInt(dBack)+2 {
					t.Fatalf("[%s] y=%d: the pane moved %+d for a backdrop move of %+d — a glass face attenuates its "+
						"backdrop, it does not amplify it", appearance.name, y, dPane, dBack)
				}
			}
		})
	}
}

// toastLabel is the sentence's own box: the pane is that box plus its 2u/3u
// padding, so it locates the face without a hard-coded row.
func toastLabel(t *testing.T, dark bool) (ui.Rect, bool) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store})
	now := time.Unix(0, 0)
	a.toastClock = func() time.Time { return now }
	a.Open(SurfaceSettings)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	a.noticeMu.Lock()
	a.notices = append(a.notices, ui.Toast{ID: "alpha", Title: "Alpha notice", Type: "info", Timeout: refusalNotice})
	a.noticeMu.Unlock()
	tt.Frame()
	now = now.Add(toastEnter)
	tt.Frame()
	return tt.Find("Alpha notice")
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// toastStackFrame renders the settings surface with `count` settled toasts and
// returns that frame plus the same surface with no toast at all, so a reading
// inside a pane can be compared against the backdrop it covers.
func toastStackFrame(t *testing.T, dark bool, count int) (*image.RGBA, *image.RGBA) {
	t.Helper()
	build := func(withToasts bool) *image.RGBA {
		store := settings.NewStore(settings.Default())
		a := New(Options{Store: store})
		now := time.Unix(0, 0)
		a.toastClock = func() time.Time { return now }
		a.Open(SurfaceSettings)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		if withToasts {
			a.noticeMu.Lock()
			a.notices = append(a.notices, ui.Toast{
				ID: "alpha", Title: "Alpha notice", Type: "info", Timeout: refusalNotice,
			})
			if count > 1 {
				a.notices = append(a.notices, ui.Toast{
					ID: "beta", Title: "Beta notice", Type: "info", Timeout: refusalNotice,
				})
			}
			a.noticeMu.Unlock()
			tt.Frame()
			// Past `--dur-4`: a half-travelled pane is not the material.
			now = now.Add(toastEnter)
			tt.Frame()
		}
		return shellFrameCopy(tt.Image())
	}
	return build(true), build(false)
}

// toastColumn is one column of readings, top to bottom, as `R` levels.
func toastColumn(img *image.RGBA, x, y0, y1 int) string {
	var s string
	for y := y0; y <= y1; y++ {
		s += " " + itoa(int(chipPixel(img, x, y).R))
	}
	return s
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
