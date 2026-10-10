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

// triggerHintText is the nudge's wording, or "" when there is none. It is the
// ordinary page's alone: a mode that already owns the field has no word to
// enter with.
func (a *App) triggerHintText() string {
	if a.mode != nil || a.clipboard || a.browser || a.calculatorMode || a.files || a.output != nil {
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
