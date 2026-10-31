package theme

import (
	"math"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		theme      string
		systemDark bool
		want       Appearance
	}{
		{"auto", false, Light},
		{"auto", true, Dark},
		{"light", false, Light},
		{"light", true, Light}, // explicit wins over the desktop
		{"dark", false, Dark},
		{"dark", true, Dark},
		{"", false, Dark}, // App.tsx paints anything else dark
		{"blue", true, Dark},
	}
	for _, tc := range cases {
		if got := Resolve(tc.theme, tc.systemDark); got != tc.want {
			t.Errorf("Resolve(%q, %v) = %q, want %q", tc.theme, tc.systemDark, got, tc.want)
		}
	}
}

func TestForScalesByTheInterfaceStep(t *testing.T) {
	base := settings.Normalize(settings.Settings{
		Theme:           "light",
		GlassStep:       settings.DefaultGlassStep,
		Language:        settings.DefaultLanguage,
		MainOpacity:     settings.DefaultMainOpacity,
		TerminalOpacity: settings.DefaultTerminalOpacity,
		UIScale:         "default",
	})
	one := For(base, false)

	if one.Appearance != Light {
		t.Errorf("appearance = %q, want light", one.Appearance)
	}
	if one.Scale != 1 {
		t.Errorf("scale = %v, want 1", one.Scale)
	}
	if one.Theme.Dark {
		t.Errorf("theme is dark, want light")
	}

	small := For(settings.Settings{Theme: "auto", UIScale: "small"}, false)
	if small.Scale != 0.9 {
		t.Errorf("small scale = %v, want 0.9", small.Scale)
	}
	if got, want := small.Theme.FontSize, round1(one.Theme.FontSize*0.9); got != want {
		t.Errorf("small font = %v, want %v", got, want)
	}
	if got, want := small.RadiusMD, float32(RadiusMD); got != want {
		t.Errorf("small radius md = %v, want the ladder's literal %v", got, want)
	}

	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Appearance != Dark {
		t.Errorf("large appearance = %q, want dark", large.Appearance)
	}
	if got, want := large.RadiusLG, float32(RadiusLG); got != want {
		t.Errorf("large radius lg = %v, want the ladder's literal %v", got, want)
	}
	// An unknown step lands on the shipped default's factor, never zero.
	if got := For(settings.Settings{Theme: "auto", UIScale: "huge"}, false).Scale; got != 0.9 {
		t.Errorf("unknown step scale = %v, want the shipped 0.9", got)
	}
}

// base.css writes the radius ladder in literal px and keeps it out of
// `--ui-scale`: the window and the card keep 14/12/9/6 at every interface step,
// unlike the `--u` metrics around them (base.css:93-96). The port used to
// multiply the ladder by the step, so a `small` chip was 8.1px and a `large` one
// 9.9px instead of the reference's 9. The framework's own control radius reads
// the same `--radius-sm`, so it is literal too.
func TestTheRadiusLadderIsLiteralAtEveryInterfaceStep(t *testing.T) {
	want := struct{ xs, sm, md, lg float32 }{RadiusXS, RadiusSM, RadiusMD, RadiusLG}
	for _, step := range []string{"small", "default", "large", "huge"} {
		tokens := For(settings.Settings{Theme: "dark", UIScale: step}, true)
		if tokens.RadiusXS != want.xs || tokens.RadiusSM != want.sm ||
			tokens.RadiusMD != want.md || tokens.RadiusLG != want.lg {
			t.Errorf("[%s] radius ladder = %v/%v/%v/%v, want the literal %v/%v/%v/%v",
				step, tokens.RadiusXS, tokens.RadiusSM, tokens.RadiusMD, tokens.RadiusLG,
				want.xs, want.sm, want.md, want.lg)
		}
		if tokens.Theme.Radius != RadiusSM {
			t.Errorf("[%s] the framework control radius = %v, want the ladder's literal %v",
				step, tokens.Theme.Radius, RadiusSM)
		}
	}
}

func TestForDoesNotMutateTheSharedTheme(t *testing.T) {
	// Hold the value the framework handed out, then check For scaled only
	// its own copy of it.
	shared := ui.LightTheme()
	_ = For(settings.Settings{Theme: "light", UIScale: "large"}, false)
	if *shared != *ui.LightTheme() {
		t.Errorf("For mutated the shared light theme: %+v", *shared)
	}
}

// The strokes are the two appearances' edge colors, per appearance, as
// base.css wrote them: a lit hairline in the dark, a dark one in the light.
func TestStrokes(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.Appearance != Dark {
		t.Fatalf("appearance = %v", dark.Appearance)
	}
	if dark.InputStroke.A >= 255 {
		t.Errorf("dark stroke = %v, want translucent", dark.InputStroke)
	}
	light := For(settings.Settings{Theme: "light"}, false)
	if light.Appearance != Light {
		t.Fatalf("appearance = %v", light.Appearance)
	}
	if light.InputStroke.R != 0 {
		t.Errorf("light stroke = %v, want the dark-on-light one", light.InputStroke)
	}
	// The focused step is the same hairline one rung brighter. P11-z 重定基线：
	// 它曾经是 accent 的 0.55（`ui.RGBA(229,72,77,0.55)` / `ui.RGBA(245,84,78,0.55)`），
	// 现在只靠明度差（0.09 → 0.16），见 TestTheFocusedCardEdgeIsNeutral。
	if dark.InputStrokeActive == dark.InputStroke {
		t.Error("the active stroke is the idle one")
	}
}

// P11-z（用户实测 F2「整体红色线条太扎眼」）· The focused card edge is a
// **neutral** hairline, not the accent.
//
// `InputStrokeActive` is what the launcher card's border and (before P11-z) the
// terminal canvas's border painted while the window had the keyboard. It used
// to be the accent at 0.55 — a constant red outline around the whole window,
// which answers 「这个窗口有焦点」 rather than §5's 「哪个被选中 / 哪个是开的」.
// The focused rung is now the resting hairline one step brighter: same hue
// (none), one more level of ink.
//
// The pin is the negative one the user's report asks for: two appearances, the
// channels may not disagree (no hue), and the rung must still be *distinguishable*
// from the resting one (the focus cue is the brightness, so it may not collapse).
func TestTheFocusedCardEdgeIsNeutral(t *testing.T) {
	spread := func(c ui.Color) int {
		d := int(c.R) - int(c.G)
		if d < 0 {
			d = -d
		}
		for _, pair := range [][2]uint8{{c.G, c.B}, {c.R, c.B}} {
			if x := int(pair[0]) - int(pair[1]); x < 0 {
				if -x > d {
					d = -x
				}
			} else if x > d {
				d = x
			}
		}
		return d
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tk := For(settings.Settings{Theme: appearance}, dark)
		active := tk.InputStrokeActive
		if s := spread(active); s > 8 {
			t.Errorf("dark=%v: the focused edge %v has channels %d levels apart: it carries a hue, "+
				"and §5 lets the accent answer only 「哪个被选中 / 哪个是开的」", dark, active, s)
		}
		if active.A == tk.InputStroke.A {
			t.Errorf("dark=%v: the focused edge is the resting one (%v): the focus cue is the brightness", dark, active)
		}
		// The resting rung is the neutral one too, so a red active rung cannot
		// hide behind a red resting one.
		rest := tk.InputStroke
		if s := spread(rest); s > 8 {
			t.Errorf("dark=%v: the resting edge %v is not neutral either (%d)", dark, rest, s)
		}
	}
}

