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
	"floter/internal/deadline"
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

	// The frame's memo of the settings snapshot and of the theme tokens built
	// from it (see `settings` and `tokens`). Both were rebuilt per row before,
	// which is where most of a frame's allocations went. The snapshot half is
	// `settings.Memo` (P11-co): the same version-keyed cache the settings and
	// terminal surfaces read, rather than a third hand-written copy of it.
	settingsMemo     settings.Memo
	tokensMemo       theme.Memo
	shortcutsVersion uint64
	shortcutsCached  map[string]string
	shortcutsValid   bool
	// badges memoizes the key badges the rows wear (see `badge`).
	badges map[badgeKey]string

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
	// pointerChose records that the change to Selected this frame came from
	// the pointer — an entry into a row, a click on it, or its star — rather
	// than from the keyboard. A row the pointer chose is the row *under the
	// pointer*, so it is in view by construction: the reveal below skips it.
	// Revealing it is not merely redundant — the framework reads a
	// ScrollIntoView as "the app has taken the offset over" and drops a wheel
	// notch already on its way (`mygo`'s `glideWheels`, `s.list.scrolled`), so
	// a pointer resting on the list turned every notch into a one-row jump
	// (P11-ah).
	pointerChose bool
	// hoverFresh says the pointer's position is the user's own: the pointer
	// moved, or the user scrolled the list with it, since the app last moved
	// the content under it. R21's entry edge is read against this one.
	//
	// The row-based edge above is not enough on its own: a row *changes under a
	// still pointer* whenever the list scrolls, and the keyboard makes it
	// scroll on every step past the window's last row. The port read that as an
	// entry, so the row that slid under the resting pointer took the choice
	// back — fifteen arrow steps advanced three rows in the frame that pinned
	// this. The reference has no such reading to copy: `onPointerEnter` is a
	// pointer event, and the launcher's own window appearing under the cursor
	// does not fire one either, so a *still* pointer is never an entry (P11-ai).
	hoverFresh bool
	// pointerAt is where the pointer was over the panel in the pass before,
	// pointerOver whether it was there at all and pointerSeen whether a pass has
	// read it yet; the three are how "the pointer moved" is answered without a
	// pointer-event hook. listFirst is the list's first visible row in the pass
	// before (listFirstSeen: has it been read), so a pass can tell that the
	// content moved under the pointer. appScrolled is armed by the keyboard's
	// own reveal and cleared once the list settles: a content move while it
	// stands is the app's doing, not the user's.
	pointerAt     [2]float32
	pointerOver   bool
	pointerSeen   bool
	listFirst     int
	listFirstSeen bool
	appScrolled   bool
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

	// modifierHeld is the app modifier's state as the last frame saw it. The
	// held modifier's own line is the panel's **pinned bottom band** (see
	// heldHintRow), never a row of the list: the user's own words are
	// 「按住 Super 显示进入终端选项要固定底部且不重复」 and 「不属于任何列表
	// 项…不占排序」 (F4-3), so nothing here is appended to Results().
	modifierHeld bool
	// heldWasDown is the modifier's state the frame before, so the press and
	// release edges are one transition each.
	heldWasDown bool
	// barRowKey is the list shape the action bar's selection was last asserted
	// for: the row count and the query, joined. It is the row the modifier
	// hands the selection to when the field has text (a typed query draws no
	// terminal line of its own), and the reference's effect re-runs on `query`
	// and `displayedResultsLength` (`main:src/App.tsx:2065-2082`), so typing
	// with the modifier down keeps the bar lit while an arrow step off it
	// survives.
	barRowKey string
	// searchRuns counts the frame's own catalog work (see Results). It exists
	// so a test can pin that a frame searches **once**: the launcher used to
	// walk the catalog twice per frame — once for the held row's presence, once
	// for the list — and a duplicated full match is the cheapest way to read a
	// keystroke as a stall.
	searchRuns int

	// drawn is the row slice the current draw built: **this frame's table**. Every
	// action that fires while the draw is running — or that acts on "the row the
	// user is looking at" — reads it instead of walking the catalog again for
	// the same frame. A second full match per keystroke was the launcher's
	// quietest cost (see App.searchRuns) and it bought nothing: the list a
	// re-walk returns is the list this draw already has.
	//
	// P11-cl brought the pointer's click onto it; P11-cn brought the rest of
	// the family: the arrow keys (`move`), the history star's chord
	// (`modeShortcuts`) and the star's own click (`toggleFavorite`). The rule is
	// the one `nextSelectableIn` states: the draw clamps its choice against the
	// slice it built.
	drawn []Item
	// catalog is the built catalog and the shape of the inputs it was built
	// from (see Catalog): the rows and their lowered search registers are made
	// once per rescan, new command set or language, not once per keystroke.
	catalog         []Item
	catalogShapeWas catalogShape
	catalogBuilt    bool
	catalogBuilds   int
	// catalogRev is the revision the setters bump: the inputs that have no
	// length of their own to compare (a rescan that returns the same number of
	// applications, an alias the user changed) still have to rebuild the rows.
	catalogRev int

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
	// rowRects is each row's box as the last frame laid it out, and listRect
	// the results list's own box (the viewport the rows are clipped to). The
	// numbered family asks them how much of a row is on screen, because the
	// framework's Visible() answers "a pixel" and a badge on a row that is
	// nine tenths scrolled away is a badge nobody sees (P11-bb). Filled while
	// the rows are built (`Bounds` is the previous frame's rectangle) and
	// cleared as the list is built, so a row that left the list leaves no
	// stale box behind.
	rowRects map[int]ui.Rect
	listRect ui.Rect
	// headingRects is the same measurement for the group headings the list
	// prints above its blocks (see `row`'s `item.heading` arm). They are
	// **siblings** of the rows, not rows, so the fold never reached them and a
	// heading cut by the fold kept its full ink (P13-x: the launcher's label
	// measured 145/133 light/dark with 8 of its 22u showing). This table feeds
	// `foldAlpha` **only**: the numbered family keeps reading `rowRects`, so
	// adding a heading here can never move a badge or a digit chord.
	headingRects map[int]ui.Rect
	// outputNumbers is the same family for the captured-output list: the
	// numbered shortcuts address the output's own rows while it owns the band
	// (see assignOutputNumbers).
	outputNumbers map[int]int
	// outputRowRects and outputListRect are the same pair as rowRects and
	// listRect, for the captured-output list: its rows' boxes as the last frame
	// laid them out and the list's own clip. Both numbered families ask the same
	// question (viewportRange), so the output's badges follow what is on screen
	// exactly as the results' do (P11-bl). Filled while the rows are built and
	// cleared as the list is built, so a row that left leaves no stale box.
	outputRowRects map[int]ui.Rect
	outputListRect ui.Rect
	// outputHeadingRects is `headingRects` for the captured-output list's group
	// headings (`outputRow`'s `row.Group` arm). Same rule, same pure function,
	// and the same exclusion: it feeds `foldAlpha` only, never
	// `outputNumberedViewport` (P13-x).
	outputHeadingRects map[int]ui.Rect
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
	// place of the result list; Output keeps its scroll offset for as long as
	// that one view is on screen (P11-ar: `showOutput` resets it when a new
	// capture replaces it, so a second output starts at its own first line
	// rather than at the first one's offset).
	output *OutputView
	Output ui.ScrollState
	// footerSpin is when the output footer's paging ring was first drawn: the
	// start of its turn (P11-bp). It lives on the app rather than in the
	// element's state because the angle is read while the frame paints
	// (`Painter.Draw`), not while the view is built, so it has to survive
	// builds the way the drawing does. Zero until a frame draws the ring; see
	// footerSpinner.
	footerSpin time.Time
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
// The viewport is read for **every** list, not only one past the ten-slot
// budget. The old build gated it on `listOverflowsBudget` (R72) because a list
// that fits cannot scroll; here it can. The window charges the field's floating
// band (60u) above the results, so a ten-row list — exactly the budget — scrolls
// under the field, and a query that also drew an action-bar row left the old
// `resultCount` at ten while the list held eleven rows. The gate then skipped
// the viewport and the badges stayed pinned to the top, `1` on the row behind
// the field (P11-bi, the user's re-report of the P11-bb family: 「超出屏幕的项也
// 显示快捷键」). Measuring a list that fits is free: every row is on screen, so
// the range is the rows the old path numbered, and the loop below still caps
// the family at ten. When the frame left no geometry (the first pass, before a
// row has been laid out) the range falls back to the whole list, which keeps a
// mid-walk measurement from leaving a list with one badge and the rest empty
// (the failure the old build's gate was written against). The viewport is the
// previous frame's, exactly as the old build's key handler read the last
// render's slots, so a scroll renumbers the family on the next frame.
func (a *App) assignNumbers(results []Item) map[int]int {
	// The family is at most ten entries and it is rebuilt every frame, so the
	// map is the frame's own working set rather than a fresh allocation per
	// keystroke: it is cleared and refilled, which keeps the frame's object
	// count (and therefore the GC work behind 「界面经常卡死」) flat. Nothing
	// outside the frame holds the returned map — the readers are the row's own
	// badge and the digit chord, both of them inside this frame — so clearing
	// it cannot be observed as a stale read.
	numbers := a.numbers
	if numbers == nil {
		numbers = make(map[int]int, maxResultSlots)
	} else {
		clear(numbers)
	}
	// The family follows the viewport whenever the last frame left geometry to
	// read; without geometry it falls back to the whole list, and the loop's
	// `count` cap still bounds it at ten (see the function's paragraph).
	first, last := 0, len(results)-1
	if visibleFirst, visibleLast, ok := a.numberedViewport(len(results)); ok {
		first, last = visibleFirst, visibleLast
	}
	count := 0
	for i, item := range results {
		if item.Run == nil || strings.HasPrefix(item.ID, "bar:") {
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

// numberVisibleShare is how much of a row has to be on screen for the numbered
// family to land on it: seven tenths, so a row scrolled out of the viewport by
// three tenths or more takes no number (P11-bb, the user's own threshold:
// 「某一项滚出可见范围达到 30% 就算作不可见项」). The slack is what makes
// scrolling read as motion rather than as the badges blinking off and on: a row
// keeps its number while it is mostly there and loses it as it leaves.
//
// The reference draws the same line the strict way — `visibleRowRange`
// (`main:src/launcher/result-budget.ts:768-810`) skips any row its top edge
// clips — because it measures in the DOM on the frame it paints. This port reads
// the last frame's boxes, so it takes the threshold instead of the pixel: a
// number is worth having only while the row is one the user can read.
const numberVisibleShare = 0.7

// numberedViewport reports the rows the numbered family may reach, from the
// boxes the last frame laid out: the first row at least `numberVisibleShare` of
// which lies inside the results list's own box, down to the last row with any
// part of it there. ok is false when the frame left no geometry to read (see
// assignNumbers).
//
// The two ends are read differently on purpose, and the reference reads them the
// same way (`visibleRowRange`, `main:src/launcher/result-budget.ts:789-810`):
// `start` skips a row the top edge clips, `end` keeps the row the bottom edge
// only partly shows. A row leaving through the top has already been read and its
// number would land on the row that took its place — the badge the user
// reported as jumping. A row arriving from the bottom is the one Enter and the
// digits should still reach: the family's tenth key (`0`) is on it.
//
// The range is contiguous because a list lays its rows out in one column.
func (a *App) numberedViewport(count int) (first, last int, ok bool) {
	return viewportRange(count, a.rowRects, a.listRect)
}

// viewportRange is the one answer to "which rows of this list may carry a
// number", asked with a list's row boxes and its viewport: the search list's
// own (numberedViewport) and the captured-output list's
// (outputNumberedViewport, P11-bl). The rule lives here only, so the two
// surfaces cannot drift into two answers the way Visible() and a 70% threshold
// would — the reference's `visibleRowRange`
// (`main:src/launcher/result-budget.ts:768-810`) is likewise one function over
// both its lists.
func viewportRange(count int, rects map[int]ui.Rect, viewport ui.Rect) (first, last int, ok bool) {
	if viewport.W <= 0 || viewport.H <= 0 || len(rects) == 0 {
		return 0, 0, false
	}
	first, last = -1, -1
	for i := 0; i < count; i++ {
		box, seen := rects[i]
		if !seen || box.W <= 0 || box.H <= 0 {
			continue
		}
		share := visibleShare(box, viewport)
		if share <= 0 {
			continue
		}
		if first < 0 && share >= numberVisibleShare {
			first = i
		}
		last = i
	}
	if first < 0 || last < first {
		return 0, 0, false
	}
	return first, last, true
}

// visibleShare is how much of box lies inside viewport, as a fraction of the
// box's own area: 1 for a row wholly on screen, 0 for one scrolled away.
func visibleShare(box, viewport ui.Rect) float64 {
	w := min(box.X+box.W, viewport.X+viewport.W) - max(box.X, viewport.X)
	h := min(box.Y+box.H, viewport.Y+viewport.H) - max(box.Y, viewport.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(w*h) / float64(box.W*box.H)
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
		// A dropped-file page with no drop on it is not a search that came
		// back empty: nothing was asked. It says so, the way the clipboard
		// and the calculator pages do (P11-bh). A typed needle that matches
		// none of the dropped files is a real miss and keeps NoResults.
		if len(a.Dropped) == 0 {
			return copy.FilesEmpty
		}
		return copy.NoResults
	default:
		// P11-cd：空查询 + 一条最近启动都没有（首次启动、或最近记录被清空）**不是**
		// 「搜索没有命中」——用户什么都没问。字段自己的占位句已经写着「输入命令或
		// 应用名称」，页面再印一句「没有匹配项」是**否认一次没发生的搜索**（P11-bk
		// 的注释以为自己让这一页留空，实际上 `len(results)==0` 落进了这条默认分支，
		// 于是全新的 floter 一打开就报「没有匹配项」）。同 P11-bh 给「没有拖入文件」
		// 另立一句话的规则：没有 needle 的空页不借 NoResults，它只留字段。
		if strings.TrimSpace(a.Query) == "" {
			return ""
		}
		return copy.NoResults
	}
}

// foldAlpha is how much of a row the list actually shows: 1 for a row wholly
// inside the viewport, the visible fraction for one the fold cuts, 0 for one
// scrolled away.
//
// P13-v · 折边之外的东西是**退后**，不是**被切**。列表没有滚动条（用户定的），
// 所以「下面还有」只能靠折边说话；而折边今天是把行硬切——视口下沿落在行的中间，
// 一行字就被水平切成两半（实测 `Walk 09` 的文字盒 24 → 15 px，墨在下沿一刀断掉，
// 见 `docs/rounds/P13-v-report.md`）。那不是「还有更多」，那是「界面坏了」。
// 轻便液态玻璃的深度语汇里，越过折边的那一行**退后**：露出一半的行画一半的墨，
// 露出一成的行几乎不见——折边于是自己就是「还剩多少」的刻度，不需要滚动条，
// 也不需要拿背景色回放的渐变遮罩（那是冒充玻璃，§0 禁）。
//
// 两个盒子都是**上一帧**的（`a.listRect` 与 `a.rowRects[i]`），所以它们互相
// 自洽；首帧还没有盒子，返回 1（一行都不退），画面不会先空一帧。
func foldAlpha(viewport, box ui.Rect) float64 {
	if viewport.H <= 0 || box.H <= 0 {
		return 1
	}
	top := box.Y
	if viewport.Y > top {
		top = viewport.Y
	}
	bottom := box.Y + box.H
	if fold := viewport.Y + viewport.H; fold < bottom {
		bottom = fold
	}
	visible := bottom - top
	if visible >= box.H {
		return 1
	}
	if visible <= 0 {
		return 0
	}
	return float64(visible) / float64(box.H)
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
	return a.badge(a.shortcut(settings.ShortcutSelectResult), strconv.Itoa(number))
}

// badge is shortcuts.Badge over a memo. Badge parses the accelerator (a split
// and a builder) on every call, and a list of ten numbered rows asks for ten
// badges every frame — the same handful of strings, rebuilt. The badge is a
// pure function of the binding and the key, and the binding rides in the key,
// so a settings change is a new entry rather than a stale hit.
func (a *App) badge(accelerator, key string) string {
	k := badgeKey{accelerator: accelerator, key: key}
	if v, ok := a.badges[k]; ok {
		return v
	}
	v := shortcuts.Badge(accelerator, key)
	if a.badges == nil {
		a.badges = make(map[badgeKey]string, 12)
	}
	a.badges[k] = v
	return v
}

// badgeKey is the pair a badge is a pure function of; a struct rather than a
// joined string, so asking for a badge a frame already drew allocates nothing.
type badgeKey struct{ accelerator, key string }

// rowActionText is a row's trailing *text* slot — a command row's `Tab`
// affordance and the armed-delete note. A key is not text: it goes through
// `rowKeycap`, which draws the chord as the keycap plates
// (`docs/raycast-design.md` §4.5). The reference's own `.launcher-result__action`
// register is the body step at weight 620, `--text-tertiary` (lifted to
// `--text-primary` on the selected row), with a fixed 30u minimum and a
// right-aligned box so the slots of a column line up.
func (a *App) rowActionText(c *ui.Context, text string, selected bool) {
	tokens := a.tokens(c)
	t := c.Theme()
	color := tokens.TextTertiary
	if selected {
		color = t.Text
	}
	ui.Text(c, text).FontSize(tokens.Body).FontWeight(620).TextColor(color).
		MinWidth(t.Space(7.5)).TextAlign(ui.End).Shrink(0)
}

// rowKeycap is a row's trailing *key* slot: the chord drawn as Raycast draws it
// — one 20x20 keycap per key, 2u apart, no `+` between them, in the tertiary
// ink (`docs/raycast-design.md` §4.5). `⌘1` on macOS is two plates; `Ctrl+1`
// elsewhere is two as well, because the words the badge spells are the same two
// keys.
func (a *App) rowKeycap(c *ui.Context, tokens theme.Tokens, badge string) ui.Element {
	keys := keycapLabels(badge)
	row := ui.Row(c).Shrink(0).Gap(c.Theme().Space(keycapGapUnits)).AlignItems(ui.Center)
	row.Children(func() {
		for _, label := range keys {
			keycap(c, tokens, c.Theme(), label)
		}
	})
	return row
}

// barHintText is the action bar's own key hint. It is the same keycap the rows
// draw — the bar's chord is a key, not a label — one step quieter in the
// reference's `.launcher-action-bar__hint` register (`.launcher-action-bar__hint`,
// `launcher.css:1608-1623`).
func (a *App) barHintText(c *ui.Context, text string) {
	a.rowKeycap(c, a.tokens(c), text)
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
func (a *App) SetApps(found []apps.App) {
	a.Apps = found
	a.catalogRev++
}

// SetCommands replaces the extensions' commands.
func (a *App) SetCommands(found []extensions.CommandEntry) {
	a.Commands = found
	a.catalogRev++
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
	a.catalogRev++
}

// SetCommandAliases replaces the user's alias per command.
func (a *App) SetCommandAliases(aliases settings.CommandAliases) {
	a.Aliases = aliases
	a.catalogRev++
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
//
// The frame reads the settings from every row it draws (the row's tokens, its
// glyphs, the bar's faces). Snapshot copies the whole value — including the
// map of keys this build does not own — so asking it once per row was the
// single largest allocation source of a frame (28% of the objects in
// BenchmarkLauncherFrame). The memo is keyed by the store's version, which is
// exact (see `settings.Memo`), so it can never serve a value the store no
// longer holds.
func (a *App) settings() settings.Settings {
	return a.settingsMemo.Get(a.Store)
}

// tokens is the frame's theme tokens. `theme.ForPreferences` builds a whole
// theme (a fresh ui.Theme plus the token ladder over it) on every call, and
// every row asked for its own copy — ten builds of the same value per frame.
// The row's tokens depend on the settings, the appearance and the desktop
// preferences only, so one memo answers the whole frame (and any later frame
// that nothing has changed for). The memo itself is shared with the settings
// surface (theme.Memo), so both surfaces answer this question one way.
func (a *App) tokens(c *ui.Context) theme.Tokens {
	t := c.Theme()
	var version uint64
	if a.Store != nil {
		version = a.Store.Version()
	}
	return a.tokensMemo.Tokens(version, t.Dark, c.Preferences(), a.settings)
}

// shortcut is settings.Shortcut over the frame's own resolved binding map.
// `ShortcutsOf` builds the whole map (a fresh map and its shipped bindings)
// on every call, and every numbered row asked for the select-result binding —
// one map per row per frame, which the profile showed as the largest single
// allocation source left. The bindings are a pure function of the settings
// value, so the memo is keyed by the store's version like the tokens are.
func (a *App) shortcut(action string) string {
	var version uint64
	if a.Store != nil {
		version = a.Store.Version()
	}
	if !a.shortcutsValid || a.shortcutsVersion != version {
		a.shortcutsCached = settings.ShortcutsOf(a.settings())
		a.shortcutsVersion, a.shortcutsValid = version, true
	}
	return a.shortcutsCached[action]
}

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
	// The held modifier's own line: the panel's pinned bottom band, not a row.
	// The chord that puts it on screen is the chord that opens it — the modifier
	// is still down when the user presses Enter, so the plain Enter's own path
	// (the field's submit) is not the one that fires. It holds on the empty
	// field and on a typed one alike: the band is drawn on both (F4-3).
	if a.heldHint() && c.Shortcut(ui.Cmd, ui.KeyEnter) {
		a.openBareTerminal()
	}
	// A query the field has not accounted for gets that query's own default
	// before any row reads the choice (see defaultedQuery).
	if a.Query != a.defaultedQuery {
		a.Selected, a.chosenRow = a.defaultSelection(results), -1
		a.defaultedQuery = a.Query
	}
	// The action bar's own claim on the selection is asserted last: on a typed
	// query the modifier hands the choice to the bar, and a keystroke that
	// rebuilds the list must not leave the bar drawn without the highlight (see
	// holdBarSelection).
	a.holdBarSelection(results)
	a.clampSelection(results)
	a.drawn = results
	a.assignNumbers(results)
	// The output list, while it owns the band, carries the same numbered family
	// over its own rows.
	a.assignOutputNumbers()

	// The window's height is the content's own: measure every row's lines
	// off the theme, so the shell can size the window to the list it holds.
	tk := a.tokens(c)
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
			Feedback: a.feedbackVisible(),
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

	// The panel is the launcher's reading surface: the old build's
	// `--surface-sunken`, one step denser than the card's own glass, so
	// the rows sit on a recess rather than on the frame (`.launcher-bottom`).
	// The field block above is opaque and paints over its top.
	//
	// It is also where the pass reads the pointer and the list's offset from:
	// both are needed before the first row is built, and both are answers about
	// *this* panel, so the element that encloses every row is the one asked
	// (`PointerPosition` is local to the element's box).
	panel := ui.Box(c).Fill().Background(tk.Sunken)
	panel.Children(func() {
		a.trackPointer(panel)
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
			// The empty state is Raycast's quiet page: the sentence in the
			// primary ink at the emphasis step, centred in the band, with no
			// plate, no accent and no mark of its own
			// (`docs/raycast-design.md` §4.7).
			//
			// P11-ag 重定基线：这块列此前从来没有拿到自己的 band。面板是**整张
			// 卡**的高度，而字段行是浮在它上面的绝对定位块（`launcher.fieldRow`，
			// 56u 不透明底），所以 `Center()` 把句子摆在卡片的**顶边**：实测句子
			// 矩形 y=12–30，字段带占 0–56，整句被字段的底盖住，一个像素都没渲染
			// 出来（改前那一帧除字段自己的字以外没有别的墨）。于是「一个查询什么
			// 都没匹配到」在界面上就是**一张空白卡**；插件模式里（剪贴板为空、
			// 浏览器没配置、计算器没有历史）更常见，因为这些页面既没有列表也没
			// 有页脚，整页只剩这句没人看得见的话。
			//
			// 现在它和列表读同一条带：上边距 `listEdge`（字段行 + 呼吸 + 模式的
			// chips），`Grow(1)` 撑满面板剩下的高度，句子于是居中在**列表会占据的
			// 那条带**里，而不是在卡片顶边。墨与字阶不动（Emphasis +
			// `TextStrong`，§4.7），只有位置换了。
			ui.Column(c).FillWidth().Grow(1).Center().
				Padding(listEdge, t.Space(3), t.Space(3), t.Space(3)).
				Children(func() {
					// P11-cd：一句话都没有的空页不是空态，是一个安静的空页（见
					// `emptyMessage`）：没有 needle、也没有最近启动时这里不画字，
					// 页面上只剩字段自己的占位句。
					if message := a.emptyMessage(copy); message != "" {
						ui.Text(c, message).FontSize(tk.Emphasis).
							TextColor(tk.TextStrong).SingleLine()
					}
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
			// P11-bb：行盒与视口盒都是**上一帧**的（`Bounds`），而编号在行被建出来之前
			// 就要发下去，所以这里先把上一帧的盒清掉，由本帧建出来的行重新填——
			// 已经离开列表的行于是不会留下一个过期的盒子去骗 `numberedViewport`。
			clear(a.rowRects)
			// P13-x：分组头的盒子与行盒同一时机清掉——两张表都只描述**本帧**建出来
			// 的东西，一个离开列表的标签不会留下过期盒子。
			clear(a.headingRects)
			// P11-bj：字段带占自己的位置，列表从带子下沿开始。字段行不再画自己的面
			// （它就是面板的第一条带，见下面 `launcher.fieldRow`），于是它不再是不透明
			// 的遮罩：一条滚上去的行会从查询文字底下透出来。把带子放进流里，滚动区的
			// 上沿就落在带子下沿 —— 行根本不进那条带，编号读的视口也还是原来那一段
			// （下面 `a.listRect` 直接读列表盒，不再手写扣减）。
			ui.Box(c).Shrink(0).FillWidth().Height(listEdge)
			list := ui.List(c.Key("launcher.results"), &a.List, barStart, func(i int) {
				a.row(c, results[i], i)
			}).Grow(1).FillWidth().MinHeight(0).
				// The list's own gutter is the row block's inset: Raycast's
				// selected row is a rounded pill that does *not* run to the card's
				// edge, so the block sits 8u in from it on both sides
				// (`docs/raycast-design.md` §4.3). The gutter and the row's own
				// padding are one sum — 8u + 8u is the 16u the row's icon column
				// has always started on — so the column moves by nothing while the
				// block's painted edge moves 4u in.
				// P11-az：底部内边距里含那条带（`statusBand`）。窗口为带子长了
				// `launcherStatusUnits`（`Geometry.Feedback`），列表 `Grow(1)` 也
				// 跟着长同样多——带子却是浮在列表上的，于是「多出来的那一截」
				// 正好是带子盖住的那一截：滚到底时列表的最后一行永远在带子底下
				// （看不见、点不到，实测 `india` 的 `y=428..443` 落在带子
				// `y≈410..440` 里）。带子在场时把这 30u 从列表的内容里让出来，
				// 列表的最后一行就停在带子上沿——和参考把 `.launcher-feedback`
				// 放在流里（`main:src/styles/launcher.css:693` 的 `min-height`）
				// 同一个答案。判据与 `Geometry.Feedback` 是同一个（见 `statusBand`）。
				Padding(0, t.Space(2), t.Space(0.5)+a.statusBand(c), t.Space(2)).Label(copy.ResultsLabel).
				ScrollbarInsets(0, 4096, 0, 0)
			// The rows are clipped to this box, so it is the viewport the
			// numbered family measures against (see numberedViewport). P11-bj：
			// 这个盒子的上沿已经是字段带（和模式的 chips）的下沿 —— 带子由上面那条
			// 占位让出来，不再靠手写扣减一段高度去躲开它。
			a.listRect = list.Bounds()
			a.actionBarFooter(c, results, barStart)
		}

		// The chips row floats under the field, the list scrolling beneath it.
		a.filtersRow(c, fieldRow+t.Space(1))

		// The rows fade into the panel under the field is gone with R31: the
		// reference deleted the launcher's scroll-edge band and kept only the
		// field block's own 1px hairline (below). The band darkened rows as
		// they passed under the field, which the user read as a shadow cast
		// by the input.

		// The field block, keyed so its identity does not depend on its *place*
		// among the panel's children: the framework names an element by its
		// parent's id and its sibling index unless it is given a Key, so moving
		// the floor rule behind this row (below) renumbered the row itself and
		// every focus and animation state under it was rebuilt from scratch —
		// the footer bar's own 120ms enter never started, and the selected pill
		// stayed un-painted on the frame the shell's pins read. The key makes the
		// row the same element before and after the rule moved.
		ui.Row(c.Key("launcher.fieldRow")).Absolute().Top(0).Left(0).Right(0).Height(fieldRow).
			Padding(0, t.Space(4)).
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
			// The leading slot holds one thing, in the rows' own icon column.
			//
			// The scope mark comes first: the mode that owns the field, its icon
			// and its muted word (`.collapsed-card__scope` / `__scope-name`).
			// The trigger word was consumed on entry, so this is the only place
			// the mode is named. It keeps its natural width — it is a word, and a
			// clipped word names nothing.
			//
			// With no owner the slot holds the reference's magnifier, at the very
			// geometry every result row's icon plate uses: a 28u cell
			// (`Space(7)`) and the row's 10u gap (`Space(2.5)`). So the query
			// starts on 16u + 28u + 10u = 54u, which is the rows' **title** column
			// (`TestTheFieldAndTheRowsShareTheReferenceTextColumn`): the field and
			// the list share one text column instead of two. The mark is muted ink
			// — the placeholder's own register — and spends no accent
			// (docs/raycast-design.md §4.2 / §5).
			if glyph, name, ok := a.scopeMark(copy); ok {
				mark := ui.Row(c).Shrink(0).Gap(t.Space(1.5)).AlignItems(ui.Center).
					Margin(0, t.Space(2.5), 0, 0)
				mark.Children(func() {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
					ui.Text(c, name).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
			} else if glyph, ok := Glyph("search"); ok {
				slot := ui.Box(c).Shrink(0).Size(t.Space(7), t.Space(7)).
					AlignItems(ui.Center).Justify(ui.Center).
					Margin(0, t.Space(2.5), 0, 0)
				slot.Children(func() {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
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
					tokens := a.tokens(c)
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

		// The field block's own floor: one *weak* hairline
		// (`docs/raycast-design.md` §4.2). Raycast's search row is not a
		// floating input box — it is the card's first band, and the line under
		// it is the same rule a block draws inside itself, not the card's own
		// 1px edge, so it takes the quieter rung.
		//
		// It is drawn **after** the field row, not before it: both are absolute
		// children of the same box, so the later sibling paints over the earlier
		// one. Built in front of the row (where it used to be), the row's own
		// `Top(0)..Height(fieldRow)` fill — y=0..fieldRow-1 — covered this line's
		// single row at y=fieldRow-1 outright, and the rule the contract promises
		// simply was not in the frame (the port's R31 「输入框下方只保留 1px」
		// never painted from the day it landed). Same position, later paint: the
		// line is the block's last pixel, which is what 「这块的下沿」 means, and
		// the height chain moves by nothing.
		ui.Box(c).Absolute().Top(fieldRow - 1).Left(0).Right(0).Height(1).PassThrough().
			Background(tk.HairlineSoft)
	})

	// Keep the chosen row in view when the keyboard moved the choice (the
	// list leaves the scrolling to the app when it has no selection).
	//
	// The pointer's choice is scrolled into view; a keyboard step too (the
	// pinned footer is never scrolled into view — it is not a row of the
	// scroller at all, it is the band below it; see the action-bar split).
	byPointer := a.pointerChose
	a.pointerChose = false
	if a.Selected != a.chosenRow {
		a.chosenRow = a.Selected
		if !byPointer {
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
				// The list moves for the keyboard, not for the pointer: what
				// slides under a pointer that did not move was not entered
				// (see App.hoverFresh).
				a.appScrolled = true
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
	// The row the user is looking at is the row this frame drew, so the star
	// reads the frame's own table rather than walking the catalog again for a
	// row that is already on screen (see App.drawn).
	item, ok := a.selectedHistoryRow()
	if !ok {
		return
	}
	switch {
	case a.calculatorMode:
		a.toggleCalculatorFavorite(item)
	case a.clipboard:
		if item.clip != nil {
			a.toggleClipFavorite(*item.clip)
		}
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
	tokens := a.tokens(c)
	control := ui.Row(c).Focusable().Shrink(0).Size(t.Space(5), t.Space(5)).
		Radius(tokens.RadiusXS).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Tooltip(label).
		Transition(ui.ElementTransition{Duration: theme.ColorStep, Colors: true})
	ink := tokens.TextTertiary
	switch {
	case control.Pressed():
		control.Background(tokens.ControlPress).Scale(0.92, 0.92)
		ink = tokens.TextStrong
	case control.Hovered():
		// P13-h · 悬停的 20u 小格是 E1 的一个面，不是四层透镜：环（`Lens` 的
		// 第四层）在这么小的格子上读成「带框的按钮」，正是 §2 退掉的画框分层；
		// 与 P13-d 的终端页悬停图标按钮同一语汇。
		tokens.LitFace(control.Background(tokens.ControlHover))
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
//
// The row hands its own `Item` over. The star's click fires *inside* the draw,
// where asking the catalog for "the selected row" a second time would be the
// draw's second full match (see App.drawn); the chord hands over the row of the
// table the frame drew. Either way the entry starred is the one the user is
// looking at, and no path re-walks the catalog.
func (a *App) toggleFavorite(item Item) {
	switch {
	case item.clip != nil:
		a.toggleClipFavorite(*item.clip)
	case item.calc != nil:
		a.toggleCalculatorFavorite(item)
	}
}

// toggleClipFavorite stars or unstars the chosen clipboard entry.
func (a *App) toggleClipFavorite(entry clipboard.Entry) {
	if a.Clipboard == nil {
		return
	}
	if err := a.Clipboard.SetFavorite(entry.ID, !entry.Favorite); err != nil {
		a.WarnFeedback(StringsFor(a.settings().Language).ClipboardFavoriteFailed)
	}
}

// selectedClip is the clipboard entry the selection is on, read from the table
// this frame drew: a row that is on screen is the row the user is looking at.
func (a *App) selectedClip() (clipboard.Entry, bool) {
	if a.Selected < 0 || a.Selected >= len(a.drawn) || a.drawn[a.Selected].clip == nil {
		return clipboard.Entry{}, false
	}
	return *a.drawn[a.Selected].clip, true
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
//
// P11-ce：补全要把光标**放到新行尾**。这一行是 App 自己写的（`a.Query`），
// 而输入框里的编辑器拿着自己的缓冲区与光标，外部写入的文本只会被夹到长度
// （mygo `ui/editor_history.go:59` 的 `ed.caret = min(ed.caret, ed.buf.n)`）
// ——补全后的行比光标所在的位置长，于是下一个字符落进被补全的词**中间**
// （实测补全 `-mode ` 后键入 `x` 得到 `-mxode `）。参考那边没有这个状态：
// 它的输入是 DOM `<input>`，App 写 value 时浏览器的光标回到末尾。补全后用户
// 就是要接着打，所以这里和 `SetQuery`/`ResetQuery`/历史的每一条路径一样置
// `pendingCaret`（那一帧的字段把选区放到 `len(a.Query)`）。
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
	a.pendingCaret = true
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

// trackPointer reads this pass's pointer and list offset and answers whether
// the pointer's position is the user's own (`App.hoverFresh`).
//
// The pointer's own coordinates are what makes an entry an entry: a row that
// arrives under a pointer which did not move was put there by something else —
// the keyboard's reveal, a query that reflowed the list, the window resizing to
// its content — and R21's `onPointerEnter` never fires for it. The wheel is the
// exception on purpose: it *is* a pointer gesture, the user aims with the same
// pointer, so a notch that carries the content under it hands the choice over
// row by row (the contract `TestTheWheelScrollsTheListUnderThePointer` pins).
// The app's own scrolls are therefore told apart from the user's by the offset
// alone plus the flag the reveal arms: a move while `appScrolled` stands is the
// app's, and it settles over the passes that follow it.
func (a *App) trackPointer(panel ui.Element) {
	x, y, over := panel.PointerPosition()
	// An unseen pointer is not a moving one: a window that opens under the
	// cursor does not hand the choice to the row it happens to cover. Arriving
	// over the panel and leaving it both count as movement — they are the user
	// taking the pointer somewhere.
	moved := a.pointerSeen && (over != a.pointerOver || (over && (x != a.pointerAt[0] || y != a.pointerAt[1])))
	a.pointerAt, a.pointerOver, a.pointerSeen = [2]float32{x, y}, over, true
	first, _ := a.List.Visible()
	scrolled := a.listFirstSeen && first != a.listFirst
	a.listFirst, a.listFirstSeen = first, true
	switch {
	case a.appScrolled:
		a.hoverFresh = false
		if !scrolled {
			a.appScrolled = false
		}
	case moved || scrolled:
		a.hoverFresh = true
	}
}

// row builds one result: title, detail and an optional shortcut. The chosen
// row wears the accent as a tint, as the old launcher's rows did.
func (a *App) row(c *ui.Context, item Item, i int) {
	t := c.Theme()
	copy := StringsFor(a.settings().Language)
	// A block's title line: the list's own label, drawn in the row's own
	// column, never selectable and never numbered.
	if item.heading {
		// The group header is Raycast's: the caption step at weight 500 in the
		// secondary ink, left-aligned with the row *text* rather than with the
		// row's box (`docs/raycast-design.md` §4.4). Its **size** does not move
		// — the task book's size rule admits a weight, a letter-spacing and a
		// colour in this slice, never a register — so it keeps the body step and
		// takes the reference's 500/secondary pair. The band the window charges
		// is unchanged too (see headingPadding).
		//
		// P11-ac 重定基线：分组头落在**本表面结果行标题的那一列**上。旧的左缩进
		// 是两个手写数（行的左 padding 12u + 文字的 `Margin-left` 42u），从来没
		// 从「行的标题列」这个和里推出来；P8-a 把行块缩进 4u→8u、行内 padding
		// 12u→8u 之后它们一个都没动，于是标签比它管辖的标题右挂了 8u（实测 62u
		// vs 54u，旧注释却写着「sits on the same column a row's title does」）。
		// 现在左缩进就是行自己的那一份：行内 padding 8u（`rowBody` 的
		// `Padding(0, t.Space(2))`）+ 图标列 28u（`iconPlate` 的 `Space(7)`）+
		// 行自己的 gap 10u（`rowBody` 的 `Gap(t.Space(2.5))`）= 38u 的
		// `Margin-left`，加上列表 gutter 的 8u 即合约 §3 的 54u 文本列。
		tokens := a.tokens(c)
		// P13-x · 分组头也是列表的一部分，所以它也参加折边。它是 `rowBody` 的**兄弟**
		// （下面那个 `return` 让它从来没经过 `rowBody` 的 `Opacity`），于是折边切到它
		// 的时候它保持满墨——实测浅色 145 / 深色 133，与整行可见时逐像素相同。
		//
		// 盒子记进**自己的表**（`headingRects`），不进 `rowRects`：后者喂
		// `viewportRange` 的编号，一个标签混进去就会占掉一个数字位（P13-w「仍待」第 1
		// 条已经把这条写死）。`Bounds()` 是上一帧的矩形，与 `rowBody` 的
		// `a.rowRects[i] = row.Bounds()` 同一时序。
		heading := ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(2), t.Space(1), t.Space(2))
		if a.headingRects == nil {
			a.headingRects = make(map[int]ui.Rect, 4)
		}
		a.headingRects[i] = heading.Bounds()
		if fold := foldAlpha(a.listRect, a.headingRects[i]); fold < 1 {
			// 必须在 `Children` 之前：框架按建树那一刻的 opacity 画这一帧。
			heading.Opacity(float32(fold))
		}
		heading.Children(func() {
			ui.Text(c, item.Title).FontSize(t.FontSize).FontWeight(500).
				TextColor(tokens.TextSecondary).SingleLine().
				Margin(0, 0, 0, t.Space(9.5))
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
		tokens := a.tokens(c)
		note := ui.Row(c).FillWidth().MinHeight(t.Space(launcherStatusUnits)).
			AlignItems(ui.Center).Padding(0, t.Space(2)).Gap(t.Space(2.5))
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
	tokens := a.tokens(c)
	// The row's own box: one 40u height for every result, subtitle or not (see
	// rowUnits). A one-line row is not a shorter row — it is the same box with
	// one line in it — so the height the window is charged is the height the
	// row draws, and a row gaining a subtitle never resizes the window.
	floor := t.Space(rowUnits)
	subtitle := rowSubtitle(item)
	row := ui.Row(c.Key(item.ID)).FillWidth().Focusable().MinHeight(floor).
		AlignItems(ui.Center).
		Padding(0, t.Space(2)).Radius(tokens.RadiusSM).Gap(t.Space(2.5)).
		Transition(rowTransition)
	// P11-bb：这一行上一帧的盒子。编号家族靠它判「这一行读得全吗」——
	// 框架的 `Visible()` 只答「还有一个像素在框里」，而一个被裁掉九成的行
	// 拿到的徽章是没人看得见的（见 `numberedViewport`）。首帧还没有盒子，
	// 记一个空矩形，`numberedViewport` 会跳过它。
	if a.rowRects == nil {
		a.rowRects = make(map[int]ui.Rect, 16)
	}
	a.rowRects[i] = row.Bounds()
	// A history row is one step quieter than a match: the old build's
	// `.launcher-result--history { opacity: 0.92 }`.
	opacity := 1.0
	if item.clip != nil || item.calc != nil {
		opacity = 0.92
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
			row.Radius(0, 0, tokens.RadiusSM, tokens.RadiusSM)
		}
	}
	// P13-v：折边之外的那一行退后（见 `foldAlpha`）。两个盒子都是上一帧的，
	// 所以一条滚动中的行按上一帧的位置算——一帧的滞后读起来就是「它正退下去」，
	// 不是闪。行的静息档（历史的 0.92）和折边取**更暗**的那一个：退后只会更深，
	// 不会把一行退回来。
	//
	// 页脚那一行不参加：它是钉在面板底部的 footer，只是被记成列表的最后一行
	// 好让高度链把它算进去（见 `actionBarFooter`），它的盒子永远在视口之外，
	// 按折边算就是 0——那会把整条动作栏抹掉（实测：选中块的 tint 全丢、块从
	// x=-1 起算）。钉住的东西不在折边上。
	if !isBar {
		if fold := foldAlpha(a.listRect, a.rowRects[i]); fold < opacity {
			opacity = fold
		}
	}
	if opacity < 1 {
		row.Opacity(float32(opacity))
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
		// A pressed row takes Raycast's heaviest neutral rung: it sinks *into*
		// the card rather than lighting up (`docs/raycast-design.md` §4.3).
		row.Background(tokens.RowPressed)
	case i == a.Selected && isBar:
		// The action bar's own selection: the same neutral chosen-row fill a
		// result takes. Raycast's footer row is not a different material from
		// the list above it — the port gave it the settings surface's raised
		// pane and a cast shadow, which read as a second control inside the
		// footer.
		row.Background(tokens.RowSelected)
		// P12-b（docs/liquid-glass.md §6.1 第 1-3 行）：页脚动作条的选中块是 E1
		// —— 面（`RowSelected`）站起来的三个手段（面差 + 顶受光边 + 底对侧暗线，
		// 见 `theme.Tokens.LitFace`）。E1 不画投影（§2 判据 2）。
		row = tokens.LitFace(row)
	case i == a.Selected:
		// The chosen row is a *neutral* fill: Raycast's selected result is
		// 0.085 of white over the near-black card (0.06 of black over the light
		// one), with no border, no ring and no glow, and the accent is not spent
		// here at all (`docs/raycast-design.md` §1/§4.3). The emphasis is the
		// fill plus the title's own weight (700) — the accent's tint
		// (`AccentTintQuiet`) belongs to the settings surface now, not the list.
		row.Background(tokens.RowSelected)
		// P12-b（§6.1 第 1-2 行）：选中块 = E1 = 面 + 顶受光边 + 底对侧暗线
		// （`theme.Tokens.LitFace`）。旧语言只有一块平涂，块与底挤在同一个 z 平面。
		row = tokens.LitFace(row)
	}
	// The pointer's own rung: a hovered row that is *not* the choice takes the
	// lighter fill, so the pointer over a row the keyboard is on does not paint
	// a third state on top of the chosen one (the entry rule below hands the
	// choice over anyway; this covers the frame the two are the same row).
	if row.Hovered() && i != a.Selected && rowSelectable(item) && item.Run != nil {
		row.Background(tokens.RowHover)
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
		// The entry edge needs the pointer to have *moved*: `hoverFresh` is
		// false while the content slid under a still pointer because the app
		// scrolled it (see App.hoverFresh), and true again as soon as the
		// pointer itself moved or the user wheeled the list.
		if a.hoverFresh && i != a.hoveredRow && rowSelectable(item) && item.Run != nil {
			a.Selected = i
			a.pointerChose = true
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
			// The space keycap (R65): a bare `␣` on the shared keycap plate —
			// the same 20x20, radius-5, keycap-face-and-edge box every other
			// key in the list draws (`docs/raycast-design.md` §4.5) — not a
			// sentence. The whole explanation lives in the row's tooltip.
			key := keycap(c, tokens, t, "\u2423")
			key.Tooltip(copy.DirectOutputHint)
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
				a.pointerChose = true
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
			a.rowKeycap(c, tokens, a.resultNumber(number))
		}
		if item.Shortcut != "" {
			if isBar {
				a.barHintText(c, item.Shortcut)
			} else {
				a.rowKeycap(c, tokens, item.Shortcut)
			}
		}
		// The footer's own actions legend rides *this row*, not an absolute
		// overlay: `⌘K` (`docs/raycast-design.md` §4.6). It is the same keycap
		// chord the hint beside it draws, so the row's flex layout puts the two
		// side by side and the hint can never be painted under it — which is what
		// the absolute legend did once both became two-plate chords (the overlap
		// was visible in the exported frame, not just in arithmetic).
		//
		// The chord is **not bound** in this build: the task book keeps the
		// keyboard model untouched, and P8-b is where the actions surface it will
		// open arrives. The gap is listed in docs/raycast-design.md.
		if isBar {
			// The legend is its own group at the row's tail, one 8u step clear of
			// the hint beside it: the two chords are different facts (this row
			// runs on `⌘↩`; `⌘K` opens the actions surface), and with one plate
			// per key they would otherwise read as a single four-key card.
			a.rowKeycap(c, tokens, footerActionsBadge()).Margin(0, 0, 0, t.Space(2))
		}
	})
	if row.Clicked() && !removed {
		a.Selected = i
		a.pointerChose = true
		a.activate(a.drawn)
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
		// The field's text owns Enter when it produced no answer row: an
		// expression this build refused must say why, not fall through to the
		// history row the selection happens to sit on (see
		// `runCalculatorExpression`). An empty field, and a field whose answer
		// row *is* on the list, keep the row's own Enter.
		if a.calculatorEnterOwnsTheField(results) {
			a.runCalculatorExpression()
			return
		}
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
	a.Selected = nextSelectableIn(a.drawn, a.Selected, delta)
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

// nextSelectableIn is the keyboard's step, over a list the caller already
// holds: the draw clamps its choice against the slice it built, and asking for
// a fresh catalog there was the draw's second full match (see App.searchRuns).
// `move` hands it the frame's own table (`App.drawn`), which is the list the
// user is looking at when the key lands.
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
	tokens := a.tokens(c)
	band := t.Space(actionBarUnits * float32(len(results)-start))
	// The footer's own row block takes the *list's* gutter, not the panel's:
	// Raycast's footer row is a row of the same column, so its pill starts on
	// the same 8u the rows above it do (`docs/raycast-design.md` §4.3/§4.6).
	// It sat on 4u — the panel's own inset — so the held/selected footer's pill
	// was 4u wider on each side than every row above it and the column read as
	// two different columns at the seam.
	footer := ui.Column(c).FillWidth().Shrink(0).Height(band).MinHeight(0).
		Padding(0, t.Space(2))
	footer.Children(func() {
		for i := start; i < len(results); i++ {
			a.row(c, results[i], i)
		}
	})
	// The footer's own chrome is drawn *over* the band, never as a row of it:
	// the band's height and the window's height chain are the same numbers they
	// were (see actionBarUnits), so the rule costs the layout nothing. The
	// actions legend is not here: it is a child of the bar row itself (see
	// rowBody), so that the row's own hint and the legend share one flex line
	// instead of two boxes drawing over each other.
	footer.Children(func() {
		// Raycast's footer rule: the *weak* hairline that separates the band from
		// the list above it (`docs/raycast-design.md` §4.6). The footer is a
		// block of the card, not a second card, so it takes the quieter rung —
		// not the card's own 1px edge, and not the accent-tinted divider the port
		// drew.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(1).PassThrough().
			Background(tokens.HairlineSoft)
	})
}

// footerActionsBadge is the actions legend's own chord, in the platform's
// spelling: `⌘K` where the app modifier is Command, `Ctrl+K` where it is
// Control. It goes through the same `shortcuts.Badge` a real binding does, so
// the footer's legend and a row's keycap cannot disagree about how this
// platform writes a chord.
func footerActionsBadge() string {
	mod := "Ctrl"
	if runtime.GOOS == "darwin" {
		mod = "Cmd"
	}
	return shortcuts.Badge(mod+"+K", "K")
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
	// P11-az · 一条带只有一个句子，问句赢。两条带都是 `Absolute()` 且落在**同一个
	// 矩形**（`Bottom(a.footerBand)` + 30u），而 `Geometry.Feedback` 是 `bool`
	// ——窗口只为**一条**带长了 30u。两条都画的时候，toast 的句子与确认句逐像素
	// 重合（实测 `{X:40 Y:462 W:488 H:22}` 与 `{X:40 Y:462 W:664 H:22}`），确认
	// 句的按钮还压在 toast 的字上。问句是阻塞的那一句（关机/重启，3s 窗口），
	// toast 只是状态（4.5s）；问句在场时 toast 让位，时钟也不启动（`toast` 留着，
	// 问句答完或过期后它照常出现）。叠成两条要改高度链（`Feedback` 改成计数），
	// 那是红线，不碰。
	if a.toast == "" || a.pendingPower != "" {
		// P11-bb：带子现在有三个句子，优先级是问句 > toast > 按住修饰键的提示。
		// 前两句照旧（P11-az），第三句只在带子空着时上场。
		if a.toast == "" && a.pendingPower == "" && a.heldHint() {
			a.heldHintRow(c)
		}
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
	// P11-bq：句子只在到点那一帧变（4.5s 后自己消失），所以它要的是「到点唤醒
	// 一次」而不是每帧重建整棵视图（`Context.AnimationFrame` 是 rebuild，
	// `ui/runtime.go:744-748`）——120 Hz 面板上那是一条句子一秒 120 次重建。
	deadline.Wake(c, a.toastUntil.Sub(now))
	t := c.Theme()
	tokens := a.tokens(c)
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
		// P11-an：接触边的 alpha 是 `--elev-shadow-scale` 的函数，不是字面
		// 0.2（上面那段注释一直这么写着，但算术从来没实现过）。
		theme.ElevContact(feedback, t, a.Store.Snapshot().MainOpacity)
	}
	// The fill lands after the variant has picked it: painting the row before
	// the branch ran left the warning's own plane unreachable — the row read the
	// resting amber in both states.
	feedback.Background(background)
	// P12-b（§6.1 第 1-7 行）：这条带是 E1 —— 暖面（`IconSurface`，警告档是
	// 同值的 `RaisedWarm` 别名）戴上 `LitFace`（顶受光边 + 底对侧暗线）。它此前
	// 只在**警告**档画那条顶边，静息档「carries none of them」，于是同一条带在
	// 两个状态里是两个材质：警告时是一块浮起来的暖面，静息时是一块平涂的色块。
	// 受光边与色相无关（§3），警告档多出来的只是 keyline 与两层影。
	feedback = tokens.LitFace(feedback)
	feedback.Children(func() {
		ui.Icon(c, glyphAlert).Size(t.Space(4), t.Space(4)).TextColor(tone)
		// P11-cc：带子是 30u 高的一行（`Height(t.Space(7.5))`，窗口只为它长 30u），
		// 而句子的长度是**用户数据**（`internal/shell/notices.go` 的 `panelFeedback`
		// 送来的是 `i18n.Notifications` 拼的安装/卸载完成句，里面带扩展名）。不钉一行的
		// 时候，一个长名字就把句子折成两行（实测 720 宽：`{X:40 Y:404 W:664 H:44}`），
		// 越出带子的矩形压在列表最后几行的字上（同一帧里 `app-20` 的标题被压住
		// 54×15u）。同一条带里的 `heldHintRow` 与确认按钮的标签早就有这一行约束，
		// 这里是把第三种说法补齐：**一条带只有一个句子、一句话只有一行**。
		ui.Text(c, a.toast).FontSize(t.FontSize).FontWeight(weight).TextColor(tone).Grow(1).SingleLine()
	})
}

// heldHintRow draws the band's third sentence: the chord that opens a terminal
// session while the app modifier is held, as the keycaps the list itself uses
// plus the word that says what it does (P11-bb).
//
// It sits in the toast's own band and geometry — the list has already reserved
// that band (`statusBand`), so the row has nothing behind it to cover — but it
// wears no plane, no keyline and no shadow: a hint about a key the user is
// holding is not a message, and the amber `--system-icon-surface` would read as
// one (the user's F3: 不要 floating/overlay 观感，元素要 inline、要贴进布局).
func (a *App) heldHintRow(c *ui.Context) {
	t := c.Theme()
	tokens := a.tokens(c)
	copy := a.copy()
	ui.Row(c).Absolute().Bottom(a.footerBand).Left(t.Space(1)).Right(t.Space(1)).Height(t.Space(7.5)).
		Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).
		Children(func() {
			a.rowKeycap(c, tokens, a.enterBadge())
			ui.Text(c, copy.HeldTerminalHint).FontSize(t.FontSize).FontWeight(540).
				TextColor(tokens.TextSecondary).Grow(1).SingleLine()
		})
}

// statusBand is the band the docked notice occupies at the list's bottom edge:
// zero when nothing is docked, else the old build's `LAUNCHER_STATUS_UNITS`
// (30u). It is the *same* predicate as `Geometry.Feedback` — one function now
// (`feedbackVisible`), not three copies of one sentence — and the same number
// `height.go:350` charges the window, so the window, the band and the list's
// own tail stay one decision: the list's bottom padding takes the band's
// height back out of the content, and the last row stops at the band's top
// edge instead of hiding under it (P11-az).
//
// P11-cm：这里此前是判据的第二份手写拷贝，`FeedbackVisible`（shell 量窗口读的
// 那一个）是第三份，而第三份**漏了 `pendingPower`** —— 电源确认在场时 shell 量的
// 窗口比画面自己量的矮 30u，最低一行整行压在确认带底下（实测 720 宽、五条最近
// 启动：shell 532 / 画面 562，`Recent 09` 在 `506..530`、带子在 `506..528`）。
// 三处现在都读 `feedbackVisible`。
func (a *App) statusBand(c *ui.Context) float32 {
	if !a.feedbackVisible() {
		return 0
	}
	return c.Theme().Space(launcherStatusUnits)
}

// heldHint reports whether the docked band owes the held modifier's sentence:
// while the app modifier is down, `Super+Enter` opens a terminal session, and
// the band is the one place that says so — on the empty field and on a typed
// one alike (the user's own line: 「按住 Super 显示进入终端选项要固定底部且
// 不重复」, F4-3). The list never carries a row of its own for it: a row would
// take a number, a sort position and the selection, and the band is pinned to
// the panel's bottom edge instead.
//
// It is the *band's* third sentence, ranked below the question and the toast (see
// feedbackRow): a message that is going somewhere wins over a hint about a key
// the user is holding.
func (a *App) heldHint() bool {
	return a.modifierHeld && a.output == nil && !a.pluginModeActive()
}

// FeedbackVisible reports whether the docked status row is showing, for the
// shell's window sizing. It is `feedbackVisible`, the same function the draw
// reads for `Geometry.Feedback` and for the list's own bottom padding: the
// shell's window and the frame that fills it are one decision, and three
// copies of that sentence is how they drifted apart (P11-cm).
func (a *App) FeedbackVisible() bool { return a.feedbackVisible() }

// feedbackVisible is whether the panel owes its docked band this frame: a
// toast, an unanswered power confirmation, or the held modifier's hint. It is
// the *one* answer to that question — `View` charges the window with it,
// `statusBand` takes the band back out of the list's content with it, and
// `FeedbackVisible` hands it to the shell, which sizes the window with it
// before any frame has run (P11-cm).
//
// The question is "is the band drawn", not "is there a message": the power
// confirmation is a *question* and outranks the toast (see feedbackRow), but it
// is the same band, the same 30u, and the same charge — a predicate that left it
// out made the shell's window a band short exactly while the user was being
// asked something destructive.
func (a *App) feedbackVisible() bool {
	return a.toast != "" || a.pendingPower != "" || a.heldHint()
}

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
	// The state lands before the edge is handled: the band's own predicate
	// reads modifierHeld.
	a.modifierHeld = down
	// The frame's **one** catalog walk, taken after the state landed so the
	// list it returns is the list this frame draws. The caller reuses this
	// slice for the rest of the frame rather than asking again — a second
	// full match per frame was the launcher's quietest cost (see
	// App.searchRuns) and it bought nothing.
	results := a.Results()
	if down != a.heldWasDown {
		a.heldWasDown = down
		if down && strings.TrimSpace(a.Query) != "" {
			// A typed query draws no terminal line of its own (the pinned
			// band says it instead), so the modifier hands the selection to
			// the action bar, the old build's `setSelectedActionBar(true)` on
			// the modifier's keydown. The bar is a row of this list, so it is
			// selected by its index.
			if bar := actionBarIndex(results); bar >= 0 {
				a.Selected = bar
			}
		} else if !down {
			a.barRowKey = ""
			if strings.TrimSpace(a.Query) != "" {
				// The selection the press edge took goes back to the page's
				// own default — the reference's `firstRunnableResultIndex`.
				a.Selected = a.defaultSelection(results)
			}
		}
	}
	return results
}

// openBareTerminal runs the held modifier's own action: a terminal session
// with nothing to run. It is the band's action, not a row's, so it takes no
// selection and no index — Super+Enter reaches it directly (see View).
func (a *App) openBareTerminal() {
	if a.Actions.OpenTerminal != nil {
		a.Actions.OpenTerminal()
	}
}

// listShapeKey names the list shape a selection was asserted for: the row
// count and the query the row was chosen over.
func listShapeKey(results []Item, query string) string {
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
	key := listShapeKey(results, a.Query)
	if key == a.barRowKey {
		return
	}
	a.barRowKey = key
	a.Selected = bar
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
		Transition(ui.ElementTransition{Duration: theme.ColorStep, Colors: true})
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
		// P13-h · 与 `rowControl` 同一语汇：悬停的 28u 格是 E1 的面（顶受光边 +
		// 底对侧暗线），不是四层透镜（第四层是四边环）。
		e = tokens.LitFace(e.Background(tokens.ControlHover))
		ink = tokens.TextStrong
	}
	e.Children(func() { mark(ink) })
	if e.Clicked() {
		run()
	}
}

func (a *App) fieldButtons(c *ui.Context, copy i18n.Launcher) {
	t := c.Theme()
	tokens := a.tokens(c)
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
	settingsHint := copy.FieldSettingsHint(shortcuts.Display(a.shortcut(settings.ShortcutOpenSettings)))
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
	tokens := a.tokens(c)
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
	tokens := a.tokens(c)
	a.fieldSettingsButton(c, tokens, copy.OutputDetach, copy.OutputDetachHint, func(mark ui.Color) {
		if glyph, ok := Glyph("pin"); ok {
			ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(mark)
		}
	}, func() {
		a.Actions.DetachOutput(OutputView{Entry: a.mode, Args: strings.Split(a.Query, " ")})
	})
}
