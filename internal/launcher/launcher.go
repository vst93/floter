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
	// ToolAliases is the user's alias per command name, which a PATH command's
	// row is matched by as well as by its name.
	ToolAliases settings.CommandAliases

	// Scroll keeps the result list's place; Search is the field's identity,
	// for the focus the launcher keeps on it while the surface shows.
	List   ui.ListState
	Search ui.Handle

	// chosenRow tracks the row the keyboard last moved to, so the list is
	// scrolled only when the choice changed.
	chosenRow int

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
	// icons caches the decoded application icons, by path, and thumbs the
	// clipboard images'.
	icons  map[string]*ui.Bitmap
	thumbs map[string]*ui.Bitmap
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
	// pendingPower is the power action the panel is asking about: the
	// confirmation row shows until the user answers it (see power.go).
	pendingPower string
	// OutputList is the output list's own scrolling and selection state.
	OutputList ui.ListState
	// files is set while a drop's rows are listed, with Dropped the files the
	// desktop handed over.
	files   bool
	Dropped []drops.File
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
	a.icons[app.Path] = bitmap
	return bitmap
}

// assignNumbers gives the first ten rows that can be run their shortcut
// number, recomputed each frame: the numbers follow what is on screen, so
// scrolling renumbers them.
func (a *App) assignNumbers(results []Item) map[int]int {
	numbers := map[int]int{}
	count := 0
	for i, item := range results {
		if item.Run == nil {
			continue
		}
		count++
		if count > 10 {
			break
		}
		// The tenth row answers to 0, as a keyboard's digits end.
		numbers[i] = count % 10
	}
	a.numbers = numbers
	return numbers
}