// The settings content pane carries its own fill, and it rides the
// transparency slider as a single sheet: the pane is painted at the alpha that
// composites to the frame tint plus a small recess (glassmap.Spec.ContentAlpha),
// so the visible bottom is the frame the slider moves, not a second full tint
// stacked on it. The reference pinned `--surface-sunken` to a fixed 0.62 + 0.18
// band instead, which stayed ~0.64-0.80 no matter where the slider was and hid
// the frame tint it sat on - the user read that as "the bottom is opaque and the
// slider does nothing".
func TestSunkenFollowsTheOpacity(t *testing.T) {
	dim := settings.Settings{Theme: "dark", MainOpacity: 10}
	bright := settings.Settings{Theme: "dark", MainOpacity: 100}
	lowT := For(dim, false)
	highT := For(bright, false)
	if lowT.Sunken.A >= highT.Sunken.A {
		t.Errorf("the sunken fill did not rise with the slider: %v vs %v", lowT.Sunken.A, highT.Sunken.A)
	}
	// The pane is a recess over the frame, not a second sheet: at the low stop
	// its own alpha is small, because the frame underneath already carries the
	// tint the slider owns.
	if a := float64(lowT.Sunken.A) / 255; a > 0.55 {
		t.Errorf("the low stop alpha = %v, want a thin recess", a)
	}
	// At the top it reaches the near-solid face the frame itself owns.
	if a := float64(highT.Sunken.A) / 255; a < 0.97 {
		t.Errorf("the high stop alpha = %v, want the frame tint's ~0.98", a)
	}
	// A light appearance sunken fill is the light pane's own colour.
	lightT := For(settings.Settings{Theme: "light", MainOpacity: 100}, false)
	if a, b := float64(lightT.Sunken.A)/255, float64(highT.Sunken.A)/255; a != b {
		t.Errorf("the two appearances' alphas differ: %v vs %v", a, b)
	}
}

// The ladder scales once, by the step: every register at the Large step is its
// own base times 1.1, not "body plus an offset" - the offset that grew with
// every step above the shipped one.
//
// The ladder is one rung above the reference's (11/12/13/14/19), because the
// **shipped** step is `small` = 0.9: one rung up is what makes the shipped step
// draw base.css's own 10/11/12/13/17. The old ladder was the base numbers, so
// the shipped step drew the whole design at 90% (a 9pt caption, a 10.8pt
// emphasis) and every surface read cramped - 「整体 UI/UX 很 小气」.
func TestLadderScalesWithTheStep(t *testing.T) {
	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Display < 20.8 || large.Display > 21.0 {
		t.Errorf("display at Large = %v, want 19 x 1.1", large.Display)
	}
	if large.Title < 15.3 || large.Title > 15.5 {
		t.Errorf("title at Large = %v, want 14 x 1.1", large.Title)
	}
	if large.Body < 13.1 || large.Body > 13.3 {
		t.Errorf("body at Large = %v, want 12 x 1.1", large.Body)
	}
	// The shipped step: 0.9. Its registers land on base.css's own physical
	// sizes — caption 10, body 11, emphasis 12, title 13, display 17 — within
	// half a point, which is the whole point of the rung.
	small := For(settings.Settings{Theme: "dark"}, false)
	for _, reg := range []struct {
		name   string
		got    float32
		want   float64
		expect float32
	}{
		{"caption", small.Caption, 10, 9.9},
		{"body", small.Body, 11, 10.8},
		{"emphasis", small.Emphasis, 12, 11.7},
		{"title", small.Title, 13, 12.6},
		{"display", small.Display, 17, 17.1},
	} {
		if d := float64(reg.got) - reg.want; d > 0.5 || d < -0.5 {
			t.Errorf("the shipped step's %s = %v, want the reference's %v (one rung up x 0.9)",
				reg.name, reg.got, reg.want)
		}
		// And its own number, spelled out: a test that only compares against
		// the reference's sizes is satisfied by any rung that lands near it,
		// and the regression this guards is the ladder sliding back down.
		if reg.got != reg.expect {
			t.Errorf("the shipped step's %s = %v, want %v (11/12/13/14/19 x 0.9)",
				reg.name, reg.got, reg.expect)
		}
	}
	// The framework's own body size is the ladder's body register, so a widget
	// the views do not size by hand is the same size as the text beside it.
	if small.Theme.FontSize != small.Body {
		t.Errorf("the theme's font size = %v, want the body register %v", small.Theme.FontSize, small.Body)
	}
}

// The palette is the Raycast contract's, not the framework's stock one: the
// pastel accent, the translucent control ladder and the near-black card. These are
// the tokens the whole app is drawn against, so a regression here repaints
// every surface at once.
//
// P8-a 重定基线：这条测试原本钉的是旧 Tauri 版 base.css 的调色板（深色
// `#121316` / accent 蓝 `#8fb7ff`，浅色 `#fafafc` / accent 蓝 `#064aa6`）。
// 本片按 `docs/raycast-design.md` §1 换了整张调色板，所以断言的**具体数值**
// 换成合约值，断言本身（「accent 是两个具体色值、面板是近黑/近白、文字是
// 主墨、control 阶梯是半透明的」）一条没放宽。
func TestPaletteComesFromBaseCSS(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	// P11-al 重定基线（用户实测 F2）：P8-a 曾把 accent 从 base.css 的
	// `#8fb7ff` 换成「品牌红」`#F5544E`，用户实测反馈是「整体红色线条太扎眼」。
	// 参考 `main:src/styles/base.css:675` 从一开始就是 `--accent: #8fb7ff`，
	// 本仓这张表自己写着「the pastel accent」——所以旧值 → 新值回到参考：
	// `#F5544E` → `#8FB7FF`。
	if dark.Theme.Accent != ui.Hex("#8FB7FF") {
		t.Errorf("dark accent = %v, want base.css's #8FB7FF", dark.Theme.Accent)
	}
	// 旧值 #121316 → 合约 §1 的面板 #0E0E10。
	if dark.Theme.Background != ui.Hex("#0E0E10") {
		t.Errorf("dark background = %v, want the contract's panel #0E0E10", dark.Theme.Background)
	}
	// 旧值 #f2f3f5 → 合约 §1 的主文字 #EDEDEF。
	if dark.Theme.Text != ui.Hex("#EDEDEF") {
		t.Errorf("dark text = %v, want the contract's primary ink #EDEDEF", dark.Theme.Text)
	}
	// The control ladder is translucent white, not an opaque grey: the
	// framework's stock SurfaceHover would paint a solid block over the glass.
	if dark.Theme.SurfaceHover.A >= 255 {
		t.Errorf("dark hover = %v, want a translucent lift", dark.Theme.SurfaceHover)
	}
	// P11-al 重定基线：`--accent-contrast: #15171b`（base.css:676）。
	// 旧值 `#1B1B1D`（合约 §1 的 acc​ent 静默面上的字）→ 新值 `#15171B`。
	if dark.Theme.AccentText != ui.Hex("#15171B") {
		t.Errorf("dark accent text = %v, want base.css's #15171B", dark.Theme.AccentText)
	}

	light := For(settings.Settings{Theme: "light"}, false)
	// P11-al 重定基线：`--accent: #064aa6`（base.css:942）。旧值 `#E5484D` → 新值。
	if light.Theme.Accent != ui.Hex("#064AA6") {
		t.Errorf("light accent = %v, want base.css's #064AA6", light.Theme.Accent)
	}
	// 旧值 #fafafc → 合约 §1 的浅色面板 #F7F7F8。
	if light.Theme.Background != ui.Hex("#F7F7F8") {
		t.Errorf("light background = %v, want the contract's panel #F7F7F8", light.Theme.Background)
	}
	// 旧值 #1d1d1f → 合约 §1 的主文字 #1B1B1D。
	if light.Theme.Text != ui.Hex("#1B1B1D") {
		t.Errorf("light text = %v, want the contract's primary ink #1B1B1D", light.Theme.Text)
	}
	// The halo is the accent at a fixed low alpha — the live status dot's ring,
	// not a second accent fill. 旧值 dark {143,183,255,.18} / light
	// {6,74,166,.24}，按合约 accent 换色、alpha 取 §1 的 0.20。
	if want := ui.RGBA(143, 183, 255, 0.18); dark.AccentHalo != want {
		t.Errorf("dark accent halo = %v, want %v", dark.AccentHalo, want)
	}
	if want := ui.RGBA(6, 74, 166, 0.24); light.AccentHalo != want {
		t.Errorf("light accent halo = %v, want %v", light.AccentHalo, want)
	}
}

