// Package theme turns floter's stored appearance settings into the concrete
// theme the views paint with.
//
// The old build had a token layer of its own: CSS custom properties in
// src/styles/base.css (surfaces, borders, radii, the glass tint) resolved
// from `theme` and scaled by `ui_scale`. This package is that layer's Go
// equivalent, a subset: the two appearance values the app ships, the radius
// scale, and the interface-size factor. The rest of the palette comes from
// the framework's light and dark themes, which already follow the desktop's
// accent and contrast preferences.
package theme

import (
	"math"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

// Appearance is the concrete look a surface paints with, after resolving the
// stored `theme` against the desktop.
type Appearance string

const (
	// Light and Dark are the two appearances the app paints.
	Light Appearance = "light"
	Dark  Appearance = "dark"
)

// ColorStep is how long a pointer's colour answer takes on every face of the
// app: the reference stylesheet's `--dur-2` (`launcher.css:397`,
// `color var(--dur-2) ease, background var(--dur-2) ease`). It is the *one*
// step for this kind of change, and the other two are deliberately not it:
// `--dur-1` (80ms) is the transform step — the framework plays `Scale` as a
// render-time transform, not as a transition — and `--dur-3` (160ms) is a
// floating pane's entrance, which moves material rather than recolouring it.
//
// P13-ad 重定基线：`docs/liquid-glass.md` §5 此前写的是「90ms ease-out」,
// 而 `internal/` 与 `cmd/` 里从来没有出现过 90ms——实测是 12 处手写的 120ms
// 加一处 150ms（`settingsui/uninstall.go` 的组件行，照抄组件自己的
// `transition: background 150ms ease`）。同一个指针动作在一个应用里有三种速度，
// 而对话框里的行比它脚下的列表慢 25%，正是「两块不同年头的界面」的一个算术来源。
// 旧值 → 新值：文档 `90ms`（从未实现）与离群 `150ms` → 一律 `120ms`。
const ColorStep = 120 * time.Millisecond

// Resolve resolves the stored theme against the desktop's appearance. It
// mirrors App.tsx's rule exactly: `auto` follows the system, an explicit
// `light` or `dark` wins, and anything else (only reachable without the
// loader's normalization) paints dark.
func Resolve(theme string, systemDark bool) Appearance {
	switch theme {
	case "auto":
		if systemDark {
			return Dark
		}
		return Light
	case "light":
		return Light
	default:
		return Dark
	}
}

// The radius scale, at scale 1, in Raycast's own steps (`docs/raycast-design.md`
// §2): an icon plate, a row or a button, a block inside a panel, and the window
// or a card.
//
//	radius  6  icon plate (XS)
//	radius  8  row, button (SM)
//	radius 10  block inside a panel (MD)
//	radius 12  window, card (LG)
//	radius  5  keycap (KeycapRadius)
//
// The ladder is literal px (as base.css wrote it): a corner does not follow
// `--ui-scale` the way a font or a `--u` metric does.
const (
	RadiusXS = 6
	RadiusSM = 8
	RadiusMD = 10
	RadiusLG = 12

	// KeycapRadius is the corner a keycap draws: Raycast's 20x20 pressable
	// legend, one step under the icon plate's. It has no rung of its own in the
	// four-step ladder above — a keycap is smaller than any surface the ladder
	// names — so it is its own number rather than an alias.
	KeycapRadius = 5

	// BodyUnits is the body text size in this codebase's own unit, `--text-body`
	// one rung up: 12px at scale 1. Every size in this file's unit (a
	// `Space(n)` is four of them), so the framework theme's point size is set
	// from it rather than from the framework's own default.
	//
	// The reference's own register is 11px *at its scale 1*; the shipped
	// interface step is `small` (0.9), so the ladder is written one rung up and
	// the shipped step lands on the reference's physical sizes (see the ladder
	// in ForPlatform). Reading the design off the shipped step instead means
	// every user sees the design at its own size without first finding
	// 「界面大小」 — which is what made the whole app read as 小气.
	BodyUnits = 12
)

// windowRadiusFor is the corner a surface that *is* the window draws: the
// design's 12px on all three platforms (`docs/raycast-design.md` §2).
//
// Windows used to be the exception — base.css pins `--window-radius` to 8px
// inside `.platform-windows` (`:1492`) because DWM rounds every top-level window
// at its own ~8px preset, and a card corner larger than the window's leaves the
// system's frame color showing as a crescent between the two arcs. This build no
// longer lets DWM round the window: `shell.dressWindowsFrame` asks for
// `DWMWCP_DONOTROUND` and takes the system border off as well, so the window's
// rectangle is square and the card's corner is the only corner on screen. That
// corner is the design's own 12px, the same figure macOS and Linux draw — which
// is the point: the three platforms are meant to look like one app.
//
// The platform stays a parameter so the table is readable on any host.
func windowRadiusFor(string) float32 {
	return RadiusLG
}

// Tokens is the resolved theme: the appearance, the framework theme the
// widgets read, and the radius scale, all at the user's interface size.
type Tokens struct {
	// Appearance is the resolved light or dark look.
	Appearance Appearance
	// Theme is the theme to set on the view's context.
	Theme *ui.Theme
	// Scale is the interface-size factor the tokens were scaled by
	// (0.8/0.9/1.0/1.1; the shipped "small" step is 0.9).
	Scale float64

	RadiusXS float32
	RadiusSM float32
	RadiusMD float32
	// RadiusLG is the window or a full panel's corner: 12px.
	RadiusLG float32
	// WindowRadius is the corner a surface that *is* the window draws: the
	// launcher card, the settings card and the terminal panel. It is RadiusLG
	// (12) on every platform — the window's own rectangle is square everywhere
	// this build runs, because DWM's rounding is switched off (see
	// windowRadiusFor).
	WindowRadius float32
	// KeycapRadius is a keycap's corner: 5, the step under the icon plate's 6.
	KeycapRadius float32

	// InputStroke is the one edge a card or a control draws (base.css's
	// `--input-stroke`), and InputStrokeActive its focused step — the lit
	// edge a focused card takes, as `.collapsed-card:focus-within` did.
	//
	// P11-z 重定基线（用户实测 F2「整体红色线条太扎眼」）：聚焦档**不再是 accent**，
	// 而是同一根发丝线的亮一档（浅 0.09→0.16 黑，深 0.09→0.16 白）。§5 的判据
	// 是「accent 只能回答哪个被选中 / 哪个是开的」，而「整个窗口有键盘焦点」两件
	// 都不是——它是一条常量装饰。§5 的「预算只约束色相，不约束明度」正是这条的
	// 授权：焦点靠明度差说，不靠色相说。
	InputStroke       ui.Color
	InputStrokeActive ui.Color

	// Control is the resting plane a grouped card or a control sits on
	// (base.css's `--glass-control`), ControlEdge its hairline edge, and
	// Hairline the rule a row draws between itself and the next. The
	// settings panel's card language is these three plus TextStrong.
	Control     ui.Color
	ControlEdge ui.Color
	Hairline    ui.Color
	// HairlineSoft is the weaker of the design's two hairlines
	// (`docs/raycast-design.md` §1): the rule between rows and inside a block,
	// one step under the card's own edge. Hairline is the stronger rung, and it
	// is the card's 1px border; this one is what the field row draws against
	// the list and what the footer draws against the list above it.
	HairlineSoft ui.Color
	// RowHover, RowSelected and RowPressed are the launcher row's own three
	// fills: Raycast's list is *neutral*, not accent-tinted — a hovered row is
	// 0.045 of white over the near-black card, the chosen row 0.085, and a
	// pressed row 0.11, with no border and no glow at any rung. They are their
	// own tokens rather than the framework's `SurfaceHover`/`Selection` because
	// the list's ladder has to be paintable on the card's glass at these exact
	// alphas (see `Tokens.AccentTintQuiet` for the language this replaces: the
	// chosen row used to be the accent's quiet tint).
	RowHover    ui.Color
	RowSelected ui.Color
	RowPressed  ui.Color
	// KeycapFace and KeycapEdge are the pressable legend's plate: the fill and
	// its hairline (Raycast's 20x20 keycap, `docs/raycast-design.md` §4.5).
	// They are not the `KeycapHighlight`/`KeycapShadow` pair — those are the
	// Windows terminal panel's own rim, a different token for a different job.
	KeycapFace ui.Color
	KeycapEdge ui.Color
	// ControlHover and ControlPress are the control ladder's two other
	// rungs (base.css's `--glass-control-hover` / `--glass-control-press`):
	// a pointer over a pane lifts it, a pointer down sinks it *below* the
	// resting face, and the settings sidebar's current page is the raised
	// quiet pane (`--glass-raised-quiet`, an alias of ControlHover).
	ControlHover ui.Color
	ControlPress ui.Color
	// Thumb is the segmented control's **chosen slot** — the one raised pane in
	// the tree that lives *inside* a groove, so it cannot borrow the control
	// ladder: `ControlHover` lifts a pane on the light appearance by washing it
	// black (`0,0,0,0.105`), which over the groove's own face (232) lands at
	// 207 — 24 levels *below* the groove it sits in. The same token points the
	// other way on the dark appearance (white 0.14 over 58 → 86), so one token
	// was saying "raised" and "sunk" on the two appearances at once, and the
	// light one is the macOS 10.x read the user called out (「有点偏 macOS 旧
	// 版本」).
	//
	// The two appearances therefore answer the same question differently, and
	// both answers are the platform's: on the light sheet the thumb is the
	// **sheet's own colour** punched through the groove (0.85 of white over the
	// card's groove composites to 249 against the page's 247), which is the
	// iOS/macOS segmented slider; on the dark sheet there is no brighter sheet
	// to borrow (the page is 39), so the thumb is the lifted white wash the
	// control ladder already uses (0.14, the shipped value — unchanged).
	Thumb ui.Color
	// Track is the groove a switch or a segmented control is looked into
	// (base.css's `--glass-track`), TrackEdge the quieter hairline that groove
	// draws on its recessed lens (`--glass-track-edge`), TrackRim the line that
	// light pools on along that groove's *bottom* edge, and Field the slot a
	// text field cuts into the glass (`--glass-field`).
	//
	// TrackRim is not `LensRim`: a raised face's lit edge and a groove's lit
	// edge are two different lines on two different grounds (P13-o,
	// `docs/liquid-glass.md` §3). On the light appearance a raised face's rim
	// sits on a near-white face where 0.09 of white is all that can be spent
	// without washing an accent fill (P13-n), while a groove's rim sits on the
	// track *and* under the track's own 0.08 hairline — it has to be near-white
	// just to survive the ring.
	Track     ui.Color
	TrackEdge ui.Color
	TrackRim  ui.Color
	Field     ui.Color
	// Opaque is the near-solid stand-in for a surface whose glass has been
	// turned off or that must not show the desktop (base.css's
	// `--surface-opaque`): the `off` stop's card.
	Opaque ui.Color
	// FieldFill is **deprecated**: the launcher's field block used to paint its
	// own near-solid face over the panel (`--glass-field` over
	// `--surface-opaque`), which read as a card floating on the panel — the
	// user's 「像一块浮在上面的卡片」/「不连续边框」 (P11-bj). The field row now
	// paints nothing and is the panel's own first band. Kept so the token table
	// stays one table; nothing reads it.
	FieldFill ui.Color
	// TextStrong is a row's own label color, one step above the body text
	// (base.css's `--text-strong`), TextSecondary and TextTertiary the two
	// quieter steps below TextMuted.
	TextStrong    ui.Color
	TextSecondary ui.Color
	TextTertiary  ui.Color
	// The control lens family (base.css's `--glass-lens-*`): a control's top
	// rim, its broad sheen and its bottom shade. The framework draws no such
	// layers, and without them a translucent pane reads as a flat tint with
	// a border rather than as a lens; Lens stacks them as the one inset
	// shadow the old build's `--elev-0` was.
	LensRim   ui.Color
	LensSheen ui.Color
	LensEdge  ui.Color
	// SheenTop and SheenBottom are the card's top-down highlight (base.css's
	// `--sheen-top` / `--sheen-bottom`), the gradient that gives the panel's
	// glass a lit face rather than a flat tint. They are the reference's
	// **fixed** tokens — the same on every effect stop, and dropped by `off` —
	// capped at the frame's own alpha. The fill under them is `FaceFill`'s
	// compensated alpha, so the two composite back to the frame's own alpha
	// (see `FaceFill`).
	SheenTop    ui.Color
	SheenBottom ui.Color
	// SheenTopActive and SheenBottomActive are the card's lit face *while it
	// has the keyboard*: base.css's `--sheen-top-active` / `--sheen-bottom-active`
	// (`:630-631` dark, `:908-909` light), which `.collapsed-card:focus-within`
	// paints instead of the resting pair (`main:src/styles/launcher.css:136-144`).
	// Capped at the frame's alpha exactly as the resting pair is, so raising the
	// face never makes the frame denser than the slider asked for (see
	// `SheenAt`).
	SheenTopActive    ui.Color
	SheenBottomActive ui.Color
	// ShadowAmbient and ShadowContact are the window's own cast shadow
	// (base.css's `--window-shadow-ambient` / `--window-shadow-contact`): the
	// broad throw and the tight contact edge a card carries where the platform
	// has no window-server shadow (Linux, Windows).
	ShadowAmbient ui.Color
	ShadowContact ui.Color
	// EdgeHighlight, EdgeShadow and EdgeHighlightX are the panel's rim (base.css's
	// `--edge-highlight` / `--edge-shadow` / `--edge-highlight-x`): the lit top
	// edge, the dark bottom edge and the whisper of a stroke on both sides that
	// `--glass-rim-shadow` composes. The launcher card has no rim (R72 removed
	// it — the card is its own floating window); the settings and terminal
	// panels, which sit *on* a surface, do.
	EdgeHighlight  ui.Color
	EdgeShadow     ui.Color
	EdgeHighlightX ui.Color
	// EdgeHighlightActive is the lit top edge's *focused* step (base.css's
	// `--edge-highlight-active`: dark `0.18` at `:633`, light `0.96` at `:911`),
	// which the platform card rules swap in on `:focus-within`
	// (`.platform-windows .collapsed-card:focus-within`,
	// `main:src/styles/base.css:1558-1565`; the Linux pair at `:1625-1632`). The
	// port kept only the resting rung, so the top edge never brightened with the
	// stroke and the sheen (see `SheenAtFocus`) — the third half of one rule.
	EdgeHighlightActive ui.Color
	// KeycapHighlight and KeycapShadow are the *keycap* pair base.css gives a
	// key's top and bottom (`--keycap-highlight` / `--keycap-shadow`: dark
	// `0.07`/`0.28` at `:657-658`, light `0.9`/`0.09` at `:917-918`). They are
	// not a second edge palette: the terminal panel's rim is written with them
	// on Windows
	//
	//	.platform-windows .terminal-panel {
	//	  box-shadow: …, inset 0 1px 0 var(--keycap-highlight),
	//	                inset 0 -1px 0 var(--keycap-shadow);
	//	}
	//
	// (`main:src/styles/base.css:1567-1574`), because a terminal panel is a
	// *keycap-shaped* slab rather than a sheet lying on a surface: its top is
	// the lit crown of a key and its bottom the seat under it, both quieter
	// than the card's own rim (0.07 against 0.15 in the dark appearance). The
	// port drew every panel with the card's rim, so the Windows terminal wore
	// an edge twice as bright as its own rule asks for.
	KeycapHighlight ui.Color
	KeycapShadow    ui.Color
	// AccentEdge is the accent as a 1px keyline (base.css's `--accent-edge`):
	// the emphasis a chosen segment or a recommended badge carries without an
	// accent fill. AccentHalo is the accent as a soft ring (base.css's
	// `--accent-halo`): the one non-elevation shadow in the sheets, the live
	// status dot's glow (`--elev-halo` = `0 0 0 2px var(--accent-halo)`), the
	// accent at a fixed low alpha so it never reads as a second accent fill.
	AccentEdge ui.Color
	// AccentEdgeStrong is the heavier accent keyline (base.css's
	// `--accent-edge-strong`): the 1px edge the *primary* action pill carries
	// over its accent fill (`.extensions-action-button--primary`, its
	// `--accent-edge-strong` border), where `--accent-edge` is the keyline a
	// chosen slot wears without a fill. One accent, two weights: the fill's
	// edge is the strong one so the pill reads as a control, not as a tint.
	AccentEdgeStrong ui.Color
	AccentHalo       ui.Color
	// AccentGlow is the accent as a panel-sized focus ring (base.css's
	// `--accent-glow`: dark `rgba(143,183,255,0.08)` at `:687`, light
	// `rgba(6,74,166,0.2)` at `:955`). It is a *third* rung, not a repeat of
	// `AccentHalo` (0.18/0.24) or `AccentEdge` (0.30/0.36).
	//
	// Deprecated: P11-z 收掉了它的最后一处生产调用（`internal/shell/cardedge.go`
	// 的 Linux 聚焦卡 inset 环）。那圈环和 `InputStrokeActive` 是同一个语义——
	// 「整个窗口有键盘焦点」——也就是用户实测 F2 说的「整体红色线条」；焦点改由
	// `EdgeHighlightActive` 与 `SheenAtFocus` 的亮面表达（两者都保留）。
	//
	// 另：P11-z 查到一个旧账——`For` 从未给这个字段赋过值（`Tokens` 构造块里
	// 只有 `AccentEdge`/`AccentEdgeStrong`/`AccentHalo` 三行），所以那圈环一直
	// 用零值颜色画，**从来没有真的画出来过**；旧 pin 与零值比较因此永远绿。
	// 字段保留不删，同 P8-e 对 `AccentTintQuiet` 的处理。
	AccentGlow ui.Color
	// AccentTintQuiet is the accent at the quietest rung of its tint family
	// (base.css's `--accent-tint-quiet`): one rung below `--accent-tint`, and a
	// *per-appearance* token (dark 0.085, light 0.065) — the same pastel over
	// near-black gains contrast, while the same alpha over white barely moves
	// the pixel, so the light appearance needs the lighter rung. Quoting one
	// number for both appearances painted the light row's selection with the
	// dark rung (the launcher's `Accent.Alpha(0.085)`).
	//
	// Deprecated: P8-e 收掉了它的最后一处生产调用（`internal/launcher/output.go`
	// 的选中行改 `RowSelected`）——§5 明写禁止「列表选中行」用 accent，而这一处
	// 是 §5 判据落地后的最后一个例外。仓库层面已 grep 确认零生产调用点（只剩
	// 这个字段定义与 `internal/theme/theme_test.go` 的值 pin）。字段保留不删：
	// 本次只做视觉收敛，删除留给下一次清理轮（同 P8-c 对 `FieldSlot` 的处理）。
	AccentTintQuiet ui.Color
	// Sunken is the settings content pane's own fill (base.css's
	// `--surface-sunken`): the settings panel is a reading surface, and it
	// carries a fill of its own so it reads one step denser than the frame.
	// Its alpha is the *over-frame* fill that composites to the frame tint
	// plus a small recess (glassmap.Spec.ContentAlpha), so the app's whole
	// background - the bottom included - stays the slider's single sheet.
	Sunken ui.Color
	// RaisedQuiet is the sidebar's chosen page: the control-hover lift
	// composited opaque over the content pane, so the framework's own accent
	// fill behind a chosen list row cannot bleed through as a blue pill.
	RaisedQuiet ui.Color
	// RaisedWarm is the warm branch of the raised pane (base.css's
	// `--glass-raised-warm`, an alias of `--system-icon-surface`): the amber
	// fill a broken integration row paints, where the accent's blue would read
	// as "this opens something" rather than as a warning.
	RaisedWarm ui.Color
	// Float is a floating pane's own fill (base.css's `--glass-float`): the
	// frame's tint plus a fixed step, so a drawer, a dialog, a toast or the
	// terminal's resident note is always denser than the card it floats over.
	Float ui.Color
	// IconSurface and IconSurfaceHover are the warm system-icon plane (base.css's
	// `--system-icon-surface` / `--system-icon-surface-hover`): the amber tint a
	// semantic icon chip sits on. `--glass-raised-warm` is an alias of the first,
	// which is what the launcher's own feedback row and the settings sheet's
	// danger pills paint — it is a warm *surface*, not the warning ink, and
	// quoting the warning colour's alpha here painted the row's fill with the
	// label's own hue (the row read pink-red instead of warm orange).
	IconSurface      ui.Color
	IconSurfaceHover ui.Color
	// SystemIconText is the ink a warm system-icon plane carries (base.css's
	// `--system-icon-text`): the amber ink the launcher's power rows draw their
	// glyph in. It is a token of its own, not the warning ink: the dark
	// appearance reads it as a pale warm peach and the light one as a burnt
	// orange (`#a24d15`), while the warning colour is a near-neutral red-grey
	// reserved for a *label*. The port painted the warm plate's glyph with
	// `t.Warning`, which is the wrong hue on both appearances.
	SystemIconText ui.Color

	// The text ladder, in points at the current step: base.css's registers,
	// which the codebase had been spelling as offsets off the body size (a
	// +6 that is 6.6 points at the Large step, not 6).
	Caption  float32
	Body     float32
	Emphasis float32
	Title    float32
	Display  float32
}

// For resolves the appearance and scales the metrics for a settings value,
// assuming a desktop with no preferences of its own. It is what a render
// without a desktop context uses (the segmented-control tests); a caller with
// a `*ui.Context` should use ForPreferences, because the desktop's contrast
// preference lives there and the old build's `prefers-contrast: more` block
// is a real part of the look.
func For(s settings.Settings, systemDark bool) Tokens {
	return ForPreferences(s, systemDark, ui.Preferences{})
}

// ForPreferences resolves the appearance and scales the metrics for a settings
// value and the desktop's preferences. The widgets' font, spacing and radius
// all follow the interface-size step, as the old build's CSS variables did,
// and a desktop that asked for more contrast gets base.css's
// `prefers-contrast: more` tokens: the stronger strokes and the readable
// placeholder, plus the material changes (glassmap.Spec.Contrast).
//
// It is ForPlatform for the window this build is running in: the glass fill is
// compensated for the platform's own material (glassmap.MaterialVeil), so the
// tokens of a macOS window and a Linux one are not the same numbers. A caller
// that wants the other platform's look — a test reading all three tables on one
// host — asks ForPlatform directly.
func ForPreferences(s settings.Settings, systemDark bool, prefs ui.Preferences) Tokens {
	return ForPlatform(s, systemDark, prefs, runtime.GOOS)
}

// ForPlatform is ForPreferences for a named platform (the `goos` strings the
// rest of the build uses). The platform reaches the look in exactly one place:
// the glass spec, whose fill is the transparency slider less the veil the
// window's material already contributes (glassmap.SpecForPreferencesOS).
func ForPlatform(s settings.Settings, systemDark bool, prefs ui.Preferences, goos string) Tokens {
	s = settings.Normalize(s)
	appearance := Resolve(s.Theme, systemDark)

	base := ui.LightTheme()
	if appearance == Dark {
		base = ui.DarkTheme()
	}
	// Copy so the shared theme values are never mutated in place.
	t := *base

	scale := settings.UIScaleFactor(s.UIScale)
	if scale <= 0 {
		scale = 1
	}
	f := float32(scale)
	// The old app's ladder is in its own unit: `--u: 1px * ui-scale` with
	// `--text-body: 11px * ui-scale`, so a body line is eleven units and every
	// `Space(n)` in this codebase is four of them (`--u * 4n`). The framework's
	// default is 13 or 14 points, which is a quarter larger than this design
	// and made every row taller than the one it copied; the body size is the
	// old app's eleven units, and the ladder around it (caption, emphasis,
	// title, display) follows from there.
	t.FontSize = round1(BodyUnits * f)
	t.Spacing = round1(t.Spacing * f)

	// The whole palette is base.css's, not the framework's stock one. The
	// old build's tokens are what every surface was drawn against — the
	// pastel accent, the translucent control ladder, the near-black glass —
	// and the framework's defaults (a saturated blue, opaque grey hover
	// fills) are a different design. Setting them here means one place
	// decides the look and every widget the views do not style by hand —
	// buttons, inputs, switches, focus rings, the scrollbar — inherits it.
	p := paletteFor(appearance)
	if prefs.HighContrast {
		p = p.contrast(appearance)
	}
	t.Background = p.background
	t.Text = p.text
	t.TextMuted = p.textMuted
	t.Accent = p.accent
	t.AccentHover = p.accentHover
	t.AccentPressed = p.accentPressed
	t.AccentText = p.accentText
	t.Border = p.stroke
	t.Surface = p.control
	t.SurfaceHover = p.controlHover
	t.SurfacePressed = p.controlPress
	t.Selection = p.selection
	t.Focus = p.focus
	t.Warning = p.warning
	t.Scrollbar = p.scrollbar
	t.ScrollbarWidth = round1(6 * f)
	// The framework's control radius is the ladder's `--radius-sm` (9px),
	// not its own 6px stock value: a button or a field on a surface is the
	// ladder's second step, and the card and window keep their own steps.
	// base.css writes the whole radius ladder in literal px (`--radius-xs:
	// 6px; --radius-sm: 9px; --radius-md: 12px; --radius-lg: 14px`,
	// base.css:93-96), so a corner does not follow `--ui-scale` the way a
	// font or a `--u` metric does: the ladder is the one set of numbers the
	// old build kept fixed while the interface grew. See `Tokens.RadiusSM`.
	t.Radius = RadiusSM

	// The strokes and planes the views read directly are the same palette
	// values, per appearance, exactly as base.css wrote them.
	stroke, strokeActive := p.stroke, p.strokeActive
	control, controlEdge, hairline := p.control, p.controlEdge, p.hairline
	textStrong := p.textStrong
	controlHover, controlPress := p.controlHover, p.controlPress
	track, field := p.track, p.field
	textSecondary, textTertiary := p.textSecondary, p.textTertiary
	// The content pane rides the frame's own tint (the transparency slider)
	// as a single sheet: the pane is only the recess it needs to composite to
	// `frame + step`, so the app's background - top, middle and bottom - is
	// what the slider moves. The reference pinned `--surface-sunken` to a fixed
	// 0.62-0.80 band (which the slider barely moved), and an earlier rewrite
	// then painted the pane at the frame's own alpha, stacking two full tints
	// into a near-opaque bottom. See glassmap.Spec.ContentAlpha.
	spec := glassmap.SpecForPreferencesOS(goos, s.GlassStep, s.MainOpacity, prefs.HighContrast)
	sunken := spec.ContentAlpha()
	floatAlpha := spec.FloatAlpha()
	// The panel's sheen is the glass's lit face. The off stop has no glass, so
	// it paints the plain tint with no sheen - base.css's `[data-glass="off"]
	// .shell` dropped the gradient along with the material. The effect stops
	// paint the reference's **fixed** `--sheen-top` / `--sheen-bottom`
	// (base.css:628-629 dark 0.046/0.01, :906-907 light 0.66/0.22): those
	// tokens do not vary with `[data-glass]` — the only rung a stop moves on
	// the frame's own face is nothing at all; `--glass-lens-scale` moves the
	// **control** lens (LensRim/LensSheen/LensEdge below), not the card. The
	// three stops therefore read apart on their controls (P7-em) and share one
	// lit face; grading the sheen by the stop was this build's own invention
	// from when the stops had nothing else to show on Linux.
	//
	// The sheen is still capped at the frame's own alpha: a sheen heavier than
	// the frame would make the top of a very thin frame denser than the slider
	// asked for, and `FaceFill`'s compensation cannot ask for a negative fill
	// (see `baseAlpha`). The cap is what keeps the frame's *alpha* the slider's
	// on every stop — `FaceFill` gives the tint the alpha that composites with
	// the sheen back to the slider's, so the fixed sheen changes the lit face's
	// colour, not the fill's truth (the sheen used to sit over the frame's tint
	// and add its own alpha — 「开玻璃透明底失效」).
	sheenTop, sheenBottom := ui.Color{}, ui.Color{}
	sheenTopActive, sheenBottomActive := ui.Color{}, ui.Color{}
	if spec.Glass {
		sheenTop = capAlpha(p.sheenTop, spec.TintAlpha)
		sheenBottom = capAlpha(p.sheenBottom, spec.TintAlpha)
		sheenTopActive = capAlpha(p.sheenTopActive, spec.TintAlpha)
		sheenBottomActive = capAlpha(p.sheenBottomActive, spec.TintAlpha)
	}
	baseFill := t.Background
	if appearance == Dark {
		baseFill = ui.RGB(17, 18, 20)
	} else {
		baseFill = ui.RGB(243, 244, 247)
	}
	// The sidebar's chosen page is the reference's `--glass-raised-quiet`,
	// but the framework marks a chosen list row by painting the accent
	// *behind* the row, and a translucent lift over that reads as a blue
	// pill — the opposite of the quiet one the reference asks for. So this
	// rung is the opaque composite of the control-hover lift over the pane
	// it sits on: it hides the accent and keeps the lift.
	raisedQuiet := blendOpaque(baseFill, p.controlHover)
	// A floating pane's fill: the frame's tint plus the reference's fixed step
	// (`--glass-float-alpha = var(--glass-tint-alpha) + 0.1`), in the float
	// palette's own slightly lighter colour, so a drawer, dialog or toast is
	// always denser than the frame it floats over and still follows the slider.
	floatFill := ui.RGB(24, 25, 29)
	if appearance == Light {
		floatFill = ui.RGB(252, 252, 254)
	}

	// The ladder, at the step: the reference's own registers (caption 10, body
	// 11, emphasis 12, title 13, display 17) one rung up (11/12/13/14/19), each
	// scaled once here rather than spelled as an offset wherever a register is
	// needed.
	//
	// One rung up is what makes the **shipped** step the reference's own sizes:
	// `small` is 0.9, and 11/12/13/14/19 x 0.9 = 9.9/10.8/11.7/12.6/17.1, i.e.
	// base.css's 10/11/12/13/17 within a tenth of a point. The ladder used to
	// be the base numbers themselves, so the shipped step drew the design at
	// 90% — a 9pt caption, a 10.8pt emphasis, a 15.3pt display — and every
	// surface read cramped against the window it sat in (「整体 UI/UX 很
	// 小气」). A step above the shipped one now draws *larger* than the
	// reference rather than the reference shrinking below it: the ladder is a
	// named size, the step is the user's magnification of it.
	caption := round1(11 * f)
	body := round1(12 * f)
	emphasis := round1(13 * f)
	title := round1(14 * f)
	display := round1(19 * f)

	return Tokens{
		Appearance:  appearance,
		Theme:       &t,
		Sunken:      baseFill.Alpha(sunken),
		RaisedQuiet: raisedQuiet,
		RaisedWarm:  p.iconSurface,
		Float:       floatFill.Alpha(floatAlpha),
		Caption:     caption,
		Body:        body,
		Emphasis:    emphasis,
		Title:       title,
		Display:     display,
		Scale:       scale,
		// The radius ladder is literal px, as base.css wrote it: a corner is
		// not a `--u` metric and does not move with the interface step.
		RadiusXS:          RadiusXS,
		RadiusSM:          RadiusSM,
		RadiusMD:          RadiusMD,
		RadiusLG:          RadiusLG,
		WindowRadius:      windowRadiusFor(goos),
		KeycapRadius:      KeycapRadius,
		InputStroke:       stroke,
		InputStrokeActive: strokeActive,
		Control:           control,
		ControlHover:      controlHover,
		ControlPress:      controlPress,
		ControlEdge:       controlEdge,
		HairlineSoft:      p.hairlineSoft,
		RowHover:          p.rowHover,
		RowSelected:       p.rowSelected,
		RowPressed:        p.rowPressed,
		KeycapFace:        p.keycapFace,
		KeycapEdge:        p.keycapEdge,
		Thumb:             p.thumb,
		Track:             track,
		TrackEdge:         p.trackEdge,
		TrackRim:          scaleAlpha(p.trackRim, spec.LensScale),
		Field:             field,
		Opaque:            p.opaque,
		FieldFill:         p.fieldFill,
		LensRim:           litEdge(p.lensRim, spec.LensScale, prefs.HighContrast),
		LensSheen:         scaleAlpha(p.lensSheen, spec.LensScale),
		LensEdge:          scaleAlpha(p.lensEdge, spec.LensScale),
		// The frame's lit face is the reference's fixed token, capped at the
		// frame's own alpha (see the `sheenTop` note above); the stop does not
		// grade it.
		SheenTop:            sheenTop,
		SheenBottom:         sheenBottom,
		SheenTopActive:      sheenTopActive,
		SheenBottomActive:   sheenBottomActive,
		ShadowAmbient:       p.shadowAmbient,
		ShadowContact:       p.shadowContact,
		EdgeHighlight:       p.edgeHighlight,
		EdgeHighlightActive: p.edgeHighlightActive,
		EdgeShadow:          p.edgeShadow,
		EdgeHighlightX:      p.edgeHighlightX,
		KeycapHighlight:     p.keycapHighlight,
		KeycapShadow:        p.keycapShadow,
		AccentEdge:          p.accentEdge,
		AccentEdgeStrong:    p.accentEdgeStrong,
		AccentHalo:          p.accentHalo,
		AccentTintQuiet:     p.accentTintQuiet,
		Hairline:            hairline,
		TextStrong:          textStrong,
		TextSecondary:       textSecondary,
		TextTertiary:        textTertiary,
		// The warm plane is a fixed per-appearance token in base.css — unlike
		// the frame's tint it does not follow the transparency slider.
		IconSurface:      p.iconSurface,
		IconSurfaceHover: p.iconSurfaceHover,
		SystemIconText:   p.systemIconText,
	}
}

// palette is base.css's token set for one appearance, as the framework
// theme's fields plus the control-ladder rungs the views read by name.
// Everything here is the old build's value: the pastel accent, the
// translucent white (dark) or black (light) control ladder, the content
// recess's own colour and the near-solid stand-in.
//
// The surface ladder is the one set of numbers this build raised over the
// reference's: `stroke` > `controlEdge` > `hairline` > `control`, the four
// rungs that say a card, its rows and the planes on them are different depths.
// The old build drew all of them under a 0.08 alpha (dark: 0.08/0.06/0.07/
// 0.055), which over a translucent dark frame is one or two levels of grey —
// below what a 1x display resolves — so a card was a card only because of its
// shadow, its rows had no rule and its controls no edge: every surface read as
// one flat pane, and the window read as pinched however generous its padding
// was (「整体 UI/UX 很 小气」). The rungs keep their order and their ratios, one
// step brighter: the card's edge clears the row's hairline, the hairline clears
// a control plane. They stay light-handed — 0.1 white over a dark frame is a
// quarter of the contrast the contrast preference's own tokens use
// (`contrast()`'s 0.42/0.34) — so the ladder buys depth without becoming a
// border-on-every-box design.
func paletteFor(appearance Appearance) palette {
	if appearance == Light {
		return palette{
			background:    ui.Hex("#F7F7F8"),
			text:          ui.Hex("#1B1B1D"),
			textStrong:    ui.Hex("#1B1B1D"),
			textSecondary: ui.Hex("#6B6B70"),
			textTertiary:  ui.Hex("#8E8E94"),
			// The soft rung between the two: base.css's `--text-muted` sat
			// between `--text-secondary` and `--text-tertiary` too, and the
			// framework has a fourth slot for it (`ui.Theme.TextMuted`, the
			// muted label and the field placeholder). Raycast names three inks,
			// so this one is derived rather than quoted: it is the midpoint of
			// the contract's secondary `#6B6B70` and tertiary `#8E8E94`, a step a
			// 1x display resolves away from both (`docs/raycast-design.md` §1).
			// It must not collapse onto either: three of this table's own tests
			// read `--text-tertiary` and *not* `--text-muted`, and a duplicate
			// would make that an unanswerable question.
			textMuted: ui.Hex("#808087"),
			// P11-al 重定基线（用户实测 F2「整体红色线条太扎眼」的根因）：参考的
			// accent 一直是**蓝**。`--accent: #064aa6` / `--accent-contrast: #f7f9ff`
			// （`main:src/styles/base.css:942-943`），它的注释写着同一个 pastel blue
			// 在近黑上很亮、在白上几乎没有，所以浅色比深色深得多。旧值 → 新值：
			// `#E5484D` → `#064AA6`、`#F0555A` → `#0857C4`、`#CC3B40` → `#053C87`、
			// `#FFFFFF` → `#F7F9FF`。红在本仓只剩语义（`warning`）。
			accent:        ui.Hex("#064AA6"),
			accentHover:   ui.Hex("#0857C4"),
			accentPressed: ui.Hex("#053C87"),
			accentText:    ui.Hex("#F7F9FF"),
			stroke:        ui.RGBA(0, 0, 0, 0.09),
			// P11-z 重定基线：`ui.RGBA(229, 72, 77, 0.55)`（accent 0.55）→ 中性
			// 0.16。聚焦是明度差，不是色相（见 `Tokens.InputStrokeActive`）。
			strokeActive: ui.RGBA(0, 0, 0, 0.16),
			// P12-a 重定基线（docs/liquid-glass.md §1）：面的台阶抬到一眼看得见。
			// 旧值 → 新值：`0.05` → `0.07`。旧的 0.05 压在卡的 sheen 上只差 3~4 个亮度级，
			// 用户读到的就是「所有东西挤在同一个 z 平面上」（总目标第二条：空间感）。
			// 指针抬起的那一档跟着抬（0.075 → 0.105），两档之间的差仍是一档，不变成跳变。
			control:      ui.RGBA(0, 0, 0, 0.07),
			controlHover: ui.RGBA(0, 0, 0, 0.105),
			controlPress: ui.RGBA(0, 0, 0, 0.032),
			// A control's own edge is the keycap pair's hairline (§1: 0.06 light),
			// one step under the card's strong 0.09.
			controlEdge: ui.RGBA(0, 0, 0, 0.06),
			track:       ui.RGBA(0, 0, 0, 0.06),
			trackEdge:   ui.RGBA(0, 0, 0, 0.08),
			// P13-o 重定基线（docs/liquid-glass.md §1 的 0-9 行 / §3）：凹槽的底边线
			// 不能借 `LensRim`。浅色档的凹槽面是 232，环（`trackEdge` 0.08 黑）先把它
			// 压到 213，再叠 0.09 的白只剩 217——**比凹槽面还暗 15 级**，于是「洞里积光」
			// 读成「两条暗线的框」。旧值 → 新值：`0.09`（借 lensRim）→ **`0.70`**，
			// 实测底内线 221 → 243（−11 → **+11**），仍在地面 247 之下（凹槽不许比它
			// 被切进去的那张纸更亮）。参考实现浅色档本来就是近白
			// （`floter-main/src/styles/base.css:899`，`--glass-lens-rim-base: 0.9`）。
			trackRim: ui.RGBA(255, 255, 255, 0.70),
			field:    ui.RGBA(255, 255, 255, 0.5),
			hairline: ui.RGBA(0, 0, 0, 0.09),
			// Raycast's weaker hairline (`docs/raycast-design.md` §1).
			hairlineSoft: ui.RGBA(0, 0, 0, 0.055),
			// The row's neutral ladder: hover 0.04, chosen 0.06, pressed 0.08.
			rowHover:    ui.RGBA(0, 0, 0, 0.04),
			rowSelected: ui.RGBA(0, 0, 0, 0.06),
			rowPressed:  ui.RGBA(0, 0, 0, 0.08),
			keycapFace:  ui.RGBA(0, 0, 0, 0.05),
			keycapEdge:  ui.RGBA(0, 0, 0, 0.06),
			// P13-r（docs/liquid-glass.md §6.2b-5）：凹槽里的选中格在浅色档必须是
			// **纸自己的颜色**。0.85 的白压在卡面的凹槽（216）上复合到 249，页面是
			// 247 —— 抬起的面就是那张纸穿过了凹槽（现代 macOS/iOS 的滑块）。旧值：
			// 借 `ControlHover`（黑 0.105）→ 207，比凹槽面 232 还暗 24 级。
			thumb: ui.RGBA(255, 255, 255, 0.85),
			// `::selection` 在参考里就是 `--accent-tint`（`main:src/styles/base.css:1300`）：
			// 浅色 `rgba(6,74,166,0.10)`（`:946`）；`focus` 是 `--accent-ring`
			// `rgba(6,74,166,0.82)`（`:956`）。旧值 → 新值：两条都随 accent 换色。
			selection: ui.RGBA(6, 74, 166, 0.1),
			focus:     ui.RGBA(6, 74, 166, 0.82),
			warning:   ui.Hex("#9C1F18"),
			scrollbar: ui.RGBA(60, 60, 67, 0.24),
			opaque:    ui.RGBA(247, 247, 248, 0.995),
			fieldFill: ui.RGBA(250, 250, 251, 0.997),
			// The lens family, capped at 0.04: Raycast is a low-gloss design, so
			// the control's own rim, sheen and shade are a whisper rather than the
			// lens the old sheet drew (`docs/raycast-design.md` §4.1). The
			// `[data-glass]` stop still grades them through `--glass-lens-scale`.
			// P12-a 重定基线（docs/liquid-glass.md §1）：这四行曾是「无光泽」的上限
			// 0.04。新语言只要**一条**受光边：`LensRim` 是那条线（0.04 → 0.09，因为 0.04
			// 在 1x 屏上画不出「受光」，只画得出一条噪点），`LensEdge` 是它的对侧
			// （0.04 → 0.06，顶亮一分、底暗一分，控件才站起来）。`LensSheen` 与卡面的
			// sheen **不动**：面不许反光，轻便玻璃的反光是「一条线」，不是「一片渐变」。
			// 唯一材质不变量：alpha(LensRim) > alpha(LensSheen)。
			// P13-ah 重定基线（docs/liquid-glass.md §1 的 0-11 行 / §3）：浅色档的冠线一直
			// 只是**名义上**的。0.09 的白压在同一支的面（221 / 230）上只抬 **+3 / +2 级**，
			// 而深色档同一条 alpha 抬 **+19 级**——「这块面可以被碰」的记号在浅色档是一个
			// 看不见的像素。浅色的头顶空间是 221→255 = 34 级，此前只用了 3 级。旧值 →
			// 新值：浅色 `0.09` → **`0.24`**（`221×0.76+255×0.24 = 229.2`，**+8 级**，
			// 即 §1 的 8 级下沿）。为什么不是深色档那个 +19：浅色的 +19 需要 alpha 0.56，
			// 实测会撞两道已有的账——accent 面的色相（白线把最暗通道抬过 60/255，
			// 主 CTA 的顶行就不读作 accent 了）与「窗框 alpha 就是滑块的」（顶带平均
			// alpha 偏 +0.017，pin 的上界是 0.012）。0.24 是两道账之下的最大值。
			// 深色档**一个数不动**：那里 0.09 已经抬 +19 级。
			lensRim:     ui.RGBA(255, 255, 255, 0.24),
			lensSheen:   ui.RGBA(255, 255, 255, 0.04),
			lensEdge:    ui.RGBA(0, 0, 0, 0.06),
			sheenTop:    ui.RGBA(255, 255, 255, 0.04),
			sheenBottom: ui.RGBA(255, 255, 255, 0.01),
			// The focus step of the same pair: a lit face, still inside the cap.
			sheenTopActive:    ui.RGBA(255, 255, 255, 0.05),
			sheenBottomActive: ui.RGBA(255, 255, 255, 0.012),
			// P12-a 重定基线（docs/liquid-glass.md §3）：浮层的两层影加厚，受光边加亮。
			// 旧值 → 新值：ambient `0.18` → `0.22`、contact `0.10` → `0.12`、
			// `EdgeHighlight` 的深色档 `0.15` → `0.20`（浅色档 0.92 不动：白压白，浅色
			// 的受光边只能近白）。两者保持 Ambient > Contact，这是 base.css 的原始关系。
			shadowAmbient:       ui.RGBA(0, 0, 0, 0.22),
			shadowContact:       ui.RGBA(0, 0, 0, 0.12),
			edgeHighlight:       ui.RGBA(255, 255, 255, 0.92),
			edgeHighlightActive: ui.RGBA(255, 255, 255, 0.96),
			edgeShadow:          ui.RGBA(0, 0, 0, 0.07),
			edgeHighlightX:      ui.RGBA(255, 255, 255, 0.6),
			// `--keycap-highlight` / `--keycap-shadow` (`base.css:917-918`):
			// the terminal panel's own rim on Windows.
			keycapHighlight: ui.RGBA(255, 255, 255, 0.9),
			keycapShadow:    ui.RGBA(0, 0, 0, 0.09),
			// 浅色块 `--accent-edge` .36 / `--accent-edge-strong` .68 /
			// `--accent-halo` .24 / `--accent-glow` .20 / `--accent-tint-quiet` .065
			// （`main:src/styles/base.css:949-956`）：同一个 alpha 的蓝压在近白上几乎
			// 不动，所以浅色这几档比深色重。旧值（品牌红，.55/.85/.20/.12/.10）→ 新值。
			accentEdge:       ui.RGBA(6, 74, 166, 0.36),
			accentEdgeStrong: ui.RGBA(6, 74, 166, 0.68),
			accentHalo:       ui.RGBA(6, 74, 166, 0.24),
			// `--accent-glow`: the light card's focus ring.
			accentGlow: ui.RGBA(6, 74, 166, 0.2),
			// The accent as a quiet fill (`--accent-tint-quiet`).
			accentTintQuiet: ui.RGBA(6, 74, 166, 0.065),
			// The warm system-icon plane, the light appearance's amber (base.css
			// `--system-icon-surface` / `--system-icon-surface-hover`).
			iconSurface:      ui.RGBA(214, 118, 44, 0.13),
			iconSurfaceHover: ui.RGBA(214, 118, 44, 0.21),
			systemIconText:   ui.Hex("#a24d15"),
		}
	}
	return palette{
		background:    ui.Hex("#0E0E10"),
		text:          ui.Hex("#EDEDEF"),
		textStrong:    ui.Hex("#EDEDEF"),
		textSecondary: ui.Hex("#9A9AA0"),
		textTertiary:  ui.Hex("#6E6E76"),
		// The soft rung between them (see the light palette's note): the
		// midpoint of the contract's `#9A9AA0` and `#6E6E76`.
		textMuted: ui.Hex("#85858D"),
		// 深色块：`--accent: #8fb7ff`（`main:src/styles/base.css:675`）与
		// `--accent-contrast: #15171b`（`:676`）。旧值 → 新值：
		// `#F5544E` → `#8FB7FF`、`#FF6B62` → `#A3C4FF`、`#DC4640` → `#7AA7FF`、
		// `#1B1B1D` → `#15171B`。
		accent:        ui.Hex("#8FB7FF"),
		accentHover:   ui.Hex("#A3C4FF"),
		accentPressed: ui.Hex("#7AA7FF"),
		accentText:    ui.Hex("#15171B"),
		// The card's 1px edge is Raycast's *strong* hairline (`#FFFFFF` at
		// 0.09, `docs/raycast-design.md` §1); the hairline rung below is the
		// same number, because in Raycast's language a card's border and a
		// section's rule are one hairline seen at two widths, not two colours.
		stroke: ui.RGBA(255, 255, 255, 0.09),
		// P11-z 重定基线：`ui.RGBA(245, 84, 78, 0.55)`（accent 0.55）→ 中性
		// 0.16（见 `Tokens.InputStrokeActive`）。
		strokeActive: ui.RGBA(255, 255, 255, 0.16),
		// P12-a 重定基线（docs/liquid-glass.md §1）：面的台阶抬到一眼看得见。
		// 旧值 → 新值：`0.075` → `0.10`、`0.105` → `0.14`（理由见浅色那一支）。
		control:      ui.RGBA(255, 255, 255, 0.10),
		controlHover: ui.RGBA(255, 255, 255, 0.14),
		controlPress: ui.RGBA(255, 255, 255, 0.05),
		// A control's own edge is the keycap pair's hairline (§1: 0.07 dark),
		// one step under the card's strong 0.09.
		controlEdge: ui.RGBA(255, 255, 255, 0.07),
		track:       ui.RGBA(255, 255, 255, 0.09),
		trackEdge:   ui.RGBA(255, 255, 255, 0.075),
		// 深色档的凹槽底边**不动**（P13-o）：这里的环是白的 0.075，两条白线相加，
		// 实测底内线 57 对面 36（+21）——深色的凹槽本来就读得出「底下有光」。
		// 深色缺的那一半是面·对侧，P13-n 已经用 `lensEdge` 0.20 补上了。
		trackRim: ui.RGBA(255, 255, 255, 0.09),
		field:    ui.RGBA(255, 255, 255, 0.03),
		hairline: ui.RGBA(255, 255, 255, 0.09),
		// Raycast's weaker hairline: the rule between rows and inside a block
		// (0.055, `docs/raycast-design.md` §1).
		hairlineSoft: ui.RGBA(255, 255, 255, 0.055),
		// The row's own neutral ladder, in Raycast's numbers: hover 0.045, the
		// chosen row 0.085, a pressed row 0.11. None of them is the accent —
		// the list's selection is a neutral fill.
		rowHover:    ui.RGBA(255, 255, 255, 0.045),
		rowSelected: ui.RGBA(255, 255, 255, 0.085),
		rowPressed:  ui.RGBA(255, 255, 255, 0.11),
		// The keycap's plate: the fill and its hairline.
		keycapFace: ui.RGBA(255, 255, 255, 0.075),
		keycapEdge: ui.RGBA(255, 255, 255, 0.07),
		// P13-r：深色档的选中格**不动**（= `controlHover` 的白 0.14，实测 86 对面
		// 58，+28，本来就是「抬起来」）。深色没有更亮的纸可借（页面是 39），所以这一档
		// 与浅色档不同值不是不一致，是同一条规则在两种底上的唯一解。
		thumb:     ui.RGBA(255, 255, 255, 0.14),
		selection: ui.RGBA(143, 183, 255, 0.13),
		focus:     ui.RGBA(122, 167, 255, 0.78),
		warning:   ui.RGBA(255, 178, 164, 0.82),
		scrollbar: ui.RGBA(238, 239, 242, 0.15),
		opaque:    ui.RGBA(20, 20, 22, 0.99),
		fieldFill: ui.RGBA(28, 28, 31, 0.99),
		// The lens family, capped at 0.04: Raycast is a low-gloss design, so
		// the control's own rim, sheen and shade are a whisper rather than the
		// lens the old sheet drew (`docs/raycast-design.md` §4.1). The
		// `[data-glass]` stop still grades them through `--glass-lens-scale`.
		// P12-a 重定基线（docs/liquid-glass.md §1）：这四行曾是「无光泽」的上限 0.04，
		// 新语言只要一条受光边（见浅色那一支的完整说明）：`LensRim` 0.04 → 0.09、
		// `LensEdge` 0.04 → 0.06、`LensSheen` 不动。
		//
		// P13-n 重定基线（§1 的 0-9 行 / §3）：深色档的**对侧**一直读不出来。深色的面是
		// 38，一条 0.06 的黑线压在它上面只降 **2 级**（浅色那一支同一条线降 13 级）——
		// 于是深色的 E1 只有「顶亮」一半，「底暗」那一半是名义上的。旧值 → 新值：
		// `lensEdge 0.06 → 0.20`（实测面 39 的底内线 37 → 31，−2 → −8；面 65 是 −4 → −13）。
		// 黑线在 accent 面上会被色相吃掉（§6.2 的 2-6），所以它不会把强调面洗成「类玻璃边」。
		lensRim:     ui.RGBA(255, 255, 255, 0.09),
		lensSheen:   ui.RGBA(255, 255, 255, 0.04),
		lensEdge:    ui.RGBA(0, 0, 0, 0.20),
		sheenTop:    ui.RGBA(255, 255, 255, 0.04),
		sheenBottom: ui.RGBA(255, 255, 255, 0.01),
		// The focus step of the same pair: a lit face, still inside the cap.
		sheenTopActive:    ui.RGBA(255, 255, 255, 0.05),
		sheenBottomActive: ui.RGBA(255, 255, 255, 0.012),
		// Raycast's card cast: `0 18px 48px rgba(0,0,0,.55)` and
		// `0 1px 2px rgba(0,0,0,.40)` (`docs/raycast-design.md` §4.1).
		// P12-a 重定基线（docs/liquid-glass.md §3）：旧值 → 新值 ambient `0.55` → `0.62`、
		// contact `0.40` → `0.44`、`EdgeHighlight` `0.15` → `0.20`（见浅色那一支）。
		shadowAmbient: ui.RGBA(0, 0, 0, 0.62),
		shadowContact: ui.RGBA(0, 0, 0, 0.44),
		edgeHighlight: ui.RGBA(255, 255, 255, 0.20),
		// The focused step (`base.css:633`): the rim nudged brighter while the
		// card holds the keyboard, the same edge the stroke's active rung is.
		// P12-a：随静息档一起抬一档（`0.18` → `0.24`），保持「聚焦比静息亮」。
		edgeHighlightActive: ui.RGBA(255, 255, 255, 0.24),
		edgeShadow:          ui.RGBA(0, 0, 0, 0.28),
		edgeHighlightX:      ui.RGBA(255, 255, 255, 0.04),
		// `--keycap-highlight` / `--keycap-shadow` (`base.css:657-658`): the
		// terminal panel's own rim on Windows, a step quieter than the edge.
		keycapHighlight:  ui.RGBA(255, 255, 255, 0.07),
		keycapShadow:     ui.RGBA(0, 0, 0, 0.28),
		accentEdge:       ui.RGBA(143, 183, 255, 0.3),
		accentEdgeStrong: ui.RGBA(143, 183, 255, 0.72),
		accentHalo:       ui.RGBA(143, 183, 255, 0.18),
		// `--accent-glow`: the same ring, quieter. It is not drawn on the
		// launcher card (`docs/raycast-design.md` §1): the launcher's selected
		// row is neutral and the card carries no accent glow.
		accentGlow: ui.RGBA(143, 183, 255, 0.08),
		// The accent as a fill, at the quietest rung: Raycast's `rgba(245, 84,
		// 78, 0.14)`, used by the three places §1 allows the accent a face.
		accentTintQuiet: ui.RGBA(143, 183, 255, 0.085),
		// The warm system-icon plane, the dark appearance's amber (base.css
		// `--system-icon-surface` / `--system-icon-surface-hover`), and its
		// own ink (`--system-icon-text`): a pale warm peach, not the warning
		// red-grey.
		iconSurface:      ui.RGBA(255, 176, 132, 0.115),
		iconSurfaceHover: ui.RGBA(255, 176, 132, 0.2),
		systemIconText:   ui.RGBA(255, 209, 178, 0.82),
	}
}

// contrast returns the palette a desktop that asked for more contrast gets:
// base.css's `@media (prefers-contrast: more)` block, which swaps three
// tokens per appearance. `--stroke-contrast` is the card and floating-chrome
// border (the old build's `.collapsed-card`, `.settings-card`,
// `.extensions-drawer`, `.app-toast` …), `--stroke-contrast-soft` the inner
// edge of a grouped row (`.settings-option`, `.settings-section__card`), and
// `--text-placeholder-contrast` the field placeholder.
//
// The placeholder token lands on textMuted, which is the one lever the
// framework exposes for it: its editor paints a placeholder in
// `LineNumber`, and it derives that as `TextMuted.Alpha(0.75)`
// (mygo's ui/editor/theme.go). The same step also strengthens every quiet
// label one rung, which is what the old build's `color: var(--text-primary)`
// on the toast/notice copies asked for.
//
// P11-aa（用户实测 F2 遗留的可读性半边）：这个分支以前只抬**线**（`stroke`、
// `controlEdge`）和一个 placeholder（`textMuted`），把真正承载内容的二级/三级墨留在
// 原处——浅色 3.04:1、深色 3.82:1，低于 AA 的 4.5:1。开了高对比度的用户于是拿到
// 一张「线更清楚、字还是一样的淡」的界面。现在四档墨在**面板底**与**选中行填充**
// 两个表面上都过 4.5:1，且阶梯次序（Text > TextSecondary > TextMuted >
// TextTertiary）在两个表面上都保持：
//
//	浅色  #4E4E54 7.72 / 6.75 · muted 5.68 / 5.36 · #66666C 5.33 / 4.66
//	深色  #B0B0B8 8.95 / 7.37 · muted 6.27 / 5.72 · #8A8A92 5.63 / 4.64
//
// `textMuted` 本来就是这两支里最够读的一条，不动它。**默认外观一个字节都不改**：
// 安静档三档同时推到 4.5:1 在 `#F7F7F8` 上放不进一条 0.02 宽的亮度窗口（见
// `docs/rounds/P11-aa-spec.md` 附注），那是拍板项，不是这一支的事。
func (p palette) contrast(app Appearance) palette {
	if app == Light {
		p.stroke = ui.RGBA(0, 0, 0, 0.5)
		p.controlEdge = ui.RGBA(0, 0, 0, 0.36)
		p.textMuted = ui.RGBA(28, 28, 32, 0.68)
		p.textSecondary = ui.Hex("#4E4E54")
		p.textTertiary = ui.Hex("#66666C")
	} else {
		p.stroke = ui.RGBA(255, 255, 255, 0.42)
		p.controlEdge = ui.RGBA(255, 255, 255, 0.34)
		p.textMuted = ui.RGBA(242, 243, 245, 0.58)
		p.textSecondary = ui.Hex("#B0B0B8")
		p.textTertiary = ui.Hex("#8A8A92")
	}
	return p
}

// palette is one appearance's token set, kept private: Tokens is the public
// shape, and the framework theme's fields are filled from here.
type palette struct {
	background, text, textStrong, textMuted, textSecondary, textTertiary              ui.Color
	accent, accentHover, accentPressed, accentText                                    ui.Color
	stroke, strokeActive                                                              ui.Color
	control, controlHover, controlPress, controlEdge                                  ui.Color
	track, trackEdge, trackRim, field, hairline, selection, focus, warning, scrollbar ui.Color
	opaque, fieldFill                                                                 ui.Color
	lensRim, lensSheen, lensEdge                                                      ui.Color
	sheenTop, sheenBottom                                                             ui.Color
	sheenTopActive, sheenBottomActive                                                 ui.Color
	shadowAmbient, shadowContact                                                      ui.Color
	edgeHighlight, edgeShadow, edgeHighlightX                                         ui.Color
	edgeHighlightActive                                                               ui.Color
	keycapHighlight, keycapShadow                                                     ui.Color
	accentEdge                                                                        ui.Color
	accentEdgeStrong                                                                  ui.Color
	accentHalo                                                                        ui.Color
	accentGlow                                                                        ui.Color
	accentTintQuiet                                                                   ui.Color
	iconSurface, iconSurfaceHover                                                     ui.Color
	systemIconText                                                                    ui.Color
	// The Raycast surface set (`docs/raycast-design.md` §1/§4): the weak
	// hairline and the launcher row's neutral three-rung state ladder, plus
	// the pressable keycap's plate.
	hairlineSoft           ui.Color
	rowHover               ui.Color
	rowSelected            ui.Color
	rowPressed             ui.Color
	keycapFace, keycapEdge ui.Color
	// The segmented control's chosen slot (see `Tokens.Thumb`).
	thumb ui.Color
}

// round1 rounds to one decimal, so a scaled metric stays a value a CSS
// author could have written and the tests can compare exactly.
func round1(v float32) float32 {
	return float32(int(v*10+0.5)) / 10
}

// scaleAlpha multiplies a token's own alpha by a factor, keeping its colour.
// It is the `--glass-lens-scale` rung: base.css writes every lens layer as
// `calc(var(--glass-lens-*-base) * var(--glass-lens-scale))`, so the stop
// grades the control lens and nothing else.
func scaleAlpha(c ui.Color, factor float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255*factor)
}