// resultShortcuts runs the row a number names, reporting whether the key was
// one of them. The modifiers come from the settings' select_result binding, so
// rebinding the family moves every number with it.
func (a *App) resultShortcuts(c *ui.Context, results []Item) bool {
	for digit := 0; digit <= 9; digit++ {
		key := ui.Key0
		if digit > 0 {
			key = ui.Key0 + ui.Key(digit)
		}
		mods, wanted, ok := shortcuts.Parse(settings.SelectResultDigit(a.settings(), digit))
		if !ok || wanted != key {
			continue
		}
		if !c.Shortcut(mods, key) {
			continue
		}
		for index, number := range a.numbers {
			if number != digit || index >= len(results) {
				continue
			}
			a.Selected = index
			a.activate(results)
			return true
		}
		return true
	}
	return false
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
func (a *App) appShortcuts(c *ui.Context) bool {
	m := a.settings()
	for _, action := range []string{
		settings.ShortcutNewCommand,
		settings.ShortcutOpenExternalTerminal,
		settings.ShortcutOpenSettings,
	} {
		mods, key, ok := shortcuts.Parse(settings.Shortcut(m, action))
		if !ok || !c.Shortcut(mods, key) {
			continue
		}
		switch action {
		case settings.ShortcutNewCommand:
			a.ResetQuery()
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
	return false
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

// SetTools replaces the commands found on the PATH, whether the search offers
// them, and the aliases the user gave them (command name -> alias).
func (a *App) SetTools(tools []apps.App, show bool, aliases settings.CommandAliases) {
	a.Tools, a.ShowTools, a.ToolAliases = tools, show, aliases
}

// SetQuery puts text in the field, for a deep link that names what to search
// for.
func (a *App) SetQuery(query string) {
	a.Query = query
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// New builds the launcher state over a settings store and the shell's
// actions.
func New(store *settings.Store, actions Actions) *App {
	return &App{Store: store, Actions: actions}
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

	a.syncTypedMode()
	a.syncCommandMode()
	// Leaving a plugin's mode leaves its configuration sheet: the sheet is
	// that mode's own face, not a page of the app's.
	a.syncConfig()
	a.askBrowser()
	a.modeShortcuts(c)
	// The held-modifier row's premise: the app modifier down on an empty field
	// puts a "new terminal session" line at the list's end and hands it the
	// selection, so Enter reaches it. Releasing the modifier gives the
	// selection back.
	a.syncHeldModifier(c)
	// The chord that put the row on screen is the chord that opens it: the
	// modifier is still down when the user presses Enter, so the plain
	// Enter's own path (the field's submit) is not the one that fires.
	if item, ok := a.heldTerminalItem(); ok && c.Shortcut(ui.Cmd, ui.KeyEnter) {
		item.Run()
	}
	results := a.Results()
	a.clampSelection(len(results))
	a.assignNumbers(results)

	// The window's height is the content's own: measure every row's lines
	// off the theme, so the shell can size the window to the list it holds.
	tk := theme.For(a.settings(), t.Dark)
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
	rows := make([]RowInfo, len(results))
	for i, item := range results {
		if item.Detail != "" {
			rows[i].Lines = 2
		} else {
			rows[i].Lines = 1
		}
		rows[i].Heading = item.heading
		rows[i].Bar = strings.HasPrefix(item.ID, "bar:")
	}
	a.Rows = rows
	// The window follows the list exactly: the row count it holds is the
	// count the content needs (the reference's R66 rule). A one-row margin
	// would leave the window a row too tall whenever a row leaves - which is
	// the held modifier's row on release. The cap is the work area less the
	// room the other windows keep.
	a.HeldRows = HoldRows(a.HeldRows, len(results))
	if a.Actions.ResizeTo != nil {
		geometry := Geometry{
			Font: a.Font, Spacing: a.Spacing, Rows: rows, Held: a.HeldRows,
			TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
			Filter:   a.filtersVisible(),
			Feedback: a.toast != "" || a.pendingPower != "",
			Config:   a.configSchema(),
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

	ui.Box(c).Fill().Background(tk.Sunken).Children(func() {
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
			ui.List(c.Key("launcher.results"), &a.List, len(results), func(i int) {
				a.row(c, results[i], i)
			}).Fill().Padding(listEdge, t.Space(1), 0, t.Space(1)).Label(copy.ResultsLabel).
				ScrollbarInsets(0, 4096, 0, 0)
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
			AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			// The scope mark: the mode that owns the field, at its left — the
			// icon says a plugin owns it and the muted word says which one
			// (`.collapsed-card__scope` / `__scope-name`). The trigger word
			// was consumed on entry, so this is the only place the mode is
			// named.
			if glyph, name, ok := a.scopeMark(copy); ok {
				mark := ui.Row(c).Shrink(0).Gap(t.Space(1)).AlignItems(ui.Center)
				mark.Children(func() {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
					ui.Text(c, name).FontSize(t.FontSize).TextColor(t.TextMuted)
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
				a.Selected = 0
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
				if hint := a.triggerHintText(); hint != "" && triggerHintFits(c, hint) {
					ui.Text(c, hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				}
				if a.Query != "" {
					// The clear control is the same 28u transparent square the
					// field's other two buttons are (`.collapsed-card__settings`):
					// a bare mark that only lands on the resting plane while
					// hovered, never a filled pill in the search row.
					clear := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
						Radius(tk.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
						Label(copy.Clear)
					if clear.Hovered() {
						clear.Background(t.SurfaceHover)
					}
					clear.Children(func() {
						if glyph, ok := Glyph("close"); ok {
							ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
						}
					})
					if clear.Clicked() {
						a.Query = ""
						a.Selected = 0
						a.FocusSearch()
					}
				} else {
					ui.Text(c, copy.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
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
		if !a.heldRowShowing() || a.Selected != len(a.Results())-1 {
			a.List.ScrollIntoView(a.Selected)
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
	if a.appShortcuts(c) {
		return
	}
	if a.resultShortcuts(c, results) {
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
			a.armedDelete = ""
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
// the old build's key — the app modifier plus D — and is not rebindable,
// because a control the user never asked to rebind does not belong in the
// settings map.
//
// The modes' other key, the history row's delete, cannot live here: a focused
// text field's editor takes every Backspace before any `Shortcut` registration
// is asked, so the field answers it itself (see fieldkeys.go).
func (a *App) modeShortcuts(c *ui.Context) {
	switch {
	case a.calculatorMode:
		if c.Shortcut(ui.Super|ui.Ctrl, ui.KeyD) {
			a.toggleCalculatorFavorite()
		}
	case a.clipboard:
		if c.Shortcut(ui.Super|ui.Ctrl, ui.KeyD) {
			a.toggleClipFavorite()
		}
	}
}

// rowControl is one of a launcher row's own trailing controls: a 20u hit
// region and its mark, which the selection brings with it — the reference's
// `.launcher-result__pin`, never a filled pill in a list of quiet rows.
func (a *App) rowControl(c *ui.Context, name, label string, run func()) {
	t := c.Theme()
	control := ui.Row(c).Focusable().Shrink(0).Size(t.Space(5), t.Space(5)).
		AlignItems(ui.Center).Justify(ui.Center).Label(label).Tooltip(label)
	if control.Hovered() {
		control.Background(t.SurfaceHover)
	}
	control.Children(func() {
		if glyph, ok := Glyph(name); ok {
			ui.Icon(c, glyph).Size(t.Space(3.5), t.Space(3.5)).TextColor(t.TextMuted)
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
	index := strings.IndexAny(query, " \t")
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
	settingsSearch   = "设置 shezhi sz options preferences"
	terminalSearch   = "终端 终端机 tzdzdj term terminal"
	browserSearch    = "浏览器 浏览器书签 书签 历史记录 llq llqsq sq lsjl browser bookmarks bookmark history"
	clipboardSearch  = "剪贴板 剪贴板历史 粘贴历史 jtb jtbls ntls ch clipboard paste"
	calculatorSearch = "计算器 计算 jsq js c calculator calculate"
	restartSearch    = "重启 重新启动 cq cxqd zq zxqd restart reboot"
	shutdownSearch   = "关机 关闭电脑 gj gbdn shutdown power off"
	quitSearch       = "退出 tuichu tc quit exit"
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
	a.armedDelete = ""
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
	if a.Query != "" && strings.TrimRight(a.Query, " \t") != a.Query {
		tokens = append(tokens, "")
	}
	return tokens
}

// currentWord is the argument being typed: the last word when the line does
// not end in whitespace, and nothing after a space.
func (a *App) currentWord() string {
	if strings.TrimRight(a.Query, " \t") != a.Query {
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
	trimmed := strings.TrimRight(a.Query, " \t")
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
	index := strings.LastIndexAny(line, " \t")
	if index < 0 {
		return word
	}
	return line[:index+1] + word
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
	// The row is keyed by the item, so a row that stays keeps its identity
	// (and its place) while the rows around it come and go; a row that
	// appears fades in rather than flashing into place.
	ui.Column(c).FillWidth().Children(func() {
		// A block's heading, when this row starts one: an external plugin's
		// own group name, a dropped file's block.
		if item.section != "" {
			ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(3), t.Space(1), t.Space(3)).
				Children(func() {
					ui.Text(c, item.section).FontSize(t.FontSize).FontWeight(620).
						TextColor(t.TextMuted)
				})
		}
		a.rowBody(c, item, i, copy)
	})
}

// rowBody is the row itself: the plate, the text column, the controls.
func (a *App) rowBody(c *ui.Context, item Item, i int, copy i18n.Launcher) {
	t := c.Theme()
	tokens := theme.For(a.settings(), t.Dark)
	// The row's own floor: the old app's rows were fixed heights, 42u with a
	// subtitle and 34u without, and a one-line row of body text is shorter
	// than 34u — the height the window is charged must be the height the row
	// draws, so the floor is set here and read there.
	floor := t.Space(rowOneLineUnits)
	if item.Detail != "" {
		floor = t.Space(rowTwoLineUnits)
	}
	row := ui.Row(c).Key(item.ID).FillWidth().Focusable().MinHeight(floor).
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
		// 3u above it — a constant, so the bottom edge never jumps when the
		// result count crosses zero — and a floor of its own 42u, which the
		// window charges as the old build's 45u.
		floor = t.Space(11.25)
		row.Margin(t.Space(0.75), 0, 0, 0)
	}
	// The press is the one state a row takes on top of the tint: the control
	// material's own press face (base.css's --glass-control-press), which is
	// *less* opaque than the resting pane - the row sinks into the glass
	// instead of popping out of it.
	rowPressed := row.Pressed()
	switch {
	case rowPressed:
		row.Background(t.SurfacePressed)
	case i == a.Selected && isBar:
		row.Background(t.SurfaceHover)
	case i == a.Selected:
		// The old build's selection: a quiet accent tint, and the emphasis is
		// the row's own type — the title at 700 — rather than a ring or a
		// shadow. The tint places the row; the weight announces it.
		row.Background(t.Accent.Alpha(0.085))
	case row.Hovered():
		row.Background(t.SurfaceHover)
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
		warm := strings.HasPrefix(item.ID, "cmd:power-")
		plate := a.iconPlate(c, warm)
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
					if warm {
						icon.TextColor(t.Warning)
					} else {
						icon.TextColor(t.TextMuted)
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
				// The action bar's title is a command line the user is handing
				// over, so it is the terminal's own monospace at 520 (the old
				// `.launcher-action-bar__title`).
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
			if item.Detail != "" {
				// The subtitle is the body register at 510, quiet; the
				// selected row lifts it one step and thickens it, as the old
				// build's own selected subtitle did.
				subtitle := ui.Text(c, item.Detail).FontSize(t.FontSize).
					FontWeight(510).TextColor(t.TextMuted).SingleLine()
				if i == a.Selected {
					subtitle.TextColor(tokens.TextSecondary).FontWeight(600)
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
		// A history row's own star: the favourite toggle the old build drew as
		// a 20u box at the row's trailing edge — hidden until the row is
		// hovered or chosen, and shown whenever the entry is starred. Clicking
		// it stars the entry without running the row.
		if favorite, ok := rowFavorite(item); ok {
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
			color := t.TextMuted
			if favorite || star.Hovered() {
				color = t.Text
			}
			star.Children(func() {
				if glyph, ok := Glyph("star"); ok {
					ui.Icon(c, glyph).Size(t.Space(3.5), t.Space(3.5)).TextColor(color)
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
			if item.clip != nil && a.Actions.PinText != nil {
				a.rowControl(c, "text", copy.HistoryPin, func() {
					a.pinClip(*item.clip)
					removed = true
				})
			}
			// The delete is a two-step: the first press arms the row — the
			// muted "press again" note stands in for the trailing chrome, in
			// the one warm token the row may spend — and the second removes.
			// The armed state leaves with the selection, so a stray press
			// cannot remove a row the user was not looking at.
			if a.armedDelete == item.ID {
				armed := ui.Row(c).Shrink(0).AlignItems(ui.Center).Padding(0, t.Space(1))
				armed.Children(func() {
					ui.Text(c, copy.DeleteConfirm).FontSize(t.FontSize - 1).
						FontWeight(560).TextColor(t.Warning)
				})
				del := ui.Row(c).Focusable().Shrink(0).Size(t.Space(5), t.Space(5)).
					AlignItems(ui.Center).Justify(ui.Center).
					Label(copy.HistoryDelete).Tooltip(copy.HistoryDelete)
				del.Children(func() {
					if glyph, ok := Glyph("close"); ok {
						ui.Icon(c, glyph).Size(t.Space(3.5), t.Space(3.5)).TextColor(t.Warning)
					}
				})
				if del.Clicked() {
					a.armedDelete = ""
					removed = true
					a.deleteHistoryRow(item.ID)
				}
			} else {
				a.rowControl(c, "close", copy.HistoryDelete, func() {
					a.armedDelete = item.ID
				})
			}
		}
		if number, ok := a.numbers[i]; ok {
			ui.Text(c, a.resultNumber(number)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		if item.Shortcut != "" {
			ui.Text(c, item.Shortcut).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
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
	a.armedDelete = ""
}

// nextSelectable moves one step and lands on a row the keyboard can act on: a
// block's heading line is a label, never a choice, so the arrows step over it.
func (a *App) nextSelectable(from, delta int) int {
	results := a.Results()
	n := len(results)
	if n == 0 {
		return 0
	}
	index := from
	for i := 0; i < n; i++ {
		index = NextIndex(index, delta, n)
		if !results[index].heading {
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
		if !item.heading {
			return i
		}
	}
	return 0
}

// clampSelection keeps the choice on a row the keyboard can act on: a heading
// the list shrank onto is stepped past.
func (a *App) clampSelection(n int) {
	a.Selected = ClampIndex(a.Selected, n)
	results := a.Results()
	if a.Selected < len(results) && results[a.Selected].heading {
		a.Selected = a.nextSelectable(a.Selected, 1)
		if a.Selected >= len(results) {
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
	tokens := theme.For(a.settings(), t.Dark)
	// The old build's own row: a 30u band, the panel's 12u inset, the body
	// step at 560 (600 for a warning), and the alert glyph at 16u. A warning
	// is the stronger face — a border in the warning colour and a heavier
	// weight — so it reads as a blocking message rather than a status hint.
	tone := t.Warning
	weight := 560
	background := t.Warning.Alpha(0.12)
	feedback := ui.Row(c).Absolute().Bottom(0).Left(t.Space(1)).Right(t.Space(1)).Height(t.Space(7.5)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).
		Radius(tokens.RadiusSM).Background(background).
		// The row enters as the old build's `feedback-enter` did (its
		// --dur-3, 160ms).
		Transition(ui.ElementTransition{
			Enter:    &ui.Motion{Y: -2},
			Duration: 160 * time.Millisecond,
		})
	if a.toastWarning {
		background, weight = t.Warning.Alpha(0.2), 600
		feedback.Border(1, tone)
	}
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
// down or up. The row itself is part of Results (see otherResults), so the
// window's height and the row's own presence are one decision.
//
// The two edges are the reference's own (src/App.tsx R64, the heldBareTerminal
// effect): the press edge puts the row — the list's last line — in the
// selection; the release edge resets the selection to the empty page's own
// default, which is its first runnable row, unconditionally — the effect's
// dependency list carries no selection, so an arrow step off the row during
// the gesture is discarded on release, exactly as the reference's is. There is
// no delay: the row and the selection move on the same frame the modifier's
// state lands.
func (a *App) syncHeldModifier(c *ui.Context) {
	down := c.Modifiers()&ui.Cmd == ui.Cmd
	// "Was the row showing" has to be asked of the frame before: once
	// modifierHeld is down the row is already gone. The row's own predicate
	// reads modifierHeld, so the state lands before the edge is handled.
	rowShowing := a.heldRowShowing()
	// The row's own predicate reads modifierHeld, so the state lands before
	// the edge is handled.
	a.modifierHeld = down
	if down != a.heldWasDown {
		a.heldWasDown = down
		if down {
			if _, ok := a.heldTerminalItem(); ok {
				a.Selected = len(a.Results()) - 1
			}
		} else if rowShowing {
			// The row the modifier put on screen is gone; the selection goes to
			// the empty page's own default (the reference's
			// `firstRunnableResultIndex`), the first row the keyboard can act on
			// — a block's heading is never a choice. The reset is
			// unconditional: the reference's effect reads no selection, so the
			// row an arrow step moved to during the gesture goes with it.
			a.Selected = a.firstSelectable()
		}
	}
}

// heldRowShowing reports whether the held row is in the list right now: it is
// the one row whose presence rides the modifier, and the one selection the
// scroll skip and the release edge are written against.
func (a *App) heldRowShowing() bool {
	results := a.Results()
	return len(results) > 0 && results[len(results)-1].ID == bareTerminalRowID
}

// heldTerminalItem is the row the held modifier offers: a bare terminal
// session, on the ordinary page with an empty field and nothing typed. A query
// is a search, and a search has its own answers; a mode owns its own list.
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
		// The chord that put the row on screen is the chord that opens it, so
		// the hint is the action bar's own.
		Shortcut: a.enterBadge(),
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
func (a *App) fieldButtons(c *ui.Context, copy i18n.Launcher) {
	t := c.Theme()
	// The field's own buttons are the reference's `.collapsed-card__settings`:
	// a 28u square on the row, a 9u radius, its mark in the tertiary colour,
	// and a hover that lights the mark one step - with the 8u margin the old
	// row put between the buttons. Each one is a glyph with no visible word, so
	// it carries the sentence the old build's `title=` did.
	button := func(glyphName, label, hint string, run func()) {
		if run == nil {
			return
		}
		tokens := theme.For(a.settings(), t.Dark)
		e := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
			Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
			Label(label).Tooltip(hint).Margin(0, 0, 0, t.Space(2))
		if e.Hovered() {
			e.Background(t.SurfaceHover)
		}
		e.Children(func() {
			if glyph, ok := Glyph(glyphName); ok {
				ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
			}
		})
		if e.Clicked() {
			run()
		}
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
	tokens := theme.For(a.settings(), t.Dark)
	open := a.configOpen() && a.config.Schema.Plugin == id
	label, glyphName := copy.ConfigurePlugin, "settings"
	hint := copy.ConfigurePluginHint
	if open {
		label, glyphName = copy.ConfigurePluginClose, "close"
		hint = copy.ConfigurePluginCloseHint
	}
	e := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
		Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Tooltip(hint).Margin(0, 0, 0, t.Space(2))
	if e.Hovered() {
		e.Background(t.SurfaceHover)
	}
	e.Children(func() {
		if glyph, ok := Glyph(glyphName); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
		}
	})
	if e.Clicked() {
		a.toggleConfig(id)
	}
}
