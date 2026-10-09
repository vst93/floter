package launcher

import (
	"os"
	"sort"
	"strings"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calc"
	"floter/internal/calculator"
	"floter/internal/clipboard"
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
	// alias is the user's alias for a PATH command, matched at the same tiers
	// as the command's own name.
	alias string
	// Run performs the item.
	Run func()
	// complete is the text Tab inserts for the row: an extension command's
	// argument, in the command mode.
	complete string
	// entry is the extension command the row runs, when it is one: Tab
	// expands it into the argument mode.
	entry *extensions.CommandEntry
	// clip is the clipboard entry the row shows, when it is one: Tab pins
	// its text into a window.
	clip *clipboard.Entry
	// web and tab are the browser row's sources, when it is one: Tab copies
	// the URL, Enter opens the page or focuses the tab.
	web *browser.Result
	tab *browser.Tab
	// calc is the calculator history row the item shows, when it is one.
	calc *calculator.Entry
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
			ID:     "browser",
			Title:  c.CommandBrowser,
			Detail: c.CommandBrowserHint,
			Run:    a.enterBrowser,
		},
		{
			ID:     "clipboard",
			Title:  c.CommandClipboard,
			Detail: c.CommandClipboardHint,
			Run:    a.enterClipboard,
		},
		{
			ID:     "calculator",
			Title:  c.CommandCalculator,
			Detail: c.CommandCalculatorHint,
			Run:    a.enterCalculator,
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

// browserItems is the browser mode's list: the bookmarks and history for the
// query, then the browser's live tabs.
func (a *App) browserItems() []Item {
	answer := a.browserResults(a.browserQuery())
	if !answer.Found {
		return nil
	}
	copy := StringsFor(a.settings().Language)
	out := make([]Item, 0, len(answer.Results)+len(answer.Tabs))
	for _, result := range answer.Results {
		result := result
		detail := result.URL
		if !result.Visited.IsZero() {
			detail = result.URL + "  \u00b7  " + result.Visited.Local().Format("2006-01-02 15:04")
		}
		out = append(out, Item{
			ID:     "web:" + result.URL,
			Title:  result.Label(),
			Detail: detail,
			Run:    func() { a.openResult(result) },
			web:    &result,
		})
	}
	for _, tab := range answer.Tabs {
		tab := tab
		detail := tab.URL
		if detail == "" {
			detail = copy.BrowserTab
		} else {
			detail = detail + "  \u00b7  " + copy.BrowserTab
		}
		out = append(out, Item{
			ID:     "tab:" + tab.BrowserID + ":" + tab.URL,
			Title:  tab.Label(),
			Detail: detail,
			Run:    func() { a.activateTab(tab) },
			tab:    &tab,
		})
	}
	return out
}

// openResult opens a browser result's page in the browser it came from.
func (a *App) openResult(result browser.Result) {
	if a.Actions.OpenURL != nil {
		a.Actions.OpenURL(result.BrowserID, result.URL)
	}
}

// activateTab brings one of the browser's live tabs to the front.
func (a *App) activateTab(tab browser.Tab) {
	if a.Actions.ActivateTab != nil {
		a.Actions.ActivateTab(tab)
	}
}

// copyURL puts a URL on the clipboard, and says so.
func (a *App) copyURL(url string) {
	if a.Actions.Copy != nil {
		a.Actions.Copy(url)
		a.toast = StringsFor(a.settings().Language).Copied
	}
}

// copyResult puts a browser result's URL on the clipboard.
func (a *App) copyResult(result browser.Result) {
	if a.Actions.Copy != nil {
		a.Actions.Copy(result.URL)
		a.toast = StringsFor(a.settings().Language).Copied
	}
}

// browserResults returns the answer for the query: the shell's, when it is for
// this query, and an empty one while the first search runs.
func (a *App) browserResults(query string) BrowserResults {
	if a.browserAsked && a.browserAskedFor == query {
		return a.browserFound
	}
	return BrowserResults{}
}

// clipboardItems is the clipboard mode's list: the history entries matching
// what the user typed after the mode word.
func (a *App) clipboardItems() []Item {
	if a.Clipboard == nil {
		return nil
	}
	entries := a.Clipboard.Search(a.clipboardQuery(), maxClipboardResults)
	out := make([]Item, 0, len(entries))
	for _, entry := range entries {
		entry := entry
		shortcut := ""
		if entry.Favorite {
			shortcut = "★"
		}
		out = append(out, Item{
			ID:       "clip:" + entry.ID,
			Title:    entry.Label(),
			Detail:   entry.Time().Format("2006-01-02 15:04"),
			Shortcut: shortcut,
			Run:      func() { a.copyClip(entry) },
			clip:     &entry,
		})
	}
	return out
}

// maxClipboardResults bounds the rows one clipboard search builds, and
// maxRecentResults how many recent applications the empty query offers.
const (
	maxClipboardResults = 50
	maxRecentResults    = 5
)

// pinClip shows a clipboard entry's whole text in a window of its own, as
// the old build's "pin as text window" did.
func (a *App) pinClip(entry clipboard.Entry) {
	if a.Actions.PinText == nil {
		return
	}
	title := entry.Label()
	switch entry.Kind {
	case clipboard.KindText:
		a.Actions.PinText(title, entry.Text)
	case clipboard.KindFiles:
		a.Actions.PinText(title, strings.Join(entry.Paths, "\n"))
	default:
		a.Actions.PinText(title, entry.Label())
	}
}

// copyClip puts an entry back on the clipboard, whatever its kind, and
// reports it.
func (a *App) copyClip(entry clipboard.Entry) {
	if a.Actions.CopyClip != nil {
		a.Actions.CopyClip(entry)
		a.toast = StringsFor(a.settings().Language).Copied
		return
	}
	// Without the richer action, text is all that can be restored.
	if entry.Kind == clipboard.KindText && a.Actions.Copy != nil {
		a.Actions.Copy(entry.Text)
		a.toast = StringsFor(a.settings().Language).Copied
	}
}

// argumentItems is the command mode's list: the completions for the tokens
// typed so far — the command's own arguments and their values, with the
// provider's dynamic completions merged over them when it offers any.
func (a *App) argumentItems(entry extensions.CommandEntry) []Item {
	completions := a.completions(entry)
	items := make([]Item, 0, len(completions))
	for _, completion := range completions {
		value := completion.Label
		items = append(items, Item{
			ID:       "arg:" + value,
			Title:    value,
			Detail:   completion.Detail,
			complete: value,
			Run:      func() { a.appendWord(value) },
		})
	}
	return items
}

// completions merges the static completions with the provider's, asking the
// provider once per change of what is typed.
func (a *App) completions(entry extensions.CommandEntry) []extensions.Completion {
	tokens := a.commandTokens()
	key := strings.Join(tokens, "\x00")
	static := extensions.StaticCompletions(entry.Command, tokens, a.workingDirectory())

	if a.dynamicFor == key {
		return extensions.MergeCompletions(static, a.dynamic)
	}
	if a.requestedFor != key {
		a.requestedFor = key
		a.dynamicFor, a.dynamic = "", nil
		if a.Actions.Complete != nil && extensions.NeedsDynamicCompletion(entry.Command, tokens) {
			requested := append([]string{}, tokens...)
			entry := entry
			a.Actions.Complete(entry, requested, func(items []extensions.Completion) {
				// A stale answer for what was typed before is dropped.
				if a.requestedFor != key {
					return
				}
				a.dynamicFor, a.dynamic = key, items
			})
		}
	}
	return static
}

// workingDirectory is where relative path completions are resolved: the
// app's own directory.
func (a *App) workingDirectory() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// toolItems is the commands found on the PATH as result rows, only when the
// setting asks for them.
func (a *App) toolItems() []Item {
	if !a.ShowTools {
		return nil
	}
	out := make([]Item, 0, len(a.Tools))
	for _, tool := range a.Tools {
		out = append(out, Item{
			ID:    "tool:" + tool.Path,
			Title: tool.Name,
			alias: a.ToolAliases[tool.Name],
			Run:   func() { a.runTool(tool) },
		})
	}
	return out
}

func (a *App) runTool(tool apps.App) {
	if a.Actions.RunInTerminal != nil {
		a.Actions.RunInTerminal([]string{tool.Path})
	}
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
	items = append(items, a.toolItems()...)
	items = append(items, a.commandItems()...)
	return items
}

// recentItems is the most-launched applications as rows, in the order the
// usage store gave, at most limit of them: the empty query shows them under
// the built-in commands.
func (a *App) recentItems(limit int) []Item {
	if !a.ShowRecent || len(a.Recent) == 0 {
		return nil
	}
	byPath := make(map[string]apps.App, len(a.Apps))
	for _, app := range a.Apps {
		byPath[app.Path] = app
	}
	var out []Item
	for _, path := range a.Recent {
		app, ok := byPath[path]
		if !ok {
			continue // the application is gone
		}
		out = append(out, Item{
			ID:    "app:" + app.Path,
			Title: app.Name,
			Run:   func() { a.openApp(app) },
		})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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
	if a.clipboard {
		return a.clipboardItems()
	}
	if a.browser {
		return a.browserItems()
	}
	if a.calculatorMode {
		return a.calculatorItems()
	}
	var out []Item
	if item, ok := a.calculator(); ok {
		out = append(out, item)
	}
	if strings.TrimSpace(a.Query) == "" {
		out = append(out, a.commands()...)
		out = append(out, a.recentItems(maxRecentResults)...)
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
//
// A PATH command's alias runs through the same ladder as its name, so an alias
// hit ranks exactly where the name would (an exact alias is not second class).
func itemScore(item Item, term string) (int, bool) {
	title := strings.ToLower(item.Title)
	score, matched := candidateScore(title, term)
	if item.alias != "" {
		if aliasScore, aliasMatched := candidateScore(strings.ToLower(item.alias), term); aliasMatched && (!matched || aliasScore < score) {
			score, matched = aliasScore, true
		}
	}
	if matched {
		return score, true
	}
	if strings.Contains(strings.ToLower(item.Detail), term) ||
		strings.Contains(strings.ToLower(item.ID), term) ||
		strings.Contains(strings.ToLower(item.Search), term) {
		return 3, true
	}
	return 0, false
}

// candidateScore is the ladder one candidate string is scored by: the string
// starts with the term, a word of it does, or it contains it.
func candidateScore(candidate, term string) (int, bool) {
	switch {
	case strings.HasPrefix(candidate, term):
		return 0, true
	case wordPrefixMatch(candidate, term):
		return 1, true
	case strings.Contains(candidate, term):
		return 2, true
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
