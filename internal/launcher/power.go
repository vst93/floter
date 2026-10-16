package launcher

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The power rows: restart and shut down, offered when the query *is* one of
// their words.
//
// The old build claimed these queries before any other match, and so does this
// one: a user who types "restart" means the machine, not an application whose
// name contains it. The action itself is the shell's — it confirms with a
// native dialog before running anything, because both of these end the session.

// The words that claim the power rows, in both languages.
var (
	restartWords  = []string{"restart", "reboot", "重启", "重新启动"}
	shutdownWords = []string{"shutdown", "shut down", "power off", "poweroff", "关机", "关闭电脑"}
)

// PowerRestart and PowerShutdown are the action ids the shell acts on.
const (
	PowerRestart  = "restart"
	PowerShutdown = "shutdown"
)

// powerItems is the power rows for a query that is exactly one of their words.
func (a *App) powerItems() []Item {
	query := strings.ToLower(strings.TrimSpace(a.Query))
	if query == "" {
		return nil
	}
	copy := a.copy()
	var out []Item
	if matchesWord(query, restartWords) {
		out = append(out, Item{
			ID:     "power:restart",
			Title:  copy.PowerRestart,
			Detail: copy.PowerRestartHint,
			Run:    func() { a.runPower(PowerRestart) },
		})
	}
	if matchesWord(query, shutdownWords) {
		out = append(out, Item{
			ID:     "power:shutdown",
			Title:  copy.PowerShutdown,
			Detail: copy.PowerShutdownHint,
			Run:    func() { a.runPower(PowerShutdown) },
		})
	}
	return out
}

// matchesWord reports whether a query is one of a word list, exactly.
func matchesWord(query string, words []string) bool {
	for _, word := range words {
		if query == word {
			return true
		}
	}
	return false
}

// powerConfirmWindow is how long a power confirmation waits for its answer:
// the old build armed the question with `useTimedReset(pendingSystemAction,
// () => setPendingSystemAction(null))` (`src/App.tsx:546`), whose default
// timeout is 3000 ms (`src/hooks/useTimedReset.ts`). A question asked one
// moment is not still armed on a later glance.
const powerConfirmWindow = 3 * time.Second

// runPower asks in the panel, as the old build's `.launcher-system-confirm`
// did: the action's own words and the way back, docked under the list, rather
// than a system dialog over it. Nothing runs until the user presses the
// confirmation's own button — the row only asks. The question carries its own
// clock, so it clears itself unanswered after `powerConfirmWindow`.
func (a *App) runPower(action string) {
	a.pendingPower = action
	a.pendingPowerAt = a.now()
	a.Selected = 0
	a.FocusSearch()
}

// expirePowerConfirm drops a confirmation that has waited past its window,
// and keeps the frames coming while one is live so the deadline is reached on
// a still panel exactly as it is with a hand on it. A nil context (a caller
// outside a frame) still clears a lapsed question; only the animation request
// needs one.
func (a *App) expirePowerConfirm(c *ui.Context) {
	if a.pendingPower == "" {
		return
	}
	if a.now().Sub(a.pendingPowerAt) > powerConfirmWindow {
		a.pendingPower = ""
		a.pendingPowerAt = time.Time{}
		return
	}
	if c != nil {
		c.AnimationFrame()
	}
}

// PowerPending is the power action awaiting confirmation, empty for none: the
// shell sizes the window with it and the tests read it.
func (a *App) PowerPending() string { return a.pendingPower }

// powerActionName is the pending action's own word for the confirmation's
// button: the action, not "continue".
func (a *App) powerActionName() string {
	copy := a.copy()
	if a.pendingPower == PowerShutdown {
		return copy.PowerShutdownAction
	}
	return copy.PowerRestartAction
}

// powerMessage is the confirmation's sentence.
func (a *App) powerMessage() string {
	return a.copy().PowerConfirmMessage(a.pendingPower)
}

// confirmPowerRow draws the pending confirmation: its icon, its sentence, the
// action's own button and the way back. It is docked under the list, where the
// feedback row is, so it belongs to the list rather than floating over it. The
// band's fill is the reference's own `.launcher-system-confirm`
// (`main:src/styles/launcher.css:756-770`): `--system-icon-surface`, the warm
// amber plane a power entry wears — **not** the warning ink at a low alpha,
// which is the label's colour and belongs on the icon and the sentence.
func (a *App) confirmPowerRow(c *ui.Context) {
	if a.pendingPower == "" {
		return
	}
	copy := a.copy()
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	ui.Row(c).Absolute().Bottom(0).Left(t.Space(1)).Right(t.Space(1)).Height(t.Space(7)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).
		Radius(tokens.RadiusSM).Background(tokens.IconSurface).Children(func() {
		ui.Icon(c, glyphAlert).Size(t.Space(4), t.Space(4)).TextColor(t.Warning)
		ui.Text(c, a.powerMessage()).FontSize(t.FontSize).FontWeight(580).
			TextColor(t.Warning).Grow(1)
		if ui.Button(c, a.powerActionName()).Clicked() {
			a.confirmPower()
		}
		if ui.Button(c, copy.PowerCancel).Clicked() {
			a.pendingPower = ""
		}
	})
}

// confirmPower runs the action the panel asked about, once, and clears it.
func (a *App) confirmPower() {
	action := a.pendingPower
	a.pendingPower = ""
	a.pendingPowerAt = time.Time{}
	if action == "" || a.Actions.Power == nil {
		return
	}
	a.Actions.Power(action)
}

// cancelPower is Escape's own answer to the confirmation.
func (a *App) cancelPower() bool {
	if a.pendingPower == "" {
		return false
	}
	a.pendingPower = ""
	a.pendingPowerAt = time.Time{}
	return true
}