// LitEdgeFloor is how much of its token the lit edge keeps in the contrast
// mode. `--glass-lens-scale` already thins the lens (`frosted` 0.5), and the
// contrast block multiplies that by another 0.6, so a rim of 0.09 reached
// 0.027 — five levels of white on a dark face (65), i.e. the line P12-a raised
// off 0.04 was quietly back under the 1x threshold on the one appearance
// setting whose whole point is "show me the edges". A quarter off is a quieter
// edge; more than that is no edge.
const LitEdgeFloor = 0.75

// litEdge is the lit edge's alpha at a glass stop. The stop grades the lens
// (`--glass-lens-scale` 0.5 / 1 / 1.6) and the lit edge rides it — except in
// the contrast mode, where the second damping would take it under the 1x
// threshold (see LitEdgeFloor). The dark rim at the plain `frosted` stop is
// deliberately left alone: 10 of 255 levels is a quiet edge, and changing it
// would break the stop ladder the reference draws (`P13-ag` report).
//
// P13-ag：`LensRim` 0.09 是「1x 屏上真的看得见」那条线（P12-a 把 0.04 抬上来的
// 就是它）。可它此前跟着 `LensScale` **双向**缩放，于是同一句话在薄档上被推翻：
// `frosted`（0.5）落到 0.045、对比度模式下再乘 0.6 → 0.027，白线压在深色面 65 上
// 只抬 5~8 级，正是 §1 说的「画不出」那一档。合约自己把两件事分开写过
// （`glassmap.go` 的 `ContrastLensScale` 注：被压低的是 **sheen**，那颗
// 「装饰性折射」），而 §3 说受光边是 E1 的**结构线**——结构线不该因为用户选了更薄的
// 材质而消失。所以：更厚的档位照旧放大，更薄的档位保持 token 本身。
//
// `LensSheen` / `LensEdge` / `TrackRim` 仍然走 `scaleAlpha`：sheen 是装饰（该缩），
// 另外两条是别的面/别的方向上的线，各有自己的账（P13-n / P13-o）。
func litEdge(c ui.Color, factor float32, contrast bool) ui.Color {
	// `off` (factor 0) really has no lens: the stop's whole point is that the
	// control loses its material, so the floor must not resurrect a line the
	// stop removed.
	if factor <= 0 {
		return scaleAlpha(c, 0)
	}
	if contrast {
		return scaleAlpha(c, max(factor, LitEdgeFloor))
	}
	return scaleAlpha(c, factor)
}

