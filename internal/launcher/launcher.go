package launcher

import (
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calculator"
	"floter/internal/clipboard"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/tools"
)

// The launcher's window geometry: a fixed width, and a collapsed height that
// is a scale-1 measurement multiplied by the interface-size factor.
const (
	// InputWindowWidth is the launcher's fixed width, in DIPs.
	InputWindowWidth = 720

	// The collapsed launcher's height at scale 1, per platform: 78 on Linux
	// (a 58px card in the shell's 10px gutters), 72 on Windows (a 56px card
	// in the 4px + 12px gutters) and 58 elsewhere (the card alone).
	inputWindowHeightLinux   = 78
	inputWindowHeightWindows = 72
	inputWindowHeightDefault = 58

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
	// Complete asks the provider for completions of the tokens typed so
	// far; the shell answers on the main thread. Nil disables dynamic
	// completion, and the static argument list stands alone.
	Complete func(entry extensions.CommandEntry, tokens []string, done func([]extensions.Completion))
	// PinText shows some text in a window of its own.
	PinText func(title, text string)
	// CopyClip puts a clipboard entry back on the clipboard, whatever its
	// kind. Nil falls back to Copy for text.
	CopyClip func(entry clipboard.Entry)
	// OpenURL opens a page in the browser the result came from, falling back
	// to the system's default browser for an empty id.
	OpenURL func(browserID, url string)
	// ActivateTab brings one of the browser's live tabs to the front.
	ActivateTab func(tab browser.Tab)
	// RunInTerminal runs a command in the terminal surface: a tool found on
	// the PATH.
	RunInTerminal func(argv []string)
	// RunInTerminalWithEnv runs a command with extra environment variables
	// (the leading NAME=value assignments the user typed), when the shell
	// action's parse found some.
	RunInTerminalWithEnv func(argv []string, env []string)
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

	// RowLines is the text lines of each visible row, as the last View built
	// them — two for a row with a subtitle, one without — and Font and
	// Spacing the theme's own values. The shell reads them to size the
	// window to the list it holds (see Geometry).
	RowLines []float64
	Font     float64
	Spacing  float64
	// HeldRows is the row count the window is sized for, with the shrink
	// hysteresis applied: a keystroke that removes one row of two does not
	// walk the window edge for it, and a growth is immediate. Zero until
	// the first View measured the list.
	HeldRows int

	// toast is a message to show on the next frame, set by an action.
	toast string
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
// the field, and the external-terminal action opens a terminal window.
// ⌘W is handled by the shell's own appShortcuts, on every surface — this
// one is the launcher's own modes only.
func (a *App) appShortcuts(c *ui.Context) bool {
	m := a.settings()
	for _, action := range []string{settings.ShortcutNewCommand, settings.ShortcutOpenExternalTerminal} {
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
		}
		return true
	}
	return false
}

// rowTransition is how a result row appears: a short fade, no movement, so
// typing feels alive without the list jumping about.
var rowTransition = ui.ElementTransition{
	Duration: 120 * time.Millisecond,
	Colors:   false,
	Enter:    &ui.Motion{Opacity: 0},
}

// resultNumber is the badge a row's result shortcut shows.
func resultNumber(number int) string { return strconv.Itoa(number) }

