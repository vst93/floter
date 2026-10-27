package launcher

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calc"
	"floter/internal/calculator"
	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
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
	// Source is the row's provenance word, printed in a fixed slot at the
	// row's right edge when the row has one: the integration that contributed
	// a command, the package manager an install row will use. The old build's
	// `.launcher-result__source` — a slot rather than an inline suffix, so a
	// narrow window cannot ellipsize the answer away exactly when "where did
	// this come from" matters most.
	Source string
	// Shortcut is the key label shown at the row's end, if any.
	Shortcut string
	// Search is extra text a query matches but the row does not show:
	// an extension command's aliases and keywords.
	Search string
	// alias is the user's alias for a PATH command, matched at the same tiers
	// as the command's own name.
	alias string
	// matchedAlias is the alias a query actually reached this row through,
	// set by Match when the alias scored better than the row's own title. The
	// view names it in the subtitle (the old build's `launcher.aliasMatch`),
	// so a row found by "gfm" says why instead of reading like a stray hit.
	matchedAlias string
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
	// icon is the application the row shows an icon for, when it is one, and
	// thumb the clipboard entry whose image the row shows.
	icon  *apps.App
	thumb *ui.Bitmap
	// heading marks a block's title line: a row the list prints above a
	// block and never selects or runs.
	heading bool
	// status marks a plugin's own status note -- the ones the modes build, not
	// the captured-output list's rows: information rather than a door, drawn as
	// a muted one-liner in the list's icon column (the old build's
	// `.launcher-status`) and charged the notes' own 30u band instead of a
	// result row's floor (see `main:src/launcher/LauncherResults.tsx:749-760`
	// and `App.tsx:2165-2166`).
	status bool
	// warning marks a row that carries a warning dot: a command word two
	// integrations share, so the row the launcher will not enter says so.
	warning bool
	// direct marks a command whose output the launcher shows itself: the row
	// carries the space-bar nudge the old build's R65 put there, since one
	// space in the field enters the mode the output shows in.
	direct bool
	// trailing is a value printed complete at the row's right edge — the
	// calculator's answer, which the old build's R50 let the expression make
	// room for rather than truncating.
	trailing string
	// The search registers, folded once when the catalog is built (see
	// App.Catalog). Every keystroke asks each row's four registers whether the
	// term appears in them, and folding them on that path is one allocation per
	// register per row per key. The catalog is the one place that knows a row's
	// text will not change, so the fold happens there and the search reads
	// these. A row built outside the catalog (a clipboard entry, a dropped file,
	// a browser tab) leaves them empty and is folded on the spot, exactly as
	// before: `lowered` is the one accessor either way.
	lowerTitle  string
	lowerDetail string
	lowerID     string
	lowerSearch string
	lowerAlias  string
}

// lowered reports a search register in its folded form: the value cached when
// the catalog was built when there is one, the fold itself otherwise.
func lowered(cached, value string) string {
	if cached != "" {
		return cached
	}
	return strings.ToLower(value)
}

// commands is the built-in command list, labeled in the launcher's language.
//
// It is the old build's `SYSTEM_COMMANDS` (`main:src/hooks/useLauncherCatalog.ts:184-292`)
// row for row and in its order: restart, shutdown, clipboard, browser,
// calculator, terminal. Six rows — settings and quit are *not* rows there, and
// they are not rows here either: both live on the tray menu
// (`shell.InstallTray`) and on the macOS app menu (`RoleQuit`), which is where
// the old build kept them, so the front door does not grow two rows the
// reference build never drew.
func (a *App) commands() []Item {
	c := a.copy()
	power := c
	// The search vocabulary is every language's wording plus the pinyin
	// initials of the Chinese ones: the UI language says nothing about the
	// keyboard the query is typed on, so 重启 has to be found by "restart" on
	// a Chinese UI and by `cq` on a Latin one. The initials are written out —
	// these are the app's own commands, not a table-sized set.
	return []Item{
		{
			ID:     "power-restart",
			Title:  power.PowerRestart,
			Detail: power.PowerRestartHint,
			Search: restartSearch,
			Run:    func() { a.runPower(PowerRestart) },
		},
		{
			ID:     "power-shutdown",
			Title:  power.PowerShutdown,
			Detail: power.PowerShutdownHint,
			Search: shutdownSearch,
			Run:    func() { a.runPower(PowerShutdown) },
		},
		{
			ID:     "clipboard",
			Title:  c.CommandClipboard,
			Detail: c.CommandClipboardHint,
			Search: clipboardSearch,
			Run:    a.enterClipboard,
		},
		{
			ID:     "browser",
			Title:  c.CommandBrowser,
			Detail: c.CommandBrowserHint,
			Search: browserSearch,
			Run:    a.enterBrowser,
		},
		{
			ID:     "calculator",
			Title:  c.CommandCalculator,
			Detail: c.CommandCalculatorHint,
			Search: calculatorSearch,
			Run:    a.enterCalculator,
		},
		{
			ID:     "terminal",
			Title:  c.CommandTerminal,
			Detail: c.CommandTerminalHint,
			Search: terminalSearch,
			Run:    a.Actions.OpenTerminal,
		},
	}
}

