package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/settings"
)

// P13-ft（`docs/liquid-glass.md` §6.139）：`cardRow` 的**三重极限**——窄窗 × 长标签 ×
// 长值同时成立。前两轮把两维各钉了一次（§6.132 控制槽 `Shrink(0)` 从不吃标签列，
// §6.129 结果格最长那句答案），但两次都是**本仓自己的文案**。设置页里还有一类标签
// 不是本仓写的：**用户数据**——扩展自报的包名、命令名、别名，长度由发布者决定。
//
// 实测（720，深浅两套逐值相同；长臂 = 135 字符包名 / 120 字符命令名 / 40 字符别名）：
//
//	格                     短数据        长数据        结论
//	标题行 · 版本 x        653.8        653.8         一动不动
//	标题行 · 名字墨右沿    237..285      237..641      长出的 356px 全被名字列吃掉
//	命令行 · 开关盒 x      657.4        657.4         一动不动
//	命令行 · 名字墨右沿    200..278      200..641      列右沿 645.4，剩 4.4
//	命令行 · 别名框        200..689.8    200..689.8    弹性宽度不变
//
// 560（生产不可达：设置面板固定 `launcher.InputWindowWidth`，`shell.go:4743-4745`；
// 只有终端能 resize，`shell.go:1316-1320`）同样不越界：命令名墨到 478，开关盒
// 497.4..529.8，卡框右沿 543.8。**所以这一维不需要改**——但它此前没人守着，
// 用户数据一变长就会退化成本轮之前的那种「行边硬切」。
const (
	// extremePackageName is 135 characters of the shape a real package id takes
	// (`com.example.extensions.<segment>.<segment>…`), not a string of Xs: the
	// ellipsis has to land between segments to stay readable.
	extremePackageNameSeed = "com.example.extensions."
	extremePackageSegment  = "very-long-package-segment."
	// extremeCommandName is 120 characters, the shape a provider's `command`
	// key takes when it names its subcommand path.
	extremeCommandNameSeed = "run-a-very-long-command-name-"
	// extremeAlias is 40 characters — longer than any alias the launcher's
	// trigger parser would accept from a person, which is the point: the row
	// must not grow for input it cannot control.
	extremeAliasSeed = "alias-"
)

func extremePackageName() string {
	return extremePackageNameSeed + strings.Repeat(extremePackageSegment, 4) + "plugin"
}

func extremeCommandName() string {
	return strings.Repeat(extremeCommandNameSeed, 3) + "end"
}

func extremeAlias() string {
	return strings.Repeat(extremeAliasSeed, 8) + "x"
}

// dataApp is the integrations page with one integration whose user-controlled
// strings are the caller's: the same page twice, differing only in the data a
// publisher chooses, which is what makes the two renders comparable.
func dataApp(pkgName, commandName, alias string) *App {
	a := New(settings.NewStore(settings.Default()), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Page = PageIntegrations
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.tool", Name: pkgName, Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Permissions: []string{"filesystem-read", "environment"},
			Enforced:    map[string]bool{"environment": true},
			Commands: []Command{{
				ID: "c1", Name: commandName, Alias: alias,
				Description: "A description that is itself long enough to need the ellipsis, yes it is",
				Enabled:     true, Available: true, Declared: true,
			}},
		}}
	}
	return a
}

// TestAUserNameMovesNothingBesideIt is the title row's half of the triple: the
// package name is the only elastic child of the row (P13-ae's
// `Shrink(1).MinWidth(0)`), so a name three times the column's width must not
// push the version, the freshness mark or the switch — and it must not be cut
// at the row's edge either: it stops inside its own column, before the 12u gap.
func TestAUserNameMovesNothingBesideIt(t *testing.T) {
	for _, dark := range []bool{false, true} {
		short, _ := renderIn(t, dataApp("V Tools", "One command", "one"), 720, 1600, dark)
		long, _ := renderIn(t, dataApp(extremePackageName(), extremeCommandName(), extremeAlias()), 720, 1600, dark)

		_ = short
		longVer := mustFind(t, long, "v1.2.3")
		// The version follows the name (the reference's `.extension-row__title`
		// is a left-packed flex row), so the one number that says "the name was
		// clamped, not allowed to run" is where the version lands: the name
		// column's right edge, plus the row's gap. Measured 653.8; the card's
		// frame is 703.8, so the version and the switch keep their own 50u.
		if absFloat(longVer.X-653.8) > 1 {
			t.Errorf("dark=%v: under a 135-character package name the version starts at x=%v, want the "+
				"measured 653.8 (the name column's right edge plus the row's gap): the name is either "+
				"clamping somewhere else or pushing the version out of the row", dark, longVer.X)
		}
		name := mustFind(t, long, extremePackageName())
		ink, ok := rightmostInk(long, name)
		if !ok {
			t.Fatalf("dark=%v: the long package name paints nothing", dark)
		}
		// The name's box ends where the version's starts, minus the row's 12u
		// gap; the ellipsis has to land inside that, not on the row's edge.
		if gap := longVer.X - float32(ink); gap < 8 || gap > 20 {
			t.Errorf("dark=%v: the name's last painted column is %v, %v before the version at %v: want the "+
				"row's own 12u gap, so the ellipsis stops inside the name column",
				dark, ink, gap, longVer.X)
		}
		// And it really did have to stop: the name is far wider than the column.
		if float32(ink)-name.X < 300 {
			t.Errorf("dark=%v: the long name paints only %v columns, so the probe never reached the boundary",
				dark, float32(ink)-name.X)
		}
	}
}