// P8-a 重定基线：这条测试原本钉 base.css 的**四级**表面梯
// `stroke > controlEdge > hairline > control`。Raycast 合约 §1 写的是另一种
// 结构——两条发丝（强 0.09 = 卡的 1px 边，弱 0.055 = 行与块内的分隔线）加一条
// 中性的行态梯（hover 0.045 / 选中 0.085 / 按下 0.11）——所以这里的**结构**变了：
// 原来四级各自严格递减，现在强发丝 > 弱发丝，且行态三档逐级递增。逐条具体数值
// 仍然写死（没有一处加容差、没有一处放松成「非空」），断言的密度反而更高了。
//
// The surface ladder is ordered and visible: the card's strong hairline over
// the weak rule, the weak rule over the resting plane, and the row's own three
// state steps rising hover → selected → pressed.
//
// The rungs were all under 0.08 alpha, which over a translucent dark frame is
// one or two levels of grey: the ladder existed in the token file and nowhere
// on screen, so a card was a card only because of its shadow (「整体 UI/UX 很
// 小气」). These assertions are the numbers, rung by rung, plus the order: a
// revert to the flat ladder fails both the exact values and the ordering, and
// an over-correction into a border-on-every-box design fails the ceiling
// against the contrast preference's own tokens.
func TestTheSurfaceLadderHasAStepToSee(t *testing.T) {
	alpha := func(c ui.Color) float32 { return float32(c.A) / 255 }
	cases := []struct {
		name         string
		systemDark   bool
		stroke       ui.Color
		controlEdge  ui.Color
		hairline     ui.Color
		hairlineSoft ui.Color
		control      ui.Color
		controlHover ui.Color
		controlPress ui.Color
		track        ui.Color
	}{
		// P8-a 重定基线：合约 §1 只给「强发丝 0.09 / 弱发丝 0.055」，卡的 1px
		// 边就是强发丝——旧 base.css 的浅色 stroke 0.12、深色 0.11 都收进 0.09。
		// 阶梯的排序断言（stroke > controlEdge > hairline > control）随之失效
		// 在两个等值上，所以下面改成读「强发丝 > 弱发丝」这条合约真正写下的
		// 台阶，删掉的是**旧设计的第二级**，不是断言本身。
		// P12-a 重定基线（docs/liquid-glass.md §1）：面的台阶抬到一眼看得见。
		// 旧值 → 新值：control `0.05` → `0.07`、controlHover `0.075` → `0.105`。
		// 旧的 0.05 压在卡的 sheen 上只差 3~4 个亮度级，用户读到的就是「所有东西
		// 挤在同一个 z 平面上」；抬到 0.07 之后「面」与「底」在 1x 屏上差 8~11 级。
		{
			name: "light", systemDark: false,
			stroke:       ui.RGBA(0, 0, 0, 0.09),
			controlEdge:  ui.RGBA(0, 0, 0, 0.06),
			hairline:     ui.RGBA(0, 0, 0, 0.09),
			hairlineSoft: ui.RGBA(0, 0, 0, 0.055),
			control:      ui.RGBA(0, 0, 0, 0.07),
			controlHover: ui.RGBA(0, 0, 0, 0.105),
			controlPress: ui.RGBA(0, 0, 0, 0.032),
			track:        ui.RGBA(0, 0, 0, 0.06),
		},
		{
			name: "dark", systemDark: true,
			stroke:       ui.RGBA(255, 255, 255, 0.09),
			controlEdge:  ui.RGBA(255, 255, 255, 0.07),
			hairline:     ui.RGBA(255, 255, 255, 0.09),
			hairlineSoft: ui.RGBA(255, 255, 255, 0.055),
			control:      ui.RGBA(255, 255, 255, 0.10),
			controlHover: ui.RGBA(255, 255, 255, 0.14),
			controlPress: ui.RGBA(255, 255, 255, 0.05),
			track:        ui.RGBA(255, 255, 255, 0.09),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tk := ForPreferences(settings.Settings{Theme: "auto", GlassStep: "regular"}, tc.systemDark, ui.Preferences{})
			rungs := []struct {
				name string
				got  ui.Color
				want ui.Color
			}{
				{"InputStroke", tk.InputStroke, tc.stroke},
				{"ControlEdge", tk.ControlEdge, tc.controlEdge},
				{"Hairline", tk.Hairline, tc.hairline},
				{"HairlineSoft", tk.HairlineSoft, tc.hairlineSoft},
				{"Control", tk.Control, tc.control},
				{"ControlHover", tk.ControlHover, tc.controlHover},
				{"ControlPress", tk.ControlPress, tc.controlPress},
				{"Track", tk.Track, tc.track},
			}
			for _, rung := range rungs {
				if rung.got != rung.want {
					t.Errorf("%s = %v, want %v", rung.name, rung.got, rung.want)
				}
			}
			// The order the contract writes down (`docs/raycast-design.md` §1):
			// the strong hairline (the card's 1px edge) over the weak one (the
			// rule between rows and inside a block), and that rule over the
			// rest of the quiet ladder; hover lifts a pane and a press sinks it
			// below rest.
			//
			// P8-a 重定基线：旧断言是 base.css 的四级
			// `stroke > controlEdge > hairline > control`。合约只给**两**条
			// 发丝——强 0.09 / 弱 0.055——而且写明「卡的边与分区线是同一条发丝
			// 的两种宽度」，所以 controlEdge 不再是发丝之间的一级，两条发丝
			// 之间的台阶也换成了「强 > 弱」。松掉的是旧设计的第 1.5 级，留下的
			// 仍然是逐级严格不等（没有一处相等）。
			if alpha(tc.hairline) != alpha(tc.stroke) {
				t.Errorf("the card's hairline %.3f is not the strong rung %.3f",
					alpha(tc.hairline), alpha(tc.stroke))
			}
			if alpha(tc.stroke) <= alpha(tc.hairlineSoft) {
				t.Errorf("the strong hairline %.3f is not above the weak one %.3f",
					alpha(tc.stroke), alpha(tc.hairlineSoft))
			}
			// 弱发丝是合约的第二条发丝，且它必须在静止平面之上（一条画不出来的
			// 分隔线不是分隔线）。controlEdge 是**键帽/控件自己的边**，与发丝同
			// 属 0.06~0.09 的一段，合约没有给它排进发丝梯，所以只断言它可见。
			soft := tk.HairlineSoft
			if alpha(soft) >= alpha(tc.hairline) {
				t.Errorf("the weak hairline %.3f is not under the strong one %.3f",
					alpha(soft), alpha(tc.hairline))
			}
			// 弱发丝要是一条 1x 屏画得出来的线（合约的 0.055 是这一步的下界）。
			if alpha(soft) < 0.05 {
				t.Errorf("the weak hairline %.3f is below the step a 1x display resolves", alpha(soft))
			}
			// 行态三档（合约 §1）：hover < 选中 < 按下，逐级严格递增。
			row := []struct {
				name string
				a    float32
			}{
				{"hover", alpha(tk.RowHover)},
				{"selected", alpha(tk.RowSelected)},
				{"pressed", alpha(tk.RowPressed)},
			}
			for i := 1; i < len(row); i++ {
				if row[i].a <= row[i-1].a {
					t.Errorf("the row ladder is flat: %s %.3f is not above %s %.3f",
						row[i].name, row[i].a, row[i-1].name, row[i-1].a)
				}
			}
			if alpha(tc.controlHover) <= alpha(tc.control) || alpha(tc.controlPress) >= alpha(tc.control) {
				t.Errorf("the pointer ladder is flat: hover %.3f, rest %.3f, press %.3f",
					alpha(tc.controlHover), alpha(tc.control), alpha(tc.controlPress))
			}
			// A control plane is a plane, not a hairline: it has to be a step a
			// 1x display resolves over the frame it sits on.
			//
			// P12-a 重定基线：下界随「面」这一档一起抬（0.045 → 0.06）。0.045 是
			// 旧「面」档 0.05 的下沿，而新面档是 0.07/0.10，门槛必须跟它走，否则
			// 这条断言守的是旧设计的地板，不是新设计的。
			if alpha(tc.control) < 0.06 {
				t.Errorf("a control plane at %.3f is below the step a 1x display resolves", alpha(tc.control))
			}
			// And the ladder stays light-handed: the desktop that asked for more
			// contrast must still get a visibly stronger edge, or this rung has
			// eaten the preference.
			high := ForPreferences(settings.Settings{Theme: "auto", GlassStep: "regular"}, tc.systemDark, ui.Preferences{HighContrast: true})
			if alpha(high.InputStroke) <= 2*alpha(tc.stroke) {
				t.Errorf("the contrast stroke %.3f is not clearly above this rung's %.3f", alpha(high.InputStroke), alpha(tc.stroke))
			}
		})
	}
}