// capAlpha lowers a colour's alpha to at most max, keeping its colour: the
// sheen may not be heavier than the frame it sits on, or a thin frame's top
// would composite denser than the slider's own alpha.
func capAlpha(c ui.Color, max float32) ui.Color {
	if a := float32(c.A) / 255; a > max {
		return ui.RGBA(c.R, c.G, c.B, max)
	}
	return c
}

// blendOpaque lays over on base at over's alpha and returns the opaque
// result, for the one lift that has to cover a color the framework paints
// behind it.
func blendOpaque(base, over ui.Color) ui.Color {
	a := float64(over.A) / 255
	mix := func(b, o uint8) uint8 {
		return uint8(math.Round(float64(b)*(1-a) + float64(o)*a))
	}
	return ui.RGBA(mix(base.R, over.R), mix(base.G, over.G), mix(base.B, over.B), 1)
}

// FaceOver returns face as it composites over base, opaque: the colour a
// veil laid over that surface must use.
//
// A face token is often carried as a *thin* over-frame fill whose alpha is the
// transparency slider's step (`Sunken`, `Control`): painted straight onto a
// fade, `ui.Color.Alpha` multiplies the fade's own 0.85 by the token's, and the
// veil comes out at a fraction of the weight it asks for (measured on the
// extension run-output block: 0.85 × Sunken's 0.17 → the top line of the fade
// still read 90% of the ink it was meant to drown — see P13-at,
// `docs/liquid-glass.md` §6.17). Compositing first keeps the veil's alpha its
// own.
//
// A surface painted as a chain of such layers (ground, pane, card, block) needs
// one call per layer, innermost last: each call assumes an opaque base.
func FaceOver(base, face ui.Color) ui.Color { return blendOpaque(base, face) }

