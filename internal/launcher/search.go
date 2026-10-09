package launcher

import (
	"sort"
	"strings"

	"floter/internal/apps"
	"floter/internal/calc"
	"floter/internal/extensions"
)

// Item is one launcher result: a built-in command, an installed
// application, or the calculator's answer.
type Item struct {
	// ID is the stable identity, also searched.
	ID string
	// Title is the row's main text.
	Title string
	// Detail is the row's secondary line.
	Detail string
	// Shortcut is the key label shown at the row's end, if any.
	Shortcut string
	// Search is extra text a query matches but the row does not show:
	// an extension command's aliases and keywords.
	Search string
	// Run performs the item.
	Run func()
	// complete is the text Tab inserts for the row: an extension command's
	// argument, in the command mode.
	complete string
	// entry is the extension command the row runs, when it is one: Tab
	// expands it into the argument mode.
	entry *extensions.CommandEntry
}

// commands is the built-in command list, labeled in the launcher's language.
func (a *App) commands() []Item {
	c := StringsFor(a.settings().Language)
	return []Item{
		{
			ID:       "settings",
			Title:    c.CommandSettings,
			Detail:   c.CommandSettingsHint,
			Shortcut: c.ShortcutSettings,
			Run:      a.Actions.OpenSettings,
		},
		{
			ID:     "terminal",
			Title:  c.CommandTerminal,
			Detail: c.CommandTerminalHint,
			Run:    a.Actions.OpenTerminal,
		},
		{
			ID:     "quit",
			Title:  c.CommandQuit,
			Detail: c.CommandQuitHint,
			Run:    a.Actions.Quit,
		},
	}
}

// commandItems is the enabled extensions' commands as result rows.
func (a *App) commandItems() []Item {
	out := make([]Item, 0, len(a.Commands))
	for _, entry := range a.Commands {
		command := entry.Command
		detail := command.Description
		if detail == "" {
			detail = entry.IntegrationName
		}
		search := strings.Join(append(append([]string{}, command.Aliases...), command.Keywords...), " ")
		if entry.IntegrationName != "" {
			search += " " + entry.IntegrationName
		}
		entry := entry
		out = append(out, Item{
			ID:       "cmd:" + entry.IntegrationID + ":" + command.ID,
			Title:    command.Name,
			Detail:   detail,
			Search:   search,
			Shortcut: "Tab",
			Run:      func() { a.runCommand(entry) },
			entry:    &entry,
		})
	}
	return out
}

func (a *App) runCommand(entry extensions.CommandEntry) {
	if a.Actions.RunCommand != nil {
		a.Actions.RunCommand(entry, nil)
	}
}

// argumentItems is the command mode's list: the selected command's declared
// arguments, filtered by the word being typed, plus the values of every
// enum they declare.
func (a *App) argumentItems(entry extensions.CommandEntry) []Item {
	word := a.currentWord()
	var items []Item
	for _, argument := range entry.Command.Arguments {
		name := ""
		if len(argument.Names) > 0 {
			name = argument.Names[0]
		}
		if name != "" {
			items = append(items, Item{
				ID:       "arg:" + name,
				Title:    name,
				Detail:   argument.Description,
				Search:   strings.Join(append(append([]string{}, argument.Names...), argument.Values...), " "),
				complete: name,
				Run:      func() { a.appendWord(name) },
			})
		}
		for _, value := range argument.Values {
			value := value
			items = append(items, Item{
				ID:       "value:" + name + ":" + value,
				Title:    value,
				Detail:   argument.Description,
				Search:   strings.Join(argument.Names, " "),
				complete: value,
				Run:      func() { a.appendWord(value) },
			})
		}
	}
	if word == "" {
		return items
	}
	return Match(items, word)
}

// appItems is the scanned applications as result rows.
func (a *App) appItems() []Item {
	out := make([]Item, 0, len(a.Apps))
	for _, app := range a.Apps {
		out = append(out, Item{
			ID:    "app:" + app.Path,
			Title: app.Name,
			Run:   func() { a.openApp(app) },
		})
	}
	return out
}