// commandItems is the enabled extensions' commands as result rows.
func (a *App) commandItems() []Item {
	out := make([]Item, 0, len(a.Commands))
	for _, entry := range a.Commands {
		command := entry.Command
		// The description is the row's second line and the integration's name
		// is its right-hand source — two different facts the old build never
		// let one stand in for the other: a command with no description still
		// says which integration owns it, and one with a description says both.
		detail := command.Description
		search := strings.Join(append(append([]string{}, command.Aliases...), command.Keywords...), " ")
		if entry.IntegrationName != "" {
			search += " " + entry.IntegrationName
		}
		entry := entry
		out = append(out, Item{
			ID:       "cmd:" + entry.IntegrationID + ":" + command.ID,
			Title:    command.Name,
			Detail:   detail,
			Source:   entry.IntegrationName,
			Search:   search,
			Shortcut: "Tab",
			alias:    a.Aliases[command.ID],
			Run:      func() { a.runCommand(entry) },
			entry:    &entry,
			warning:  a.commandConflicts[strings.ToLower(strings.TrimSpace(command.ID))],
			direct:   entry.Route == extensions.RouteBackground,
		})
	}
	return out
}

func (a *App) runCommand(entry extensions.CommandEntry) {
	a.runCommandWith(entry, nil)
}

// runCommandWith runs a command by the route its manifest declares: the
// terminal surface, or a headless run whose output the launcher shows.
func (a *App) runCommandWith(entry extensions.CommandEntry, args []string) {
	if entry.Route == extensions.RouteBackground {
		if a.Actions.RunCommandCaptured == nil {
			return
		}
		a.Actions.RunCommandCaptured(entry, args, func(run extensions.CapturedRun, err error) {
			a.showOutput(run, err, &entry, args)
		})
		return
	}
	if a.Actions.RunCommand != nil {
		a.Actions.RunCommand(entry, args)
	}
}

// BrowserFetchLimit is how deep the browser mode's held set goes: the
// reference's `BROWSER_FETCH_LIMIT` (`src/plugins/browser/mode.ts:70`), the
// number of rows the inline mode fetches and keeps.
//
// The reference has *two* ceilings and only one of them is ours to keep. The
// plugin output view leaves the group ceiling at its own `BROWSER_GROUP_LIMIT =
// MAX_RESULTS = 10` (`mode.ts:84`, `launcher/result-budget.ts:46`) because its
// box cannot scroll; the inline launcher raises that ceiling to this fetch depth
// and pages the held rows ten at a time (`paginatePluginRows`). This build's
// launcher list scrolls instead of paging, so the fetch depth *is* the reachable
// depth — the page size would be a second, half-wired mechanism with no scroll
// to feed it (the `BROWSER_GROUP_LIMIT` side is deliberately not replicated; see
// the "尚未做" note in docs/mygo-rewrite-status.md).
//
// The *backend* reads deeper than this (the reference's own `MAX_LIMIT` is also
// 500, but `internal/browser/browser.go:411`'s per-profile SQL window is 4000,
// documented as "enough that the newest visits and bookmarks are all there"):
// the shell hands the launcher everything the backend matched, and the cap
// below is where the mode decides what to hold, exactly as `browserSearchRows`
// does in the reference.
const BrowserFetchLimit = 500