// FaceFill is the card's face at the frame's own alpha: the tint colour as the
// two stops of its top-down gradient, at the alpha that **composites with the
// stop's sheen back to the frame's alpha**. The sheen is painted over it as
// the second layer (`Tokens.SheenTop` / `SheenBottom`, `View`), and the base
// is chosen so `1-(1-base)(1-sheen) == alpha`: a plain tint under a sheen
// layer would composite to `1-(1-alpha)(1-sheen)`, denser than the slider's
// own alpha — in the light appearance up to +0.11 in the result band at the
// default 47%, which is what the user read as 「开玻璃透明底失效」. Here the
// frame's alpha is the transparency slider's on every stop, while the stop
// still grades the lit face's colour (the sheen the scene paints itself).
func (t Tokens) FaceFill(alpha float32) (top, bottom ui.Color) {
	return t.FaceFillAt(alpha, false)
}

// FaceFillAt is FaceFill for a named focus state: `focus` picks the reference's
// raised `--sheen-*-active` pair, so the fill under it is compensated against
// *that* sheen and the card's frame alpha stays the slider's while the lit face
// lifts (`.collapsed-card:focus-within`).
func (t Tokens) FaceFillAt(alpha float32, focus bool) (top, bottom ui.Color) {
	bg := ui.RGB(18, 19, 22)
	if t.Theme != nil {
		bg = t.Theme.Background
	}
	sheenTop, sheenBottom := t.SheenAt(alpha)
	if focus {
		sheenTop, sheenBottom = t.SheenAtFocus(alpha)
	}
	return ui.RGBA(bg.R, bg.G, bg.B, baseAlpha(alpha, sheenTop)),
		ui.RGBA(bg.R, bg.G, bg.B, baseAlpha(alpha, sheenBottom))
}

