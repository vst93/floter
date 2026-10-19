package launcher

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calculator"
	"floter/internal/clipboard"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/theme"
	"floter/internal/tools"
)

// The launcher's window geometry: a fixed width, and a collapsed height that
// is a scale-1 measurement multiplied by the interface-size factor.
const (
	// InputWindowWidth is the launcher's fixed width, in DIPs.
	InputWindowWidth = 720

	// bareTerminalRowID is the held modifier's own row (the reference's
	// BARE_TERMINAL_ROW_ID): a stable id the renderer, the scroll skip and
	// the release edge all read, never the query that happened to be typed
	// when the modifier went down.
	bareTerminalRowID = "system-terminal-bare"

	// The collapsed launcher's height at scale 1, per platform: the card's own
	// chrome (66u — the field's 56u row, the 4u breath under it and the panel's
	// 4u/2u insets) in the shell's gutters: 10px a side on Linux, 4px + 12px
	// vertically on Windows, none elsewhere.
	inputWindowHeightLinux   = 86
	inputWindowHeightWindows = 82
	inputWindowHeightDefault = 66

	// ResultsAreaHeight is the room under the input row for the result list:
	// about four rows at the shipped interface size. The old shell grew the
	// window to a measured band as results arrived; P1 opens at the collapsed
	// row plus this fixed band so typing never resizes the window.
	ResultsAreaHeight = 240
)

// InputWindowHeight is the collapsed launcher window's height at scale 1.
func InputWindowHeight() float64 {
	switch runtime.GOOS {
	case "windows":
		return inputWindowHeightWindows
	case "linux":
		return inputWindowHeightLinux
	default:
		return inputWindowHeightDefault
	}
}

// WindowHeight is the height the launcher opens at: the collapsed input
// window's height at the settings' interface step, plus the result band. The
// step scales the input row exactly as the old native path scaled its
// fallback height.
func WindowHeight(uiScale string) float64 {
	return InputWindowHeight()*settings.UIScaleFactor(uiScale) + ResultsAreaHeight
}

// Actions are what a result can do, supplied by the shell: the launcher does
// not know how surfaces are opened or how an application launches, only
// that the user asked for one.
type Actions struct {
	// OpenSettings, OpenTerminal and Quit run the built-in commands.
	OpenSettings func()
	// OpenSessions opens the settings on the sessions page: the field row's
	// own terminal button, as the old build kept beside the field.
	OpenSessions func()
	// ConfigFor is what a plugin's configuration is: the schema and the
	// values its settings hold, or nil for a plugin with none.
	ConfigFor func(plugin string) *Config
	// ConfigChange writes one field and returns the values the plugin's
	// settings hold afterwards, or nil when the write was refused.
	ConfigChange func(plugin, key string, value any) plugincfg.Values
	// ConfigAction runs an action field's command, reporting whether it was
	// accepted.
	ConfigAction func(plugin, key string) bool
	// DrawConfig draws the sheet's body: the fields, in the settings pages'
	// own card language. The launcher owns the band; the shell owns the form.
	DrawConfig   func(c *ui.Context, config *Config)
	OpenTerminal func()
	Quit         func()
	// Dismiss is Escape with an empty query: hide the launcher window, as
	// the old shell did.
	Dismiss func()
	// ResizeTo grows or shrinks the window to the rows the list holds: the
	// old shell walked its window edge to the measured band, and the shell
	// does the same walking here. Nil (tests) skips it.
	ResizeTo func(height int)
	// Copy puts text on the clipboard, for the calculator row.
	Copy func(text string)
	// OpenApp launches an installed application.
	OpenApp func(app apps.App)
	// RunCommand runs an extension's command with the arguments the user
	// typed in the command mode (none when the row was clicked).
	RunCommand func(entry extensions.CommandEntry, args []string)
	// RunCommandCaptured runs a command whose manifest sends its output to
	// the background, and hands the captured output back on the main thread.
	RunCommandCaptured func(entry extensions.CommandEntry, args []string, done func(extensions.CapturedRun, error))
	// LoadOutputPage runs a self-paging command again with the cursor from
	// its last emission, for the next page of an output list. Nil leaves a
	// self-paging list's footer at "more" without a way to fetch it.
	LoadOutputPage func(entry extensions.CommandEntry, args []string, cursor string, done func(extensions.CapturedRun, error))
	// Complete asks the provider for completions of the tokens typed so
	// far; the shell answers on the main thread. Nil disables dynamic
	// completion, and the static argument list stands alone.
	Complete func(entry extensions.CommandEntry, tokens []string, done func([]extensions.Completion))
	// PinText shows some text in a window of its own, and DetachOutput opens
	// a command's page in a window that stays put and can re-run it.
	PinText      func(title, text string)
	DetachOutput func(view OutputView)
	// CopyClip puts a clipboard entry back on the clipboard, whatever its
	// kind. Nil falls back to Copy for text.
	CopyClip func(entry clipboard.Entry)
	// OpenURL opens a page in the browser the result came from, falling back
	// to the system's default browser for an empty id.
	// OpenURL opens a page in a browser and reports whether it did: a failure
	// is the row's own line (the old build's launcher.error.url).
	OpenURL func(browserID, url string) error
	// ActivateTab brings one of the browser's live tabs to the front.
	ActivateTab func(tab browser.Tab)
	// RunInTerminal runs a command in the terminal surface: a tool found on
	// the PATH.
	RunInTerminal func(argv []string)
	// RunInShell types a command line into a fresh session of the user's own
	// shell, for a line that carries shell syntax (a pipe, a redirect, a
	// chain) — no argv can say `ls | grep x`, and a real shell is the only
	// reader that can. The old build's run entry typed every line into the
	// interactive shell; this is that route, kept for the lines that need it.
	RunInShell func(line string)
	// RunInTerminalWithEnv runs a command with extra environment variables
	// (the leading NAME=value assignments the user typed), when the shell
	// action's parse found some.
	RunInTerminalWithEnv func(argv []string, env []string)
	// SpawnDetached starts a program of its own with no terminal and no
	// capture: a tool's own GUI action (the flameshot precedent).
	SpawnDetached func(argv []string)
	// OpenPath hands a path to the system's own opener, for a dropped file's
	// "open" action.
	OpenPath func(path string)
	// OpenInTerminal opens the terminal surface in a directory, for a
	// dropped file's "cd" action.
	OpenInTerminal func(dir string)
	// OpenTerminalWindow opens the user's own terminal emulator, for the
	// external-terminal shortcut.
	OpenTerminalWindow func()
	// OpenInstallSession opens a bare terminal session with a command typed
	// into it, for the tool catalog's install rows.
	OpenInstallSession func(command string)
	// Power restarts or shuts down the machine, after the shell has confirmed
	// it.
	Power func(action string)
	// SearchBrowser searches the browser the settings name; the shell answers
	// on the main thread, once with the file results and again when the live
	// tabs land. Nil leaves the mode empty.
	SearchBrowser func(query string, done func(BrowserResults))
}

// BrowserResults is one browser search's answer: the merged bookmarks and
// history, and the browser's live tabs.
type BrowserResults struct {
	// Found is false when no browser profile was found at all.
	Found bool
	// Profile is the profile the search ran against.
	Profile browser.Profile
	// Results are the bookmarks and history rows, already merged, ordered and
	// capped.
	Results []browser.Result
	// Disabled marks a search the browser plugin could not run, because the
	// plugin is switched off: the mode says so rather than claiming no browser
	// has a profile.
	Disabled bool
	// Tabs are the tabs the browser has open right now, when they could be
	// read. A tab that is also a bookmark stays two rows: switching to it and
	// opening it again are two different actions.
	Tabs []browser.Tab
}

// ClipboardSource is the clipboard history the launcher searches and edits:
// the mode's own star and delete keys act through it, and an image entry's row
// reads its thumbnail through ImagePath.
type ClipboardSource interface {
	Search(query string, limit int) []clipboard.Entry
	SetFavorite(id string, favorite bool) error
	Remove(id string) error
	ImagePath(entry clipboard.Entry) string
}

// App is the launcher surface's state: the settings store it reads, the
// query the field edits, the chosen result, and the handles the framework
// needs to keep the field focused across builds.
type App struct {
	Store   *settings.Store
	Actions Actions

	Query    string
	Selected int

	// pendingChord and pendingDigit are the user's own bindings as the field
	// claimed them (see `appChordKeys`): the field is the only seam in front
	// of the editor, so it resolves the chord and the view runs it on the
	// frame that follows, where the list and its numbers are the ones on
	// screen.
	pendingChord    string
	pendingDigit    int
	pendingDigitSet bool
	// pendingFavorite is the history star's mode-local chord as the field
	// claimed it (⌘D / Ctrl+D, see `historyStarMods`): the editor would take
	// the press on macOS, so the view runs it a frame later.
	pendingFavorite bool

	// Apps is the installed applications the shell scanned, and Commands the
	// enabled extensions' commands; both join the results once the user
	// types.
	Apps     []apps.App
	Commands []extensions.CommandEntry
	// Recent are the most-launched applications' paths, newest habit first,
	// and ShowRecent says whether the empty query offers them. The shell
	// keeps both in step with the usage file and the setting.
	Recent     []string
	ShowRecent bool
	// commandConflicts marks the command words two integrations share, so a
	// row whose word is shadowed says so.
	commandConflicts map[string]bool
	// Tools are the commands found on the PATH, and ShowTools whether the
	// search offers them (the show_commands_in_search setting).
	Tools     []apps.App
	ShowTools bool
	// InstallCatalog is the install catalog's state: the tools floter knows
	// where to get, and whether this machine has them.
	InstallCatalog []tools.State
	// Aliases is the user's alias per command, which a row is matched by as
	// well as by its name: a PATH command's own name, an extension command's id
	// (the Integrations alias editor's key). It is a property of the catalog,
	// not of the PATH scan, so it stays put when the PATH rows are off.
	Aliases settings.CommandAliases

	// Scroll keeps the result list's place; Search is the field's identity,
	// for the focus the launcher keeps on it while the surface shows.
	List   ui.ListState
	Search ui.Handle

	// chosenRow tracks the row the keyboard last moved to, so the list is
	// scrolled only when the choice changed.
	chosenRow int

	// hoveredRow is the row the pointer was over the frame before, and
	// hoverFrameRow the one it is over in the frame being built. The pair is
	// what makes the pointer's selection an *entry edge* rather than a
	// per-frame re-assert — the reference's `onPointerEnter`
	// (`LauncherResults.tsx:859-862`). Re-asserting every frame would undo a
	// keyboard step the moment the pointer rested on the row the selection
	// had just left (the conflict `launcher.css:1653-1663` names); an edge
	// leaves the two devices sharing one state.
	hoveredRow    int
	hoverFrameRow int
	// footerBand is the band the pinned action-bar footer occupies at the
	// panel's bottom edge this frame, in pixels (see actionBarSplit). The
	// docked rows — the toast and the power confirmation — sit above it, so a
	// footer and a message never paint over each other.
	footerBand float32

	// Rows are what the last View built, in order: each visible row's text
	// lines and whether it is a block's heading. Font and Spacing are the
	// theme's own values. The shell reads them to size the window to the list
	// it holds (see Geometry).
	Rows         []RowInfo
	Font         float64
	TitleLine    float64
	SubtitleLine float64
	Spacing      float64
	// HeldRows is the row count the window is sized for, with the shrink
	// hysteresis applied: a keystroke that removes one row of two does not
	// walk the window edge for it, and a growth is immediate. Zero until
	// the first View measured the list.
	HeldRows int

	// modifierHeld is the app modifier's state as the last frame saw it. While
	// the modifier is down on an empty field, the list ends with a row that
	// opens a bare terminal session — the row the user asked for, and the one
	// the keyboard lands on so Enter reaches it without a second key.
	modifierHeld bool
	// heldWasDown is the modifier's state the frame before, so the press and
	// release edges are one transition each.
	heldWasDown bool
	// heldRowKey is the list shape the held row's selection was last asserted
	// for: the row count and the query, joined. The reference's own effect
	// re-runs on `query` and `displayedResultsLength` (`main:src/App.tsx:2065-2082`,
	// the heldBareTerminal effect), so a keystroke while the modifier is down
	// re-takes the selection for the held row instead of letting the fresh
	// query's default land on the first match — the report 「按住时…应该有选中
	// 样式（当前缺失）」, where typing with the modifier held left the row on
	// screen, unselected, and the highlight pinned to the list's first line.
	// Empty when the modifier is up, so the next press asserts again.
	heldRowKey string
	// barRowKey is the same shape key for the action bar, which is the row the
	// modifier hands the selection to when the field has text: the reference
	// draws no held row for a typed query (R64) and the bar is the row that
	// acts on the line. It is asserted on the same shape changes the held row
	// is, so typing with the modifier down keeps the bar lit while an arrow
	// step off it survives (the reference's effect re-runs on `query` and the
	// list's length, not on the selection).
	barRowKey string
	// heldRowShown is whether the held row was the list's last line on the
	// frame before. The release edge asks it to decide whether the selection
	// has to be handed back to the page's default, and the answer is a fact
	// about the *previous* frame: once the modifier is up the row is already
	// gone, so this frame's own list cannot answer it.
	//
	// It is recorded here rather than recomputed by asking Results() again
	// (which is what `heldRowShowing` did, once per frame — a second full
	// catalog match beside the frame's own, on an idle launcher as much as a
	// busy one; see App.searchRuns).
	heldRowShown bool
	// searchRuns counts the frame's own catalog work (see Results). It exists
	// so a test can pin that a frame searches **once**: the launcher used to
	// walk the catalog twice per frame — once for the held row's presence, once
	// for the list — and a duplicated full match is the cheapest way to read a
	// keystroke as a stall.
	searchRuns int

	// toast is a message to show in the panel's docked status row, set by an
	// action, and toastWarning marks it a warning — the blocking tone the old
	// build's `.launcher-feedback--warning` carried.
	toast        string
	toastWarning bool
	// toastUntil is when the message expires and toastShown the text the
	// deadline was armed for, so a *new* message restarts the clock.
	toastUntil time.Time
	toastShown string
	// pendingCaret puts the caret at the end of the field on the next
	// build, for text the app set itself.
	pendingCaret bool

	// defaultedQuery is the query whose default selection is already in
	// place. A query can change without the field: the typed-command history,
	// a plugin filling the field, a mode's own text. The reference reset the
	// choice whenever the query did (`firstRunnableResultIndex`), so a frame
	// that sees a query the field has not accounted for puts the choice back
	// on that query's default before anything reads it — otherwise a stale
	// index points at whatever row now sits there.
	defaultedQuery string

	// mode is the extension command being typed: while it is set, the field
	// holds the command's argv and the list offers its arguments.
	mode *extensions.CommandEntry
	// dynamicFor is the token key the provider's completions belong to, and
	// requestedFor the last key asked for, so one edit asks once.
	dynamicFor   string
	dynamic      []extensions.Completion
	requestedFor string
	// clipboard is set while the clipboard history is searched: the field
	// holds the mode word and the query, and the list offers entries.
	clipboard bool
	// clipboardFilter is the clipboard mode's chosen chip, and browserFilter
	// the browser mode's — the axes' current values, walked by Tab and by the
	// chips row.
	clipboardFilter string
	browserFilter   string
	// History is the query history, newest first, and historyIndex the line
	// the field is showing (-1 when it shows the user's own draft). The draft
	// the user was typing is kept, so ↓ can bring it back.
	History            []string
	historyIndex       int
	draftBeforeHistory string
	// numbers maps a row's index to the number its result shortcut answers
	// to, recomputed every frame: 1..9 then 0 for the tenth, over the rows
	// that can be run.
	numbers map[int]int
	// outputNumbers is the same family for the captured-output list: the
	// numbered shortcuts address the output's own rows while it owns the band
	// (see assignOutputNumbers).
	outputNumbers map[int]int
	// icons caches the decoded application icons, by path, and thumbs the
	// clipboard images'.
	icons  map[string]*ui.Bitmap
	thumbs map[string]*ui.Bitmap
	// pathStat is the action bar's memo of the last typed path's own answer.
	// The bar is rebuilt every frame, so a path query used to `os.Stat` the
	// path sixty times a second — on a network share or a sleeping disk that
	// is the whole window stopped answering, not a slow row.
	pathStatPath   string
	pathStatIsDir  bool
	pathStatExists bool
	pathStatAt     time.Time
	// output is the captured output of a background command run, shown in
	// place of the result list; Output keeps its scroll offset.
	output *OutputView
	Output ui.ScrollState
	// config is the open plugin-configuration overlay, which takes the result
	// list's place (see config.go).
	config *Config
	// modeWord is the trigger word the mode on screen was entered with: the
	// scope marker draws it as the icon and the name beside the field, and
	// leaving the mode restores it as an ordinary query.
	modeWord string
	// armedDelete is the selected row's two-step delete: the first press of
	// the row's own control (or of the history-delete key, see fieldkeys.go)
	// arms it — the row prints the muted "press again" note in place of its
	// trailing chrome — the second removes. It holds the armed row's item id,
	// the empty string while nothing is armed, so a list that shifted under
	// the arm cannot confirm on a different row.
	armedDelete string
	// armedDeleteAt is when the arm was set. The old build's
	// `HISTORY_DELETE_CONFIRM_MS` window (history-actions.ts): an armed row
	// waits a few seconds for its confirming press and then disarms itself, so
	// a confirmation asked for one moment never outlives it. A press after the
	// window re-arms rather than deleting.
	armedDeleteAt time.Time
	// armedDeleteAnnounced tracks whether the current arm has been
	// announced to screen readers (the old build's `aria-live="polite"`).
	armedDeleteAnnounced bool
	// clock is the launcher's time source, injectable so the history-delete
	// window can be exercised without waiting for it; nil means the wall
	// clock.
	clock func() time.Time
	// pendingPower is the power action the panel is asking about: the
	// confirmation row shows until the user answers it (see power.go).
	pendingPower string
	// pendingPowerAt is when the question was asked. The old build cleared
	// it with `useTimedReset` (three seconds): an unanswered confirmation
	// goes away by itself rather than waiting for a later glance.
	pendingPowerAt time.Time
	// OutputList is the output list's own scrolling and selection state.
	OutputList ui.ListState
	// files is set while a drop's rows are listed, with Dropped the files the
	// desktop handed over, and dropsExpanded whether the "and N more" row has
	// been run (the reference's `dropsExpanded`).
	files         bool
	Dropped       []drops.File
	dropsExpanded bool
	// calculatorMode is set while the calculator history is open, with the
	// entries the shell last handed over and the filter Tab cycles.
	calculatorMode   bool
	calculatorFound  []calculator.Entry
	calculatorFilter string
	// CopyMode is what Enter copies from a calculator row: the whole line or
	// the result alone, from the calculator plugin's settings.
	CopyMode string
	// Calculator is the history the mode shows; nil leaves it empty.
	Calculator CalculatorSource
	// browser is set while the browsers' history is searched, with the
	// answer the shell last gave and the query it belongs to.
	browser      bool
	browserFound BrowserResults
	// browserAskedFor is the query the results belong to and browserAsked
	// whether any arrived (the empty query is a real query, so a bool is
	// needed); browserPending is the query the shell is working on.
	browserAskedFor string
	browserAsked    bool
	browserPending  bool
	// Clipboard is the history the launcher searches; nil until the shell
	// gives it one.
	Clipboard ClipboardSource
}

