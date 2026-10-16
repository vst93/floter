package settingsui

import (
	"time"

	"github.com/egoist/mygo/ui"

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

// expireSaveFailed clears a banner whose window has passed, and keeps the
// frames coming while one is up so the deadline is reached on a still page.
func (a *App) expireSaveFailed(c *ui.Context) {
	if !a.saveFailed {
		return
	}
	if a.now().Sub(a.saveFailedAt) > saveFailedWindow {
		a.dismissSaveFailed()
		return
	}
	if c != nil {
		c.AnimationFrame()
	}
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
	// The banner comes in rather than appearing: the reference's
	// `@keyframes settings-toast-enter` carries it from transparent, a hair
	// small and 6px low. A value "starts at the first target", so the first
	// frame fixes the animation's start at 0 and the frames after walk it to
	// its place; drawing it at 1 at once would have nowhere to come from.
	if !a.saveAlertShown {
		a.saveAlertShown = true
		banner.AnimateTo(saveAlertEnterKey, 0, saveAlertEnterAnimation)
		a.saveAlertProgress = 0
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
				// carries the raised lens rung; this build's banner has one ✕ that
				// is transparent at rest, so the pointer is where it can be graded.
				tokens.Lens(button)
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