// The sidebar's chosen page is the opaque raised rung: the framework paints
// the accent behind a chosen list row, so a translucent lift would read as a
// blue pill, which is exactly what the reference's quiet pill is not.
func TestRaisedQuietIsOpaqueAndNeutral(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    Tokens
	}{
		{"dark", For(settings.Settings{Theme: "dark"}, true)},
		{"light", For(settings.Settings{Theme: "light"}, false)},
	} {
		if tc.t.RaisedQuiet.A != 255 {
			t.Errorf("%s raised-quiet alpha = %d, want opaque", tc.name, tc.t.RaisedQuiet.A)
		}
		// A neutral lift over a near-neutral pane keeps R, G and B within a
		// few steps of each other; an accent-tinted pill would not.
		r, g, b := int(tc.t.RaisedQuiet.R), int(tc.t.RaisedQuiet.G), int(tc.t.RaisedQuiet.B)
		if spread := max(r, g, b) - min(r, g, b); spread > 12 {
			t.Errorf("%s raised-quiet = %v, want a neutral lift", tc.name, tc.t.RaisedQuiet)
		}
	}
}

// The control lens is three inset layers plus the pane edge: a control that
// carries it reads as a lens rather than as a flat tint with a border.
func TestLensTokensAreTranslucent(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.LensRim.A == 0 || dark.LensRim.A >= 255 {
		t.Errorf("lens rim = %v, want a translucent highlight", dark.LensRim)
	}
	if dark.LensEdge.R != 0 {
		t.Errorf("lens edge = %v, want a dark bottom shade", dark.LensEdge)
	}
}

// The glass stop scales the control lens (base.css's `--glass-lens-scale`:
// 0.5 / 1 / 1.6). It is the one rung of the old build's effect table the
// scene can paint on every platform, so it is what makes `frosted`, `regular`
// and `liquid` read apart even where the window has no material to blur the
// desktop with.
func TestLensScalesWithTheGlassStop(t *testing.T) {
	base := settings.Settings{Theme: "dark"}
	frosted := For(withStep(base, "frosted"), true)
	regular := For(withStep(base, "regular"), true)
	liquid := For(withStep(base, "liquid"), true)
	if !(frosted.LensRim.A < regular.LensRim.A && regular.LensRim.A < liquid.LensRim.A) {
		t.Errorf("lens rim did not rise with the stop: %d / %d / %d", frosted.LensRim.A, regular.LensRim.A, liquid.LensRim.A)
	}
	if !(frosted.LensSheen.A < regular.LensSheen.A && regular.LensSheen.A < liquid.LensSheen.A) {
		t.Errorf("lens sheen did not rise with the stop: %d / %d / %d", frosted.LensSheen.A, regular.LensSheen.A, liquid.LensSheen.A)
	}
	// P13-o: the groove's lit rim rides the same chain (`--glass-lens-scale`
	// grades the recessed lens exactly as it grades the raised one).
	if !(frosted.TrackRim.A < regular.TrackRim.A && regular.TrackRim.A < liquid.TrackRim.A) {
		t.Errorf("the groove's lit rim did not rise with the stop: %d / %d / %d", frosted.TrackRim.A, regular.TrackRim.A, liquid.TrackRim.A)
	}
	// The regular stop is the reference's own lens strength: the palette's
	// base value unchanged.
	palette := paletteFor(Dark)
	if regular.LensRim.A != palette.lensRim.A {
		t.Errorf("regular lens rim = %d, want the palette's %d", regular.LensRim.A, palette.lensRim.A)
	}
	// The off stop drops the lens entirely.
	if off := For(withStep(base, "off"), true); off.LensRim.A != 0 || off.LensSheen.A != 0 || off.LensEdge.A != 0 || off.TrackRim.A != 0 {
		t.Errorf("off kept a lens: %v / %v / %v / %v", off.LensRim, off.LensSheen, off.LensEdge, off.TrackRim)
	}
}

// withStep is s with its glass stop set, for the lens tests.
func withStep(s settings.Settings, step string) settings.Settings {
	s.GlassStep = step
	return s
}

// base.css's `--sheen-top` / `--sheen-bottom` are fixed tokens per appearance
// (:628-629 dark 0.046/0.01, :906-907 light 0.66/0.22) and do **not** vary
// with `[data-glass]`: the only token a stop moves is `--glass-lens-scale`,
// and that grades the **control** lens (TestLensScalesWithTheGlassStop), not
// the frame's lit face. An earlier rewrite graded the sheen by the stop to make
// the stops visible where the window has no material, which is exactly the
// deviation F3b removes; the off stop drops it with the material.
//
// The stop *does* reach the painted sheen through one number: `capAlpha` holds
// the sheen at or under the frame's own alpha (`spec.TintAlpha`), which is per
// stop **and** per platform (a macOS window's material is compensated out of the
// tint, so its frame is thinner than Linux's). Where the cap binds — the light
// appearance's 0.66 token always, the dark one's 0.046 only on a very thin
// frame — the three stops therefore read apart, and that is the cap, not a
// graded token. So the invariant is asserted per stop: each stop's sheen is the
// palette's fixed token capped at *that stop's* frame alpha, never the token
// scaled by the stop's own lens rung.
func TestTheFrameSheenIsFixedAcrossStops(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		base := settings.Settings{Theme: appearance, MainOpacity: settings.DefaultMainOpacity}
		palette := paletteFor(Appearance(appearance))
		var tokens []Tokens
		for _, step := range []string{"frosted", "regular", "liquid"} {
			got := For(withStep(base, step), true)
			tokens = append(tokens, got)
			spec := glassmap.SpecForOS(runtime.GOOS, step, settings.DefaultMainOpacity)
			wantTop := capAlpha(palette.sheenTop, spec.TintAlpha)
			wantBottom := capAlpha(palette.sheenBottom, spec.TintAlpha)
			if got.SheenTop != wantTop || got.SheenBottom != wantBottom {
				t.Errorf("%s/%s: sheen = %v / %v, want the palette's fixed token capped at the frame's %v / %v",
					appearance, step, got.SheenTop, got.SheenBottom, wantTop, wantBottom)
			}
			// The sheen never makes the frame denser than the slider asked for.
			if float32(got.SheenTop.A)/255 > spec.TintAlpha+0.004 || float32(got.SheenBottom.A)/255 > spec.TintAlpha+0.004 {
				t.Errorf("%s/%s: the sheen is heavier than the frame: %v / %v over %v",
					appearance, step, got.SheenTop, got.SheenBottom, spec.TintAlpha)
			}
		}
		// Where the cap is inert on all three stops the lit face is one token,
		// i.e. the stop graded nothing at all.
		if tokens[0].SheenTop.A == palette.sheenTop.A && tokens[1].SheenTop.A == palette.sheenTop.A && tokens[2].SheenTop.A == palette.sheenTop.A {
			for _, got := range tokens[1:] {
				if got.SheenTop != tokens[0].SheenTop || got.SheenBottom != tokens[0].SheenBottom {
					t.Errorf("%s: the stop graded an uncapped sheen: %v / %v", appearance, got.SheenTop, got.SheenBottom)
				}
			}
		}
		// The off stop has no glass, so it paints no sheen at all.
		if off := For(withStep(base, "off"), true); off.SheenTop.A != 0 || off.SheenBottom.A != 0 {
			t.Errorf("%s: the off stop kept a sheen: %v / %v", appearance, off.SheenTop, off.SheenBottom)
		}
	}
}