// TestACommandNameStaysInsideItsOwnColumn is the command plate's half: the
// command's name, its description and its summon line are all caption-step
// single-line texts whose content is the provider's, and the switch rides the
// same line. All three must stop left of the switch, and the switch must not
// move for them.
func TestACommandNameStaysInsideItsOwnColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		short, _ := renderIn(t, dataApp("V Tools", "One command", "one"), 720, 1600, dark)
		long, _ := renderIn(t, dataApp(extremePackageName(), extremeCommandName(), extremeAlias()), 720, 1600, dark)

		shortSwitch := mustFind(t, short, "Enable One command in the launcher")
		longSwitch := mustFind(t, long, "Enable "+extremeCommandName()+" in the launcher")
		if absFloat(longSwitch.X-shortSwitch.X) > 0.5 {
			t.Errorf("dark=%v: the command's switch sits at x=%v under a 120-character command name but %v "+
				"under a short one: the switch is absorbing the name instead of the name giving",
				dark, longSwitch.X, shortSwitch.X)
		}
		if absFloat(longSwitch.W-shortSwitch.W) > 0.5 {
			t.Errorf("dark=%v: the switch is %v wide under the long name and %v under the short one: it must "+
				"keep its own size, not grow with its label", dark, longSwitch.W, shortSwitch.W)
		}
		left := longSwitch.X
		// The plate's own text column does not move for the data: the name, the
		// description and the summon line all start where the short render's do
		// (the row's own inset), and the identity block's left edge is that same
		// column in both renders — measured through `Alias`, the one label on the
		// plate that is this build's own words.
		if got, want := mustFind(t, long, "Alias").X, mustFind(t, short, "Alias").X; absFloat(got-want) > 0.5 {
			t.Errorf("dark=%v: the plate's label column starts at x=%v under the long name but %v under the "+
				"short one: the data is moving the layout, not filling it", dark, got, want)
		}
		nameX := mustFind(t, long, extremeCommandName()).X
		for _, arm := range []struct {
			what string
			text string
		}{
			{"name", extremeCommandName()},
			{"description", "A description that is itself long enough to need the ellipsis, yes it is"},
			{"summon line", "Summon with: c1, " + extremeCommandName()},
		} {
			r := mustFind(t, long, arm.text)
			ink, ok := rightmostInk(long, r)
			if !ok {
				t.Fatalf("dark=%v: the %s paints nothing", dark, arm.what)
			}
			if float32(ink) >= left {
				t.Errorf("dark=%v: the %s paints to x=%d, at or past the switch's own left edge %v: the line "+
					"is running under the control", dark, arm.what, ink, left)
			}
			if absFloat(r.X-nameX) > 0.5 {
				t.Errorf("dark=%v: the %s starts at x=%v, not on the identity column's own %v", dark, arm.what, r.X, nameX)
			}
		}
		// The three lines are one column: same left edge, one caption step apart.
		prev := float32(0)
		for i, text := range []string{
			extremeCommandName(),
			"A description that is itself long enough to need the ellipsis, yes it is",
			"Summon with: c1, " + extremeCommandName(),
		} {
			r := mustFind(t, long, text)
			if i == 0 {
				prev = r.Y
				continue
			}
			if pitch := r.Y - prev; absFloat(pitch-17) > 1 {
				t.Errorf("dark=%v: caption line %d sits %v under the one above it, want the caption step's "+
					"measured 17px", dark, i+1, pitch)
			}
			prev = r.Y
		}
	}
}

// TestNoInkLeavesTheCardEvenAtTheLongestName is the "narrow" arm of the triple,
// measured at the one width production draws (720) and at 560, which production
// cannot reach — kept because a rule that only holds at 720 is not a rule, it is
// a coincidence. Nothing painted by the card may cross its own frame: the card's
// right border is at width-16.2 on both (measured: 703.8 and 543.8).
func TestNoInkLeavesTheCardEvenAtTheLongestName(t *testing.T) {
	for _, width := range []int{720, 560} {
		for _, dark := range []bool{false, true} {
			a := dataApp(extremePackageName(), extremeCommandName(), extremeAlias())
			tt, _ := renderIn(t, a, width, 1600, dark)
			frame := float32(width) - 16.2
			for _, text := range tt.Texts() {
				r, ok := tt.Find(text)
				if !ok {
					continue
				}
				ink, ok := rightmostInk(tt, r)
				if !ok {
					continue
				}
				if float32(ink) > frame-0.5 {
					label := text
					if len(label) > 32 {
						label = label[:32] + "…"
					}
					t.Errorf("width=%d dark=%v: %q paints to x=%d, past the card's own right border %v",
						width, dark, label, ink, frame)
				}
			}
		}
	}
}

// dumpLongName writes the two appearances at the one real width for the round's
// report. Opt-in: `FLOTER_P13FT_DIR=/tmp/p13ft` (a plain `go test` writes
// nothing).
func TestDumpLongName(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FT_DIR")
	if dir == "" {
		t.Skip("opt-in: set FLOTER_P13FT_DIR to write the round's PNGs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, arm := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		tt, _ := renderIn(t, dataApp(extremePackageName(), extremeCommandName(), extremeAlias()), 720, 1600, arm.dark)
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("floter-P13-ft-longname-%s.png", arm.name)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
