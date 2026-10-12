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
// store refused. The change still shows in memory (the store keeps it), so the
// banner is the only thing that says the file was not written.
func (a *App) noteSaveFailed() {
	a.saveFailed = true
	a.saveFailedAt = a.now()
}

// dismissSaveFailed drops the banner, by its own button or its window.
func (a *App) dismissSaveFailed() {
	a.saveFailed = false
	a.saveFailedAt = time.Time{}
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

// saveAlert draws the banner over the page's content: the warning pane, the
// sentence, and the ✕ that clears it early (the old build's
// `.settings-save-alert--toast`).
func (a *App) saveAlert(c *ui.Context, copy i18n.Settings) {
	if !a.saveFailed {
		return
	}
	t := c.Theme()
	tokens := a.tokens(c)
	ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).Gap(t.Space(2)).
		AlignItems(ui.Center).Radius(tokens.RadiusSM).
		Background(t.Warning.Alpha(0.12)).Children(func() {
		ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(t.Warning)
		ui.Text(c, copy.SaveFailedAlert).FontSize(t.FontSize).FontWeight(580).
			TextColor(t.Warning).Grow(1)
		button := ui.Row(c).Focusable().Shrink(0).Size(t.Space(6), t.Space(6)).
			Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
			Label(copy.Close)
		if button.Hovered() {
			button.Background(tokens.ControlHover)
		}
		button.Children(func() {
			ui.Icon(c, mustGlyph("close")).Size(t.Space(3.5), t.Space(3.5)).TextColor(t.Warning)
		})
		if button.Clicked() {
			a.dismissSaveFailed()
		}
	})
}