// SheenAt is the frame's lit face capped at `alpha`, the alpha the face is
// actually painted at: the layer FaceFill compensates for. The tokens are
// already capped at the *frame's* alpha (see the `sheenTop` note in
// ForPlatform), which is the right cap for the window's own card; a surface
// whose face carries a thinner alpha — the terminal's, which shares the
// terminal slider with its panel (glassmap.Spec.TerminalFrameAlpha) — has to
// cap it again, or a light appearance's 0.66 sheen would paint the whole face
// at its own alpha and the frame would be denser than the slider asked for.
func (t Tokens) SheenAt(alpha float32) (top, bottom ui.Color) {
	return capAlpha(t.SheenTop, alpha), capAlpha(t.SheenBottom, alpha)
}

// SheenAtFocus is SheenAt for the card that has the keyboard: the reference's
// `--sheen-*-active` pair, capped at the alpha the face is painted at for the
// same reason the resting pair is (see SheenAt).
//
// The reference lifts the card's lit face on `:focus-within` — the same rule
// that lights its stroke (`main:src/styles/launcher.css:136-144`:
// `border-color: var(--input-stroke-active); background:
// linear-gradient(180deg, var(--sheen-top-active), var(--sheen-bottom-active)),
// var(--launcher-tint)`). The port carried the stroke half of that rule and not
// the face half, so the launcher's card looked identical whether or not it had
// the keyboard — the one focus cue the design spends on the whole panel was
// half missing on all three platforms.
func (t Tokens) SheenAtFocus(alpha float32) (top, bottom ui.Color) {
	return capAlpha(t.SheenTopActive, alpha), capAlpha(t.SheenBottomActive, alpha)
}

