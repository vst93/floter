package launcher

// The ordinary search page's trigger hint: while a typed word is on its way to
// an extension command's mode, the field nudges "space enters this command".
// The rule is the entry rule's left half — a *prefix* of an enabled command's
// trigger word (its id, its name or an alias) typed as a whole query with no
// whitespace yet — so the nudge and the transition cannot disagree: one space
// then enters the mode.
//
// The nudge rides the field row and costs no band, as the old build's R48
// decided: the window's height is unaffected whether it shows or not.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// commandTriggers is a command's trigger vocabulary: its id, its name and its
// aliases, each reduced to its first word — the same list the Integrations
// panel prints, so "type the command name" and "the command's own name" are
// one thing.
func commandTriggers(entry extensions.CommandEntry) []string {
	var triggers []string
	seen := map[string]bool{}
	for _, source := range append([]string{entry.Command.ID, entry.Command.Name}, entry.Command.Aliases...) {
		word := firstWord(strings.TrimSpace(source))
		key := strings.ToLower(word)
		if word == "" || seen[key] {
			continue
		}
		seen[key] = true
		triggers = append(triggers, word)
	}
	return triggers
}

// commandDisplayName is a command's user-facing name: its own, or its first
// trigger, or its id.
func commandDisplayName(entry extensions.CommandEntry) string {
	if name := strings.TrimSpace(entry.Command.Name); name != "" {
		return name
	}
	if triggers := commandTriggers(entry); len(triggers) > 0 {
		return triggers[0]
	}
	return entry.Command.ID
}

// triggerHint is the command a whole-word query is on its way to, and how many
// commands that word could still become.
type triggerHint struct {
	entry extensions.CommandEntry
	// count is how many enabled commands the word matches; more than one
	// reads as "…and N more".
	count int
}

// externalTriggerHint resolves the nudge for a query: only a whole single word
// with no whitespace anywhere (a leading or trailing space is not a trigger,
// and an interior space is a phrase), and only a word that a *prefix* match of
// an enabled command's trigger.
func (a *App) externalTriggerHint(query string) (triggerHint, bool) {
	if query == "" || strings.ContainsAny(query, " \t\n") {
		return triggerHint{}, false
	}
	word := strings.ToLower(query)
	var matches []extensions.CommandEntry
	for _, entry := range a.Commands {
		for _, trigger := range commandTriggers(entry) {
			if strings.HasPrefix(strings.ToLower(trigger), word) {
				matches = append(matches, entry)
				break
			}
		}
	}
	if len(matches) == 0 {
		return triggerHint{}, false
	}
	return triggerHint{entry: matches[0], count: len(matches)}, true
}

// triggerHintMinCells is the room the nudge keeps before it is dropped: below
// six cells of its own font the box holds an ellipsis and little else — a mark
// with no words left — so the nudge goes instead of shrinking to noise. Six is
// the space glyph plus a two-letter word (the old build's
// `TRIGGER_HINT_MIN_CELLS`). The cell is the nudge's *own* font, so the floor
// follows the interface step with no second number.
const triggerHintMinCells = 6

// triggerHintRoom is the width the field row leaves the nudge. The row is
// [field][nudge][clear][buttons]: the field is its grower and yields the whole
// leftover before the nudge gives up a pixel — a squeezed row collapses the
// field to nothing and shrinks the nudge (the framework's own order, the
// reference's `flex: 1 1 0` beside `flex: 0 1 auto`) — so only the controls
// that keep their size and the gaps between them count against the row's inner
// width. The nudge itself is not counted: this is the room it may take, capped
// at half the row's inner width as the reference's `max-width: 50%` is
// (`.collapsed-card__trigger-hint`): a long name never becomes the row's
// dominant element, so the drawn box is the smaller of the two.
func (a *App) triggerHintRoom(c *ui.Context, t *ui.Theme) float32 {
	width, _ := c.Size()
	// The row's trailing controls are the two ways in the old build kept
	// beside the field, each drawn only when the app wired it.
	buttons := 0
	if a.Actions.OpenSessions != nil {
		buttons++
	}
	if a.Actions.OpenSettings != nil {
		buttons++
	}
	gap := t.Space(1)
	inner := width - 2*t.Space(4)
	// Items after the field: the nudge, the clear control, then the buttons.
	// Every adjacent pair carries the row's gap, and each button also keeps
	// the margin the old row put before it (`.collapsed-card__settings`).
	room := inner -
		gap*float32(2+buttons) -
		t.Space(7)*float32(1+buttons) -
		t.Space(2)*float32(buttons)
	if cap := inner / 2; room > cap {
		room = cap
	}
	return room
}

// triggerHintFits reports whether a nudge whose own text measures `natural`
// still has room to be read in a box of `room` pixels, given one cell of its
// own font advancing `cell` pixels. The box is what the row leaves, capped by
// how much text there is. A cell that cannot be measured keeps the nudge: a
// wrong *hide* loses it for good, while a wrong *show* costs one line of muted
// text.
func triggerHintFits(room, natural, cell float32) bool {
	if !(cell > 0) {
		return true
	}
	box := natural
	if room < box {
		box = room
	}
	return box >= cell*triggerHintMinCells
}

// triggerHintShown is the nudge's own fit, measured in the frame it is drawn
// in: the row's leftover box for this text, against six cells of the nudge's
// font. The text is measured at the size the row draws it (one step under the
// body, `--text-caption`).
func (a *App) triggerHintShown(c *ui.Context, t *ui.Theme, text string) bool {
	if text == "" {
		return false
	}
	size := t.FontSize - 1
	cell, _ := c.MeasureText(0, ui.Span{Text: "0", Size: size})
	natural, _ := c.MeasureText(0, ui.Span{Text: text, Size: size})
	return triggerHintFits(a.triggerHintRoom(c, t), natural, cell)
}

// triggerHintText is the nudge's wording, or "" when there is none. It is the
// ordinary page's alone: a mode that already owns the field has no word to
// enter with.
func (a *App) triggerHintText() string {
	if a.pluginModeActive() || a.output != nil {
		return ""
	}
	hint, ok := a.externalTriggerHint(a.Query)
	if !ok {
		return ""
	}
	copy := a.copy()
	name := commandDisplayName(hint.entry)
	if hint.count > 1 {
		return copy.TriggerHintMore(name, hint.count-1)
	}
	return copy.TriggerHint(name)
}