// appIcon decodes an application's icon once and keeps it, so a redraw is a
// map lookup rather than a file read and a PNG decode.
func (a *App) appIcon(app apps.App) *ui.Bitmap {
	if a.icons == nil {
		a.icons = map[string]*ui.Bitmap{}
	}
	if bitmap, ok := a.icons[app.Path]; ok {
		return bitmap
	}
	var bitmap *ui.Bitmap
	if data := apps.Icon(app); len(data) > 0 {
		if decoded, err := ui.DecodeBitmap(data); err == nil {
			bitmap = decoded
		}
	}
	if bitmap == nil && apps.IconPending(app) {
		// The icon is being extracted in the background (a Windows
		// shortcut's, which is a process launch). Remembering nothing for
		// this frame keeps the row's answer open: the frame the extraction
		// asks for reads the icon, and the drawing thread never waits for a
		// process — a launch per row on the first summon was the seconds of
		// 「卡死」.
		return nil
	}
	a.icons[app.Path] = bitmap
	return bitmap
}

// maxResultSlots is the size of the numbered family: `1`-`9` and then `0`,
// one digit behind the modifiers. The old build's `MAX_RESULTS`.
const maxResultSlots = 10

// assignNumbers gives the numbered rows their shortcut number, recomputed each
// frame. The numbers follow the **scroll viewport**: `1`-`9` are the first
// nine runnable rows on screen and `0` the tenth, so scrolling renumbers the
// list to what the user can see (the old build's R34, "what you see is what
// you select"). Rows outside the viewport carry no number, and the modifier
// plus a digit cannot reach them.
//
// The old build gates the viewport on the list overflowing the ten-slot
// budget (R72, `listOverflowsBudget`): a list that fits the budget is numbered
// purely in order off the rendered rows, because a list that fits cannot be
// scrolled and the viewport measurement adds nothing but risk (a viewport
// measured mid-walk once left a ten-row list with one badge and nine rows with
// nothing). Past the budget the viewport rule stands. The viewport is the
// previous frame's, exactly as the old build's key handler read the last
// render's slots, so a scroll renumbers the family on the next frame.
func (a *App) assignNumbers(results []Item) map[int]int {
	numbers := map[int]int{}
	// The action bar is not a result: the old build's `resultShortcutSlots`
	// ran over `displayedResults` (which excludes the bar), so the bar takes
	// no number and the family starts on the first real result. Counting it
	// here would shift every badge by one and let `⌘1` run the shell row.
	resultCount := 0
	for _, item := range results {
		if !strings.HasPrefix(item.ID, "bar:") && item.ID != bareTerminalRowID {
			resultCount++
		}
	}
	// A list that fits is numbered whole; only an overflowing one consults
	// the viewport.
	first, last := 0, len(results)-1
	if resultCount > maxResultSlots {
		first, last = 0, maxResultSlots-1
		if visibleFirst, visibleLast := a.List.Visible(); visibleLast >= visibleFirst {
			first, last = visibleFirst, visibleLast
		}
	}
	count := 0
	for i, item := range results {
		if item.Run == nil || strings.HasPrefix(item.ID, "bar:") || item.ID == bareTerminalRowID {
			continue
		}
		if i < first || i > last {
			continue
		}
		count++
		if count > maxResultSlots {
			break
		}
		// The tenth row answers to 0, as a keyboard's digits end.
		numbers[i] = count % 10
	}
	a.numbers = numbers
	return numbers
}

// takePendingDigit takes the digit of the result family the field claimed, if
// any. The field is the one seam in front of the editor (see
// `appChordKeys`), so the chord arrives here already resolved; taking it
// clears it, and the caller cannot ask twice.
func (a *App) takePendingDigit() (int, bool) {
	if !a.pendingDigitSet {
		return 0, false
	}
	a.pendingDigitSet = false
	return a.pendingDigit, true
}

// pressedResultDigit reports the digit of the result family whose chord is
// down, if any, for the views that do not keep the keyboard on the search
// field (the captured-output list). The modifiers come from the settings'
// select_result binding, so rebinding the family moves every number with it.
func pressedResultDigit(a *App, c *ui.Context) (int, bool) {
	for digit := 0; digit <= 9; digit++ {
		key := ui.Key0
		if digit > 0 {
			key = ui.Key0 + ui.Key(digit)
		}
		mods, wanted, ok := shortcuts.Parse(settings.SelectResultDigit(a.settings(), digit))
		if !ok || wanted != key {
			continue
		}
		if c.Shortcut(mods, key) {
			return digit, true
		}
	}
	return 0, false
}

// resultShortcuts runs the row a number names, reporting whether the key was
// one of them.
func (a *App) resultShortcuts(results []Item) bool {
	digit, ok := a.takePendingDigit()
	if !ok {
		return false
	}
	a.runResultDigit(results, digit)
	return true
}

// runResultDigit runs the row the digit names, against the numbers the last
// frame handed out. It is the one place a numbered row is run: the shortcut
// layer reaches it through `resultShortcuts`, and the field reaches it
// through `appChordKeys` — a bound chord the editor would otherwise take as
// text (an `Option`+letter on macOS) never gets to the shortcut layer at all.
func (a *App) runResultDigit(results []Item, digit int) {
	for index, number := range a.numbers {
		if number != digit || index >= len(results) {
			continue
		}
		a.Selected = index
		a.activate(results)
		return
	}
}

// outputShortcuts runs the numbered row of the captured-output list. The
// output's rows are the launcher's own numbered family — the old build fed the
// plugin view's items through the same `resultShortcutSlots` as any result
// (`main:src/App.tsx:1602-1611`) — so `⌘1`-`⌘9` run the first nine runnable
// rows and `⌘0` the tenth, against the output list's own viewport (see
// assignOutputNumbers). A digit with no row behind it is still consumed: it
// belongs to the family, not to the field.
func (a *App) outputShortcuts(c *ui.Context) bool {
	if a.output == nil || !a.output.IsList {
		return false
	}
	// The output view does not keep the keyboard on the search field, so the
	// digit never reaches the field's seam: it is asked here, where the view
	// registers it, as the ordinary page's digits used to be.
	digit, ok := pressedResultDigit(a, c)
	if !ok {
		return false
	}
	a.runOutputDigit(digit)
	return true
}

// runOutputDigit runs the output list's row the digit names, the output view's
// half of `runResultDigit`.
func (a *App) runOutputDigit(digit int) {
	if a.output == nil || !a.output.IsList {
		return
	}
	for index, number := range a.outputNumbers {
		if number != digit || index >= len(a.output.Rows) {
			continue
		}
		a.output.Selected = index
		a.runOutputRow()
		return
	}
}

// appShortcuts runs the app's own rebindable keys: a new command clears
// the field, the external-terminal action opens a terminal window, and the
// open-settings action leaves for the settings surface. ⌘W is handled by the
// shell's own appShortcuts, on every surface — this one is the launcher's own
// modes only.
//
// The shortcut list is the settings page's own `ShortcutActions`; a binding
// the page lets the user record and never answers is a promise the app does
// not keep. The launcher is where the old build answered `open_settings`
// (the collapsed surface), so this is the surface that owes it.
func (a *App) appShortcuts() bool {
	action := a.pendingChord
	if action == "" {
		return false
	}
	a.pendingChord = ""
	switch action {
	case settings.ShortcutNewCommand:
		// On the launcher surface the shell answers the new-command chord by
		// hiding the window, and that is `Actions.Dismiss` (the shell's own
		// `appShortcuts` leaves this surface to the field now, so the chord
		// would otherwise fall between the two). Without a window to hide —
		// a test, a detached view — the chord keeps its own meaning: the
		// empty field a new command asks for.
		if a.Actions.Dismiss != nil {
			a.Actions.Dismiss()
		} else {
			a.ResetQuery()
		}
	case settings.ShortcutOpenExternalTerminal:
		if a.Actions.OpenTerminalWindow != nil {
			a.Actions.OpenTerminalWindow()
		}
	case settings.ShortcutOpenSettings:
		if a.Actions.OpenSettings != nil {
			a.Actions.OpenSettings()
		}
	}
	return true
}

// emptyMessage is what the list says when it has nothing: a plugin mode names
// its own state (nothing copied yet, no favorites, no matches) rather than
// claiming a search found nothing, and the ordinary page says so plainly.
func (a *App) emptyMessage(copy i18n.Launcher) string {
	switch {
	case a.mode != nil:
		return copy.CommandModeHint
	case a.clipboard:
		switch {
		case a.clipboardQuery() != "":
			return copy.ClipboardEmptyFilter
		case a.clipboardFilter == filterFavorites:
			return copy.ClipboardEmptyFavorites
		default:
			return copy.ClipboardEmpty
		}
	case a.browser:
		answer := a.browserResults(a.browserQuery())
		if answer.Disabled {
			// The plugin is switched off: the mode says so, and where to turn
			// it on, rather than claiming the machine has no browser.
			return copy.BrowserDisabled
		}
		if !answer.Found {
			// No profile at all says why rather than claiming the query
			// matched nothing.
			return copy.BrowserNoProfile
		}
		return copy.BrowserEmpty
	case a.calculatorMode:
		if a.calculatorFilter == filterFavorites {
			return copy.CalculatorEmptyFavorites
		}
		return copy.CalculatorEmpty
	case a.files:
		return copy.NoResults
	default:
		return copy.NoResults
	}
}

// rowTransition is how a result row appears: a short fade, no movement, so
// typing feels alive without the list jumping about.
var rowTransition = ui.ElementTransition{
	Duration: 120 * time.Millisecond,
	Colors:   false,
	Enter:    &ui.Motion{Opacity: 0},
}