// baseAlpha is the fill alpha a sheen of alpha s composites over to frame:
// `1-(1-base)(1-s) == frame`, clamped so a sheen heavier than the frame at the
// slider's bottom end cannot ask for a negative fill.
func baseAlpha(frame float32, sheen ui.Color) float32 {
	s := float32(sheen.A) / 255
	if s >= 1 {
		return 0
	}
	base := (frame - s) / (1 - s)
	if base < 0 {
		return 0
	}
	if base > frame {
		return frame
	}
	return base
}

// Lens draws the control lens on a pane: the top rim, the broad sheen and the
// bottom shade, then the shared pane edge — base.css's `--glass-lens-stack`,
// which every control on a surface carried as its `--elev-0`. The framework
// leaves a control as a flat tint plus a border; these four inset layers are
// what make the same box read as a pane of glass. They stack in the order
// added, so the rim lands last, over the sheen's fade, as the old rule's
// declaration order had it.
func (t Tokens) Lens(e ui.Element) ui.Element {
	e.InsetShadow(0, 1, 0, 0, t.LensRim)
	e.InsetShadow(0, 6, 10, -6, t.LensSheen)
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 0, 0, 1, t.ControlEdge)
	return e
}

// LitFace draws the **E1** material on a face: one 1px lit edge along its top,
// one 1px shade along its bottom, and nothing else (`docs/liquid-glass.md` §2/§3).
//
// It is deliberately *not* `Lens`: `Lens` is the four-layer control lens (rim,
// broad sheen, bottom shade, and a `ControlEdge` ring on all four sides), and
// the ring is the 「画框分层」 this generation retires — a chosen row's contract
// says 「no border, no ring, no glow」 (`docs/raycast-design.md` §4.3). E1 is a
// **面** that stands on its ground by three things only: the face step itself,
// the lit line on top, and the shade under it.
//
// Why the pair rather than the lit line alone: on the light appearance the
// face is itself near-white (`Control` 0.07 of black over the sunken panel), so
// the lit line has very little headroom there — the 0.09 white it carried until
// P13-ah lifted the pixel by **1~3 levels** (§3 的末尾), which is why the
// counter-side was doing all the work: 顶亮一分、底暗一分，控件才站起来 (§3).
// P13-ah raised the light rim to 0.24 (+8 levels, measured: 221 → 229 on the
// session icon button; the dark appearance's same 0.09 lifts +19, unchanged) —
// the crown now speaks, but the shade is still half of the sentence. So both
// are painted, and a surface that shows only one of them is not this material.
//
// E1 casts no shadow (`§2 判据 2`) — this function adds none.
func (t Tokens) LitFace(e ui.Element) ui.Element {
	e.InsetShadow(0, 1, 0, 0, t.LensRim)
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	return e
}