// The sheen is a fixed token, not the frame's tint scaled: where the tint is
// thicker than the dark appearance's own 0.046 the cap is inert and the pixel is
// the reference's fixed alpha, while the light appearance's 0.66 is always
// capped to the thin frame. Either way the frame's alpha stays the slider's —
// the `FaceFill` compensation is the other half of that promise.
//
// The reading is taken at the shipped default rather than at the slider's
// bottom: a macOS window's material is compensated out of the tint
// (glassmap.MaterialVeil), so at the bottom end the fill can be 0 there and the
// cap would bite on every platform's token, which says nothing about the token.
func TestTheFrameSheenIsTheFixedTokenNotTheTint(t *testing.T) {
	at := func(appearance string, opacity uint8) Tokens {
		return For(settings.Settings{Theme: appearance, MainOpacity: opacity}, true)
	}
	// The dark token is under the tint at the shipped default on every
	// platform, so the cap is inert and the pixel is the reference's own fixed
	// alpha.
	if got, want := at("dark", settings.DefaultMainOpacity).SheenTop, paletteFor(Dark).sheenTop; got != want {
		t.Errorf("the dark sheen = %v, want the fixed token %v", got, want)
	}
	// The light token is the same kind of number.
	//
	// P8-a 重定基线：旧 base.css 的浅色 `--sheen-top` 是 0.66，比任何一档窗框
	// 都厚，所以这条断言原本读的是「cap 咬住了 token」。合约 §4.1 把**玻璃
	// 高光/透镜上限压到 0.04**（Raycast 是低光泽设计），浅色 token 因此变成
	// 0.04，比每一档窗框都薄 —— cap 在任何档位都不咬，于是「cap 咬住了」这句
	// 话失去了主语。改钉合约真正写下的两件事，且都是**等值**断言：每个档位、
	// 每个透明度下，画出来的 sheen 都严格等于 `capAlpha(固定 token, 该档窗框)`；
	// 且 token 本身就在 0.04 这个上限之内。
	light := paletteFor(Light)
	if light.sheenTop.A > 10 {
		t.Errorf("the light sheen token = %d/255, want at most the contract's 0.04 cap (10/255)", light.sheenTop.A)
	}
	for _, opacity := range []uint8{settings.MinWindowOpacity, settings.DefaultMainOpacity, 100} {
		for _, step := range []string{"frosted", "regular", "liquid"} {
			spec := glassmap.SpecForOS(runtime.GOOS, step, opacity)
			want := capAlpha(light.sheenTop, spec.TintAlpha)
			got := at("light", opacity).SheenTop
			if got != want {
				t.Errorf("the light sheen at %s/%d%% = %v, want the capped fixed token %v",
					step, opacity, got, want)
			}
		}
	}
}

// A desktop that asked for more contrast gets base.css's
// `@media (prefers-contrast: more)` tokens: the card stroke, the grouped
// row's inner edge and the field placeholder are swapped per appearance. The
// rest of the palette is untouched, which is what makes the preference a
// change of *contrast* rather than a second look.
func TestHighContrastSwapsTheStrokeAndPlaceholderTokens(t *testing.T) {
	// P11-aa 重定基线：旧值 = 只动三条（stroke / controlEdge / textMuted）。用户实测
	// F2 的可读性半边要求「字也要够读」，所以二级与三级墨同样进这支：浅 `#6B6B70` →
	// `#4E4E54`、`#8E8E94` → `#66666C`；深 `#9A9AA0` → `#B0B0B8`、`#6E6E76` →
	// `#8A8A92`（两个表面都 ≥ 4.5:1，见 TestEveryInkStepClearsTheReadableContrast）。
	cases := []struct {
		name        string
		systemDark  bool
		stroke      ui.Color
		soft        ui.Color
		placeholder ui.Color
		secondary   ui.Color
		tertiary    ui.Color
	}{
		{
			name: "light", systemDark: false,
			stroke:      ui.RGBA(0, 0, 0, 0.5),
			soft:        ui.RGBA(0, 0, 0, 0.36),
			placeholder: ui.RGBA(28, 28, 32, 0.68),
			secondary:   ui.Hex("#4E4E54"),
			tertiary:    ui.Hex("#66666C"),
		},
		{
			name: "dark", systemDark: true,
			stroke:      ui.RGBA(255, 255, 255, 0.42),
			soft:        ui.RGBA(255, 255, 255, 0.34),
			placeholder: ui.RGBA(242, 243, 245, 0.58),
			secondary:   ui.Hex("#B0B0B8"),
			tertiary:    ui.Hex("#8A8A92"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			plain := ForPreferences(s, tc.systemDark, ui.Preferences{})
			high := ForPreferences(s, tc.systemDark, ui.Preferences{HighContrast: true})

			if high.InputStroke != tc.stroke {
				t.Errorf("InputStroke = %v, want %v", high.InputStroke, tc.stroke)
			}
			if high.ControlEdge != tc.soft {
				t.Errorf("ControlEdge = %v, want %v", high.ControlEdge, tc.soft)
			}
			if high.Theme.TextMuted != tc.placeholder {
				t.Errorf("theme TextMuted = %v, want the placeholder contrast %v", high.Theme.TextMuted, tc.placeholder)
			}
			if high.TextSecondary != tc.secondary {
				t.Errorf("TextSecondary = %v, want the contrast ink %v", high.TextSecondary, tc.secondary)
			}
			if high.TextTertiary != tc.tertiary {
				t.Errorf("TextTertiary = %v, want the contrast ink %v", high.TextTertiary, tc.tertiary)
			}
			// The framework theme the widgets read is the same palette: its
			// Border is the card's stroke, so a button or a field the views
			// do not paint by hand gets the stronger edge too.
			if high.Theme.Border != tc.stroke {
				t.Errorf("theme Border = %v, want the contrast stroke %v", high.Theme.Border, tc.stroke)
			}
			// The preference has to change something, or it is an inert knob.
			if high.InputStroke == plain.InputStroke && high.ControlEdge == plain.ControlEdge && high.Theme.TextMuted == plain.Theme.TextMuted {
				t.Error("more contrast changed no token")
			}

			// Nothing else moved: blank the three contrast tokens, the
			// pointer and the three fields the material floor moves, and the
			// two resolutions have to be the same value.
			blank := func(tk Tokens) Tokens {
				tk.Theme = nil
				tk.InputStroke, tk.ControlEdge = ui.Color{}, ui.Color{}
				// P11-aa：两档安静墨也是高对比度**允许**移动的（这是本轮的正题）。
				tk.TextSecondary, tk.TextTertiary = ui.Color{}, ui.Color{}
				// The three the material floor and the lens damping move.
				tk.Sunken, tk.RaisedQuiet, tk.Float = ui.Color{}, ui.Color{}, ui.Color{}
				tk.LensRim, tk.LensSheen, tk.LensEdge = ui.Color{}, ui.Color{}, ui.Color{}
				tk.TrackRim = ui.Color{}
				tk.SheenTop, tk.SheenBottom = ui.Color{}, ui.Color{}
				// The focused pair is capped at the same frame alpha, so the
				// material floor moves it too (see SheenAtFocus).
				tk.SheenTopActive, tk.SheenBottomActive = ui.Color{}, ui.Color{}
				return tk
			}
			if got, want := blank(high), blank(plain); got != want {
				t.Errorf("contrast moved tokens besides the three:\n got %+v\nwant %+v", got, want)
			}
			// The material floor rides through: the content pane and a
			// floating pane are painted at the raised frame's own alpha.
			if high.Sunken.A <= plain.Sunken.A {
				t.Errorf("the content pane did not follow the raised floor: %d vs %d", high.Sunken.A, plain.Sunken.A)
			}
			if high.Float.A <= plain.Float.A {
				t.Errorf("a floating pane did not follow the raised floor: %d vs %d", high.Float.A, plain.Float.A)
			}
		})
	}
}

// IconSurface is the warm amber plane (base.css's `--system-icon-surface`),
// not the warning ink at a low alpha: quoting the label's colour here painted
// the launcher feedback row pink-red. The two tokens must be measurably
// different in both appearances.
func TestIconSurfaceIsTheWarmPlaneNotTheWarningInk(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
		wantHover  ui.Color
	}{
		{"dark", true, ui.RGBA(255, 176, 132, 0.115), ui.RGBA(255, 176, 132, 0.2)},
		{"light", false, ui.RGBA(214, 118, 44, 0.13), ui.RGBA(214, 118, 44, 0.21)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto"}
			tokens := ForPreferences(s, tc.systemDark, ui.Preferences{})
			if tokens.IconSurface != tc.want {
				t.Errorf("IconSurface = %v, want %v", tokens.IconSurface, tc.want)
			}
			if tokens.IconSurfaceHover != tc.wantHover {
				t.Errorf("IconSurfaceHover = %v, want %v", tokens.IconSurfaceHover, tc.wantHover)
			}
			// The plane must not equal the warning ink: the launcher feedback
			// row quotes this token as its fill, and the warning colour is
			// reserved for the label.
			if tokens.IconSurface == tokens.Theme.Warning {
				t.Error("IconSurface equals Warning: the feedback row would read pink-red instead of warm amber")
			}
		})
	}
}

