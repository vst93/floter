package shell

import (
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/theme"
)

// The app's one notice: a sentence raised by an external trigger, shown where
// the user is even when the launcher is not the surface on screen.
//
// The old build's toast stack was that surface — a sibling of the mode shell,
// so it stayed put across a surface switch — and a refusal raised by a
// `floter://` link is exactly the case the docked launcher row cannot serve:
// the link lands on the settings page, and that row is drawn under a list the
// user is not looking at. The framework draws these
// (`ui.Context.AddToast`), so the app keeps no second toast stack of its own.

// refusalWindow is how long one refusal stays quiet after it has been raised
// once: the old build's FAILURE_NOTIFY_DEDUP_MS. The trigger is external and
// can repeat without anybody pressing anything — a page retrying a broken
// link, a shell loop — so one bad link must not fill the window with copies of
// the same sentence, while the same refusal coming back later is news again.
const refusalWindow = 30 * time.Second

// refusalKey is the one key the refusals share: the old build sent a single
// dictionary key (`settings.deepLinkRejected`), so every refused link is the
// same failure and coalesces onto the same window.
const refusalKey = "deep-link-rejected"

// refusalNotice is how long the refusal sentence stays readable: the old
// build's error-toast window (its `TOAST_DISMISS_MS.error`), longer than the
// framework's four-second default because it carries something the user may
// want to act on.
const refusalNotice = 8 * time.Second

// warningNotice is the advisory window: the old build's
// `TOAST_DISMISS_MS.warning`, one step shorter than the error's because a
// warning says what will not happen yet rather than what failed.
const warningNotice = 6 * time.Second

// successNotice is the confirmation window: the old build's
// `TOAST_DISMISS_MS.success`, the shortest of the three because the news is
// good and needs no time to digest.
const successNotice = 4 * time.Second

// refusalGate coalesces an automatic, externally triggered failure per key,
// the rule the old build's `createFailureDeduper` implemented. The clock is
// injectable so a test can drive a burst without waiting wall-clock time.
type refusalGate struct {
	mu     sync.Mutex
	window time.Duration
	now    func() time.Time
	at     map[string]time.Time
}

func newRefusalGate(window time.Duration, now func() time.Time) *refusalGate {
	return &refusalGate{window: window, now: now, at: map[string]time.Time{}}
}

// allow reports whether key may be raised now, and if so marks it raised: the
// second and later calls inside the window return false, the first call after
// it elapses returns true again.
func (g *refusalGate) allow(key string) bool {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if last, ok := g.at[key]; ok && now.Sub(last) < g.window {
		return false
	}
	g.at[key] = now
	return true
}

// queueNotice remembers a toast for the next frame. The view cannot be built
// from a background goroutine, so a notice is queued and drawn by View; the
// one id means a second notice replaces the first rather than stacking beside
// it.
func (a *App) queueNotice(message string, window time.Duration) {
	a.queueToast(ui.Toast{
		ID:      "floter-notice",
		Title:   message,
		Type:    "error",
		Timeout: window,
	})
}