// DimFace draws the material of a control that is **there but cannot be
// touched** (`docs/liquid-glass.md` §6.2b-6): the counter-side shade of an E1
// face, and **no lit edge**.
//
// The lit edge is not decoration — it is the mark of a surface the pointer can
// land on (§3: 「受光边是「这块面可以被碰」的记号」). A disabled control keeps its
// place in the stack — its face still steps off the ground and its bottom still
// shades, so it still occludes what is under it — but the light is off, and
// that is what a user reads as 「不可用」.
//
// The face itself is the caller's and stays `Control`: the framework's painter
// already multiplies a disabled subtree by **0.5** (`ui/paint.go:257-259`), so
// a disabled pane's face lands at 238 on the light sheet (ground 247) and 27 on
// the dark one (ground 15) — half of the resting step, which is exactly the
// dim rung this material needs. The settings page used to add its own
// `Opacity(0.4)` **on top of** that 0.5, and the product was therefore drawing
// disabled controls at an effective 0.2: the face step fell to 4 levels and the
// E1 pair vanished into noise (measured 247 → 243 and 14 → 19). Two fades
// multiplied is not a design — it is one fade drawn twice.
//
// So: one fade (the framework's), and the material says the rest by going
// unlit. The shade is what is left of the pair.
func (t Tokens) DimFace(e ui.Element) ui.Element {
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	return e
}