// RaisedWarm is base.css's `--glass-raised-warm`, an alias of
// `--system-icon-surface` (`base.css:611`): the warm raised pane a broken
// integration row paints. It is the same amber plane as IconSurface, not the
// accent and not the warning ink, and unlike the frame's tint it does not
// follow the transparency slider.
func TestRaisedWarmIsTheWarmPlane(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		{"dark", true, ui.RGBA(255, 176, 132, 0.115)},
		{"light", false, ui.RGBA(214, 118, 44, 0.13)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.RaisedWarm != tc.want {
				t.Errorf("RaisedWarm = %v, want the reference's --glass-raised-warm %v", tokens.RaisedWarm, tc.want)
			}
			if tokens.RaisedWarm != tokens.IconSurface {
				t.Errorf("RaisedWarm %v is not the --system-icon-surface alias %v", tokens.RaisedWarm, tokens.IconSurface)
			}
			if tokens.RaisedWarm == tokens.Theme.Warning {
				t.Error("RaisedWarm equals the warning ink; the broken row's fill would read pink-red")
			}
			if tokens.RaisedWarm == tokens.Theme.Accent {
				t.Error("RaisedWarm equals the accent; a broken row would read as an action")
			}
		})
	}
}

// AccentTintQuiet is the accent as a *quiet fill* — the one rung of the
// accent-tint family the contract still allows a face to wear.
//
// P8-a 重定基线：这条测试原本钉的是 base.css 的 `--accent-tint-quiet`
// （深色 `rgba(143,183,255,0.085)` / 浅色 `rgba(6,74,166,0.065)`），那时它是
// **启动器选中行**的填充。合约 §1 把启动器选中行改成中性填充（`RowSelected`），
// 并把 accent 收窄到三处：设置侧栏选中项的前导图标、主 CTA 填充、焦点环；
// `AccentTintQuiet` 的角色随之变成这三处用的「静默 accent 面」，数值取自合约
// §1 的「accent 静默 0.14 / 0.10」。测试名不变，断言的形状也不变：两个外观各
// 一个**具体**色值 + alpha，并且必须是 accent 自己的色相。
func TestAccentTintQuietStepsDownInTheLightAppearance(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		// P11-al 重定基线：回到 base.css:686 的 `--accent-tint-quiet`
		// `rgba(143,183,255,0.085)`。旧（品牌红 0.14）→ 新。
		{"dark", true, ui.RGBA(143, 183, 255, 0.085)},
		// `rgba(6,74,166,0.065)`（base.css:948）；旧（品牌红 0.10）→ 新。
		{"light", false, ui.RGBA(6, 74, 166, 0.065)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.AccentTintQuiet != tc.want {
				t.Errorf("AccentTintQuiet = %v, want the reference's --accent-tint-quiet %v", tokens.AccentTintQuiet, tc.want)
			}
			// It is the accent's own hue at a quieter alpha, not a second
			// colour: the two must share R/G/B and differ only in alpha.
			if tokens.AccentTintQuiet.R != tokens.Theme.Accent.R ||
				tokens.AccentTintQuiet.G != tokens.Theme.Accent.G ||
				tokens.AccentTintQuiet.B != tokens.Theme.Accent.B {
				t.Errorf("AccentTintQuiet %v is a different hue from the accent %v", tokens.AccentTintQuiet, tokens.Theme.Accent)
			}
		})
	}
}

// SystemIconText is the ink a warm system-icon plane carries (base.css's
// `--system-icon-text`): the amber ink the launcher's power rows draw their
// glyph in. It is a token of its own, not the warning colour, and it differs
// by appearance — a pale warm peach on dark, a burnt orange on light.
func TestSystemIconTextIsItsOwnAmberInkNotTheWarningColour(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
		want       ui.Color
	}{
		{"dark", true, ui.RGBA(255, 209, 178, 0.82)},
		{"light", false, ui.Hex("#a24d15")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := ForPreferences(settings.Settings{Theme: "auto"}, tc.systemDark, ui.Preferences{})
			if tokens.SystemIconText != tc.want {
				t.Errorf("SystemIconText = %v, want the reference's --system-icon-text %v", tokens.SystemIconText, tc.want)
			}
			if tokens.SystemIconText == tokens.Theme.Warning {
				t.Error("SystemIconText equals the warning ink: the power glyph would wear the label's red-grey")
			}
			// Warm, not neutral: the plate it sits on is amber, so its ink
			// leads red over blue on both appearances.
			if tokens.SystemIconText.R <= tokens.SystemIconText.B {
				t.Errorf("SystemIconText %v does not read warm (R=%d, B=%d)", tokens.SystemIconText, tokens.SystemIconText.R, tokens.SystemIconText.B)
			}
		})
	}
}

// The platform reaches the tokens: a macOS window's backdrop material already
// lays a veil of its own over the desktop (glassmap.MaterialVeil), so the tint
// the app paints is thinner there than on a platform with no material — while
// the alpha the user *sees* is the slider's on both. That is the whole point of
// the compensation: one slider position, one solidity, whichever material the
// window server puts behind it.
func TestThePlatformsMaterialReachesTheTokens(t *testing.T) {
	s := settings.Settings{Theme: "dark", GlassStep: "regular", MainOpacity: settings.DefaultMainOpacity}
	darwin := ForPlatform(s, true, ui.Preferences{}, "darwin")
	linux := ForPlatform(s, true, ui.Preferences{}, "linux")
	windows := ForPlatform(s, true, ui.Preferences{}, "windows")

	// The fill is thinner under a material...
	if darwin.Sunken.A >= linux.Sunken.A {
		t.Errorf("the darwin content pane = %d, want thinner than the material-free %d", darwin.Sunken.A, linux.Sunken.A)
	}
	if darwin.Float.A >= linux.Float.A {
		t.Errorf("the darwin floating pane = %d, want thinner than the material-free %d", darwin.Float.A, linux.Float.A)
	}
	// ...and the sheen is capped to that thinner fill, or `FaceFill` could not
	// composite back to the slider's own alpha.
	if darwin.SheenTop.A > linux.SheenTop.A {
		t.Errorf("the darwin sheen = %d, want at most the material-free %d", darwin.SheenTop.A, linux.SheenTop.A)
	}
	// Windows has one backdrop for every stop and this build does not
	// compensate for it yet (glassmap.MaterialVeil), so it paints the tint a
	// material-free platform does.
	if windows.Sunken != linux.Sunken {
		t.Errorf("windows content pane = %v, want the uncompensated %v", windows.Sunken, linux.Sunken)
	}
	// The visible alpha is the slider's on all three.
	for _, goos := range []string{"darwin", "windows", "linux"} {
		spec := glassmap.SpecForOS(goos, s.GlassStep, s.MainOpacity)
		if diff := spec.Composite() - float32(s.MainOpacity)/100; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("%s composite = %v, want the slider's %v", goos, spec.Composite(), float32(s.MainOpacity)/100)
		}
	}
}

// The window's own corner is one number on all three platforms.
//
// P8-a 重定基线：本文件下面这段历史说明讲的是 base.css 的 14px/8px。合约 §2
// 把窗口/卡片这一级定为 **12px**，三平台一致这条结论不变，数值从 14 换成 12。
//
// base.css carries two `--window-radius` values: 14px for the default sheet and
// 8px inside `.platform-windows` (`base.css:66,1492`). The 8px was not a taste
// decision — it followed DWM, which rounds every top-level window at its own
// ~8px preset, and a 14px card corner inside an 8px window corner leaves the
// system's frame color showing as a crescent in each corner (one of the pieces
// the user read as 「窗口周边外圈还有一圈额外的底色」).
//
// This build takes the system's rounding off the window instead
// (`DWMWCP_DONOTROUND`, see shell.dressWindowsFrame and the constants pinned in
// chrome_windows_test.go), so the window's rectangle is square and the card's
// corner is the only one on screen — and it is the design's own 14px, the figure
// macOS and Linux draw. The three answers are pinned by name here so no host's
// suite can only see its own.
func TestTheWindowCornerIsTheDesignsOnEveryPlatform(t *testing.T) {
	s := settings.Settings{}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		tokens := ForPlatform(s, true, ui.Preferences{}, goos)
		if tokens.WindowRadius != RadiusLG {
			t.Errorf("%s window radius = %v, want the design's %v", goos, tokens.WindowRadius, float32(RadiusLG))
		}
		// The rung the card takes and the rung the ladder names are the same
		// number: a card holding controls and the window are one corner here.
		if tokens.RadiusLG != RadiusLG {
			t.Errorf("%s RadiusLG = %v, want the ladder's %v", goos, tokens.RadiusLG, float32(RadiusLG))
		}
	}
	// P8-a 重定基线：旧 base.css 的 radius 梯是 6/9/12/14，窗口 rung = 14px。
	// 合约 §2 的梯是 6/8/10/12 + 键帽 5，窗口与卡片这一级 = 12px。
	if RadiusLG != 12 {
		t.Errorf("the ladder's window rung = %v, want the contract's 12px", float32(RadiusLG))
	}
	// 合约 §2 的四个梯级逐个写死，另加键帽自己的一级 5：这五个数是这一片
	// 唯一允许出现的圆角。
	for _, rung := range []struct {
		name string
		got  float32
		want float32
	}{
		{"RadiusXS (icon plate)", RadiusXS, 6},
		{"RadiusSM (row, button)", RadiusSM, 8},
		{"RadiusMD (block in a panel)", RadiusMD, 10},
		{"RadiusLG (window, card)", RadiusLG, 12},
		{"KeycapRadius", KeycapRadius, 5},
	} {
		if rung.got != rung.want {
			t.Errorf("%s = %v, want the contract's %v", rung.name, rung.got, rung.want)
		}
	}
}