// resultNumber is the badge a row's result shortcut shows: the select_result
// binding's own modifiers with the digit appended — `⌘1` on macOS, `Ctrl+1`
// elsewhere, as the old build's `formatResultShortcut` printed it. A binding
// the settings could not parse shows the bare digit, as theirs did.
func (a *App) resultNumber(number int) string {
	return shortcuts.Badge(settings.Shortcut(a.settings(), settings.ShortcutSelectResult), strconv.Itoa(number))
}

// rowActionText is a row's trailing key slot — the numbered badge or a
// command row's `Tab` affordance — in the reference's own register
// (`.launcher-result__action`): the body step at weight 620, `--text-tertiary`
// (lifted to `--text-primary` on the selected row), with a fixed 30u minimum
// and a right-aligned box so the badges of a column line up. The port used the
// body-minus-one step in `--text-muted`, one register too quiet and one size
// too small, and the slot was content-sized rather than the reference's fixed
// 30u. The ⌘1-⌘0 family and the `Tab` hint are drawn by this one helper so
// the two cannot drift apart.
func (a *App) rowActionText(c *ui.Context, text string, selected bool) {
	tokens := theme.ForPreferences(a.settings(), c.Theme().Dark, c.Preferences())
	t := c.Theme()
	color := tokens.TextTertiary
	if selected {
		color = t.Text
	}
	ui.Text(c, text).FontSize(tokens.Body).FontWeight(620).TextColor(color).
		MinWidth(t.Space(7.5)).TextAlign(ui.End).Shrink(0)
}

// barHintText is the action bar's own `⌘↩` hint, in the reference's
// `.launcher-action-bar__hint` register — one step quieter than a result row's
// trailing badge: the caption size at 500, `--text-tertiary`, in the
// terminal's monospace stack with the badge's own 2u/6u padding.
// (`.launcher-action-bar__hint`, `launcher.css:1608-1623`.)
func (a *App) barHintText(c *ui.Context, text string) {
	tokens := theme.ForPreferences(a.settings(), c.Theme().Dark, c.Preferences())
	t := c.Theme()
	ui.Text(c, text).Font(a.settings().FontFamily).FontSize(tokens.Caption).
		FontWeight(500).TextColor(tokens.TextTertiary).
		Padding(0, t.Space(1.5)).Shrink(0)
}

// Feedback shows a line under the list, for a caller outside the launcher
// (the shell's power action, when the system refused it).
func (a *App) Feedback(message string) {
	if message == "" {
		return
	}
	a.toast = message
	a.toastWarning = false
}

// WarnFeedback is Feedback's blocking tone: a message that says something did
// not work, drawn with the warning edge so it reads as a problem rather than a
// note.
func (a *App) WarnFeedback(message string) {
	if message == "" {
		return
	}
	a.toast = message
	a.toastWarning = true
}

// copy is the launcher's copy in the stored language.
func (a *App) copy() i18n.Launcher { return StringsFor(a.settings().Language) }

// InClipboardMode reports whether the clipboard history mode is open, for a
// test or a caller that has to know where a custom shortcut landed.
func (a *App) InClipboardMode() bool { return a.clipboard }

// InBrowserMode reports whether the browser search mode is open.
func (a *App) InBrowserMode() bool { return a.browser }

// EnterClipboard opens the clipboard history mode, for a custom shortcut that
// summons it directly.
func (a *App) EnterClipboard() { a.enterClipboard() }

// EnterBrowser opens the browser search mode.
func (a *App) EnterBrowser() { a.enterBrowser() }

