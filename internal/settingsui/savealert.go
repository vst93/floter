package settingsui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/deadline"
	"floter/internal/i18n"
)

// saveFailedWindow is how long the settings page's write-failure banner stays
// up. The old build cleared it with `useTimedReset(settingsSaveFailed,
// dismissSaveError, 5000)` (`src/App.tsx:596`), whose timeout is passed
// explicitly; the settings page's own controls call that out as
// `settings.saveFailed` ("Changes could not be applied. Check your settings
// and try again.").
const saveFailedWindow = 5 * time.Second

// now is the panel's time source.
func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// noteSaveFailed records a write one of the page's own controls made and the
// store refused. The store rolled the value back to its last committed
// snapshot, so the banner is what says the file was not written.
func (a *App) noteSaveFailed() {
	a.saveFailed = true
	a.saveFailedAt = a.now()
}

// dismissSaveFailed drops the banner, by its own button or its window.
func (a *App) dismissSaveFailed() {
	a.saveFailed = false
	a.saveFailedAt = time.Time{}
	a.saveAlertShown = false
	a.saveAlertProgress = 0
	a.saveAlertAnimating = false
}

// expireSaveFailed clears a banner whose window has passed, and arms one wake
// at the deadline while it is up so a still page reaches it. The banner's
// entrance gets its frames from its own `AnimateTo` motion; the seconds after it
// settles are a sentence that does not change, so they are one wake and not a
// rebuild per refresh (P11-bq).
func (a *App) expireSaveFailed(c *ui.Context) {
	if !a.saveFailed {
		return
	}
	waited := a.now().Sub(a.saveFailedAt)
	if waited > saveFailedWindow {
		a.dismissSaveFailed()
		return
	}
	deadline.Wake(c, saveFailedWindow-waited)
}

// SaveFailedVisible reports whether the write-failure banner is showing, for
// the shell and the tests.
func (a *App) SaveFailedVisible() bool { return a.saveFailed }

// SaveAlertAnimating reports whether the write-failure banner is still in its
// enter animation, for the tests.
func (a *App) SaveAlertAnimating() bool { return a.saveAlertAnimating }

// SaveAlertEnterProgress reports where the banner's enter animation is, 0 at
// its start and 1 in place, for the tests.
func (a *App) SaveAlertEnterProgress() float32 { return a.saveAlertProgress }

// saveAlertEnterKey names the banner's enter animation among its own motion.
const saveAlertEnterKey = "settings.saveAlert.enter"

// saveAlertEnterAnimation is the reference's `@keyframes settings-toast-enter`
// motion: the banner comes in from transparent, a hair small and 6px low, on a
// spring, rather than appearing at full strength.
var saveAlertEnterAnimation = ui.Snappy