// inkStep is one rung of the text ladder, top to bottom, as a name and the ink
// it is painted in.
type inkStep struct {
	name string
	ink  ui.Color
}

// inkSteps is the ladder in reading order: the primary ink first, the quietest
// last. The order is the point — a step below must never read stronger than the
// step above it.
func inkSteps(tk Tokens) []inkStep {
	return []inkStep{
		{"Text", tk.Theme.Text},
		{"TextSecondary", tk.TextSecondary},
		{"TextMuted", tk.Theme.TextMuted},
		{"TextTertiary", tk.TextTertiary},
	}
}

// inkSurface is a plane text is actually painted on, as a name and the colour it
// composites to: the panel itself, and a chosen row's fill over it.
type inkSurface struct {
	name string
	face ui.Color
}

func inkSurfaces(tk Tokens) []inkSurface {
	return []inkSurface{
		{"the panel", tk.Theme.Background},
		{"a chosen row", tk.RowSelected.Over(tk.Theme.Background)},
	}
}

// contrastRatio is WCAG 2.1's contrast ratio between two opaque colours, in the
// 1:1 (nothing) to 21:1 (black on white) range. The body-text floor the
// accessibility guideline names is 4.5:1.
func contrastRatio(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relativeLuminance is WCAG 2.1's L: the sRGB channels linearised, then weighted
// by the eye's own response.
func relativeLuminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// TestEveryInkStepClearsTheReadableContrast is P11-aa's pin, and it is the other
// half of the user's F2 (「保持浅色背景下的可读性与对比度达标」): P11-z measured
// strokes, this measures **words**.
//
// Two claims, in two directions:
//
//   - **高对比度**: every rung of the ladder clears AA's 4.5:1 against both the
//     panel and a chosen row's fill. Before this round the quiet two rungs never
//     moved (`palette.contrast` touched the strokes and the placeholder only), so
//     a user who asked the system for more contrast got sharper lines over the
//     same faint words — 3.04:1 in light, 3.82:1 in dark.
//   - **默认外观**: the ladder still strictly descends. The quiet rungs are *not*
//     asserted to clear 4.5 there: three steps cannot all clear it on `#F7F7F8`
//     and stay distinguishable (the arithmetic is in
//     `docs/rounds/P11-aa-spec.md`), so that is a decision for the user, not a
//     number this test may invent.
func TestEveryInkStepClearsTheReadableContrast(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
	}{
		{"light", false},
		{"dark", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			plain := ForPreferences(s, tc.systemDark, ui.Preferences{})
			high := ForPreferences(s, tc.systemDark, ui.Preferences{HighContrast: true})

			// The gap this round closes: the quiet two rungs used to be the same
			// bytes in both resolutions.
			if high.TextSecondary == plain.TextSecondary {
				t.Errorf("more contrast left TextSecondary at %v: the words stay as faint as the lines get sharp", high.TextSecondary)
			}
			if high.TextTertiary == plain.TextTertiary {
				t.Errorf("more contrast left TextTertiary at %v: the words stay as faint as the lines get sharp", high.TextTertiary)
			}

			// High contrast: 4.5:1 on both surfaces, and the ladder still descends.
			for _, surface := range inkSurfaces(high) {
				previous := 0.0
				for i, step := range inkSteps(high) {
					got := contrastRatio(step.ink.Over(surface.face), surface.face)
					if got < 4.5 {
						t.Errorf("high contrast: %s on %s = %.2f:1, want at least 4.5:1", step.name, surface.name, got)
					}
					if i > 0 && got >= previous {
						t.Errorf("high contrast: %s on %s = %.2f:1 does not sit below the rung above it (%.2f:1)", step.name, surface.name, got, previous)
					}
					previous = got
				}
			}

			// The default appearance keeps its own ladder (and may stay quiet).
			for _, surface := range inkSurfaces(plain) {
				previous := 0.0
				for i, step := range inkSteps(plain) {
					got := contrastRatio(step.ink.Over(surface.face), surface.face)
					if i > 0 && got >= previous {
						t.Errorf("%s on %s = %.2f:1 does not sit below the rung above it (%.2f:1)", step.name, surface.name, got, previous)
					}
					previous = got
				}
			}

			// And the switch only ever moves the inks *up*: asking for contrast
			// can never make a word quieter than the default does.
			for _, step := range inkSteps(high) {
				var plainInk ui.Color
				switch step.name {
				case "TextSecondary":
					plainInk = plain.TextSecondary
				case "TextMuted":
					plainInk = plain.Theme.TextMuted
				case "TextTertiary":
					plainInk = plain.TextTertiary
				default:
					continue
				}
				face := plain.Theme.Background
				was := contrastRatio(plainInk.Over(face), face)
				now := contrastRatio(step.ink.Over(face), face)
				if now < was {
					t.Errorf("more contrast made %s quieter: %.2f:1 vs the default's %.2f:1", step.name, now, was)
				}
			}
		})
	}
}

// TestTheAccentFamilyAlreadyClearsTheReadableContrast is P11-am's second half: it
// closes P11-al's 「仍待」第 2 条 — the accent changed hue and `palette.contrast()`
// was never re-read against it — by **measuring** instead of guessing.
//
// The answer is that the switch must keep its hands off the family. Both
// appearances' accent already clear AA as ink, as the fill a control is
// identified by, and as the pair `AccentText` is read on; the high-contrast
// branch does not touch `accent`, `accentText` or `accentEdge` at all, and this
// pin says that stays a decision rather than a drift (P11-aa's argument for the
// inks was that they *were* too quiet; the accent never was).
func TestTheAccentFamilyAlreadyClearsTheReadableContrast(t *testing.T) {
	for _, tc := range []struct {
		name       string
		systemDark bool
	}{
		{"light", false},
		{"dark", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			plain := ForPreferences(s, tc.systemDark, ui.Preferences{})
			high := ForPreferences(s, tc.systemDark, ui.Preferences{HighContrast: true})

			// ① The contrast preference leaves the accent family alone.
			for _, pair := range []struct {
				name      string
				high, low ui.Color
			}{
				{"Accent", high.Theme.Accent, plain.Theme.Accent},
				{"AccentText", high.Theme.AccentText, plain.Theme.AccentText},
				{"AccentEdge", high.AccentEdge, plain.AccentEdge},
				{"AccentHalo", high.AccentHalo, plain.AccentHalo},
			} {
				if pair.high != pair.low {
					t.Errorf("high contrast moved %s: %v → %v; the family already clears the floor below, "+
						"so moving it has to be a design decision, not a side effect", pair.name, pair.low, pair.high)
				}
			}

			// ② The accent as **ink** (a chosen page's mark, a keyline's glyph):
			//    AA's 4.5:1 on the panel and on a chosen row's fill.
			for _, surface := range inkSurfaces(high) {
				got := contrastRatio(high.Theme.Accent.Over(surface.face), surface.face)
				if got < 4.5 {
					t.Errorf("accent ink on %s = %.2f:1, want at least 4.5:1", surface.name, got)
				}
				t.Logf("%s: accent ink on %s = %.2f:1", tc.name, surface.name, got)
			}

			// ③ The ink the accent's own fills carry (the primary CTA's label, a
			//    switch's ON mark): 4.5:1 against the fill.
			if got := contrastRatio(high.Theme.AccentText, high.Theme.Accent); got < 4.5 {
				t.Errorf("AccentText %v on the accent fill %v = %.2f:1, want at least 4.5:1",
					high.Theme.AccentText, high.Theme.Accent, got)
			} else {
				t.Logf("%s: accentText on the accent fill = %.2f:1", tc.name, got)
			}

			// ④ The fill as a non-text control surface (a switch's ON track, the
			//    primary CTA's face): WCAG's 3:1 for components.
			if got := contrastRatio(high.Theme.Accent, high.Theme.Background); got < 3 {
				t.Errorf("the accent fill against the panel = %.2f:1, want at least 3:1 for a control surface", got)
			} else {
				t.Logf("%s: accent fill against the panel = %.2f:1", tc.name, got)
			}
		})
	}
}