// browserItems is the browser mode's list: the bookmarks and history for the
// query, then the browser's live tabs — narrowed by the mode's range chip.
func (a *App) browserItems() []Item {
	answer := a.browserResults(a.browserQuery())
	if !answer.Found {
		return nil
	}
	wantTabs := a.browserFilter == filterAll || a.browserFilter == filterTabs
	out := make([]Item, 0, len(answer.Results)+len(answer.Tabs))
	for _, result := range answer.Results {
		switch a.browserFilter {
		case filterBookmarks:
			if result.Kind != browser.KindBookmark {
				continue
			}
		case filterHistory:
			if result.Kind != browser.KindHistory {
				continue
			}
		case filterTabs:
			continue // the range is the live tabs alone
		}
		// The first group is the curated bookmarks and the history merged; the
		// reference breaks it at `cap` rows (`plugins/browser/mode.ts:199-200`)
		// and the inline mode's cap is the fetch depth.
		if len(out) >= BrowserFetchLimit {
			break
		}
		result := result
		// The row's subtitle is the URL alone: the reference's browser rows
		// carry `subtitle: row.url` (`plugins/browser/mode.ts:195`), and the
		// "this is history" fact is the clock glyph, not a date string (the
		// reference parses `last_visit` for ordering only — `browser-page.ts:37`
		// — and never renders it).
		out = append(out, Item{
			ID:     "web:" + result.URL,
			Title:  result.Label(),
			Detail: result.URL,
			Run:    func() { a.openResult(result) },
			web:    &result,
		})
	}
	if !wantTabs {
		return out
	}
	for _, tab := range answer.Tabs {
		// The tabs are a second group, added *on top of* the first: the
		// reference stops when the total reaches `cap * 2` (`mode.ts:218`), so a
		// browser whose bookmarks already fill the first group still shows its
		// live tabs. A tab is deliberately not deduplicated against the first
		// group ("the merge does not fold the two" — `mode.ts:90`): a page that
		// is both a bookmark and an open tab is two different actions.
		if len(out) >= 2*BrowserFetchLimit {
			break
		}
		tab := tab
		// R80's tab key is *position*, not URL: the same page can be open in
		// two windows and the two rows must stay distinct
		// (`browser-page.ts:216`, `tabRowKey`). The subtitle is the URL alone,
		// as the reference's tab row carries `subtitle: tab.url` and is told
		// apart by the app-window glyph (`mode.ts:208`, R73).
		out = append(out, Item{
			ID:     "tab:" + tab.BrowserID + ":" + strconv.Itoa(tab.Window) + ":" + strconv.Itoa(tab.Index),
			Title:  tab.Label(),
			Detail: tab.URL,
			Run:    func() { a.activateTab(tab) },
			tab:    &tab,
		})
	}
	return out
}