func (a *App) openApp(app apps.App) {
	if a.Actions.OpenApp != nil {
		a.Actions.OpenApp(app)
	}
}

// Catalog is every item the launcher can show: the built-in commands, the
// scanned applications and the extensions' commands.
func (a *App) Catalog() []Item {
	items := a.commands()
	items = append(items, a.appItems()...)
	items = append(items, a.commandItems()...)
	return items
}

// Results is what the current query shows. The calculator's answer comes
// first when the query is a sum; an empty query shows the built-in
// commands alone (a wall of applications is not an empty state). The rest
// is the ranked catalog: the list view builds only the rows in view, so a
// broad query is as cheap a frame as a narrow one.
func (a *App) Results() []Item {
	if a.mode != nil {
		return a.argumentItems(*a.mode)
	}
	var out []Item
	if item, ok := a.calculator(); ok {
		out = append(out, item)
	}
	if strings.TrimSpace(a.Query) == "" {
		out = append(out, a.commands()...)
	} else {
		out = append(out, Match(a.Catalog(), a.Query)...)
	}
	return out
}

// calculator is the answer row for an arithmetic query, if the query is
// one. Enter copies the answer and clears the field, as the old launcher's
// calculator did.
func (a *App) calculator() (Item, bool) {
	value, err := calc.Eval(a.Query)
	if err != nil {
		return Item{}, false
	}
	result := calc.Format(value)
	expression := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(a.Query), "="))
	detail := expression
	if detail == result {
		detail = ""
	}
	return Item{
		ID:     "calc",
		Title:  result,
		Detail: detail,
		Run: func() {
			if a.Actions.Copy != nil {
				a.Actions.Copy(result)
			}
			a.toast = StringsFor(a.settings().Language).Copied
			a.Query = ""
			a.Selected = 0
		},
	}, true
}

// Match ranks items for a query: every whitespace-separated term must appear
// in the item's title, detail or id, and the item's rank is its best
// (lowest) term score. Equal scores keep the catalog's order.
//
// The score ladder, from best to worst: the title starts with the term, a
// word of the title starts with it, the title contains it, and only the
// detail or the id contains it.
func Match(items []Item, query string) []Item {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		out := make([]Item, len(items))
		copy(out, items)
		return out
	}

	ranked := make([]struct {
		item  Item
		score int
	}, 0, len(items))
	for _, item := range items {
		best := 0
		ok := true
		for _, term := range terms {
			score, found := itemScore(item, term)
			if !found {
				ok = false
				break
			}
			if score > best {
				best = score
			}
		}
		if ok {
			ranked = append(ranked, struct {
				item  Item
				score int
			}{item, best})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score < ranked[j].score })

	out := make([]Item, len(ranked))
	for i, r := range ranked {
		out[i] = r.item
	}
	return out
}

// itemScore scores one term against an item, and reports whether it matched.
func itemScore(item Item, term string) (int, bool) {
	title := strings.ToLower(item.Title)
	switch {
	case strings.HasPrefix(title, term):
		return 0, true
	case wordPrefixMatch(title, term):
		return 1, true
	case strings.Contains(title, term):
		return 2, true
	}
	if strings.Contains(strings.ToLower(item.Detail), term) ||
		strings.Contains(strings.ToLower(item.ID), term) ||
		strings.Contains(strings.ToLower(item.Search), term) {
		return 3, true
	}
	return 0, false
}

// wordPrefixMatch reports whether any word of s starts with term.
func wordPrefixMatch(s, term string) bool {
	for _, word := range strings.FieldsFunc(s, func(r rune) bool { return !isWordRune(r) }) {
		if strings.HasPrefix(word, term) {
			return true
		}
	}
	return false
}

func isWordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	case r > 0x7f: // CJK and other letters: one rune is one word
		return true
	default:
		return false
	}
}

// ClampIndex keeps a selection inside a list of n rows, 0 for an empty list.
func ClampIndex(index, n int) int {
	if n <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= n {
		return n - 1
	}
	return index
}

// NextIndex moves a selection by delta, wrapping around.
func NextIndex(index, delta, n int) int {
	if n <= 0 {
		return 0
	}
	return ((index+delta)%n + n) % n
}