// Feedback shows a line under the list, for a caller outside the launcher
// (the shell's power action, when the system refused it).
func (a *App) Feedback(message string) {
	if message == "" {
		return
	}
	a.toast = message
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
func (a *App) SetCommands(found []extensions.CommandEntry) { a.Commands = found }

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
	a.askBrowser()
	a.modeShortcuts(c)
	results := a.Results()
	a.clampSelection(len(results))
	a.assignNumbers(results)

	// The window's height is the content's own: measure every row's lines
	// off the theme, so the shell can size the window to the list it holds.
	a.Font = float64(t.FontSize)
	a.Spacing = float64(t.Spacing)
	rowLines := make([]float64, len(results))
	for i, item := range results {
		if item.Detail != "" {
			rowLines[i] = 2
		} else {
			rowLines[i] = 1
		}
	}
	a.RowLines = rowLines
	// The window follows the list: grow as soon as a row arrives, and wait
	// for a row of margin before walking the edge back down (the old
	// launcher's hysteresis). The cap is the work area less the room the
	// other windows keep.
	a.HeldRows = HoldRows(a.HeldRows, len(results))
	if a.Actions.ResizeTo != nil {
		height := Geometry{
			Font: a.Font, Spacing: a.Spacing, RowLines: rowLines, Held: a.HeldRows,
			Filter: a.filtersVisible(),
		}.Height()
		a.Actions.ResizeTo(height)
	}

	// The field row floats over the list; the list's top padding is the
	// row plus the edge, so a row scrolls under the field rather than to
	// its edge.
	fieldRow := t.Space(7)
	edge := fieldRow + t.Space(2)
	// A plugin list mode carries its filter chips under the field: the list
	// starts below them, and the window's height charges their band too.
	chips := a.filtersHeight(c)
	listEdge := edge + chips

	ui.Box(c).Fill().Children(func() {
		if a.output != nil {
			a.outputBody(c, copy, edge)
			return
		}
		if len(results) == 0 {
			ui.Column(c).FillWidth().Padding(t.Space(3)).Center().Children(func() {
				message := copy.NoResults
				switch {
				case a.browser && !a.browserResults(a.browserQuery()).Found:
					// The browser mode with no profile at all says why rather
					// than claiming the query matched nothing.
					message = copy.BrowserNoProfile
				case a.mode != nil || a.clipboard || a.browser:
					message = copy.CommandModeHint
				}
				ui.Text(c, message).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
		} else {
			// The list builds only the rows in view, so a catalog of
			// hundreds costs a frame like a catalog of three. The choice is
			// painted by the row itself (see row), not by the list's own
			// selection. Padding puts the first row below the field, and
			// scrolls with the content, so rows pass under the field.
			ui.List(c.Key("launcher.results"), &a.List, len(results), func(i int) {
				a.row(c, results[i], i)
			}).Fill().Padding(listEdge, 0, 0, 0).Label(copy.ResultsLabel)
		}

		// The chips row floats under the field, the list scrolling beneath it.
		a.filtersRow(c, fieldRow+t.Space(1))

		// The rows fade into the panel under the field. PassThrough lets
		// the pointer reach a row the strip covers.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(edge).PassThrough().
			Material(glass.ScrollEdge{Background: t.Background})

		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(fieldRow).
			AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			field := ui.TextInput(c.Key("launcher.search"), &a.Query).
				Bind(&a.Search).
				Label(copy.Label).
				Placeholder(copy.Placeholder).
				// The query is the card's display register: the old build's
				// `.collapsed-card__input` took `--text-display` (17px at
				// scale 1) against the rows' body text — one step above —
				// at weight 460.
				FontSize(t.FontSize + 3).
				FontWeight(460).
				Grow(1)
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
			case a.Query != "":
				if ui.Button(c, "✕").Label(copy.Clear).Clicked() {
					a.Query = ""
					a.Selected = 0
					a.FocusSearch()
				}
			default:
				ui.Text(c, copy.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
		})
	})

	// Keep the chosen row in view when the keyboard moved the choice (the
	// list leaves the scrolling to the app when it has no selection).
	if a.Selected != a.chosenRow {
		a.chosenRow = a.Selected
		a.List.ScrollIntoView(a.Selected)
	}

	// An action that asks for feedback (the calculator's copy) shows it
	// once, on the frame after it ran.
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}

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
		switch {
		case a.files:
			a.leaveFiles()
		case a.calculatorMode:
			a.leaveCalculator()
		case a.browser:
			a.leaveBrowser()
		case a.clipboard:
			a.leaveClipboard()
		case a.mode != nil:
			a.leaveCommand()
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
		case a.clipboard && item.clip != nil:
			a.pinClip(*item.clip)
		case a.browser && item.tab != nil:
			a.copyURL(item.tab.URL)
		case a.browser && item.web != nil:
			a.copyResult(*item.web)
		case a.mode == nil && !a.clipboard && item.entry != nil:
			a.enterCommand(*item.entry)
		}
	}
}

