package launcher

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/settings"
)

// The launcher's window geometry. The width and the collapsed height are the
// old shell's contract — `INPUT_WINDOW_WIDTH` and `INPUT_WINDOW_HEIGHT` in
// src-tauri/src/lib.rs — and the collapsed height is a scale-1 measurement
// multiplied by the interface-size factor, exactly as
// `scaled_input_window_height` did.
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
	// Copy puts text on the clipboard, for the calculator row.
	Copy func(text string)
	// OpenApp launches an installed application.
	OpenApp func(app apps.App)
	// RunCommand runs an extension's command with the arguments the user
	// typed in the command mode (none when the row was clicked).
	RunCommand func(entry extensions.CommandEntry, args []string)
	// Complete asks the provider for completions of the tokens typed so
	// far; the shell answers on the main thread. Nil disables dynamic
	// completion, and the static argument list stands alone.
	Complete func(entry extensions.CommandEntry, tokens []string, done func([]extensions.Completion))
	// PinText shows some text in a window of its own.
	PinText func(title, text string)
	// CopyClip puts a clipboard entry back on the clipboard, whatever its
	// kind. Nil falls back to Copy for text.
	CopyClip func(entry clipboard.Entry)
}

// ClipboardSource is the clipboard history the launcher searches.
type ClipboardSource interface {
	Search(query string, limit int) []clipboard.Entry
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

	// Scroll keeps the result list's place; Search is the field's identity,
	// for the focus the launcher keeps on it while the surface shows.
	List   ui.ListState
	Search ui.Handle

	// chosenRow tracks the row the keyboard last moved to, so the list is
	// scrolled only when the choice changed.
	chosenRow int

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
	// Clipboard is the history the launcher searches; nil until the shell
	// gives it one.
	Clipboard ClipboardSource
}

// SetApps replaces the scanned applications.
func (a *App) SetApps(found []apps.App) { a.Apps = found }

// SetCommands replaces the extensions' commands.
func (a *App) SetCommands(found []extensions.CommandEntry) { a.Commands = found }

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

	a.syncCommandMode()
	results := a.Results()
	a.clampSelection(len(results))

	// The field row floats over the list; the list's top padding is the
	// row plus the edge, so a row scrolls under the field rather than to
	// its edge.
	fieldRow := t.Space(7)
	edge := fieldRow + t.Space(2)

	ui.Box(c).Fill().Children(func() {
		if len(results) == 0 {
			ui.Column(c).FillWidth().Padding(t.Space(3)).Center().Children(func() {
				message := copy.NoResults
				if a.mode != nil || a.clipboard {
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
			}).Fill().Padding(edge, 0, 0, 0).Label(copy.ResultsLabel)
		}

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
			case a.mode != nil || a.clipboard:
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

	// The field keeps the focus, so the list's arrows are read here: a
	// single-line text input leaves plain Up and Down to shortcuts.
	if c.Shortcut(0, ui.KeyDown) {
		a.move(1, len(results))
	}
	if c.Shortcut(0, ui.KeyUp) {
		a.move(-1, len(results))
	}
	if c.Shortcut(0, ui.KeyEscape) {
		switch {
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
	if c.Shortcut(0, ui.KeyTab) && a.Selected >= 0 && a.Selected < len(results) {
		item := results[a.Selected]
		switch {
		case a.mode != nil && item.complete != "":
			a.appendWord(item.complete)
		case a.clipboard && item.clip != nil:
			a.pinClip(*item.clip)
		case a.mode == nil && !a.clipboard && item.entry != nil:
			a.enterCommand(*item.entry)
		}
	}
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

// syncCommandMode leaves the mode when the line no longer starts with the
// word that entered it, which is what deleting it does.
func (a *App) syncCommandMode() {
	if a.mode == nil && !a.clipboard {
		return
	}
	word := firstWord(a.Query)
	want := clipboardWord
	if a.mode != nil {
		want = a.mode.Command.ID
	}
	if !strings.EqualFold(word, want) {
		a.leaveCommand()
		a.leaveClipboard()
	}
}

// clipboardWord is what the field starts with in the clipboard mode.
const clipboardWord = "clipboard"

// enterClipboard starts searching the clipboard history.
func (a *App) enterClipboard() {
	a.mode = nil
	a.clipboard = true
	a.Query = clipboardWord + " "
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
	row := ui.Row(c).FillWidth().Focusable().Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Gap(t.Space(2))
	if i == a.Selected {
		row.Background(t.Accent.Alpha(0.14))
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, item.Title).FontSize(t.FontSize).TextColor(t.Text)
			if item.Detail != "" {
				ui.Text(c, item.Detail).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		if item.Shortcut != "" {
			ui.Text(c, item.Shortcut).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
	})
	if row.Clicked() {
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
	if a.Selected < 0 || a.Selected >= len(results) {
		return
	}
	if run := results[a.Selected].Run; run != nil {
		run()
	}
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
	if a.Actions.RunCommand != nil {
		a.Actions.RunCommand(entry, args)
	}
}

// move moves the selection by delta, wrapping around the list.
func (a *App) move(delta, n int) {
	a.Selected = NextIndex(a.Selected, delta, n)
}

func (a *App) clampSelection(n int) {
	a.Selected = ClampIndex(a.Selected, n)
}