// openResult opens a browser result's page in the browser it came from.
func (a *App) openResult(result browser.Result) {
	if a.Actions.OpenURL == nil {
		return
	}
	if err := a.Actions.OpenURL(result.BrowserID, result.URL); err != nil {
		a.WarnFeedback(a.copy().OpenURLFailed)
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

// ClipboardDisabled is the clipboard mode's own status row when the history is
// switched off: the id the reference's emission gives it
// (`clipboardStatusRow("clipboard-disabled", …)`, `useLauncherCatalog.ts:814-816`).
const ClipboardDisabled = "clipboard-disabled"

// clipboardItems is the clipboard mode's list: the history entries matching
// what the user typed after the mode word, or the mode's own "the history is
// off" note when the plugin is switched off.
//
// The reference answers a switched-off plugin *before* it reads anything
// (`useLauncherCatalog.ts:814-816`): `clipboardEmission` returns a single
// `clipboardStatusRow` and no history at all, so the mode names what happened
// rather than claiming nothing was ever copied. That row is one of R40's
// status lines -- the same shape the browser's "plugin is off" row uses -- so
// the launcher draws it muted, unselectable and unnumbered, and the window
// charges it the notes' own band. The settings are read here rather than
// through `ClipboardSource`: the store is *kept* either way (the setting turns
// the watcher off, not the history file), so a mode that trusted the
// interface's mere presence would list a history the app has stopped
// recording.
func (a *App) clipboardItems() []Item {
	if !settings.ClipboardOf(a.settings()).Enabled {
		return []Item{{
			ID:     ClipboardDisabled,
			Title:  a.copy().ClipboardDisabled,
			status: true,
		}}
	}
	if a.Clipboard == nil {
		return nil
	}
	// The needle is the store's own search; the *depth* handed to it is the
	// mode's fetch limit, and the chip filter runs before that limit is spent
	// (the reference's order in `clipboardModeRows`:
	// `filterClipboardEntries(needle).filter(clipboardEntryMatchesFilter)`,
	// then `matched.slice(0, limit)`). Capping inside the search instead would
	// spend the budget on rows a chip hides: with `limit` matching entries
	// taken first, a query with many text hits would leave the `images` chip
	// empty even though the history is full of them — the same rule P7-ec
	// fixed for the browser's range chips.
	entries := a.Clipboard.Search(a.clipboardQuery(), 0)
	copy := a.copy()
	now := a.now()
	out := make([]Item, 0, len(entries))
	kept := make([]clipboard.Entry, 0, min(len(entries), ClipboardFetchLimit))
	for _, entry := range entries {
		entry := entry
		if !a.clipboardFilterAllows(entry) {
			continue
		}
		if len(out) >= ClipboardFetchLimit {
			break
		}
		kept = append(kept, entry)
		// A favourited row shows its state through the star *control* at its
		// trailing edge (see rowBody's rowFavorite branch), never through a
		// `★` badge in the row's key slot: the reference's own row carries no
		// such text, and printing both made a starred entry wear two stars.
		out = append(out, Item{
			ID:     "clip:" + entry.ID,
			Title:  clipboardRowTitle(copy, entry),
			Detail: clipboardRowSubtitle(copy, entry, now),
			Run:    func() { a.copyClip(entry) },
			clip:   &entry,
			thumb:  a.clipThumb(entry),
		})
	}
	// The thumbnail memo is bounded by the rows this search actually builds
	// (the old build's R97 passed the render's own ids), not by everything the
	// store matched.
	a.pruneThumbs(kept)
	return out
}

// clipboardRowPreviewMax is how long a clipboard row's own preview may be
// before it is elided: the old build's `clipboardPreview(entry, 96)`, a row's
// worth of text rather than a paragraph's.
const clipboardRowPreviewMax = 96

// clipboardRowTitle is a clipboard row's first line: a file list's first name
// with a `+N` for the rest, otherwise the entry's preview, falling back to the
// type word when there is nothing to show — an empty row would read as a bug.
func clipboardRowTitle(copy i18n.Launcher, entry clipboard.Entry) string {
	title := ""
	if entry.Kind == clipboard.KindFiles {
		base, extra := clipboard.FilesSummary(entry.Paths)
		title = base
		if extra > 0 {
			title += " +" + strconv.Itoa(extra)
		}
	} else {
		title = entry.Preview(clipboardRowPreviewMax)
	}
	if title == "" {
		return clipboardTypeWord(copy, entry)
	}
	return title
}

// clipboardRowSubtitle is a clipboard row's second line: the entry's type word
// and, when the entry has a timestamp, its compact age (`42m`, `5h`, or a
// concrete local time beyond a day). The word leads because it is the fact the
// chips are named by, so the row and the chip that revealed it agree.
func clipboardRowSubtitle(copy i18n.Launcher, entry clipboard.Entry, now time.Time) string {
	word := clipboardTypeWord(copy, entry)
	if age := clipboard.Age(entry.Time(), now); age != "" {
		return word + " · " + age
	}
	return word
}

// clipboardTypeWord is the word a clipboard row and its chip share: the four
// kinds the chips row offers, with a link being the text entry whose whole
// value is a page address.
func clipboardTypeWord(copy i18n.Launcher, entry clipboard.Entry) string {
	switch clipboardEntryKind(entry) {
	case filterImage:
		return copy.ClipboardTypeImage
	case filterFiles:
		return copy.ClipboardTypeFiles
	case filterLink:
		return copy.ClipboardTypeLink
	default:
		return copy.ClipboardTypeText
	}
}

// ClipboardFetchLimit is how many clipboard rows the mode *holds*: the
// reference's `MAX_CLIPBOARD_MAX_ITEMS` (`main:src/clipboard-history.ts:440` —
// the ceiling its `max_items` setting honours), which the inline launcher
// passes as the mode's fetch depth (`main:src/hooks/useLauncherCatalog.ts:833-834`:
// `clipboardModeRows(entries, mode, t, Date.now(), MAX_CLIPBOARD_MAX_ITEMS)`).
// The mode's own `CLIPBOARD_FETCH_LIMIT` of ten rows is the *plugin output
// view*'s group budget, which this build deliberately does not take: the
// launcher list is a virtualized scroll list, so a ten-row ceiling would only
// hide rows the scroll can reach (the decision P7-ec made for the browser's
// `BROWSER_FETCH_LIMIT`).
const ClipboardFetchLimit = clipboard.MaxMaxItems

// maxRecentResults is how many recent applications the empty query offers: the
// old build's whole result budget (`MAX_RESULTS` in src/launcher/
// result-budget.ts), because the front door is the recents and nothing else
// shares the slots there.
const maxRecentResults = 10

// pinClip shows a clipboard entry's single piece of text in a window of its
// own: the row's own title, and the entry's text — the caption for an image,
// the body for a text entry. The title comes from the row the user is looking
// at, never from a re-render, so the window is named for what they saw (the old
// build's `pinTextFor`, R90).
func (a *App) pinClip(entry clipboard.Entry) {
	if a.Actions.PinText == nil || !clipboardPinApplies(entry) {
		return
	}
	a.Actions.PinText(clipboardRowTitle(a.copy(), entry), entry.Text)
}

// clipboardPinApplies reports whether a clipboard row earns a "pin as a text
// window" control: only the kinds that carry one piece of text do. A text entry
// always does — an empty capture is still a capture, and the empty string is a
// real value. An image does when it stored a caption, because the caption is
// its text. A file list does not: it holds several paths, and pinning their
// concatenation would make the window claim to hold one body the entry never
// had.
func clipboardPinApplies(entry clipboard.Entry) bool {
	switch entry.Kind {
	case clipboard.KindFiles:
		return false
	case clipboard.KindImage:
		return entry.Text != ""
	default:
		return true
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
			Source:   entry.IntegrationName,
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

// pathSourceWord is the source word a PATH command's row carries — the old
// catalog's `source_name: "PATH"` for `SystemCommand` entries, naming the
// provider the row came from rather than a package.
const pathSourceWord = "PATH"

// toolItems is the commands found on the PATH as result rows, only when the
// setting asks for them.
//
// Each row carries the two facts the old build's catalog gave a PATH command
// (catalog.rs `system_command_entries`): the source word "PATH" — the same
// slot an integration's name fills for its own commands — and the executable's
// full path as the second line, which tells *which* command of that name the
// row runs when several directories put one on the PATH, and doubles as
// searchable text.
func (a *App) toolItems() []Item {
	if !a.ShowTools {
		return nil
	}
	out := make([]Item, 0, len(a.Tools))
	for _, tool := range a.Tools {
		out = append(out, Item{
			ID:     "tool:" + tool.Path,
			Title:  tool.Name,
			Detail: tool.Path,
			Source: pathSourceWord,
			alias:  a.Aliases[tool.Name],
			Run:    func() { a.runTool(tool) },
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
		app := app
		out = append(out, appItem(app, func() { a.openApp(app) }))
	}
	return out
}

func (a *App) openApp(app apps.App) {
	if a.Actions.OpenApp != nil {
		a.Actions.OpenApp(app)
	}
}

// Catalog is every item the launcher can show: the scanned applications, the
// built-in commands and the extensions' commands, in that order.
//
// The order is the reference's own tie-break, not an accident: its search walks
// `searchableApps` before `SYSTEM_COMMANDS` and then sorts *stably* by score
// (`main:src/hooks/useLauncherCatalog.ts:1201-1293`), so when an application
// and a built-in command score the same rung — `terminal` names the Terminal.app
// and the 「Terminal」 command alike — the application is the row that wins.
// Listing the commands first inverted that: the same query put the built-in
// 「Terminal」 above the application the user was actually reaching for.
//
// The catalog is built once per set of inputs and then reused (see
// catalogShape): every keystroke walks it, and making the rows first — an
// `Item` per application, a `strings.Join` of its aliases, a folded copy of its
// four search registers for every one of them — was half of a keystroke's cost
// and nearly all of its garbage (600 applications: ~200µs, 3.4k allocations
// per frame, measured). The returned slice is shared, so read it and never
// append to it: `Match` copies the rows it ranks and the callers that compose a
// list build their own.
func (a *App) Catalog() []Item {
	shape := a.catalogShape()
	if a.catalogBuilt && a.catalogShapeWas == shape {
		return a.catalog
	}
	items := a.appItems()
	items = append(items, a.commands()...)
	items = append(items, a.toolItems()...)
	items = append(items, a.toolInstallItems()...)
	items = append(items, a.toolInvokeItems()...)
	items = append(items, a.commandItems()...)
	for i := range items {
		items[i].lowerTitle = strings.ToLower(items[i].Title)
		items[i].lowerDetail = strings.ToLower(items[i].Detail)
		items[i].lowerID = strings.ToLower(items[i].ID)
		items[i].lowerSearch = strings.ToLower(items[i].Search)
		items[i].lowerAlias = strings.ToLower(items[i].alias)
	}
	a.catalog, a.catalogBuilt, a.catalogShapeWas = items, true, shape
	// Counted only so a test can read the cache: how many times the rows were
	// made, which the search must not be paying per keystroke.
	a.catalogBuilds++
	return items
}

// catalogShape is what a built catalog was made from. Every input the rows read
// is in it: the setters' revision (a rescan, a new command set, a changed
// alias), the interface language (the built-in commands' wording), and the
// lists' own lengths — the last only so a caller that assigns a field instead
// of calling its setter still rebuilds rather than serving rows for a list it
// no longer has.
type catalogShape struct {
	revision  int
	language  string
	apps      int
	tools     int
	showTools bool
	commands  int
	installs  int
	aliases   int
}

// catalogShape is the current shape of the catalog's inputs.
func (a *App) catalogShape() catalogShape {
	return catalogShape{
		revision:  a.catalogRev,
		language:  a.settings().Language,
		apps:      len(a.Apps),
		tools:     len(a.Tools),
		showTools: a.ShowTools,
		commands:  len(a.Commands),
		installs:  len(a.InstallCatalog),
		aliases:   len(a.Aliases),
	}
}

// appItem is one application as a row, titled with the name the user's own
// desktop shows. The bundle's other name is deliberately *not* printed: the old
// build dropped it as a second line spent on zero information — "企业微信 /
// WeCom", "QQ音乐 / QQMusic" (R14: 「还是很丑，清爽一点」) — and it stays reachable
// through the search key instead. Every alias and the pinyin initials are
// searchable beside both names. The row's *only* subtitle is the platform's own
// one-line description (a Linux `.desktop` entry's `Comment=`), and only when
// there is no localized name: the old build's subtitle slot reached for the
// alternate name first (`(localizedName && name) || comment || …`) and then
// dropped it, so a localized entry never showed its comment either.
func appItem(app apps.App, run func()) Item {
	title := app.Name
	if app.Localized != "" {
		title = app.Localized
	}
	detail := ""
	if app.Localized == "" {
		detail = app.Comment
	}
	search := append([]string{app.Name, app.Localized, app.Initials}, app.Aliases...)
	return Item{
		ID:     "app:" + app.Path,
		Title:  title,
		Detail: detail,
		Search: strings.Join(search, " "),
		Run:    run,
		icon:   &app,
	}
}

// recentItems is the most-launched applications as rows, in the order the
// usage store gave, at most limit of them. It mirrors the old build's
// `recentItems` (src/launcher.ts): only applications with a launch count take
// a slot, the order is the count descending with the application list's own
// scan order between equals, and the empty query shows them. The scan-order
// tie-break is applied in the shell (`refreshRecents`), which is the only
// place that knows the application list; here the paths are already ranked.
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
		out = append(out, appItem(app, func() { a.openApp(app) }))
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Results is what the current query shows. The calculator's answer comes
// first when the query is a sum; an empty query shows the built-in
// commands and then the recent applications (a wall of applications is not
// an empty state). The rest is the ranked catalog: the list view builds only
// the rows in view, so a broad query is as cheap a frame as a narrow one.
func (a *App) Results() []Item {
	// One count per full walk of the catalog. A frame calls this exactly once
	// (View reuses the slice), and that is an invariant a test can read: the
	// launcher used to walk it twice — once to ask whether the held row was on
	// screen, once for the list — so every keystroke paid two full matches.
	a.searchRuns++
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
	if a.files {
		return a.filesItems()
	}
	var out []Item
	// A power word claims the query before anything else matches: a user who
	// types "restart" means the machine, so those rows lead. Then the answer
	// itself — the calculator's, the built-in commands over an empty field, the
	// ranked catalogue over a typed one.
	other := a.otherResults()
	out = append(out, a.powerItems()...)
	out = append(out, other...)
	// The action bar is the panel's **pinned footer**, not a first row: the old
	// build drew it *below* the scroller (`.launcher-action-bar`, a sibling of
	// `.launcher-results`, not a row inside it), which is why it is appended
	// after every match here. Its row is the field's own hand-off — run the line
	// in the terminal, open the URL, open the path — and it is drawn whenever
	// the query is something the field itself can act on *and* the list has a
	// match to sit under (`visibleActionBar`'s gate, which reads the matches and
	// not the field's text; see `actionBarItemsWith`). The user's report
	// 「现在固定在第一项」 was this row drawn first, above the results; the
	// reference's footer position is the fix.
	out = append(out, a.actionBarItemsWith(other, true)...)
	// The held modifier's own line closes the panel — on the *empty* field only,
	// which is the reference's one placement (R64,
	// `main:src/launcher/terminal-row.ts:77-85`: a typed query draws the row not
	// at all). It rides the modifier — it is gone the moment the key is
	// released — and it is appended after the bar so it is genuinely the last
	// line, below the recents, the way the reference appended its own held row.
	if item, ok := a.heldTerminalItem(); ok {
		out = append(out, item)
	}
	return out
}

// otherResults is everything but the action bar and the held-modifier row:
// the calculator, the built-in commands, the recents, and the ranked catalog.
func (a *App) otherResults() []Item {
	var out []Item
	if item, ok := a.calculator(); ok {
		out = append(out, item)
	}
	if strings.TrimSpace(a.Query) == "" {
		out = append(out, a.commands()...)
		// The empty page's own block: the most-launched applications, under
		// their own heading, in the order the shell ranked them (count
		// descending, the application list's scan order between equals — the
		// old build's `recentItems`).
		if recents := a.recentItems(maxRecentResults); len(recents) > 0 {
			out = append(out, Item{ID: "section:recent", heading: true, Title: a.copy().RecentSection})
			out = append(out, recents...)
		}
	} else {
		out = append(out, a.matchedCatalog()...)
	}
	return out
}

// matchedCatalog ranks the catalog for the query and names the alias any row
// was reached through, in the row's own subtitle — the old build's
// `launcher.aliasMatch`. The composition lives here (not in Match) because the
// wording is the app's, not the ranking's: Match answers *which* alias carried
// the row, this answers what the row says about it.
func (a *App) matchedCatalog() []Item {
	items := Match(a.Catalog(), a.Query)
	copy := a.copy()
	for i := range items {
		if items[i].matchedAlias == "" {
			continue
		}
		hint := copy.AliasMatch(items[i].matchedAlias)
		if items[i].Detail == "" {
			items[i].Detail = hint
		} else {
			items[i].Detail = hint + " \u00b7 " + items[i].Detail
		}
	}
	return items
}

// calculator is the answer row for an arithmetic query, if the query is
// one. Enter copies the answer and clears the field, as the old launcher's
// calculator did.
func (a *App) calculator() (Item, bool) {
	expression := calc.Expression(a.Query)
	value, err := calc.Eval(expression)
	if err != nil {
		return Item{}, false
	}
	result := calc.Format(value)
	// The answer is the trailing value, complete at the row's right edge, and
	// the expression is the title that gives up room — the same row R50 drew
	// for the history, with the two registers swapped when the query is the
	// expression rather than a history entry.
	return Item{
		ID:       "calc",
		Title:    expression,
		Detail:   "",
		trailing: result,
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

	// The ranking is over *indices*, not over copies of the rows: an Item is
	// a dozen strings and a closure, and the port used to copy every match
	// into an intermediate slice (and again into the result) on every
	// keystroke. The catalog is a cached slice the App keeps, so the rows are
	// read in place and only the winners are copied out — once, in order.
	scores := make([]int, len(items))
	order := make([]int, 0, len(items))
	for i := range items {
		best := 0
		ok := true
		for _, term := range terms {
			score, found := itemScore(&items[i], term)
			if !found {
				ok = false
				break
			}
			if score > best {
				best = score
			}
		}
		if !ok {
			continue
		}
		scores[i] = best
		order = append(order, i)
	}
	// Stable over the catalog's own order: equal scores keep the order they
	// were made in, which is what `sort.SliceStable` on the indices does.
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] < scores[order[b]] })

	out := make([]Item, len(order))
	for k, i := range order {
		out[k] = items[i]
		// The alias the row was reached through is named in its subtitle, and
		// it is written on the *copy*: the catalog slice is cached across
		// frames, so a hint left on it would outlive the query that earned it.
		if hint := aliasHint(items[i], terms[0]); hint != "" {
			out[k].matchedAlias = hint
		}
	}
	return out
}

// aliasHint is the alias a query reached a row through, or "".
//
// The old build's rule: the row names the alias only when the alias is what
// carried the match — the alias scores *better* than the row's own title for
// the token typed. An alias that merely ties or loses to the name stays quiet
// (the name is the answer, and naming the alias would explain nothing); a
// query that does not reach the alias at all says nothing either.
func aliasHint(item Item, term string) string {
	if item.alias == "" || term == "" {
		return ""
	}
	aliasScore, aliasMatched := aliasRank(lowered(item.lowerAlias, item.alias), term)
	if !aliasMatched {
		return ""
	}
	titleScore, titleMatched := aliasRank(lowered(item.lowerTitle, item.Title), term)
	if titleMatched && titleScore <= aliasScore {
		return ""
	}
	return item.alias
}

// aliasRank is candidateScore with an exact hit above a prefix hit, which the
// shared ladder folds together (it only asks whether a candidate *starts with*
// the term). The old build's `candidateMatchScore` ranks exact > prefix, so an
// alias that is exactly what the user typed beats a name that merely starts
// with it — that is the case the hint exists for. Everything below exact
// keeps candidateScore's order, shifted up by one to make room for it.
func aliasRank(candidate, term string) (int, bool) {
	if candidate == term {
		return 0, true
	}
	score, ok := candidateScore(candidate, term)
	if !ok {
		return 0, false
	}
	return score + 1, true
}

// itemScore scores one term against an item, and reports whether it matched.
//
// A PATH command's alias runs through the same ladder as its name, so an alias
// hit ranks exactly where the name would (an exact alias is not second class).
func itemScore(item *Item, term string) (int, bool) {
	title := lowered(item.lowerTitle, item.Title)
	score, matched := candidateScore(title, term)
	if item.alias != "" {
		if aliasScore, aliasMatched := candidateScore(lowered(item.lowerAlias, item.alias), term); aliasMatched && (!matched || aliasScore < score) {
			score, matched = aliasScore, true
		}
	}
	if matched {
		return score, true
	}
	if strings.Contains(lowered(item.lowerDetail, item.Detail), term) ||
		strings.Contains(lowered(item.lowerID, item.ID), term) ||
		strings.Contains(lowered(item.lowerSearch, item.Search), term) {
		return 3, true
	}
	return 0, false
}

// candidateScore is the ladder one candidate string is scored by: the string
// starts with the term, a word of it does, or it contains it. A term that is a
// subsequence of the candidate (its characters in order, with anything
// between) matches too, one rung below a contained term — that is what makes
// `vsc` find Visual Studio Code and `wyyyy` find 网易云音乐 (the pinyin
// initials are a subsequence of the name).
func candidateScore(candidate, term string) (int, bool) {
	switch {
	case strings.HasPrefix(candidate, term):
		return 0, true
	case wordPrefixMatch(candidate, term):
		return 1, true
	case strings.Contains(candidate, term):
		return 2, true
	}
	if isSubsequence(term, candidate) {
		return 3, true
	}
	return 0, false
}

// isSubsequence reports whether every rune of term appears in candidate, in
// order, with anything between.
//
// It walks both strings' runes in place: the rune-slice version allocated two
// slices for every row a query did not match — a few thousand per keystroke on
// a machine-wide catalog, on the very path a stall is read from.
func isSubsequence(term, candidate string) bool {
	cursor := 0
	for i := 0; i < len(term); {
		want, size := utf8.DecodeRuneInString(term[i:])
		i += size
		matched := false
		for cursor < len(candidate) {
			got, gotSize := utf8.DecodeRuneInString(candidate[cursor:])
			cursor += gotSize
			if got == want {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// wordPrefixMatch reports whether any word of s starts with term.
//
// The words are scanned in place: `strings.FieldsFunc` allocated a slice (and
// a string header per word) for every row of the catalog on every keystroke,
// which is one allocation per row for the rung a query reaches on almost every
// row — the ranking's whole garbage. A word starts where a word rune follows a
// non-word one (or the string itself), which is the split FieldsFunc made.
func wordPrefixMatch(s, term string) bool {
	start := true
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if start && isWordRune(r) && strings.HasPrefix(s[i:], term) {
			return true
		}
		start = !isWordRune(r)
		i += size
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