// queueToast remembers one of the app's own toasts for the next frame, drawn
// by `toastViewport`: a background task cannot build a view, so it queues the
// toast and asks for a frame. The caller owns the id and the kind, so notices
// of different sorts do not replace one another.
func (a *App) queueToast(toast ui.Toast) {
	a.noticeMu.Lock()
	a.notices = append(a.notices, toast)
	a.noticeMu.Unlock()
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// warnNotice is the app's advisory notice: the old build's
// `onNotify("warning", …)`, raised for something the user should know about
// but that did not fail. The extension form's missing toolchain is the case
// that needed it — the entry is saved either way, so the sentence has to
// arrive wherever the user is, not in the launcher's docked row. It carries
// no `Type`: the framework's default toast look paints one tone whatever the
// type says, and the old build's warning tone has nowhere to land in it.
func (a *App) warnNotice(message string) {
	if message == "" {
		return
	}
	a.queueNotice(message, warningNotice)
}

// successNotice is the app's confirmation notice: the old build's
// `onNotify("success", …)`, raised for something that completed without a
// failure. The About page's deep-link copy is the raiser the reference had —
// it called `onCopied` so the app's toast stack said "Link copied" as well as
// the button — and the app's viewport paints this kind with the neutral strong
// ink and the check mark (`toastMark("success")`): P8-e 把它的色相收回
// 中性，成功由 ✓ 字形与文案说。
func (a *App) notifySuccess(message string) {
	if message == "" {
		return
	}
	a.queueToast(ui.Toast{
		ID:      "floter-notice",
		Title:   message,
		Type:    "success",
		Timeout: successNotice,
	})
}

// panelFeedback tells the surface that is *on screen* about a completion or a
// failure, in that surface's own panel.
//
// Each surface answers where it can: the launcher has its docked row
// (launcher.go's feedbackRow) and the terminal its floating note
// (terminalui's feedbackNote). The settings page has neither — it draws no
// status row of its own — so it speaks through the app's toast stack, the same
// channel a refused `floter://` link uses. That is what keeps the settings
// page's Connect failure reading the same sentence whether it came from the
// button or from `floter://connect`: before this, both the completion and the
// failure branches handed their sentence to the *launcher's* row, which is not
// drawn while the settings page is the surface on screen — so the user saw
// nothing, and the sentence sat in the launcher until they went back there and
// it surfaced, stale, for three seconds.
//
// The read of a.Surf and the writes it decides happen on the main thread: a
// caller may be a background install that finished off it.
func (a *App) panelFeedback(message string, warning bool) {
	if message == "" {
		return
	}
	a.onMain(func() {
		switch {
		case a.Surf == SurfaceTerminal && a.Terminal != nil:
			a.Terminal.ShowFeedback(message)
		case a.Surf == SurfaceLauncher:
			if warning {
				a.Launcher.WarnFeedback(message)
			} else {
				a.Launcher.Feedback(message)
			}
		default:
			// The settings page (and any surface without a status row): the app's
			// toast stack, drawn by toastViewport on every surface.
			kind, window := "success", successNotice
			if warning {
				kind, window = "error", warningNotice
			}
			a.queueToast(ui.Toast{ID: "floter-notice", Title: message, Type: kind, Timeout: window})
		}
	})
}

// flushNotices hands the queued notices to the framework's own toast stack,
// once, at the start of a frame (the engine rebuilds a frame up to three
// times, so clearing the queue here is what keeps one notice from being added
// again on the next pass).
func (a *App) flushNotices(c *ui.Context) {
	a.noticeMu.Lock()
	pending := a.notices
	a.notices = nil
	a.noticeMu.Unlock()
	for _, toast := range pending {
		c.AddToast(toast)
	}
}

// toastTone is the color a toast's mark and its sentence wear, by the kind its
// raiser tagged it with — the old build's `.app-toast--error` / `--warning` /
// `--success`.
//
// Error and warning share the amber band: the reference's own R9-1 note on the
// CSS says so ("it shares the error band's amber *label*"), and the kind earns
// its keep by the *icon* and the accessibility role, not a second colour.
// An untagged toast wears `--text-secondary`, the rule's own default
// (`.app-toast { color: var(--text-secondary) }`, `extensions.css:291-296`), so
// a caller that sets no `Type` still reads as plain information rather than as
// a row's label.
//
// P8-e 重定基线（§5 判据）：success 旧值是 `Theme.Accent`，但「一次动作成功了」
// 不是「哪个被选中 / 哪个是开的」，accent 无权回答它——成功语义由 ✓ 字形与
// 文案说，不靠色相。新值 `TextStrong`：比未标记 toast 的 `TextSecondary` 高
// 一档明度，「确认」与「普通信息」仍分得出上下（§5 注脚：只约束色相，鼓励
// 明度分层），又与同族的导入成功提示（P8-d 的 `transferReport`）同语汇。
func toastTone(tokens theme.Tokens, kind string) ui.Color {
	switch kind {
	case "error", "warning":
		return tokens.Theme.Warning
	case "success":
		return tokens.TextStrong
	default:
		return tokens.TextSecondary
	}
}

// toastEnter is the entrance's length: the old build's `--dur-4` (220ms,
// `base.css:349`), the surface-arrival duration its `app-toast-in` ran at.
const toastEnter = 220 * time.Millisecond

// toastSpring carries an entrance's progress 0→1 along the curve `--spring`
// draws (`cubic-bezier(0.34, 1.56, 0.64, 1)`, `base.css:334`): an ease-out-back
// whose overshoot is small, so the pane lands on its place rather than stopping
// at it, and the travel above the rest position reads the same as the CSS one.
func toastSpring(p float32) float32 {
	u := p - 1
	const pull = 0.6
	return 1 + (pull+1)*u*u*u + pull*u*u
}

// toastNow reads the entrance clock: the injected one in a test, the wall clock
// otherwise.
func (a *App) toastNow() time.Time {
	if a.toastClock != nil {
		return a.toastClock()
	}
	return time.Now()
}

// toastEntrances reports how far each showing toast is through its entrance —
// 0 on the frame it joined the stack, 1 once `toastEnter` has passed. Ids it
// saw and no longer sees are forgotten, so a toast raised again under the same
// id (the framework replaces that toast and restarts its time) springs in
// again rather than arriving already settled.
// toastEntrances reports how far each showing toast is through its entrance,
// 0 on the frame it joined the stack and 1 once `toastEnter` has passed.
//
// R161 · a desktop that asked for less motion gets every toast at rest: the
// travel is this app's own clock (see toastViewport), so the framework's
// collapse never sees it, and the old build's `prefers-reduced-motion` block
// named `.app-toast` outright — `animation: none` (`main:src/styles/base.css:
// 1316-1321`). The clocks are still taken, so a toast that appears while the
// preference is on does not animate later when it is turned off.
func (a *App) toastEntrances(now time.Time, toasts []ui.Toast, reduceMotion bool) map[string]float32 {
	if a.toastSeen == nil {
		a.toastSeen = make(map[string]time.Time, len(toasts))
	}
	showing := make(map[string]bool, len(toasts))
	for _, toast := range toasts {
		showing[toast.ID] = true
	}
	for id := range a.toastSeen {
		if !showing[id] {
			delete(a.toastSeen, id)
		}
	}
	progress := make(map[string]float32, len(toasts))
	for _, toast := range toasts {
		at, ok := a.toastSeen[toast.ID]
		if !ok {
			at = now
			a.toastSeen[toast.ID] = at
		}
		if reduceMotion {
			progress[toast.ID] = 1
			continue
		}
		p := float32(now.Sub(at)) / float32(toastEnter)
		progress[toast.ID] = min(max(p, 0), 1)
	}
	return progress
}

// toastMark is the icon each kind leads with: the old build's `ToastStack` drew
// lucide's AlertCircle, TriangleAlert and Check at 15px. An untagged toast has
// no mark.
func toastMark(kind string) string {
	switch kind {
	case "error":
		return "alert"
	case "warning":
		return "triangle-alert"
	case "success":
		return "check"
	default:
		return ""
	}
}

// toastViewport draws the app's notices with the old build's own look, in place
// of the framework's theme one.
//
// The framework's default paints every toast the same inverted pill: its
// `buildToasts` takes `t.inverse()` and reads nothing of `Toast.Type`, so a
// refused deep link and an advisory warning arrive indistinguishable. The old
// build's `.app-toast` was a floating pane on `--glass-float` with a 1px edge
// and the kind's own mark and tone. The framework offers exactly one seam for
// that — `ui.ToastViewportBase`, which replaces the theme's viewport for the
// frame — and this is it: the `Float` token is the old `--glass-float`, the
// `InputStroke` its `--stroke-control`, and the tone and mark come from the
// kind above.
func (a *App) toastViewport(c *ui.Context) {
	// The old build's stack had one geometry per surface
	// (`src/styles/extensions.css:210-290`): the two full-height surfaces
	// (settings, the terminal, a plugin page) hang it from the top right, under
	// the 56u card header; the collapsed launcher — a ~58px strip where no
	// placement is overlap-free — pins its *newest* toast to a 20u band on the
	// bottom-left and drops the ones queued behind it. `ui.ToastViewportBase` is
	// the one seam that replaces the theme's own bottom-centre pill.
	collapsed := a.Surf == SurfaceLauncher
	width, _ := c.Size()
	stored := a.settings()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		t := c.Theme()
		tokens := theme.ForPreferences(stored, t.Dark, c.Preferences())
		copy := i18n.For(stored.Language).Settings
		// The entrance: the old `.app-toast` arrived on `app-toast-in`
		// (`src/styles/extensions.css:393-403`), a `--dur-4` travel at `--spring`
		// from `scale(0.98) translateY(-6px)` and zero opacity to rest. React
		// mounted the element, so the keyframes ran from the frame the toast
		// joined the stack; the clock here starts on the same edge. Computed over
		// every showing toast, before the launcher drops the older ones from
		// layout, so a toast hidden by that rule still keeps its start time.
		now := a.toastNow()
		entrances := a.toastEntrances(now, toasts, c.Preferences().ReduceMotion)
		if collapsed {
			// One toast, newest wins. Earlier ones are dropped from layout, not
			// just hidden, so the thin window never reserves a second row.
			if len(toasts) > 1 {
				toasts = toasts[len(toasts)-1:]
			}
			viewport.Bottom(t.Space(1.5)).Left(t.Space(1.5)).
				AlignItems(ui.Start).Gap(t.Space(2)).PassThrough()
		} else {
			viewport.Top(t.Space(16)).Right(t.Space(4)).
				AlignItems(ui.End).Justify(ui.Start).Gap(t.Space(2)).PassThrough()
		}
		for _, toast := range toasts {
			p := ui.ToastBase(c, toast)
			if !p.Root.Valid() {
				continue
			}
			tone := toastTone(tokens, toast.Type)
			// The column's own width: `min(360px, 100% - 32px)` on the full-height
			// surfaces, `min(300px, 100% - 12px)` on the launcher strip.
			want, room := t.Space(90), width-t.Space(8)
			if collapsed {
				want, room = t.Space(75), width-t.Space(3)
			}
			p.Root.Row().AlignItems(ui.Center).Gap(t.Space(2)).
				Width(min(want, room)).MaxWidth(room).
				Padding(t.Space(2), t.Space(3)).Radius(tokens.RadiusMD).
				Background(tokens.Float).Border(1, tokens.InputStroke).TextColor(tone)
			dismiss := t.Space(6)
			mark := t.Space(3.25)
			if collapsed {
				// The strip: one forced line at the caption step, and a slimmer
				// dismiss control so the 20u band still leaves a hit target.
				p.Root.Height(t.Space(5)).Padding(0, t.Space(2)).Gap(t.Space(1.5))
				dismiss = t.Space(4.5)
			}
			// The cast shadow the floating pane was missing: `--elev-3` on a
			// full-height surface (`base.css:208-211` — `0 10px 26px` ambient,
			// `0 1px 2px` contact, and the lit top rim) and its compact derivation
			// `--elev-3-compact` (`:222-223`) for the launcher's 20px strip.
			//
			// P13-g：这一处是**第五个**浮层，也是唯一没进 `theme.ElevThrow` 那个
			// 档的——它借的是窗口自己那对 token（`ShadowAmbient` 0.62 /
			// `ShadowContact` 0.44，`shell.go` 给桌面窗口投影用的），于是两条
			// 毛病一起犯：①不随 `main_opacity` 缩放（`--elev-shadow-scale`
			// 0.45~1.26，P11-an 已在另外四处落了这条算术），框拉到最薄时别处
			// 重了 2.8 倍，这块纹丝不动；②墨比 `--elev-3` 的 0.30/0.20 重一倍，
			// 读成一圈黑边而不是一层薄影。两条 throw 交给 token 层，几何各自
			// 自己的（满高 10u/26u，二十高的带子 6u/18u）。
			//
			// 折叠臂的冠线是本轮新画的：§2 判据 3 + §3 的表说 E2 的顶边是
			// `EdgeHighlight`，而参考的 compact 派生丢的是冠线、不是接触影。
			scale := stored.MainOpacity
			if collapsed {
				theme.ElevThrowCompact(p.Root, t, scale)
				p.Root.InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)
			} else {
				theme.ElevThrow(p.Root, t, scale)
				p.Root.InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)
			}
			// The entrance itself: one eased travel 0→1 driving all three of the
			// keyframes' properties. The curve overshoots a touch past its target,
			// as `--spring` does, so the pane settles instead of stopping; the
			// opacity clamps at 1, which is what a spring keyframe does too.
			progress := entrances[toast.ID]
			if progress < 1 {
				c.AnimationFrame()
			}
			settle := toastSpring(progress)
			p.Root.Opacity(min(settle, 1)).
				Scale(0.98+0.02*settle, 0.98+0.02*settle).
				Translate(0, -t.Space(0.75)*(1-settle))
			p.Root.Children(func() {
				if name := toastMark(toast.Type); name != "" {
					if glyph, ok := launcher.Glyph(name); ok {
						ui.Icon(c, glyph).Size(t.Space(3.75), t.Space(3.75)).TextColor(tone)
					}
				}
				sentence := ui.Text(c, toast.Title).TextColor(tone).Grow(1)
				if collapsed {
					sentence.FontSize(tokens.Caption).SingleLine().Ellipsis("\u2026")
				} else {
					sentence.FontSize(t.FontSize)
				}
				// The optional action chip (a page failure's retry), the old
				// `.app-toast__action`: a quiet control-material chip the same
				// height as the dismiss control. Pressing it runs the caller's
				// closure and closes the toast, so a retry never leaves a stale
				// one behind.
				if toast.Action != "" {
					action := ui.Row(c).Focusable().Shrink(0).Height(t.Space(5.5)).
						Padding(0, t.Space(2)).Radius(tokens.RadiusSM).
						AlignItems(ui.Center).Justify(ui.Center).
						Background(tokens.Control).Label(toast.Action)
					// P13-j · the chip is an **E1** face inside the toast's E2 pane
					// (`docs/liquid-glass.md` §2 判据 1: everything inside a floating
					// layer is a face). It used to wear the four-layer `Lens`, whose
					// ring is the `ControlEdge` frame this generation retires — and
					// the ring is painted *last*, on the same pixel row as the first
					// `LensRim`, so the lit edge P12-a raised to 0.09 never lit up
					// here either (P13-h / P13-i measured the same root cause).
					//
					// Why no `Border` in its place (§6.6 第 6-3 行 asked this): the
					// chip's face step over the pane's `Float` face is what bounds
					// it — measured at 6~9 levels on both appearances, the pin's ⑤.
					// A `Border(1, ControlEdge)` would also shrink the padding box by
					// a pixel (`insetShadow` measures from the padding edge), which is
					// geometry, not ink; and a frame is exactly the 「画框分层」 §1
					// forbids. E1 casts no shadow (§2 判据 2) — this adds none.
					tokens.LitFace(action)
					if action.Pressed() {
						// P13-al · the chip's third rung (`detach.go`'s `barButton` has
						// the same one): held down, the face sinks to `ControlPress` and the
						// capsule steps back to 0.97 — the capsule's number, not the icon
						// key's. `LitFace` is re-applied because the pair is per state.
						action = action.Background(tokens.ControlPress).Scale(pressScaleChip, pressScaleChip)
						action = tokens.LitFace(action)
					} else if action.Hovered() {
						action.Background(tokens.ControlHover)
					}
					action.Children(func() {
						ui.Text(c, toast.Action).FontSize(tokens.Caption).FontWeight(600).
							TextColor(tokens.TextStrong)
					})
					if action.Clicked() {
						if toast.OnAction != nil {
							toast.OnAction()
						}
						c.CloseToast(toast.ID)
					}
				}
				button := ui.Row(c).Focusable().Shrink(0).Size(dismiss, dismiss).
					Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
					Label(copy.DismissNotice)
				// `.app-toast__dismiss:hover` (`extensions.css:383-386`) moves the
				// glyph's ink to `--text-strong` as well as the fill to
				// `--glass-control-hover`: the control darkens, it does not just
				// gain a tint.
				dismissInk := tokens.TextTertiary
				if button.Pressed() {
					// P13-al · the ✕'s third rung, and the last key in the shell that had
					// only two: held down it sinks to `ControlPress` and steps back to
					// 0.92 (the icon key's number), keeping the E1 pair P13-z gave it.
					// The ink stays at `--text-strong`: the reference moves it on
					// `:hover` and `:active` inherits it.
					button = tokens.LitFace(button.Background(tokens.ControlPress).Scale(pressScaleKey, pressScaleKey))
					dismissInk = tokens.TextStrong
				} else if button.Hovered() {
					// P13-z · the ✕ is a key on an **E2** pane, so it is an E1 face
					// (`docs/liquid-glass.md` §2 判据 1: everything inside a floating
					// layer is a face). It used to be a flat `ControlHover` wash with no
					// line at all — measured crown = face = bottom on both appearances
					// (247/247/247 light, 20/20/20 dark) — which is the same shape the
					// plugin windows' bar keys had before P13-y. Same pair as those keys
					// (`detach.go` `barButton`), the toast's own action chip above
					// (P13-j) and every other E1 face: one lit edge on top, one
					// counter-side line at the bottom. No ring and no shadow — a key is
					// E1, not E2 (§2 判据 2).
					button = tokens.LitFace(button.Background(tokens.ControlHover))
					dismissInk = tokens.TextStrong
				}
				button.Children(func() {
					ui.Icon(c, launcherGlyph("close")).Size(mark, mark).
						TextColor(dismissInk)
				})
				if button.Clicked() {
					c.CloseToast(toast.ID)
				}
			})
		}
	})
}