// Tip is `ui.Element.Tooltip` wearing this design's floating face
// (`docs/liquid-glass.md` §2 E2 / §6.20).
//
// The framework's own look is the one surface this design never reached:
// `ui/tooltip.go:99-105` paints `Radius(Space(1.25))` — 5px, which is the
// *keycap*'s corner, not a floating pane's 8 — the inverse pair as an
// **opaque** fill, one `0 2px 8px` blur at 0.2 as its only shadow, and no lit
// edge at all. On the dark appearance `Inverse` is left zero and falls back to
// `Text` (`ui/theme.go:37-40`), so every tip in the app was a **white chip**
// over dark glass: the system-tooltip look this generation retires.
//
// E2 is three things (§2 判据 3): the `Float` face, one 1px `EdgeHighlight`
// lit edge along the top, and two shadows (ambient over contact). This paints
// exactly those, and keeps the framework's behaviour untouched — the 0.6s
// rest, the warm 0.4s hand-off between tips, Escape to dismiss, the anchor and
// its gap, the 80u cap, and the accessibility description (the anchor keeps
// the words as its description, as `Tooltip` did).
//
// The two shadows are the `--elev-3-compact` pair at scale 1 rather than
// `ElevThrow`: `--elev-shadow-scale` follows `main_opacity`, which is the
// *window frame's* own glass — a tip is not part of that frame, it is painted
// by the app over its own content at whatever frame alpha the user chose.
func (t Tokens) Tip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor.Description(text)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.MaxWidth(t.Theme.Space(80)).
			Padding(t.Theme.Space(1.25), t.Theme.Space(2)).
			Radius(t.RadiusSM).Border(1, t.InputStroke).
			Background(t.Float).TextColor(t.Theme.Text).
			FontSize(t.Theme.FontSize - 1)
		tip.Shadow(0, t.Theme.Space(1.5), t.Theme.Space(4.5), 0, ui.RGBA(0, 0, 0, 0.30))
		tip.Shadow(0, t.Theme.Space(0.25), t.Theme.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.20))
		tip.InsetShadow(0, 1, 0, 0, t.EdgeHighlight)
		tip.Children(func() { ui.Text(c, text) })
	})
	return anchor
}

// LensInset draws the recessed lens on a groove: base.css's
// `--glass-lens-stack-inset`, the *same* lens read backwards. The light pools
// along the bottom edge of the groove and the top edge is the dark one — the
// exact opposite of Lens — with the inner slot shadow crouched under the top
// edge and the track's own quieter hairline closing the loop. It is what makes
// a switch or a segmented control's track read as a recess in the same sheet
// of glass the raised panes sit on, rather than as a flat darker box, and
// `--glass-lens-scale` grades it with the glass stop exactly as Lens is.
//
// The layers are added last-on-top (see `InsetShadow`), so the lit rim goes in
// last: base.css lists it first, and CSS paints the first shadow on top.
//
// The lit rim is `TrackRim`, not `LensRim` (P13-o, `docs/liquid-glass.md` §3):
// on the light appearance the groove's own hairline (black 0.08) sits on the
// same row and pulls the fill down 19 levels, so a 0.09 rim that arrives on top
// of it lands *below* the groove's fill (measured 232 → 217, −15) and the
// recess read as a frame of two dark lines instead of a hole with light in it.
// 旧值 → 新值：浅色 `LensRim 0.09` → `TrackRim 0.70`（实测底内线 221 → 243）；
// 深色两者同值 0.09，不动（那里两条线都是白的，实测 +21）。
func (t Tokens) LensInset(e ui.Element) ui.Element {
	e.InsetShadow(0, 0, 0, 1, t.TrackEdge)
	e.InsetShadow(0, 1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 1, 2, 0, t.ShadowContact)
	e.InsetShadow(0, -1, 0, 0, t.TrackRim)
	return e
}

// FieldSlot draws the inset a text or number field is cut into the glass with:
// base.css's `--glass-field-shadow` (`:524-527` dark, `:887-888` light) — the
// window's contact inset crouched under the top edge plus the lens's own rim
// (and, on dark glass, its bottom shade). It is the one layer the plugin
// configuration sheet's 28u controls (`.plugin-config-text` /
// `.plugin-config-number`, `plugin-config.css:97-118`) carry that a bare
// `--glass-field` fill does not: without it the slot reads as a flat brighter
// pane rather than as a slot cut *into* the glass. The stack consumes the lens
// family, so `[data-glass]` grades it exactly as it grades Lens and LensInset.
//
// Deprecated: P8-c 收掉了它的最后一处调用（`docs/raycast-design.md` §0(b)）。
// 这个叠层在浅色下的白边是 **0.85**，而合约 §1 把光泽（rim/sheen/lens）一律
// 压到 alpha ≤ 0.04——上限的 21 倍。Raycast 的输入框是一档浅面 + 一条发丝边，
// 没有内侧白边，而本叠层在图上就是「输入框像一块玻璃砖」。设置页的字段现在走
// `theme.Tokens.Control` + `ControlEdge` 一档（`settingsui.fieldFace` /
// `sheetFieldSlot` / `configMultiselectBox`）。函数体保留不删：本次只做视觉
// 收敛，删除留给下一次清理轮（仓库层面已 grep 确认无其它调用点）。
//
// The layers append one at a time through InsetShadow, so the first layer
// lands under the later ones; the reference lists the contact shadow first and
// CSS paints the first shadow on top, so it goes in last — the same reversal
// LensInset does. Light glass has no bottom shade in its field shadow, and its
// rim is a fixed near-opaque white rather than the graded lens token.
func (t Tokens) FieldSlot(e ui.Element) ui.Element {
	if t.Appearance == Light {
		e.InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.85))
		e.InsetShadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
		return e
	}
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 1, 0, 0, t.LensRim)
	e.InsetShadow(0, 1, 2, 0, t.ShadowContact)
	return e
}

// ANSI is the sixteen program colours a command's own output is drawn with,
// per appearance: base.css's `--ansi-black` through `--ansi-bright-white`.
//
// A CLI's stdout carries SGR sequences — `git diff`, `ls --color` — and the
// text form has to read them as a terminal would (the old build's R65:
// 「有些是带格式的终端字符」). The colours are the app's own rather than the
// user's terminal palette, which is what the old build's CSS variables were:
// the sheet and the terminal each have their own sixteen, and neither drifts
// with the other's settings.
func ANSI(appearance Appearance) [16]ui.Color {
	if appearance == Light {
		return [16]ui.Color{
			ui.Hex("#1d1f21"), ui.Hex("#c72e3c"), ui.Hex("#19733a"), ui.Hex("#7a5c00"),
			ui.Hex("#1659b7"), ui.Hex("#9c2ca0"), ui.Hex("#007c86"), ui.Hex("#535762"),
			ui.Hex("#767b86"), ui.Hex("#d63d4b"), ui.Hex("#1a6e38"), ui.Hex("#7a5900"),
			ui.Hex("#246fd1"), ui.Hex("#ac38a9"), ui.Hex("#006e7a"), ui.Hex("#2c3037"),
		}
	}
	return [16]ui.Color{
		ui.Hex("#4b5563"), ui.Hex("#f38ba8"), ui.Hex("#a6e3a1"), ui.Hex("#f9e2af"),
		ui.Hex("#89b4fa"), ui.Hex("#f5c2e7"), ui.Hex("#94e2d5"), ui.Hex("#cdd6f4"),
		ui.Hex("#6b7280"), ui.Hex("#f38ba8"), ui.Hex("#a6e3a1"), ui.Hex("#f9e2af"),
		ui.Hex("#89b4fa"), ui.Hex("#f5c2e7"), ui.Hex("#94e2d5"), ui.Hex("#eef2ff"),
	}
}

// ANSIColor resolves one program colour for an appearance: the sixteen named
// ones by index, the 256-colour cube and 24-bit forms by value.
func ANSIColor(appearance Appearance, kind int, n uint8, r, g, b uint8) ui.Color {
	switch kind {
	case 1: // named
		palette := ANSI(appearance)
		if int(n) < len(palette) {
			return palette[n]
		}
		return ui.Transparent
	case 2: // the 256-colour cube
		return cubeColor(n)
	case 3: // truecolour
		return ui.RGBA(r, g, b, 1)
	}
	return ui.Transparent
}

// cubeColor resolves a 256-colour index: the first sixteen are the named
// colours, then a 6×6×6 cube, then a grey ramp.
func cubeColor(n uint8) ui.Color {
	if n < 16 {
		// The named colours again; the caller resolves them by appearance.
		return ui.Transparent
	}
	if n < 232 {
		index := int(n) - 16
		steps := [6]uint8{0, 95, 135, 175, 215, 255}
		return ui.RGBA(steps[index/36], steps[(index/6)%6], steps[index%6], 1)
	}
	grey := uint8(8 + 10*(int(n)-232))
	return ui.RGBA(grey, grey, grey, 1)
}