// The mode-local keys: the history's star and delete, which belong to the
// calculator and clipboard modes rather than to the launcher's own shortcut
// map. Both are the old build's keys — the app modifier plus D to star, plus
// Backspace to delete — and neither is rebindable, because a control the user
// never asked to rebind does not belong in the settings map.
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
		a.toast = StringsFor(a.settings().Language).ClipboardFavoriteFailed
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

// enterCommand starts typing an extension command's arguments: the field
// holds its argv, and the list offers the command's declared arguments.
func (a *App) enterCommand(entry extensions.CommandEntry) {
	copied := entry
	a.mode = &copied
	a.Query = entry.Command.ID
	if entry.Command.ID != "" {
		a.Query += " "
	}
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveCommand returns to the search.
func (a *App) leaveCommand() {
	a.mode = nil
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// syncTypedMode enters a built-in mode when the user types its trigger word
// followed by a space, as the old build did: the word names the plugin and
// everything after it is the needle, so `clip foo` is the clipboard mode
// without a trip through the command list. A bare word stays an ordinary
// query, which is what makes entering and leaving one keystroke.
func (a *App) syncTypedMode() {
	if a.mode != nil || a.clipboard || a.browser || a.calculatorMode || a.files {
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
		a.files = true
		a.Selected, a.chosenRow = 0, -1
	}
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

// syncCommandMode leaves the mode when the line no longer starts with the
// word that entered it, which is what deleting it does.
func (a *App) syncCommandMode() {
	if a.mode == nil && !a.clipboard && !a.browser && !a.calculatorMode && !a.files {
		return
	}
	word := firstWord(a.Query)
	keep := false
	switch {
	case a.mode != nil:
		keep = strings.EqualFold(word, a.mode.Command.ID)
	case a.browser:
		keep = wordIn(word, browserWords)
	case a.clipboard:
		keep = wordIn(word, clipboardWords)
	case a.calculatorMode:
		keep = wordIn(word, calculatorWords)
	case a.files:
		keep = wordIn(word, filesWords)
	}
	if !keep {
		a.leaveCommand()
		a.leaveClipboard()
		a.leaveBrowser()
		a.leaveCalculator()
		a.leaveFiles()
	}
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
	browserWords   = []string{"browser", "bookmarks", "bookmark", "history", "浏览器", "书签"}
)

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
	a.clipboardFilter = filterAll
	a.Query = query
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveClipboard returns to the search.
func (a *App) leaveClipboard() {
	if !a.clipboard {
		return
	}
	a.clipboard = false
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// clipboardQuery is what the user typed after the mode word.
func (a *App) clipboardQuery() string {
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return ""
	}
	return strings.Join(words[1:], " ")
}

// commandArgs is the typed argument words, without the command itself.
func (a *App) commandArgs() []string {
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return nil
	}
	return words[1:]
}

// commandTokens is what the completions are asked about: the argument words,
// with an empty last token while a new word is being typed (the field ends
// in a space), as a completion request needs.
func (a *App) commandTokens() []string {
	words := splitArgs(a.Query)
	tokens := []string{}
	if len(words) > 1 {
		tokens = append(tokens, words[1:]...)
	}
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
	// The row is keyed by the item, so a row that stays keeps its identity
	// (and its place) while the rows around it come and go; a row that
	// appears fades in rather than flashing into place.
	row := ui.Row(c).Key(item.ID).FillWidth().Focusable().
		Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Gap(t.Space(2)).
		Transition(rowTransition)
	// The action bar is the one row that is not a result: its selection is
	// the neutral raised pane the old build's settings used (`.launcher-
	// action-bar--selected`), not the accent tint a match takes, and the
	// ⌘↩ badge is what says it runs on Enter.
	isBar := strings.HasPrefix(item.ID, "bar:")
	if i == a.Selected {
		if isBar {
			row.Background(t.SurfaceHover)
		} else {
			// The old build's selection: a quiet accent tint, and the
			// emphasis is the row's own type — the title at 700 — rather
			// than a ring or a shadow. The tint places the row; the weight
			// announces it.
			row.Background(t.Accent.Alpha(0.085))
		}
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	// A history row's own delete control: the focused text field owns the
	// editing keys, so the delete the old build put on a key is a button here
	// — on the selected row only, so a stray click cannot remove a row the
	// user was not looking at.
	removed := false
	row.Children(func() {
		if item.thumb != nil {
			ui.Image(c, item.thumb).Size(t.Space(8), t.Space(6)).Radius(t.Radius)
		}
		if item.icon != nil {
			if bitmap := a.appIcon(*item.icon); bitmap != nil {
				ui.Image(c, bitmap).Size(t.Space(4), t.Space(4))
			}
		}
		ui.Column(c).Grow(1).Children(func() {
			title := ui.Text(c, item.Title).FontSize(t.FontSize).TextColor(t.Text)
			// The selected row's emphasis is its own type: the title at 700,
			// as `.launcher-result--selected .launcher-result__title` did.
			if i == a.Selected {
				title.FontWeight(700)
			}
			if item.Detail != "" {
				subtitle := ui.Text(c, item.Detail).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				if i == a.Selected {
					subtitle.TextColor(t.Text)
				}
			}
		})
		if i == a.Selected && (item.clip != nil || item.calc != nil) {
			if ui.Button(c, "✕").Label(copy.HistoryDelete).Clicked() {
				removed = true
				a.deleteSelectedHistory()
			}
		}
		if number, ok := a.numbers[i]; ok {
			ui.Text(c, resultNumber(number)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
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

// deleteSelectedHistory removes the chosen history row — a clipboard entry or
// a calculation — and keeps the selection on the first row.
func (a *App) deleteSelectedHistory() {
	results := a.Results()
	if a.Selected < 0 || a.Selected >= len(results) {
		return
	}
	copy := StringsFor(a.settings().Language)
	item := results[a.Selected]
	switch {
	case item.clip != nil && a.Clipboard != nil:
		if err := a.Clipboard.Remove(item.clip.ID); err != nil {
			a.toast = copy.ClipboardDeleteFailed
			return
		}
		a.toast = copy.ClipboardDeleted
	case item.calc != nil && a.Calculator != nil:
		if err := a.Calculator.Delete(item.calc.ID); err != nil {
			a.toast = copy.CalculatorDeleteFailed
			return
		}
		a.toast = copy.CalculatorDeleted
		a.refreshCalculator()
	default:
		return
	}
	a.Selected, a.chosenRow = 0, -1
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

// enterBrowserWord opens the browser mode with the field's text.
func (a *App) enterBrowserWord(query string) {
	a.mode = nil
	a.clipboard = false
	a.calculatorMode = false
	a.browserFilter = filterAll
	a.browser = true
	a.Query = query
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// browserQuery is what the user typed after the mode word.
func (a *App) browserQuery() string {
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return ""
	}
	return strings.Join(words[1:], " ")
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
	a.runCommandWith(entry, args)
}

// move moves the selection by delta, wrapping around the list.
func (a *App) move(delta, n int) {
	a.Selected = NextIndex(a.Selected, delta, n)
}

func (a *App) clampSelection(n int) {
	a.Selected = ClampIndex(a.Selected, n)
}
