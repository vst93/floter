package launcher

import (
	"sort"
	"strings"
)

// Item is one launcher result. P1 ships the built-in commands; later rounds
// add applications, plugin commands and the calculator to the same shape.
type Item struct {
	// ID is the stable identity, also searched.
	ID string
	// Title is the row's main text.
	Title string
	// Detail is the row's secondary line.
	Detail string
	// Shortcut is the key label shown at the row's end, if any.
	Shortcut string
	// Run performs the item.
	Run func()
}

// Catalog is the built-in command list, labeled in the launcher's language.
func (a *App) Catalog() []Item {
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

// Results filters and ranks the catalog for the current query. An empty
// query shows the whole catalog in its authored order.
func (a *App) Results() []Item {
	return Match(a.Catalog(), a.Query)
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
	if strings.Contains(strings.ToLower(item.Detail), term) || strings.Contains(strings.ToLower(item.ID), term) {
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