// P12-a · 三档 elevation（docs/liquid-glass.md §2）：底 / 面 / 浮。
//
// 空间感不是「更多层」，而是「层数少、差值一眼看得见」。这条 pin 钉四件事：
//
//	① 面比底高：`Control` 的 alpha 必须 ≥ 0.07（1x 屏上 8 级亮度差的下沿）；
//	② 浮比面高：`Float` 的 alpha 必须严格高于 `Control` —— 浮层是「压在其他内容上」
//	   的那一层，它比任何长在底上的面都厚；
//	③ 指针抬起比静息高一档：`ControlHover > Control > ControlPress`；
//	④ **受光边严格亮于面**：`LensRim > LensSheen` —— 这是这一支唯一的材质不变量
//	   （「一条线」而不是「一片渐变」）。上限按外观分档：深 `≤ 0.09`、浅 `≤ 0.24`
//	   （旧的无光泽上限 0.04 已在 §1 末尾重定基线；浅色那一条是 P13-ah 抬的，见 0-11 行）。
//
// 反面臂：受光边不许靠色相说事（三通道相等 = 中性），也不许变成不透明的白线。
func TestTheThreeElevationRungsAreVisibleAndOrdered(t *testing.T) {
	alpha := func(c ui.Color) float32 { return float32(c.A) / 255 }
	for _, tc := range []struct {
		name string
		dark bool
	}{
		{"dark", true}, {"light", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tk := ForPreferences(settings.Settings{Theme: "auto", GlassStep: "regular"}, tc.dark, ui.Preferences{})
			// ① 面比底高一个看得见的台阶。0.07 是浅色那一档；深色是 0.10。
			if a := alpha(tk.Control); a < 0.07 {
				t.Errorf("the face rung (Control) is %.3f, below the 0.07 floor: it would read as the ground", a)
			}
			// ② 浮比面高。`Float` 是「窗框 tint + 0.10」，默认 47% 时是 0.57，
			// 所以这条断言读的是「浮层的 alpha 高于面」，不是「等于某个数」。
			if !(alpha(tk.Float) > alpha(tk.Control)) {
				t.Errorf("the float rung (%.3f) is not above the face rung (%.3f): a floating pane must be denser than anything that sits on the ground",
					alpha(tk.Float), alpha(tk.Control))
			}
			// ③ 指针阶梯：抬起一档、按下一档，静息在中间。
			if !(alpha(tk.ControlHover) > alpha(tk.Control) && alpha(tk.ControlPress) < alpha(tk.Control)) {
				t.Errorf("the pointer ladder is not lift/rest/sink: hover %.3f, rest %.3f, press %.3f",
					alpha(tk.ControlHover), alpha(tk.Control), alpha(tk.ControlPress))
			}
			// ④ 受光边亮于面，且仍是一条「轻」的线。
			//
			// P13-ah 重定基线（V）：旧断言是「`LensRim ≤ 0.09`」，那是深色档的级数。
			// 0.09 的白在浅色档的面（221）上只抬 **+3 级**，深色档同一条 alpha 抬 +19——
			// 「这块面可以被碰」的记号在浅色档是一个看不见的像素。浅色新值 **0.24**：
			// `221×0.76+255×0.24 = 229.2`，+8 级（§1 的 8 级下沿）；再往上顶时两道账会红——
			// accent 面上的色相（受光边把最暗通道抬过 60/255 就不读作 accent 了）与
			// 「窗框的 alpha 就是滑块的」（顶带的平均 alpha 偏移超过 0.012）。
			// 旧值 → 新值：全局 `≤ 0.09` → 深 `≤ 0.09` / 浅 `≤ 0.24`。深色档不动。
			if tk.LensRim.A <= tk.LensSheen.A {
				t.Errorf("the lit edge (LensRim %d) is not brighter than the face (LensSheen %d)", tk.LensRim.A, tk.LensSheen.A)
			}
			rimCap := float32(0.09)
			if !tc.dark {
				rimCap = 0.24
			}
			if a := alpha(tk.LensRim); a > rimCap+0.004 {
				t.Errorf("the lit edge is %.3f, above this appearance's cap %.2f (liquid-glass.md §1 的 0-11 行)", a, rimCap)
			}
			if tk.LensRim.A >= 255 {
				t.Errorf("the lit edge is opaque (%v): a rim, not a white bar", tk.LensRim)
			}
			if tk.LensRim.R != tk.LensRim.G || tk.LensRim.G != tk.LensRim.B {
				t.Errorf("the lit edge %v carries a hue; light is neutral", tk.LensRim)
			}
		})
	}
}

// P13-ac · 三档 elevation 在**透明度滑块的两端**还成不成立？
//
// 上面那条只量默认 47%（`settings.DefaultMainOpacity`）。任务书「先量再改的两个数」
// 第 1 条猜的是「低透明度端太弱，可能要按框的 alpha 缩放面档」——量下来的方向是反的
// （见 `internal/glassmap/floatladder_test.go` 的那张表：alpha 差是常数 0.10，10% 时
// 浮层是底的两倍；塌的是顶端 0.02）。这条 pin 把结论钉住：**面档不随框的 alpha 缩放**，
// 滑块推到 10% 或 100%，`Control` / `ControlHover` / `ControlPress` 三档一个字节都不动，
// 浮层仍严格高于面档。
//
// 反面臂：面档也不许跟着框一起变淡——那会把「面」退回「底」，正是用户说的
// 「所有东西挤在同一个 z 平面上」。
func TestTheElevationRungsDoNotRideTheOpacitySlider(t *testing.T) {
	alpha := func(c ui.Color) float32 { return float32(c.A) / 255 }
	base := settings.Settings{Theme: "auto", GlassStep: "regular"}
	reference := map[bool]struct{ face, hover, press uint8 }{}
	for _, dark := range []bool{true, false} {
		tk := ForPreferences(base, dark, ui.Preferences{})
		reference[dark] = struct{ face, hover, press uint8 }{tk.Control.A, tk.ControlHover.A, tk.ControlPress.A}
	}
	for _, opacity := range []uint8{10, 47, 100} {
		for _, dark := range []bool{true, false} {
			s := base
			s.MainOpacity = opacity
			tk := ForPreferences(s, dark, ui.Preferences{})
			want := reference[dark]
			if tk.Control.A != want.face || tk.ControlHover.A != want.hover || tk.ControlPress.A != want.press {
				t.Errorf("%d%% (dark=%v): the face rungs moved with the frame's alpha — face %d/%d, hover %d/%d, press %d/%d; a face is a wash over whatever is under it, so it does not ride the slider",
					opacity, dark, tk.Control.A, want.face, tk.ControlHover.A, want.hover, tk.ControlPress.A, want.press)
			}
			if !(alpha(tk.Float) > alpha(tk.Control)) {
				t.Errorf("%d%% (dark=%v): the float (%.3f) fell behind the face (%.3f)", opacity, dark, alpha(tk.Float), alpha(tk.Control))
			}
			// 受光边是「面」的一部分，也不许随框变淡：它画的是一条线，不是一层玻璃。
			if tk.LensRim.A != ForPreferences(base, dark, ui.Preferences{}).LensRim.A {
				t.Errorf("%d%% (dark=%v): the lit edge moved with the frame's alpha: %d", opacity, dark, tk.LensRim.A)
			}
		}
	}
}