// saveAlert draws the banner over the page's content: the warning pane, the
// sentence, and the ✕ that clears it early (the old build's
// `.settings-save-alert--toast`).
func (a *App) saveAlert(c *ui.Context, copy i18n.Settings) {
	if !a.saveFailed {
		return
	}
	t := c.Theme()
	tokens := a.tokens(c)
	// The banner's fill is the reference toast's own pane
	// (`.settings-save-alert--toast`, `main:src/styles/settings.css:204-210`):
	// `--glass-float`, the floating pane's denser tint. The port quoted the
	// *warning ink* at 12% instead, so the pane carried the sentence's own
	// colour as its wash — the ink belongs on the glyph and the words, and the
	// pane is the sheet they sit on (the same mistake the launcher's feedback
	// row made with `t.Warning.Alpha`, see `Tokens.IconSurface`). The
	// reference's `--elev-3` cast is not reproduced here: this build draws the
	// banner in the page's own column rather than as a fixed floater, so a
	// broad ambient blur would darken the page under it rather than lift the
	// banner off it.
	banner := ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).Gap(t.Space(2)).
		AlignItems(ui.Center).Radius(tokens.RadiusSM).
		Background(tokens.Float)
	// P12-c（§6.2 第 2-8 行）：这条带的面是 `Float`（浮层那一档的填充），
	// 所以它的冠线是 **E2 的** `EdgeHighlight`，不是 E1 的 `LensRim`——
	// 一层面配一条线，这个对应关系是 §2 的判据，不是审美选择。
	//
	// 它**不**加 `ElevThrow`：这条带画在页面列里、随内容一起滚（`body()` 的
	// `Scroll` 内容），不是固定在窗口上的浮层。6.5u 的宽投影会在滚动视口与
	// 列的内边距上被裁成一条硬边——那就是 §0 硬禁令 1 说的「假装有 blur」的
	// 同类错误：用一个画不完整的影冒充浮起来。把它做成真浮层是布局与交互
	// 改动（不在这支的红线内），已写进 `.pending-bugs.md` 的「需用户拍板」。
	banner.InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)
	// The banner comes in rather than appearing: the reference's
	// `@keyframes settings-toast-enter` carries it from transparent, a hair
	// small and 6px low. A value "starts at the first target", so the first
	// frame fixes the animation's start at 0 and the frames after walk it to
	// its place; drawing it at 1 at once would have nowhere to come from.
	if !a.saveAlertShown {
		a.saveAlertShown = true
		banner.AnimateTo(saveAlertEnterKey, 0, saveAlertEnterAnimation)
		a.saveAlertProgress = 0
		// A value starts at its first target, so the motion begins on the frame
		// *after* this one: ask for it. The frames of the travel itself are the
		// animation's own (`AnimateTo` asks while it moves, `ui/motion.go:340`) —
		// and the seconds after it settles belong to the banner's deadline, which
		// is one wake and not a rebuild per refresh (P11-bq).
		c.AnimationFrame()
	} else {
		a.saveAlertProgress = banner.AnimateTo(saveAlertEnterKey, 1, saveAlertEnterAnimation)
	}
	a.saveAlertAnimating = !banner.Settled(saveAlertEnterKey)
	banner.Opacity(a.saveAlertProgress).
		Scale(0.97+0.03*a.saveAlertProgress, 0.97+0.03*a.saveAlertProgress).
		Translate(0, 6*(1-a.saveAlertProgress)).
		Children(func() {
			ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(t.Warning)
			ui.Text(c, copy.SaveFailedAlert).FontSize(t.FontSize).FontWeight(580).
				TextColor(t.Warning).Grow(1)
			button := ui.Row(c).Focusable().Shrink(0).Size(t.Space(6), t.Space(6)).
				Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
				Label(copy.Close)
			if button.Hovered() {
				button.Background(tokens.ControlHover)
				// `.settings-save-alert button`: the reference's own alert button
				// carries the raised rung; this build's banner has one ✕ that is
				// transparent at rest, so the pointer is where it can be graded.
				// E1 (`theme.Tokens.LitFace`): the ✕ sits inside the E2 banner, so
				// it is a face there — one lit edge, one shade, no ring.
				tokens.LitFace(button)
			}
			button.Children(func() {
				ui.Icon(c, mustGlyph("close")).Size(t.Space(3.5), t.Space(3.5)).TextColor(t.Warning)
			})
			if button.Clicked() {
				a.dismissSaveFailed()
			}
		})
}

// shortcutRejectedAlert is the reference's plain save alert (not the toast):
// the sentence a recording the page refused leaves behind, drawn at the top of
// the Shortcuts page on the control material in the warning tone, as
// `.settings-save-alert` did (`src/styles/settings.css:279-302`). It stays up
// until the next recording starts, which is what the old page's
// `setRejectedAction(null)` on every `capture`/`toggle` amounted to.
func (a *App) shortcutRejectedAlert(c *ui.Context, copy i18n.Settings) {
	if a.rejectedShortcut == "" {
		return
	}
	t := c.Theme()
	tokens := a.tokens(c)
	ui.Box(c).FillWidth().Padding(t.Space(2.25), t.Space(2.5)).
		Radius(tokens.RadiusMD).Background(tokens.Control).
		Border(1, tokens.ControlEdge).Children(func() {
		ui.Text(c, copy.ShortcutRejected).FontSize(t.FontSize).FontWeight(580).TextColor(t.Warning)
	})
}