// ResetQuery clears the field and the selection, for an action that wants the
// search back rather than the last query.
func (a *App) ResetQuery() {
	a.leaveCommand()
	a.leaveClipboard()
	a.leaveBrowser()
	a.leaveCalculator()
	a.leaveFiles()
	// The search page has no plugin overlay: leaving the mode takes the
	// configuration sheet with it, the same rule the View's syncConfig
	// enforces on every other frame.
	a.syncConfig()
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// SetApps replaces the scanned applications.
func (a *App) SetApps(found []apps.App) { a.Apps = found }

// SetCommands replaces the extensions' commands.
func (a *App) SetCommands(found []extensions.CommandEntry) {
	a.Commands = found
	// Two integrations may declare the same command word; the launcher enters
	// the first, so the rows say the word is shared rather than letting the
	// shadowed one look like the only answer.
	counts := map[string]int{}
	for _, entry := range found {
		counts[strings.ToLower(strings.TrimSpace(entry.Command.ID))]++
	}
	a.commandConflicts = map[string]bool{}
	for word, count := range counts {
		if count > 1 {
			a.commandConflicts[word] = true
		}
	}
}

// SetRecent replaces the recent applications, and whether the empty query
// shows them.
func (a *App) SetRecent(paths []string, show bool) {
	a.Recent, a.ShowRecent = paths, show
}

// SetTools replaces the commands found on the PATH and whether the search
// offers them. The user's aliases are a separate, catalog-wide fact — see
// SetCommandAliases — so turning the PATH rows off does not drop them.
func (a *App) SetTools(tools []apps.App, show bool) {
	a.Tools, a.ShowTools = tools, show
}

// SetCommandAliases replaces the user's alias per command.
func (a *App) SetCommandAliases(aliases settings.CommandAliases) {
	a.Aliases = aliases
}

// SetQuery puts text in the field, for a deep link that names what to search
// for.
func (a *App) SetQuery(query string) {
	a.Query = query
	// The same default a typed query takes: the action bar for a command line,
	// URL or path, the first runnable result otherwise.
	a.Selected, a.chosenRow = a.defaultSelection(a.Results()), -1
	a.defaultedQuery = query
	a.pendingCaret = true
}

// New builds the launcher state over a settings store and the shell's
// actions.
func New(store *settings.Store, actions Actions) *App {
	// The pointer's baseline starts at "no row": the zero value would make the
	// first frame read row 0 as already entered (see a.hoveredRow).
	return &App{Store: store, Actions: actions, hoveredRow: -1, hoverFrameRow: -1}
}

// settings is the current normalized settings.
func (a *App) settings() settings.Settings { return a.Store.Snapshot() }

// FocusSearch asks for the keyboard focus on the query field. The launcher
// calls it every frame: the field owns the keyboard while the surface shows,
// exactly as the old collapsed shell reasserted focus.
func (a *App) FocusSearch() { a.Search.Focus() }

// View builds the launcher: a search field pinned over the result list,
// which scrolls under it behind a soft scroll edge, as the old shell's
// results did. The shell draws the window chrome and the glass panel; this
// is the content inside it.
func (a *App) View(c *ui.Context) {
	copy := StringsFor(a.settings().Language)
	t := c.Theme()

	// The field owns the keyboard while the launcher shows, exactly as the
	// old collapsed shell reasserted focus on every reveal.
	a.FocusSearch()

	// A half-finished row delete is disarmed once its window has passed; while
	// one is live the frame keeps coming so the deadline is reached.
	a.expireArmedDelete(c)
	// An unanswered power confirmation does the same, on its own window.
	a.expirePowerConfirm(c)

	a.syncTypedMode()
	a.syncCommandMode()
	// Leaving a plugin's mode leaves its configuration sheet: the sheet is
	// that mode's own face, not a page of the app's.
	a.syncConfig()
	a.askBrowser()
	a.modeShortcuts()
	// The held-modifier row's premise: the app modifier down on an empty field
	// puts a "new terminal session" line at the list's end and hands it the
	// selection, so Enter reaches it. Releasing the modifier gives the
	// selection back.
	// The frame's one catalog walk: syncHeldModifier builds it after the
	// modifier's state lands, so it is the list this frame draws and there is
	// no second search to disagree with it (see App.searchRuns).
	results := a.syncHeldModifier(c)
	// The chord that put the row on screen is the chord that opens it: the
	// modifier is still down when the user presses Enter, so the plain
	// Enter's own path (the field's submit) is not the one that fires.
	if item, ok := a.heldTerminalItem(); ok && c.Shortcut(ui.Cmd, ui.KeyEnter) {
		item.Run()
	}
	// A query the field has not accounted for gets that query's own default
	// before any row reads the choice (see defaultedQuery).
	if a.Query != a.defaultedQuery {
		a.Selected, a.chosenRow = a.defaultSelection(results), -1
		a.defaultedQuery = a.Query
	}
	// The held row's own claim on the selection is asserted last: while the
	// modifier is down the row is on screen, and a keystroke that rebuilds the
	// list must not leave it drawn without the highlight (see
	// holdHeldRowSelection).
	a.holdHeldRowSelection(results)
	a.holdBarSelection(results)
	a.clampSelection(results)
	a.assignNumbers(results)
	// The output list, while it owns the band, carries the same numbered family
	// over its own rows.
	a.assignOutputNumbers()

	// The window's height is the content's own: measure every row's lines
	// off the theme, so the shell can size the window to the list it holds.
	tk := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	a.Font = float64(t.FontSize)
	a.Spacing = float64(t.Spacing)
	// The rows' two line boxes, measured rather than assumed: the framework
	// lays a line out at the font's own ascent, descent and gap, which is not
	// a fixed multiple of the size, and the window is charged what the text
	// takes.
	if _, height := c.MeasureText(0, ui.Span{Text: "Ag", Size: t.FontSize + rowTitleStep}); height > 0 {
		a.TitleLine = float64(height)
	}
	if _, height := c.MeasureText(0, ui.Span{Text: "Ag", Size: t.FontSize}); height > 0 {
		a.SubtitleLine = float64(height)
	}
	rows, held, sections := a.windowRows(results)
	a.Rows = rows
	// The window follows the list exactly: the row count it holds is the
	// count the content needs (the reference's R66 rule). A one-row margin
	// would leave the window a row too tall whenever a row leaves - which is
	// the held modifier's row on release. The cap is the work area less the
	// room the other windows keep.
	a.HeldRows = HoldRows(a.HeldRows, held)
	if a.Actions.ResizeTo != nil {
		geometry := Geometry{
			Font: a.Font, Spacing: a.Spacing, Rows: rows, Held: len(rows),
			TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
			Filter:   a.filtersVisible(),
			Feedback: a.toast != "" || a.pendingPower != "",
			Config:   a.configSchema(),
			Sections: sections,
		}
		a.Actions.ResizeTo(geometry.Height())
	}

	// The field row floats over the list; the list's top padding is the
	// row plus the edge, so a row scrolls under the field rather than to
	// its edge.
	fieldRow := t.Space(fieldRowUnits)
	edge := fieldRow + t.Space(breathUnits)
	// A plugin list mode carries its filter chips under the field: the list
	// starts below them, and the window's height charges their band too.
	chips := a.filtersHeight(c)
	listEdge := edge + chips
	// The action bar's pinned footer: where the composed list stops being the
	// scroller's and the band the footer reserves at the panel's bottom edge
	// (see actionBarSplit). Resolved before the panel is built because the
	// docked rows below need it too.
	var barStart int
	barStart, a.footerBand = a.actionBarSplit(c, results)

	ui.Box(c).Fill().Background(tk.Sunken).Children(func() {
		// R21 · the pointer's selection is an *entry edge*, not a per-frame
		// re-assert (the reference's `onPointerEnter`,
		// `LauncherResults.tsx:859-862`). The row this pass builds reports
		// itself into `hoverFrameRow`; the pass before it is `hoveredRow`, the
		// baseline each row's entry is read against. The roll happens here,
		// before any early return below can skip it, and the counter is then
		// reset for the pass about to build: a pointer over no row (or a page
		// with no rows — the empty state, the configuration sheet, the output
		// view) therefore clears the state rather than leaving a stale index
		// a later list rebuild would read as "already entered".
		a.hoveredRow = a.hoverFrameRow
		a.hoverFrameRow = -1
		// The panel is the launcher's reading surface: the old build's
		// `--surface-sunken`, one step denser than the card's own glass, so
		// the rows sit on a recess rather than on the frame (`.launcher-bottom`).
		// The field block above is opaque and paints over its top.
		if a.output != nil {
			a.outputBody(c, copy, edge)
			return
		}
		// The configuration sheet takes the result list's place: same band,
		// same card, the mode's own face (the old build's R29 overlay). It
		// starts below the chips row, which stays — the gear that opened the
		// sheet is there, and it is the sheet's only close control — and so
		// does the field, which the sheet's own budget is measured above.
		if a.configOpen() {
			a.configBody(c, listEdge)
		} else if len(results) == 0 {
			ui.Column(c).FillWidth().Padding(t.Space(3)).Center().Children(func() {
				ui.Text(c, a.emptyMessage(copy)).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
		} else {
			// The list builds only the rows in view, so a catalog of
			// hundreds costs a frame like a catalog of three. The choice is
			// painted by the row itself (see row), not by the list's own
			// selection. Padding puts the first row below the field, and
			// scrolls with the content, so rows pass under the field.
			// The list has no scrollbar, as the old build's own list did: the
			// user's call, after two rounds of the bar refusing to sit flush
			// (「启动器列表不要滚动条」). Scrolling still works - the wheel,
			// the trackpad and the arrows - and the choice scrolls itself
			// into view; it is only the bar that is gone. The scrollbar's
			// insets push it past the list's own clip, which is what hides it.
			//
			// The action bar is **not** one of these rows: it is the panel's
			// pinned footer, drawn below this scroller (see actionBarFooter).
			// As a row it was the list's last one, so a query that filled the
			// window pushed it past the bottom edge and only a scroll (or the
			// held modifier's own scroll-into-view) brought it back — the
			// reference's `.launcher-action-bar` is a *sibling* of
			// `.launcher-results` (`main:src/launcher/LauncherResults.tsx:1043-1073`)
			// and never scrolls. The scroller's bottom padding is the band the
			// footer occupies, so its last row stops above the footer instead
			// of running under it.
			ui.List(c.Key("launcher.results"), &a.List, barStart, func(i int) {
				a.row(c, results[i], i)
			}).Grow(1).FillWidth().MinHeight(0).
				Padding(listEdge, t.Space(1), t.Space(0.5), t.Space(1)).Label(copy.ResultsLabel).
				ScrollbarInsets(0, 4096, 0, 0)
			a.actionBarFooter(c, results, barStart)
		}

		// The chips row floats under the field, the list scrolling beneath it.
		a.filtersRow(c, fieldRow+t.Space(1))

		// The rows fade into the panel under the field is gone with R31: the
		// reference deleted the launcher's scroll-edge band and kept only the
		// field block's own 1px hairline (below). The band darkened rows as
		// they passed under the field, which the user read as a shadow cast
		// by the input.

		// The field block's own floor: one hairline, the reference's R31
		// 「输入框下方阴影可以只保留 1px」. The field row is a near-solid,
		// brighter block (`.collapsed-card__input-row`'s
		// `--glass-field` over `--surface-opaque`); the line is what says
		// where it ends.
		ui.Box(c).Absolute().Top(fieldRow - 1).Left(0).Right(0).Height(1).PassThrough().
			Background(tk.Hairline)

		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(fieldRow).
			Background(tk.FieldFill).Padding(0, t.Space(4)).
			AlignItems(ui.Center).Children(func() {
			// The reference's `.collapsed-card__input-row` is a plain flex row with
			// **no** `gap`: every child's spacing is its own margin — the scope mark
			// ends with `margin-right: 10u` (`launcher.css:324`), the trigger nudge and
			// the trailing buttons each open with `margin-left: 8u`
			// (`:543`, `:388`). The port put a uniform 4u `Gap` on the row instead,
			// which added a step the design does not spend in front of the nudge and
			// the buttons (12u where the reference draws 8u) and shortened the field
			// by 4u per trailing item. The row keeps the reference's own seams now.
			// The launcher's implicit drag handle (R10-B): a 28px band
			// across the card's top, painted under the field and the buttons
			// so the field keeps every one of its clicks. The card itself is
			// no longer a drag surface — a press on a result row or a hint
			// must never move the window — so the one place that does is
			// this invisible band. The field and the buttons are drawn after
			// it (later children paint over it), so their clicks land on
			// them, and only the band's own blank area starts a drag.
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(t.Space(7)).
				DragWindow().Background(ui.Transparent)
			// The scope mark: the mode that owns the field, at its left — the
			// icon says a plugin owns it and the muted word says which one
			// (`.collapsed-card__scope` / `__scope-name`). The trigger word
			// was consumed on entry, so this is the only place the mode is
			// named.
			if glyph, name, ok := a.scopeMark(copy); ok {
				mark := ui.Row(c).Shrink(0).Gap(t.Space(1.5)).AlignItems(ui.Center).
					Margin(0, t.Space(2.5), 0, 0)
				mark.Children(func() {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
					ui.Text(c, name).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
			}
			field := ui.TextInput(c.Key("launcher.search"), &a.Query).
				Bind(&a.Search).
				Label(copy.Label).
				Placeholder(copy.Placeholder).
				// The query is the card's display register: the old build's
				// `.collapsed-card__input` took `--text-display` (17px at
				// scale 1) against the rows' body text — one step above —
				// at weight 460.
				FontSize(tk.Display).
				FontWeight(460).
				Grow(1).
				// The field answers two keys itself, before the editor takes
				// them as edits: the history row's delete and the chord that
				// empties the typed-command history (see fieldkeys.go).
				HandleInput(a.fieldKeys)
			// The field is borderless text inside the card, not a boxed
			// control: the reference's `.collapsed-card__input` carries no
			// border, no fill and no padding, and the card's own edge is the
			// only visible shape. The framework's input draws its own pane
			// and focus ring, so both are taken off here.
			field.Background(ui.Transparent).Border(0, ui.Transparent).Padding(0)
			if a.pendingCaret {
				field.SetTextSelection(len(a.Query), len(a.Query))
				a.pendingCaret = false
			}
			if field.Changed() {
				// A fresh query starts on its own default: the action bar for a
				// dead-key query, the first runnable result otherwise (the
				// reference's `defaultsToActionBar` / `firstRunnableResultIndex`).
				// The bar is not a row of this list — it is the panel's pinned
				// footer (see Results) — so its default is its own index, past
				// the last match. The results are recomputed here: `results` was
				// built before the field took the keystroke and no longer matches
				// the query.
				next := a.Results()
				a.Selected = a.defaultSelection(next)
				a.chosenRow = -1
				// The keystroke is accounted for, so the next frame's own query
				// check does not undo a step the keyboard takes from here.
				a.defaultedQuery = a.Query
			}
			if field.Submitted() {
				a.activate(results)
			}
			switch {
			case a.mode != nil || a.clipboard || a.browser:
				ui.Text(c, copy.CommandModeHint).FontSize(t.FontSize).TextColor(t.TextMuted)
			default:
				// The inline trigger nudge (R48): a word on its way to a
				// command's mode says so here, in the field row, where it
				// costs no band and never moves the window. The clear button
				// follows it, so the nudge and the button share the row's
				// tail.
				//
				// It is one line whatever the row can hold, the old build's
				// `white-space: nowrap` + `text-overflow: ellipsis`: a
				// wrapped nudge would take back the band R48 removed. The
				// only loss is the hide `triggerHintShown` decides.
				if hint := a.triggerHintText(); hint != "" && a.triggerHintShown(c, t, hint) {
					// The nudge is two spans in the old build (R48): the
					// space key in `--text-tertiary` at weight 620, then the
					// command's name in `--text-muted` at 520 — the key
					// reads as the quiet half of the pair, not a second
					// label competing with the field. The pair rides the
					// row as `flex: 0 1 auto` with `min-width: 0` and a
					// `max-width: 50%` cap (`.collapsed-card__trigger-hint`):
					// the field yields the room, the nudge shrinks to it —
					// the key never does (`flex: 0 0 auto`), and the name
					// is the one part that may be cut, so a collision trims
					// the nudge's tail rather than the key.
					tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
					ui.RichText(c,
						ui.Span{Text: "␣", Weight: 620, Color: tokens.TextTertiary},
						ui.Span{Text: hint, Weight: 520, Color: t.TextMuted},
					).FontSize(t.FontSize-1).SingleLine().Ellipsis("…").
						MinWidth(0).MaxWidthPercent(50).Margin(0, 0, 0, t.Space(2))
				}
				if a.Query != "" {
					// The clear control is the same 28u transparent square the
					// field's other buttons are (`.collapsed-card__settings`),
					// drawn through the one control the row shares: `--text-tertiary`
					// at rest, `--text-strong` under the pointer, and the 0.92 press
					// the reference's `:active` rule takes (`launcher.css:397-411`).
					// The port had given this one button its own face — muted ink,
					// plain hover fill, no press — so the four trailing controls did
					// not answer the pointer alike.
					a.fieldSettingsButton(c, tk, copy.Clear, "", func(mark ui.Color) {
						if glyph, ok := Glyph("close"); ok {
							ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(mark)
						}
					}, func() {
						a.Query = ""
						a.Selected = 0
						a.FocusSearch()
					})
				} else {
					// The empty field's own trailing hint (a port addition: the
					// reference leaves the wording to the input's `placeholder`). It opens
					// on the same 8u the trailing controls use, so the field's tail is one
					// rhythm with no row-wide gap to lean on.
					ui.Text(c, copy.Hint).FontSize(t.FontSize).TextColor(t.TextMuted).
						Margin(0, 0, 0, t.Space(2))
				}
			}
			// The field row's trailing control. In a built-in plugin mode it is
			// the plugin's own gear (R29): the row's tail belongs to the plugin,
			// so the sessions/settings pair — not plugin actions — steps aside.
			// Outside a plugin mode the two ways in stay exactly as before.
			if id := a.filterPluginID(); id != "" {
				a.pluginGear(c, copy, id)
			} else {
				a.fieldButtons(c, copy)
			}
			// R84 · the pin. Puts the plugin page's command + current argv
			// into its own window that never follows the launcher's
			// summon/hide — the user's 「独立固定、不自动消失」. Sits inside
			// the field row, after the ternary, so both branches keep their
			// original trailing sets and the pin is additive only.
			if a.mode != nil && a.mode.Command.Execution.Mode == "external" && a.Actions.DetachOutput != nil {
				a.pinButton(c, copy)
			}
		})
	})

	// Keep the chosen row in view when the keyboard moved the choice (the
	// list leaves the scrolling to the app when it has no selection).
	//
	// The held modifier's own row is the one exception, the reference's R67
	// rule (LauncherResults.tsx's isBareTerminalRow branch): the window is
	// grown for the row, so it is visible as soon as the resize lands, and
	// scrolling it in first — while the list is still the old, shorter box —
	// makes the landing clamp the scroll back: the list slid up and back down
	// on the press edge (「先上移然后再下移恢复」). The row is never scrolled
	// into view; it is simply given the room.
	if a.Selected != a.chosenRow {
		a.chosenRow = a.Selected
		// The held row is the list's last line and it was recorded from this
		// frame's own list (see heldRowShown), so neither question below
		// needs a second catalog walk.
		if !a.heldRowShown || a.Selected != len(results)-1 {
			// The pinned footer is never scrolled into view: it is not a row of
			// the scroller at all, it is the band below it, so it is on screen
			// by construction. Asking the list for its index would land the
			// scroll past the last row it has.
			row := a.Selected
			if bar := actionBarIndex(results); bar >= 0 && row >= bar {
				row = bar - 1
			}
			if row >= 0 {
				a.List.ScrollIntoView(row)
			}
		}
	}

	// An action that asks for feedback (the calculator's copy) shows it
	// once, on the frame after it ran.
	// The feedback is the panel's own docked row, never a floating toast: a
	// message under the list belongs to the list, and the old build moved it
	// there for exactly that reason.
	a.feedbackRow(c)
	// The power confirmation is docked in the same band: the panel asks, the
	// list keeps its place, and the answer is one of the row's two buttons.
	a.confirmPowerRow(c)

	// The output view owns the keys while it is open: its list walks, its
	// text copies, and Escape closes it.
	if a.output != nil {
		switch {
		case c.Shortcut(0, ui.KeyEscape):
			a.leaveOutput()
		case a.outputShortcuts(c):
			// The numbered family runs the output list's rows, exactly as ⌘N
			// runs a result on the ordinary page.
		case c.Shortcut(0, ui.KeyDown):
			a.moveOutputSelection(1)
		case c.Shortcut(0, ui.KeyUp):
			a.moveOutputSelection(-1)
		case c.Shortcut(0, ui.KeyEnter):
			if a.output.IsList {
				a.runOutputRow()
			} else if a.Actions.Copy != nil {
				a.Actions.Copy(a.output.Text)
				a.toast = copy.Copied
			}
		}
		return
	}

	// The overlay's own keys come first: Escape closes the sheet rather than
	// leaving the mode, as the old build's dismiss table did.
	if a.configKeys(c) {
		return
	}
	// A pending power confirmation answers Escape the same way its own cancel
	// button does: nothing runs, and the panel goes back to what it was. The
	// predicate comes first: asking for a key consumes it, and a panel with
	// nothing pending must leave Escape to the handlers below.
	if a.pendingPower != "" && c.Shortcut(0, ui.KeyEscape) {
		a.pendingPower = ""
		return
	}

	// The app's own keys (new command, external terminal), then the result
	// shortcuts: the app modifier plus a number runs the n-th row that can be
	// run, as the old build's ⌘1–⌘0 did.
	if a.appShortcuts() {
		return
	}
	if a.resultShortcuts(results) {
		return
	}

	// The field keeps the focus, so the list's arrows are read here: a
	// single-line text input leaves plain Up and Down to shortcuts. With no
	// results and no action bar, they fall through to the query history.
	hasList := len(results) > 0 || a.output != nil
	if c.Shortcut(0, ui.KeyDown) {
		if !hasList && a.historyDown() {
			return
		}
		a.move(1, len(results))
	}
	if c.Shortcut(0, ui.KeyUp) {
		if !hasList && a.historyUp() {
			return
		}
		a.move(-1, len(results))
	}
	if c.Shortcut(0, ui.KeyEscape) {
		// An armed delete is the first thing Escape answers.
		if a.armedDelete != "" {
			a.disarmDelete()
			return
		}
		// The mode ladder is `leavePluginMode`'s, so the two ways out — Escape
		// here and an empty field's Backspace on the field itself — cannot name
		// the modes in two orders.
		switch {
		case a.leavePluginMode():
		case a.Query != "":
			a.Query = ""
			a.Selected = 0
		case a.Actions.Dismiss != nil:
			a.Actions.Dismiss()
		}
	}
	// Tab completes the chosen argument while a command is being typed,
	// pins the chosen clipboard entry, and expands a command row into the
	// argument mode in the search.
	// Tab walks the active mode's filter axis (the browser's ranges, the
	// clipboard's kinds, the calculator's two), wrapping at both ends; on the
	// ordinary page it completes the chosen argument, pins the chosen
	// clipboard entry, or expands a command row into the argument mode.
	// (Each Shortcut call consumes the press, so the chord is asked once and
	// the answer carried.)
	shiftTab := c.Shortcut(ui.Shift, ui.KeyTab)
	tab := false
	if !shiftTab {
		tab = c.Shortcut(0, ui.KeyTab)
	}
	if tab || shiftTab {
		if _, _, ok := a.filterAxisFor(); ok {
			direction := 1
			if shiftTab {
				direction = -1
			}
			a.cycleFilter(direction)
			return
		}
	}
	if tab && a.Selected >= 0 && a.Selected < len(results) {
		item := results[a.Selected]
		switch {
		case a.mode != nil && item.complete != "":
			a.appendWord(item.complete)
		case a.calculatorMode:
			a.cycleCalculatorFilter()
		case a.browser && item.tab != nil:
			a.copyURL(item.tab.URL)
		case a.browser && item.web != nil:
			a.copyResult(*item.web)
		case a.mode == nil && !a.clipboard && item.entry != nil:
			a.enterCommand(*item.entry)
		}
	}
}

// The mode-local keys: the history's star, which belongs to the calculator
// and clipboard modes rather than to the launcher's own shortcut map. It is
// the old build's key — `CmdOrCtrl+D`, the bookmark convention the star
// echoes (`main:src/launcher.ts:461,521`) — and is not rebindable, because a
// control the user never asked to rebind does not belong in the settings map.
//
// The press itself is claimed by the field (`historyStarMods`, fieldkeys.go):
// a focused text field's editor takes a modified D before any `Shortcut`
// registration is asked — macOS reads Ctrl+D as delete-char — so the field is
// the seam and this is where the claim is spent.
//
// The modes' other key, the history row's delete, cannot live here for the
// same reason (see fieldkeys.go).
func (a *App) modeShortcuts() {
	favorite := a.pendingFavorite
	a.pendingFavorite = false
	if !favorite {
		return
	}
	switch {
	case a.calculatorMode:
		a.toggleCalculatorFavorite()
	case a.clipboard:
		a.toggleClipFavorite()
	}
}

// rowControl is one of a launcher row's own trailing controls: a 20u hit
// region and its mark, which the selection brings with it — the reference's
// `.launcher-result__pin`, never a filled pill in a list of quiet rows.
//
// The reference's pin is literally `.launcher-result__pin collapsed-card__settings`
// (`LauncherResults.tsx:1001-1005`): it wears the field row's own shared control
// faces at the row's 20u size — resting in `--text-tertiary`
// (`main:src/styles/launcher.css:397`), the pointer lifting it to
// `--glass-control-hover` + `--text-strong` + the resting lens (`:400-404`), and
// the press sinking it 0.92 over `--glass-control-press` (`:407-411`). The port
// had given it a face of its own — muted ink, a flat `--glass-control-hover`
// fill and no press — so the row's pin and the field's four trailing controls
// did not answer the pointer alike.
func (a *App) rowControl(c *ui.Context, name, label string, run func()) {
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	control := ui.Row(c).Focusable().Shrink(0).Size(t.Space(5), t.Space(5)).
		Radius(tokens.RadiusXS).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Tooltip(label).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	ink := tokens.TextTertiary
	switch {
	case control.Pressed():
		control.Background(tokens.ControlPress).Scale(0.92, 0.92)
		ink = tokens.TextStrong
	case control.Hovered():
		tokens.Lens(control.Background(tokens.ControlHover))
		ink = tokens.TextStrong
	}
	control.Children(func() {
		if glyph, ok := Glyph(name); ok {
			ui.Icon(c, glyph).Size(t.Space(3.5), t.Space(3.5)).TextColor(ink)
		}
	})
	if control.Clicked() {
		run()
	}
}

// rowFavorite reports a history row's favourite state, and whether the row has
// one to show.
func rowFavorite(item Item) (bool, bool) {
	switch {
	case item.clip != nil:
		return item.clip.Favorite, true
	case item.calc != nil:
		return item.calc.Favorite, true
	}
	return false, false
}

// toggleFavorite stars or unstars a history row's entry, whichever store it
// came from: the row's own star and the mode's own key act through here.
func (a *App) toggleFavorite(item Item) {
	switch {
	case item.clip != nil:
		a.toggleClipFavorite()
	case item.calc != nil:
		a.toggleCalculatorFavorite()
	}
}

// toggleClipFavorite stars or unstars the chosen clipboard entry.
func (a *App) toggleClipFavorite() {
	if a.Clipboard == nil {
		return
	}
	entry, ok := a.selectedClip()
	if !ok {
		return
	}
	if err := a.Clipboard.SetFavorite(entry.ID, !entry.Favorite); err != nil {
		a.WarnFeedback(StringsFor(a.settings().Language).ClipboardFavoriteFailed)
	}
}

// selectedClip is the clipboard entry the selection is on.
func (a *App) selectedClip() (clipboard.Entry, bool) {
	results := a.Results()
	if a.Selected < 0 || a.Selected >= len(results) || results[a.Selected].clip == nil {
		return clipboard.Entry{}, false
	}
	return *results[a.Selected].clip, true
}

// enterCommand opens an extension command's mode from its own row: the scope
// marker names the command beside the field, and the list offers the command's
// declared arguments. The field starts empty — the trigger word was consumed,
// as `enterPluginMode` did. Tab's entry has no argv to carry; the trigger-word
// path carries one (see enterCommandWith).
func (a *App) enterCommand(entry extensions.CommandEntry) {
	a.enterCommandWith(entry, "")
}

// enterCommandWith opens a command's mode with the argv the trigger line
// already carried: `jv -f ` opens the mode with `-f` in the field, exactly as
// the old build's trigger entry read it (the word is consumed, the rest is the
// field). Tab's entry has no argv to carry and calls this with the empty
// string.
func (a *App) enterCommandWith(entry extensions.CommandEntry, argv string) {
	copied := entry
	a.mode = &copied
	a.modeWord = entry.Command.ID
	a.Query = argv
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveCommand returns to the search.
func (a *App) leaveCommand() {
	a.mode = nil
	a.modeWord = ""
	a.Selected, a.chosenRow = 0, -1
}

// syncTypedMode enters a mode when the user types its trigger word followed by
// a space, as the old build did: the word names the plugin and everything after
// it is the needle, so `clip foo` is the clipboard mode without a trip through
// the command list. An extension command's own trigger word works the same way
// (`jv -f ` opens the command's mode with `-f` as its argv), which is the
// transition the field's own nudge promises and the `␣` keycap points at. A
// bare word stays an ordinary query, which is what makes entering and leaving
// one keystroke.
func (a *App) syncTypedMode() {
	if a.pluginModeActive() {
		return
	}
	word, hasRest := modeWord(a.Query)
	if !hasRest {
		return
	}
	switch {
	case wordIn(word, clipboardWords):
		a.enterClipboardWord(a.Query)
	case wordIn(word, browserWords):
		a.enterBrowserWord(a.Query)
	case wordIn(word, calculatorWords):
		a.enterCalculatorWord(a.Query)
	case wordIn(word, filesWords):
		a.enterFiles()
	default:
		// An extension command's own trigger word: `jv ` enters the command's
		// mode, the transition the field's nudge promised ("one space then
		// enters the mode") and the one the catalog row's `␣` keycap points
		// at. The built-ins win the namespace, exactly as their parsers do,
		// so a command named after one is still reachable by its id.
		if entry, ok := a.commandForTrigger(word); ok {
			a.enterCommandWith(entry, strings.TrimSpace(a.Query[len(word):]))
		}
	}
}

// commandForTrigger resolves a whole trigger word to the command that owns it:
// the same vocabulary the field's nudge reads (id, name, alias — each first
// word, see commandTriggers), so the nudge and the transition cannot disagree.
// The first command in the catalog wins, which is the order the rows are
// listed in.
func (a *App) commandForTrigger(word string) (extensions.CommandEntry, bool) {
	for _, entry := range a.Commands {
		for _, trigger := range commandTriggers(entry) {
			if strings.EqualFold(trigger, word) {
				return entry, true
			}
		}
	}
	return extensions.CommandEntry{}, false
}

// modeWord is the query's first word and whether whitespace follows it, the
// rule every trigger word obeys. Whitespace alone is enough: `clip ` is the
// mode with an empty needle, and a bare word stays an ordinary query.
func modeWord(query string) (string, bool) {
	index := indexWordGap(query)
	if index <= 0 {
		return "", false
	}
	return strings.ToLower(query[:index]), true
}

// syncCommandMode keeps the mode on screen. The trigger word was consumed on
// entry — the field holds the needle, and the scope marker names the mode — so
// there is nothing in the line to re-check: a mode leaves when the user leaves
// it (Escape, or the row that hands the query back), not because of what the
// field happens to hold.
func (a *App) syncCommandMode() {
}

// wordIn reports whether a query's first word is one of a mode's trigger
// words, so every spelling the old build accepted enters (and stays in) the
// mode.
func wordIn(word string, words []string) bool {
	for _, candidate := range words {
		if strings.EqualFold(word, candidate) {
			return true
		}
	}
	return false
}

// The words the field starts with in the built-in modes; the other spellings
// enter the same mode (see syncCommandMode).
const (
	clipboardWord = "clipboard"
	browserWord   = "browser"
)

// clipboardWords and browserWords are the trigger words the old build
// accepted, so a habit from it still works.
var (
	clipboardWords = []string{"clip", "clipboard", "剪贴板", "粘贴板"}
	// browserWords are the words that open the browser mode; the word also
	// chooses the range it opens on (see browserFilterForWord). `history` is
	// included: it is the shell's own command as a bare word, but `history `
	// with an argument is the browser history search, exactly as the old build
	// had it.
	browserWords = []string{"browser", "bookmarks", "bookmark", "history", "hist", "浏览器", "书签", "历史", "历史记录"}
)

// browserFilterForWord is the range a trigger word opens the mode on: the
// bookmark words open on the bookmarks, the history words on the history, and
// the plugin's own word on everything — the old build's `parseBrowserMode`, so
// the word the user typed is the list they asked for.
func browserFilterForWord(word string) string {
	switch {
	case wordIn(word, []string{"bookmarks", "bookmark", "书签"}):
		return filterBookmarks
	case wordIn(word, []string{"history", "hist", "历史", "历史记录"}):
		return filterHistory
	default:
		return filterAll
	}
}

// The built-in commands' search vocabulary: every language's wording plus the
// pinyin initials of the Chinese ones, written out because these are the app's
// own commands rather than a table-sized set. The readings follow the old
// build's notes, including 重's second one (chóng/zhòng), which an IME trains
// either way.
const (
	terminalSearch   = "终端 终端机 tzdzdj term terminal"
	browserSearch    = "浏览器 浏览器书签 书签 历史记录 llq llqsq sq lsjl browser bookmarks bookmark history"
	clipboardSearch  = "剪贴板 剪贴板历史 粘贴历史 jtb jtbls ntls ch clipboard paste"
	calculatorSearch = "计算器 计算 jsq js c calculator calculate"
	restartSearch    = "重启 重新启动 cq cxqd zq zxqd restart reboot"
	shutdownSearch   = "关机 关闭电脑 gj gbdn shutdown power off"
)

// enterClipboard starts searching the clipboard history.
func (a *App) enterClipboard() {
	a.enterClipboardWord(clipboardWord + " ")
}

// enterClipboardWord opens the clipboard mode with the field's text, which is
// either the canonical word (from the command row) or what the user typed (a
// trigger word and their needle).
func (a *App) enterClipboardWord(query string) {
	a.mode = nil
	a.browser = false
	a.calculatorMode = false
	a.clipboard = true
	a.modeWord = firstWord(query)
	a.Query = ""
	a.clipboardFilter = filterAll
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveClipboard returns to the search, keeping what the user typed: the
// needle the user typed becomes an ordinary query, as the old build's own
// dismissal did (R31: the user's words are never thrown away by a dismissal).
func (a *App) leaveClipboard() {
	if !a.clipboard {
		return
	}
	a.clipboard = false
	a.modeWord = ""
	// A half-finished delete does not outlive the mode: re-entering it with
	// the same row under the choice must not let one press confirm an arm the
	// user had already walked away from.
	a.disarmDelete()
	a.Selected, a.chosenRow = 0, -1
}

// clipboardQuery is what the user typed: the field holds the needle alone, the
// trigger word having been consumed on entry.
func (a *App) clipboardQuery() string {
	return a.Query
}

// commandArgs is the typed argument words, without the command itself.
func (a *App) commandArgs() []string {
	return splitArgs(a.Query)
}

// commandTokens is what the completions are asked about: the argument words,
// with an empty last token while a new word is being typed (the field ends
// in a space), as a completion request needs.
func (a *App) commandTokens() []string {
	tokens := splitArgs(a.Query)
	if a.Query != "" && trimTrailingWordGaps(a.Query) != a.Query {
		tokens = append(tokens, "")
	}
	return tokens
}

// currentWord is the argument being typed: the last word when the line does
// not end in whitespace, and nothing after a space.
func (a *App) currentWord() string {
	if trimTrailingWordGaps(a.Query) != a.Query {
		return ""
	}
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return ""
	}
	return words[len(words)-1]
}

// appendWord adds a completed argument to the line: it finishes the word
// being typed, or starts a new one after a space, and leaves a space ready
// for the next.
func (a *App) appendWord(word string) {
	if word == "" {
		return
	}
	trimmed := trimTrailingWordGaps(a.Query)
	if trimmed != a.Query {
		a.Query = trimmed + " " + word + " "
	} else {
		a.Query = replaceLastWord(trimmed, word) + " "
	}
	a.Selected, a.chosenRow = 0, -1
	a.FocusSearch()
}

// replaceLastWord swaps the word being typed for a completion, keeping what
// came before it.
func replaceLastWord(line, word string) string {
	end := lastWordGapEnd(line)
	if end < 0 {
		return word
	}
	return line[:end] + word
}

// row builds one result: title, detail and an optional shortcut. The chosen
// row wears the accent as a tint, as the old launcher's rows did.
func (a *App) row(c *ui.Context, item Item, i int) {
	t := c.Theme()
	copy := StringsFor(a.settings().Language)
	// A block's title line: the list's own label, drawn in the row's own
	// column, never selectable and never numbered.
	if item.heading {
		// A block's title line: the body step at 1.4, weight 620, muted, with
		// its own 6u/4u padding pair — `.launcher-section-title`, which the
		// window charges as a band of its own rather than a row's floor.
		ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(3), t.Space(1), t.Space(3)).
			Children(func() {
				ui.Text(c, item.Title).FontSize(t.FontSize).FontWeight(620).
					TextColor(t.TextMuted)
			})
		return
	}
	// A status note -- the mode's own "the plugin is off" line -- is
	// information, not a door: the reference draws it as the list's mute
	// one-liner in the icon column, with no plate, no pointer state and no
	// key (`.launcher-status`, `LauncherResults.tsx:749-760`). It is charged
	// its own 30u band rather than a result row's floor (`LAUNCHER_STATUS_UNITS`,
	// see `windowRows`), which is why it is not drawn as an ordinary row.
	if item.status {
		// The icon column's own step, one quieter than the sentence: the old
		// build's `--text-tertiary` on `.launcher-status__icon`.
		tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
		note := ui.Row(c).FillWidth().MinHeight(t.Space(launcherStatusUnits)).
			AlignItems(ui.Center).Padding(0, t.Space(3)).Gap(t.Space(2.5))
		note.Children(func() {
			// The 28u icon column, as `.launcher-status__icon` was: no plate,
			// only the mark — and the mark is the reference's **literal** 14
			// (`<InfoIcon size={14}/>`, `main:src/launcher/LauncherResults.tsx:752-754`),
			// not a `--u` step: the glyph stays 14px at every interface size while the
			// column around it scales.
			ui.Box(c).Size(t.Space(7), t.Space(7)).Shrink(0).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if glyph, ok := Glyph("info"); ok {
					ui.Icon(c, glyph).Size(14, 14).
						TextColor(tokens.TextTertiary)
				}
			})
			ui.Text(c, item.Title).FontSize(t.FontSize).FontWeight(480).
				TextColor(t.TextMuted).SingleLine()
		})
		return
	}
	// The row is keyed by the item, so a row that stays keeps its identity
	// (and its place) while the rows around it come and go; a row that
	// appears fades in rather than flashing into place.
	ui.Column(c).FillWidth().Children(func() {
		a.rowBody(c, item, i, copy)
	})
}

// rowBody is the row itself: the plate, the text column, the controls.
func (a *App) rowBody(c *ui.Context, item Item, i int, copy i18n.Launcher) {
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	// The row's own box: one 40u height for every result, subtitle or not (see
	// rowUnits). A one-line row is not a shorter row — it is the same box with
	// one line in it — so the height the window is charged is the height the
	// row draws, and a row gaining a subtitle never resizes the window.
	floor := t.Space(rowUnits)
	subtitle := rowSubtitle(item)
	row := ui.Row(c.Key(item.ID)).FillWidth().Focusable().MinHeight(floor).
		AlignItems(ui.Center).
		Padding(0, t.Space(3)).Radius(tokens.RadiusMD).Gap(t.Space(2.5)).
		Transition(rowTransition)
	// A history row is one step quieter than a match: the old build's
	// `.launcher-result--history { opacity: 0.92 }`.
	if item.clip != nil || item.calc != nil {
		row.Opacity(0.92)
	}
	// The action bar is the one row that is not a result: its selection is
	// the neutral raised pane the old build's settings used (`.launcher-
	// action-bar--selected`), not the accent tint a match takes, and the
	// ⌘↩ badge is what says it runs on Enter.
	isBar := strings.HasPrefix(item.ID, "bar:")
	if isBar {
		// The bar is the panel's own pinned footer (`.launcher-action-bar`):
		// a floor of its own 42u — one `Space(n)` is four reference units, so
		// that is `Space(10.5)` — and the 3u breath above it as this row's own
		// margin (`margin-top: calc(var(--u) * 3)`). The window charges the two
		// together as the old build's 45u band (see Height's bar arm), and the
		// breath is a *constant*, so the bottom edge never jumps when the result
		// count crosses zero.
		floor = t.Space(10.5)
		row.Margin(t.Space(0.75), 0, 0, 0)
		// The bar is welded to the list above it: the old build turned the
		// footer's top corners square whenever a result list preceded it
		// (`.launcher-results + .launcher-action-bar { border-radius: 0 0
		// var(--radius-md) var(--radius-md) }`, `main:src/styles/launcher.css:
		// 1556-1562`), so the two read as one card rather than as a separate
		// pane. The bar is this list's last row, so "a list precedes it" is
		// "some row is drawn above it" — a bar that is the *only* row (a URL
		// that matched no application, say) keeps its own full rounding, which
		// is the reference's no-scroller case.
		if i > 0 {
			row.Radius(0, 0, tokens.RadiusMD, tokens.RadiusMD)
		}
	}
	// The press is the one state a row takes on top of the tint: the control
	// material's own press face (base.css's --glass-control-press), which is
	// *less* opaque than the resting pane - the row sinks into the glass
	// instead of popping out of it.
	rowPressed := row.Pressed()
	// `.launcher-result:not(.launcher-result--unavailable):active { transform:
	// scale(0.986) }` and `.launcher-action-bar:active` likewise
	// (`main:src/styles/launcher.css:1512-1519,1647-1651`): the press sinks the
	// row a hair into the glass, over the same 120ms transform transition the
	// transition carries (Colors only, left to the row's own `--dur-2` ease).
	if rowPressed {
		row.Scale(0.986, 0.986)
	}
	switch {
	case rowPressed:
		row.Background(t.SurfacePressed)
	case i == a.Selected && isBar:
		// The action bar's own selection: the neutral raised pane the old
		// build's settings used (`.launcher-action-bar--selected` —
		// `--glass-raised-quiet` plus `--glass-raised-shadow`,
		// `main:src/styles/launcher.css:1579-1583`). It is the control-hover
		// lift with the raised pane's own shadow — the `--elev-2` contact cast
		// (`0 1px 2px rgba(0,0,0,0.2)`) and the 1px lit top rim
		// (`--glass-raised-rim`) — so the bar reads as a lit pane among the
		// transparent result rows rather than as a flat grey block, and the
		// `⌘↩` badge is what says it runs on Enter. The port had only the
		// bare hover fill: the bar was the one *selected* row with no lift.
		row.Background(tokens.RaisedQuiet)
		row.Shadow(0, t.Space(0.25), t.Space(0.5), 0, tokens.ShadowContact)
		row.InsetShadow(0, 1, 0, 0, tokens.LensRim)
	case i == a.Selected:
		// The old build's selection: the accent's quietest tint rung, and the
		// emphasis is the row's own type — the title at 700 — rather than a
		// ring or a shadow. The tint places the row; the weight announces it.
		// The rung is `--accent-tint-quiet`, a *per-appearance* token (dark
		// 0.085, light 0.065), not one hard-coded alpha: quoting the dark
		// number painted the light appearance's selection too heavy.
		row.Background(tokens.AccentTintQuiet)
	}
	// R21 · the pointer takes the selection over on *entry*, exactly as
	// `LauncherResults.tsx:859-862` does: entering a row selects it, while
	// resting on a row does nothing, so the row under the cursor and the row
	// Enter runs are one row and a keyboard step is never undone by a pointer
	// that happens to sit still on the row the selection just left (the
	// conflict `launcher.css:1653-1663` names). The row is the list's own key
	// (`Key(item.ID)`), so its hover flag is asked here, as the row is built.
	if row.Hovered() {
		a.hoverFrameRow = i
		if i != a.hoveredRow && rowSelectable(item) && item.Run != nil {
			a.Selected = i
		}
	}
	// A history row's own delete control: the focused text field owns the
	// editing keys, so the delete the old build put on a key is a button here
	// — on the selected row only, so a stray click cannot remove a row the
	// user was not looking at.
	removed := false
	row.Children(func() {
		// The icon plate: every row leads with the same 28u cell — the resting
		// control fill, a small radius and a hairline shadow — so a list of
		// rows reads as one column whatever each row is. Its content is the
		// row's own icon, a clipboard thumbnail, or a glyph from the closed
		// set (see glyphs.go).
		//
		// Three families override the base rule, exactly as the reference's
		// own CSS did:
		//
		//   * the browser list drops the plate and keeps a bare muted glyph
		//     (`.launcher-result__icon--browser`, R30): eight identical grey
		//     plates down a column read as eight controls, not eight links;
		//   * the two power actions wear the warm system plane and its own
		//     text colour (`.launcher-result__icon--system`);
		//   * a recents (history) row wears `--surface-control` at muted ink
		//     (`.launcher-result--history .launcher-result__icon`).
		//
		// The glyph ink follows the plate it sits on: `--text-secondary` for a
		// plated row (the base `.launcher-result__icon` colour), `--text-muted`
		// for a browser row. The port drew every glyph in `--text-muted` on a
		// plate, one register too quiet for the reference.
		warm := strings.HasPrefix(item.ID, "power-")
		browserRow := item.web != nil || item.tab != nil
		// A browser row's cell is bare — transparent, unshadowed, no lens — so
		// a column of links reads as text rather than as a toolbar
		// (`.launcher-result__icon--browser`). iconPlate draws the plated cell
		// every other family wears; the browser family takes a plain 28u box.
		var plate ui.Element
		if browserRow {
			plate = ui.Box(c).Size(t.Space(7), t.Space(7)).Shrink(0).
				AlignItems(ui.Center).Justify(ui.Center)
		} else {
			plate = a.iconPlate(c, warm)
		}
		plate.Children(func() {
			switch {
			case item.icon != nil:
				if bitmap := a.appIcon(*item.icon); bitmap != nil {
					iconCell(c, t, bitmap)
				}
			case item.thumb != nil:
				iconCell(c, t, item.thumb)
			default:
				if glyph, ok := rowGlyph(item); ok {
					icon := ui.Icon(c, glyph).Size(t.Space(4), t.Space(4))
					switch {
					case warm:
						// The warm plane's own ink (`--system-icon-text`), not the
						// warning colour: the warning ink is a near-neutral red-grey
						// reserved for a *label*, while the power plate's glyph is the
						// amber ink the reference's `.launcher-result__icon--system`
						// carried (`launcher.css:1234`).
						icon.TextColor(tokens.SystemIconText)
					case browserRow:
						icon.TextColor(t.TextMuted)
					default:
						icon.TextColor(tokens.TextSecondary)
					}
				}
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			// The title line: the title itself, and the trailing value a row
			// prints complete at its right edge.
			//
			// Both text registers are single-line and ellipsized, as the old
			// build's `white-space: nowrap; text-overflow: ellipsis` was: a
			// title longer than its column is cut, never wrapped into extra
			// lines the window never charged for.
			ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				// The title is the ladder's title register (`--text-title`,
				// one step above the body) at 580, and the selected row's
				// emphasis is its own type: 700, as
				// `.launcher-result--selected .launcher-result__title` did.
				title := ui.Text(c, item.Title).FontSize(t.FontSize + 2).
					FontWeight(580).TextColor(tokens.TextStrong).SingleLine()
				// The action bar's title is the string the user is handing
				// over — the command line, the URL, the path — so it is the
				// terminal's own monospace at 520 (the old
				// `.launcher-action-bar__title`). The action's own word is the
				// row's subtitle, which keeps the interface face.
				if isBar {
					title.Font(a.settings().FontFamily).FontWeight(520)
				}
				if i == a.Selected {
					title.FontWeight(700)
				}
				// A trailing value prints complete at the row's right edge —
				// the calculator's answer, which the expression makes room
				// for rather than truncating (`.launcher-result__calculator-
				// result`: nowrap, strong, 620, tabular figures).
				if item.trailing != "" {
					// The answer rolls its digits when it changes
					// (`ui.ContentTransition{Numeric: true}`) — a calculator
					// answer is the app's own counter.
					ui.Text(c, item.trailing).FontSize(t.FontSize).
						FontWeight(620).TextColor(tokens.TextStrong).Grow(1).
						TextAlign(ui.End).SingleLine().
						ContentTransition(ui.ContentTransition{Numeric: true})
				}
			})
			if subtitle != "" {
				// The subtitle is the body register at 510, quiet; the
				// selected row lifts it one step and thickens it, as the old
				// build's own selected subtitle did.
				line := ui.Text(c, subtitle).FontSize(t.FontSize).
					FontWeight(510).TextColor(t.TextMuted).SingleLine()
				if i == a.Selected {
					line.TextColor(tokens.TextSecondary).FontWeight(600)
				}
			}
		})
		// The row's right-edge group, in the old build's order: the warning dot,
		// the space keycap, then the source word. They are siblings of the text
		// column rather than of its title line, so a narrow window cannot
		// ellipsize them away with the title — which is exactly when "where did
		// this result come from" matters most (`.launcher-result__source`).
		//
		// The warning dot is a real 8u circle, never a glyph in truncatable
		// text: it reads at any width (`.launcher-result__warning`).
		if item.warning {
			ui.Box(c).Size(t.Space(2), t.Space(2)).Shrink(0).
				Radius(t.Space(2)).Background(t.Warning).
				Label(copy.CommandConflict).Tooltip(copy.CommandConflict)
		}
		// The space-bar nudge: a command whose output the launcher shows itself
		// is entered with one space, and the row says so with a keycap —
		// deliberately generic (「就笼统点，简洁点，让用户意识到可以按空格就行了」),
		// a visual mark whose tooltip carries the sentence.
		if item.direct {
			// The space keycap (R65): a bare `␣` on a 18u bevel — border,
			// control fill, keycap shadow — not a sentence. The whole
			// explanation lives in the row's tooltip.
			key := ui.Row(c).Shrink(0).MinWidth(t.Space(4.5)).Height(t.Space(4.5)).
				Padding(0, t.Space(1)).Radius(tokens.RadiusXS).
				Border(1, tokens.ControlEdge).Background(tokens.Control).
				AlignItems(ui.Center).Justify(ui.Center).
				Tooltip(copy.DirectOutputHint)
			key.InsetShadow(0, 1, 0, 0, tokens.LensRim)
			key.Children(func() {
				ui.Text(c, "\u2423").FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
			})
		}
		// The source word: the row's provenance, in a fixed slot at the row's
		// right edge — the integration that contributed a command, the package
		// manager an install row will use. An inline suffix is the first thing
		// ellipsis eats on a narrow window, precisely when the answer matters.
		if item.Source != "" {
			source := ui.Text(c, item.Source).FontSize(t.FontSize).
				FontWeight(510).TextColor(tokens.TextTertiary).SingleLine().
				MaxWidthPercent(30).TextAlign(ui.End).Tooltip(item.Source)
			if i == a.Selected {
				source.TextColor(tokens.TextSecondary)
			}
		}
		// The delete is a two-step: the first press arms the row — the
		// muted "press again" note stands in for the trailing chrome, in
		// the one warm token the row may spend — and the second removes.
		// The armed state leaves with the selection, so a stray press
		// cannot remove a row the user was not looking at.
		//
		// The old build's delete is keyboard-only (⌘⌫); the row shows
		// the note while armed, never a click target. The note is the
		// whole confirmation. It renders before the star and pin, as
		// the old build's armed span sat before them in the row's tail.
		//
		// The old build's `.launcher-result__armed` was `white-space:
		// nowrap` — the note never wraps, however narrow the row —
		// and `flex: 0 0 auto`, so it never shrinks. The old build's
		// `aria-live="polite"` announced the note to screen readers
		// when it appeared.
		if a.armedDelete == item.ID {
			ui.Text(c, copy.DeleteConfirm).FontSize(t.FontSize - 1).
				FontWeight(560).TextColor(t.Warning).NoWrap().Shrink(0)
			if !a.armedDeleteAnnounced {
				c.Announce(copy.DeleteConfirm)
				a.armedDeleteAnnounced = true
			}
		}
		// A history row's own star: the favourite toggle the old build drew as
		// a 20u box at the row's trailing edge — hidden until the row is
		// hovered or chosen, and shown whenever the entry is starred. Clicking
		// it stars the entry without running the row.
		//
		// Its ink is the reference's own ladder: `--text-tertiary` at rest
		// (`.launcher-result__favorite`, `main:src/styles/launcher.css:1378-1387`),
		// lifted to `--text-secondary` on hover or while favourited (`:1395-1397`)
		// — **never** the accent, and never a pane behind it. The port had rested
		// on the heavier `--text-muted` and lifted all the way to `--text-primary`,
		// one register heavy at both ends.
		//
		// R50 · the star is hidden while the row's delete is armed: the old
		// build's `!deleteArmed` condition swapped the star for the "press
		// again" note, so a half-finished confirmation never offered a
		// second control competing with the key that completes it.
		if favorite, ok := rowFavorite(item); ok && a.armedDelete != item.ID {
			starLabel := copy.HistoryFavorite
			if favorite {
				starLabel = copy.HistoryUnfavorite
			}
			star := ui.Row(c).Focusable().Shrink(0).
				Size(t.Space(5), t.Space(5)).AlignItems(ui.Center).Justify(ui.Center).
				Label(starLabel).Tooltip(starLabel)
			shown := favorite || i == a.Selected || row.Hovered()
			if !shown {
				star.Opacity(0)
			}
			// The star is a state marker, not a control: `--text-secondary` is the
			// strongest colour it ever takes (`:1395-1397`), whether hovered or on.
			color := tokens.TextTertiary
			if favorite || star.Hovered() {
				color = tokens.TextSecondary
			}
			// The star's mark is the reference's literal 13
			// (`<StarIcon size={13}/>`, `LauncherResults.tsx:987-991`) and it is
			// **painted in** while the entry is starred — the same icon with
			// `fill={currentColor}` — where a stroke-only mark reads as "not starred"
			// at this size. The port drew it at `Space(3.5)` (14u, scaling with the
			// interface step) and never filled it.
			star.Children(func() {
				name := "star"
				if favorite {
					name = "star-filled"
				}
				if glyph, ok := Glyph(name); ok {
					ui.Icon(c, glyph).Size(13, 13).TextColor(color)
				}
			})
			if star.Clicked() {
				// The toggle acts on the row the star belongs to, which the
				// pointer has just made the choice.
				a.Selected = i
				a.toggleFavorite(item)
				removed = true
			}
		}
		if i == a.Selected && (item.clip != nil || item.calc != nil) {
			// A clipboard entry's whole text can be shown in a window of its
			// own: the row's text is the smallest honest cut of "pin one
			// entry", and the control rides the selected row so a stray click
			// cannot pin something the user was not looking at.
			// The row's own controls are invisible click regions that come
			// with the selection, as the reference's `.launcher-result__pin`
			// was: a 20u box and a mark, never a filled pill in a list of
			// quiet rows.
			if item.clip != nil && a.Actions.PinText != nil && clipboardPinApplies(*item.clip) {
				a.rowControl(c, "text", copy.HistoryPin, func() {
					a.pinClip(*item.clip)
					removed = true
				})
			}
		}
		if number, ok := a.numbers[i]; ok {
			a.rowActionText(c, a.resultNumber(number), i == a.Selected)
		}
		if item.Shortcut != "" {
			if isBar {
				a.barHintText(c, item.Shortcut)
			} else {
				a.rowActionText(c, item.Shortcut, i == a.Selected)
			}
		}
	})
	if row.Clicked() && !removed {
		a.Selected = i
		a.activate(a.Results())
	}
}

// activate runs the result at the current selection — or the command being
// typed, in the command mode, where Enter means "run what is typed".
func (a *App) activate(results []Item) {
	if a.mode != nil {
		a.runMode()
		return
	}
	if a.clipboard {
		a.runClipboard(results)
		return
	}
	if a.browser {
		if a.Selected >= 0 && a.Selected < len(results) {
			if run := results[a.Selected].Run; run != nil {
				run()
			}
		}
		a.leaveBrowser()
		a.Query = ""
		a.Hide()
		return
	}
	if a.files {
		if a.Selected >= 0 && a.Selected < len(results) {
			if run := results[a.Selected].Run; run != nil {
				run()
			}
		}
		return
	}
	if a.calculatorMode {
		if a.Selected >= 0 && a.Selected < len(results) {
			if run := results[a.Selected].Run; run != nil {
				run()
			}
		}
		a.leaveCalculator()
		a.Hide()
		return
	}
	if a.Selected < 0 || a.Selected >= len(results) {
		// The old build's Enter ran the action bar whenever the list had no
		// results (`useLauncherActions`: `selectedActionBar ||
		// !launcherResults.length`), so a typed command that matched nothing
		// still ran in the terminal even though no row was drawn for it (the
		// bar is drawn only over a matched list — see actionBarItemsWith).
		if len(results) == 0 {
			if bar := a.actionBarItems(); len(bar) > 0 {
				if run := bar[0].Run; run != nil {
					run()
				}
			}
		}
		return
	}
	if run := results[a.Selected].Run; run != nil {
		run()
	}
}

// Hide hides the launcher window, for an action that finished its job.
func (a *App) Hide() {
	if a.Actions.Dismiss != nil {
		a.Actions.Dismiss()
	}
}

// leaveBrowser returns to the search.
func (a *App) leaveBrowser() {
	if !a.browser {
		return
	}
	a.browser = false
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// enterBrowser starts searching the browsers' history.
func (a *App) enterBrowser() {
	a.enterBrowserWord(browserWord + " ")
}

// enterBrowserWord opens the browser mode with the field's text, on the range
// the trigger word names (bookmarks, history, or everything).
func (a *App) enterBrowserWord(query string) {
	a.mode = nil
	a.clipboard = false
	a.calculatorMode = false
	a.browserFilter = browserFilterForWord(firstWord(query))
	a.browser = true
	a.modeWord = firstWord(query)
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// browserQuery is what the user typed: the field holds the needle alone.
func (a *App) browserQuery() string {
	return a.Query
}

// askBrowser asks the shell for results once per query.
func (a *App) askBrowser() {
	if !a.browser {
		return
	}
	query := a.browserQuery()
	if a.browserAsked && a.browserAskedFor == query {
		return
	}
	if a.browserPending {
		return // one query is in flight; the next build asks again
	}
	a.browserPending = true
	if a.Actions.SearchBrowser == nil {
		a.browserPending = false
		a.browserAsked, a.browserAskedFor, a.browserFound = true, query, BrowserResults{}
		return
	}
	a.Actions.SearchBrowser(query, func(answer BrowserResults) {
		// An answer for a query the user has typed past is dropped, so a slow
		// tab read cannot replace the results of a newer one.
		if !a.browser || a.browserQuery() != query {
			return
		}
		a.browserPending = false
		a.browserAsked, a.browserAskedFor, a.browserFound = true, query, answer
	})
}

// runClipboard copies the chosen entry and leaves the mode, as picking a
// clip means pasting it next.
func (a *App) runClipboard(results []Item) {
	if a.Selected >= 0 && a.Selected < len(results) {
		if run := results[a.Selected].Run; run != nil {
			run()
		}
	}
	a.leaveClipboard()
	// A run clears the field, as the old build's own copy did: the next
	// summon starts from the empty needle, not from the query that produced
	// the row the user just used.
	a.Query = ""
	if a.Actions.Dismiss != nil {
		a.Actions.Dismiss()
	}
}

// runMode leaves the command mode and runs the command with the arguments
// the user typed.
func (a *App) runMode() {
	entry := *a.mode
	args := a.commandArgs()
	a.leaveCommand()
	// A run clears the field, as the old build's own runs did.
	a.Query = ""
	a.runCommandWith(entry, args)
}

// move moves the selection by delta, wrapping around the list.
func (a *App) move(delta, n int) {
	a.Selected = a.nextSelectable(a.Selected, delta)
	if a.Selected >= n && n > 0 {
		a.Selected = ClampIndex(a.Selected, n)
	}
	// The armed delete belongs to the row the user was looking at; the choice
	// moving away takes it with it.
	a.disarmDelete()
}

// rowSelectable reports whether the keyboard can land on a row: a block's
// heading line is a label and a plugin's status note is information, neither
// of them a choice, so the arrows step over both.
func rowSelectable(item Item) bool { return !item.heading && !item.status }

// nextSelectable moves one step and lands on a row the keyboard can act on: a
// block's heading line is a label and a plugin's status note is information,
// neither of them a choice, so the arrows step over both.
func (a *App) nextSelectable(from, delta int) int {
	return nextSelectableIn(a.Results(), from, delta)
}

// nextSelectableIn is nextSelectable over a list the caller already holds: the
// draw clamps its choice against the slice it built, and asking for a fresh
// catalog there was the draw's second full match (see App.searchRuns).
func nextSelectableIn(results []Item, from, delta int) int {
	n := len(results)
	if n == 0 {
		return 0
	}
	index := from
	for i := 0; i < n; i++ {
		index = NextIndex(index, delta, n)
		if rowSelectable(results[index]) {
			return index
		}
	}
	return from
}

// firstSelectable is the empty page's own default selection: the first row
// the keyboard can act on, stepping over a block's heading — the reference's
// `firstRunnableResultIndex < 0 ? 0 : ...` on the page where the held row
// shows. Zero when the list has nothing selectable.
//
// The release edge applies it unconditionally, the reference's own shape: its
// effect reads no selection, so the row an arrow step moved to during the
// gesture goes with the row.
func (a *App) firstSelectable() int {
	results := a.Results()
	for i, item := range results {
		if rowSelectable(item) {
			return i
		}
	}
	return 0
}

// actionBarIndex is the index of the action bar's first row in the composed
// list, -1 when the field holds no action bar. The bar is a real row of this
// list (see actionBarItemsWith), so "the bar is the default" and "the bar is
// selected" are both this row's index.
func actionBarIndex(results []Item) int {
	for i, item := range results {
		if strings.HasPrefix(item.ID, "bar:") {
			return i
		}
	}
	return -1
}

// defaultsToActionBar is the old build's `shouldDefaultToActionBar`: whether
// Enter belongs to the action bar rather than the first result. A non-shell
// action bar (a URL or a path) always wins; a shell one only when the query
// is a command line rather than a name a runnable result already answers.
func (a *App) defaultsToActionBar(results []Item) bool {
	kind := classifyActionBar(a.Query)
	if kind == "" {
		return false
	}
	if kind != actionShell {
		return true
	}
	return a.defaultsToShell(results)
}

// actionBarSplit is where the composed list stops being the scroller's and
// becomes the pinned footer, and the band that footer occupies in pixels. A
// page with no bar answers the whole list and no band.
//
// The split is by *index*: the bar rows keep the indices they have in
// `Results`, so the keyboard, the pointer, `defaultSelection` and the numbers
// reach them exactly as they did when they were the list's own last rows. Only
// their place on the panel changed.
func (a *App) actionBarSplit(c *ui.Context, results []Item) (int, float32) {
	start := actionBarIndex(results)
	if start < 0 {
		return len(results), 0
	}
	return start, c.Theme().Space(actionBarUnits * float32(len(results)-start))
}

// actionBarFooter draws the action bar as the panel's pinned footer: the band
// under the scroller that the reference gave `.launcher-action-bar`, a sibling
// of `.launcher-results` rather than one of its rows
// (`main:src/launcher/LauncherResults.tsx:1043-1073`). The scroller above it
// grows into the rest of the panel, so the two never overlap: a row scrolled to
// the split stops at the footer's top edge, and the footer needs no fill of its
// own (the panel's glass stays one pane).
func (a *App) actionBarFooter(c *ui.Context, results []Item, start int) {
	if start >= len(results) {
		return
	}
	t := c.Theme()
	band := t.Space(actionBarUnits * float32(len(results)-start))
	ui.Column(c).FillWidth().Shrink(0).Height(band).MinHeight(0).
		Padding(0, t.Space(1)).Children(func() {
		for i := start; i < len(results); i++ {
			a.row(c, results[i], i)
		}
	})
}

// defaultSelection is the row a fresh query starts on: the action bar when
// `defaultsToActionBar` says Enter belongs to it, else the first runnable
// result — the reference's `firstRunnableResultIndex < 0 ? 0 : ...`.
//
// Only the *dead-key* case reaches the bar: a query that answers nothing (or
// nothing runnable) has no result to sit on, and the footer is then the only
// row Enter could mean. A query that matches a catalog command keeps the
// selection on that command and the bar stays the footer it is.
func (a *App) defaultSelection(results []Item) int {
	bar := actionBarIndex(results)
	if bar >= 0 && a.defaultsToActionBar(results) {
		return bar
	}
	for i, item := range results {
		if strings.HasPrefix(item.ID, "bar:") || !rowSelectable(item) || item.Run == nil {
			continue
		}
		return i
	}
	if bar >= 0 {
		return bar
	}
	return 0
}

// clampSelection keeps the choice on a row the keyboard can act on: a heading
// the list shrank onto is stepped past. It reads the list the draw already
// built — the same list the caller measured the count from.
func (a *App) clampSelection(results []Item) {
	n := len(results)
	a.Selected = ClampIndex(a.Selected, n)
	if a.Selected < n && !rowSelectable(results[a.Selected]) {
		a.Selected = nextSelectableIn(results, a.Selected, 1)
		if a.Selected >= n {
			a.Selected = ClampIndex(a.Selected-1, n)
		}
	}
}

// feedbackDuration is how long the docked status row stays readable. The old
// build's feedback line lived on a timer of its own; three seconds is long
// enough to read a short sentence and short enough that the row does not
// outstay its message.
const feedbackDuration = 3 * time.Second

// feedbackRow draws the panel's docked status row: a message under the list,
// with an icon and — for a warning — the warm blocking tone. The row is
// charged to the window's height (see Geometry.Feedback) and its band is the
// same whether it is drawn or not within one message's life; the frames keep
// coming while it shows, so it expires on time.
func (a *App) feedbackRow(c *ui.Context) {
	if a.toast == "" {
		return
	}
	now := c.Now()
	if a.toast != a.toastShown {
		a.toastShown = a.toast
		a.toastUntil = now.Add(feedbackDuration)
	}
	if now.After(a.toastUntil) {
		a.toast, a.toastWarning, a.toastShown, a.toastUntil = "", false, "", time.Time{}
		return
	}
	c.AnimationFrame()
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	// The old build's own row: a 30u band, the panel's 12u inset, the body
	// step at 560 (600 for a warning), and the alert glyph at 16u. A warning
	// is the stronger face — a border in the warning colour and a heavier
	// weight — so it reads as a blocking message rather than a status hint.
	tone := t.Warning
	weight := 560
	// `.launcher-feedback`'s own fill is `--system-icon-surface`, the warm
	// amber plane — **not** the warning ink at a low alpha. Quoting the
	// label's colour here painted the row pink-red; the reference's plane is a
	// separate token (`theme.Tokens.IconSurface`), and the warning variant's
	// `--glass-raised-warm` is an alias of the very same value.
	background := tokens.IconSurface
	feedback := ui.Row(c).Absolute().Bottom(a.footerBand).Left(t.Space(1)).Right(t.Space(1)).Height(t.Space(7.5)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).
		Radius(tokens.RadiusSM).
		// The row enters as the old build's `feedback-enter` did (its
		// --dur-3, 160ms).
		Transition(ui.ElementTransition{
			Enter:    &ui.Motion{Y: -2},
			Duration: 160 * time.Millisecond,
		})
	if a.toastWarning {
		// `.launcher-feedback--warning` takes the raised-warm plane, the warning
		// keyline and `--glass-raised-shadow`: the `--elev-2` contact step (a
		// function of the frame's own alpha, so its 0.2 is scaled here exactly as
		// the terminal note's is) plus the pane's lit top rim. The non-warning row
		// carries none of them.
		//
		// The warning's fill is `--glass-raised-warm`, which base.css declares as an
		// **alias of `--system-icon-surface`** (`base.css:611`) — the same amber
		// plane the resting row already wears. Only the keyline, the shadows and the
		// weight step up.
		background, weight = tokens.RaisedWarm, 600
		feedback.Border(1, tone)
		feedback.Shadow(0, t.Space(0.25), t.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.2))
		feedback.InsetShadow(0, 1, 0, 0, tokens.LensRim)
	}
	// The fill lands after the variant has picked it: painting the row before
	// the branch ran left the warning's own plane unreachable — the row read the
	// resting amber in both states.
	feedback.Background(background)
	feedback.Children(func() {
		ui.Icon(c, glyphAlert).Size(t.Space(4), t.Space(4)).TextColor(tone)
		ui.Text(c, a.toast).FontSize(t.FontSize).FontWeight(weight).TextColor(tone).Grow(1)
	})
}

// FeedbackVisible reports whether the docked status row is showing, for the
// shell's window sizing.
func (a *App) FeedbackVisible() bool { return a.toast != "" }

// syncHeldModifier reads the app modifier's state for this frame and moves the
// selection to (or away from) the held row at the edge where the modifier goes
// down or up. The row itself is part of Results (see Results), so the window's
// height and the row's own presence are one decision.
//
// The two edges are the reference's own (src/App.tsx R64, the heldBareTerminal
// effect): the press edge puts the row — the list's last line — in the
// selection; the release edge resets the selection to the page's own default,
// which is its first runnable row, unconditionally — the effect's dependency
// list carries no selection, so an arrow step off the row during the gesture
// is discarded on release, exactly as the reference's is. There is no delay:
// the row and the selection move on the same frame the modifier's state lands.
func (a *App) syncHeldModifier(c *ui.Context) []Item {
	down := c.Modifiers()&ui.Cmd == ui.Cmd
	// "Was the row showing" has to be asked of the frame before: once
	// modifierHeld is down the row is already gone (see heldRowShown).
	rowShowing := a.heldRowShown
	// The row's own predicate reads modifierHeld, so the state lands before
	// the edge is handled.
	a.modifierHeld = down
	// The frame's **one** catalog walk, taken after the state landed so the
	// list it returns is the list this frame draws: the held row is in it
	// exactly when this frame's modifier state says it is. The caller reuses
	// this slice for the rest of the frame rather than asking again — a second
	// full match per frame was the launcher's quietest cost (see
	// App.searchRuns) and it bought nothing: the two calls disagreed by
	// definition whenever the state had just changed.
	results := a.Results()
	a.heldRowShown = len(results) > 0 && results[len(results)-1].ID == bareTerminalRowID
	if down != a.heldWasDown {
		a.heldWasDown = down
		if down {
			if _, ok := a.heldTerminalItem(); ok {
				// The held row is the list's last line, so its index is the
				// last one. The key is recorded here so
				// holdHeldRowSelection does not re-assert it again this frame.
				a.Selected = len(results) - 1
				a.heldRowKey = heldRowKey(results, a.Query)
			} else if strings.TrimSpace(a.Query) != "" {
				// No held row — a typed query draws none (R64), and a plugin
				// mode or the output view owns the list — so the modifier
				// hands the selection to the action bar, the old build's
				// `setSelectedActionBar(true)` on the modifier's keydown. The
				// bar is a row of this list, so it is selected by its index;
				// it is drawn for a non-empty query now (see
				// actionBarItemsWith).
				if bar := actionBarIndex(results); bar >= 0 {
					a.Selected = bar
				}
			}
		} else {
			a.heldRowKey = ""
			a.barRowKey = ""
			if rowShowing || strings.TrimSpace(a.Query) != "" {
				// The row the modifier put on screen is gone. The selection
				// returns to the page's own default — the reference's
				// `firstRunnableResultIndex`.
				a.Selected = a.defaultSelection(results)
			}
		}
	}
	return results
}

// holdHeldRowSelection keeps the selection on the held row for as long as the
// row is on screen and the list it lives in has changed under it.
//
// The reference's heldBareTerminal effect depends on `query` and
// `displayedResultsLength` (`main:src/App.tsx:2065-2082`), so every rebuild of
// the list re-runs it and the row takes the selection again. The port asserted
// the selection on the modifier's press edge only, so a later rebuild could
// leave the row drawn without the highlight — a row the user read as pinned and
// unselected: 「…应该有选中样式（当前缺失）」. It runs after the query's own
// default reset, so the held row wins while it is on screen.
//
// The row only exists on an empty field (R64, see heldTerminalItem), so this is
// the empty page's own assertion; a typed query never has a row to keep.
//
// A step off the row with the arrows is *not* undone: the key only changes
// when the list's own shape does (the row count or the query), exactly as the
// reference's dependency list re-runs only on those changes.
func (a *App) holdHeldRowSelection(results []Item) {
	if !a.modifierHeld || len(results) == 0 || results[len(results)-1].ID != bareTerminalRowID {
		a.heldRowKey = ""
		return
	}
	key := heldRowKey(results, a.Query)
	if key == a.heldRowKey {
		return
	}
	a.heldRowKey = key
	a.Selected = len(results) - 1
}

// heldRowKey names the list shape the held row's selection was asserted for:
// the row count and the query the row was placed over.
func heldRowKey(results []Item, query string) string {
	return strconv.Itoa(len(results)) + "\x00" + query
}

// holdBarSelection keeps the action bar selected while the modifier is held
// over a *typed* query, which is the row the modifier's press edge hands the
// selection to there (see syncHeldModifier: a typed query draws no held row,
// R64, so the bar is the row that acts on the line).
//
// Without it the selection the press edge set was lost on the very next
// keystroke: the query's own default reset (the `a.Query != a.defaultedQuery`
// branch in View) lands on the first match, so the bar stayed on screen and
// unselected for as long as the user kept typing with the key down — the report
// 「应该有选中样式（当前缺失）」. The reference's effect depends on `query` and
// the list's length (`main:src/App.tsx:2065-2082`), so it re-runs on exactly
// those changes and not on an arrow step: the key below is the same shape key
// the held row uses, so an arrow step off the bar survives until the list
// itself changes.
func (a *App) holdBarSelection(results []Item) {
	if !a.modifierHeld || a.output != nil || a.pluginModeActive() || strings.TrimSpace(a.Query) == "" {
		a.barRowKey = ""
		return
	}
	bar := actionBarIndex(results)
	if bar < 0 {
		a.barRowKey = ""
		return
	}
	key := heldRowKey(results, a.Query)
	if key == a.barRowKey {
		return
	}
	a.barRowKey = key
	a.Selected = bar
}

// heldTerminalItem is the row the held modifier offers: a bare terminal
// session, on the ordinary page, **on an empty field**. A plugin mode owns its
// own list and shows no row of its own; the output view owns the keys and shows
// none either.
//
// The reference's own history has both shapes: R60 appended the row under the
// modifier whatever the field held (「按住 cmd 时也自动增加底部的终端执行项」),
// and R64 retired that half at the user's own request — 「前面加的按住在列表
// 额外增加终端选项的逻辑要去掉」 — so `bareTerminalPlacement` returns null for
// a non-empty field (`main:src/launcher/terminal-row.ts:77-85`) and a typed
// query draws the row not at all. The port had kept R60's wider gesture, so
// with a query typed the action word showed up as an ordinary result beside
// the matches — the report 「输入框中有输入内容时出现的「在终端中打开」项：
// 现在固定在第一项（错误）」. One placement only: the empty page, last line.
func (a *App) heldTerminalItem() (Item, bool) {
	if !a.modifierHeld || a.output != nil || a.pluginModeActive() {
		return Item{}, false
	}
	if strings.TrimSpace(a.Query) != "" {
		return Item{}, false
	}
	copy := a.copy()
	return Item{
		ID:     bareTerminalRowID,
		Title:  copy.BareTerminal,
		Detail: copy.BareTerminalHint,
		// No trailing badge: the reference's row is an ordinary `system` row
		// with no `shortcut` of its own (`main:src/launcher/terminal-row.ts`
		// `bareTerminalRow`), and the numbered family never reaches it (R67) —
		// `LauncherResults` draws `resultShortcutSlots[index]`, which is null
		// there. The port had printed the action bar's ⌘⏎ on it, a slot the
		// design leaves empty; the chord still opens the row (the modifier is
		// down when Enter is pressed, see `View`).
		Run: func() {
			if a.Actions.OpenTerminal != nil {
				a.Actions.OpenTerminal()
			}
		},
	}, true
}

// scopeMark is the mode that owns the field: its mark and its name. Ok is
// false on the ordinary search page, whose field has no owner.
func (a *App) scopeMark(copy i18n.Launcher) (*ui.SVG, string, bool) {
	name := func(scope string) string { return scope }
	switch {
	case a.mode != nil:
		glyph, _ := Glyph("terminal")
		label := a.mode.Command.Name
		if label == "" {
			label = name(copy.ScopePlugin)
		}
		return glyph, label, true
	case a.clipboard:
		glyph, _ := Glyph("clipboard")
		return glyph, name(copy.ScopeClipboard), true
	case a.browser:
		glyph, _ := Glyph("globe")
		return glyph, name(copy.ScopeBrowser), true
	case a.calculatorMode:
		glyph, _ := Glyph("calculator")
		return glyph, name(copy.ScopeCalculator), true
	}
	return nil, "", false
}

// fieldButtons draws the field row's trailing controls: the two ways in the
// old build kept beside the field — the sessions page and the settings — each
// a glyph button whose own label names it for assistive technology.
// fieldSettingsButton draws one of the field row's trailing icon controls:
// the reference's `.collapsed-card__settings` (`main:src/styles/launcher.css:
// 382-419`), which all three of them (sessions, settings, the plugin gear and
// the pin) share as one control. It is a 28u transparent square with a 9u
// corner, its mark at rest in `--text-tertiary` (`:397`), and the 8u margin the
// old row put before it.
//
// Two states ride on top of the resting face, and the port had neither:
//
//   - `:hover:not(:disabled)` (`:400-404`) lifts the mark one step to
//     `--text-strong` *and* lights the control's own hover fill with the
//     resting plane's lens (`--elev-0`); and
//   - `:active:not(:disabled)` (`:407-411`) sinks the button a hair into the
//     glass — `transform: scale(0.92)` over `--glass-control-press`, its
//     shadow gone — which is the press the row's other controls already take.
//
// The transition (`:397`, `color var(--dur-2) ease, background var(--dur-2)
// ease, transform var(--dur-1) ease`) is where the 120ms colour step and the
// 80ms transform step come from; the framework animates both off one
// `ElementTransition`, so the colours ride the 120ms step and the press scale
// is a render-time transform under the same use.
func (a *App) fieldSettingsButton(c *ui.Context, tokens theme.Tokens, label, hint string, mark func(mark ui.Color), run func()) {
	t := c.Theme()
	e := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
		Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Tooltip(hint).Margin(0, 0, 0, t.Space(2)).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	// The reference's `.collapsed-card__settings` carries three faces: the
	// resting mark in `--text-tertiary`, the hover lift (`--glass-control-hover`
	// + `--elev-0` + `--text-strong`), and the press (the scale and
	// `--glass-control-press`). The press wins over the hover, as the CSS's
	// `:active` rule does over `:hover`.
	ink := tokens.TextTertiary
	switch {
	case e.Pressed():
		e = e.Background(tokens.ControlPress).Scale(0.92, 0.92)
		ink = tokens.TextStrong
	case e.Hovered():
		e = tokens.Lens(e.Background(tokens.ControlHover))
		ink = tokens.TextStrong
	}
	e.Children(func() { mark(ink) })
	if e.Clicked() {
		run()
	}
}

func (a *App) fieldButtons(c *ui.Context, copy i18n.Launcher) {
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	// The field's own buttons are the reference's `.collapsed-card__settings`:
	// a 28u square on the row, a 9u radius, its mark in the tertiary colour,
	// and a hover that lights the mark one step - with the 8u margin the old
	// row put between the buttons. Each one is a glyph with no visible word, so
	// it carries the sentence the old build's `title=` did.
	button := func(glyphName, label, hint string, run func()) {
		if run == nil {
			return
		}
		a.fieldSettingsButton(c, tokens, label, hint, func(mark ui.Color) {
			if glyph, ok := Glyph(glyphName); ok {
				ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(mark)
			}
		}, run)
	}
	// The settings button's line names the key that opens the same surface, and
	// it is the user's own binding: the old build interpolated the live
	// `open_settings` accelerator into `settings.openHint`.
	settingsHint := copy.FieldSettingsHint(shortcuts.Display(settings.Shortcut(a.settings(), settings.ShortcutOpenSettings)))
	button("terminal", copy.FieldSessions, copy.FieldSessionsHint, a.Actions.OpenSessions)
	button("settings", copy.FieldSettings, settingsHint, a.Actions.OpenSettings)
}

// pluginGear is the field row's trailing control inside a built-in plugin
// mode: the plugin's own door to its configuration (R29). It replaces the
// sessions/settings pair — those are not plugin actions — and, while the sheet
// is open, it is the sheet's only close control (R41): it flips from the gear
// to an ✕, so the place that opened the sheet is the place that closes it. The
// chips row is the list's own filter and hides with the list while the sheet
// is up, so this is the control that stays.
func (a *App) pluginGear(c *ui.Context, copy i18n.Launcher, id string) {
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	open := a.configOpen() && a.config.Schema.Plugin == id
	label, glyphName := copy.ConfigurePlugin, "settings"
	hint := copy.ConfigurePluginHint
	if open {
		label, glyphName = copy.ConfigurePluginClose, "close"
		hint = copy.ConfigurePluginCloseHint
	}
	a.fieldSettingsButton(c, tokens, label, hint, func(mark ui.Color) {
		if glyph, ok := Glyph(glyphName); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(mark)
		}
	}, func() { a.toggleConfig(id) })
}

// pinButton is the field row's trailing control for an external command
// (R84): a pin that puts the plugin page's command + current argv into
// its own window that never follows the launcher's summon/hide. Same
// 28u transparent square as the other settings buttons — the gear
// that opened the sheet, and the pin are additive only.
func (a *App) pinButton(c *ui.Context, copy i18n.Launcher) {
	t := c.Theme()
	tokens := theme.ForPreferences(a.settings(), t.Dark, c.Preferences())
	a.fieldSettingsButton(c, tokens, copy.OutputDetach, copy.OutputDetachHint, func(mark ui.Color) {
		if glyph, ok := Glyph("pin"); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(mark)
		}
	}, func() {
		a.Actions.DetachOutput(OutputView{Entry: a.mode, Args: strings.Split(a.Query, " ")})
	})
}