// launcherGlyph is launcher.Glyph with the fallback the app's other glyph reads
// use: the names here are program constants, so a missing one is a bug rather
// than a runtime condition.
func launcherGlyph(name string) *ui.SVG {
	glyph, ok := launcher.Glyph(name)
	if !ok {
		glyph, _ = launcher.Glyph("alert")
	}
	return glyph
}

// refuseLink is what a refused `floter://` link does: it brings the panel
// forward without changing what it is showing (the old build's `focus_main`),
// and raises the one refusal sentence — unless the same refusal is still inside
// its window, in which case one attempt's worth of noise is enough. Safe to
// call from a background goroutine (a `connect` that failed after the review).
//
// The window coalesces the sentence, not the reveal: a repeat inside the
// window is still a reason to come forward, so `reveal` runs before the gate
// and only the toast is dropped.
func (a *App) refuseLink() {
	a.reveal()
	if a.refusals == nil || !a.refusals.allow(refusalKey) {
		return
	}
	message := i18n.For(a.Store.Snapshot().Language).Settings.DeepLinkRejected
	a.onMain(func() {
		a.queueNotice(message, refusalNotice)
	})
}

// reveal brings the panel forward without changing what it is showing: the old
// build's `focus_main`. A surface switch is a different gesture (`Open`); a
// refused or foreign link only asks to be seen where the user already is.
func (a *App) reveal() {
	a.onMain(func() { a.Show() })
}
