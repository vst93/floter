package launcher

import (
	"strings"

	"floter/internal/tools"
)

// The install rows: a tool the catalog knows, which this machine does not have,
// matched by what the user typed.
//
// Floter is not a package manager and never runs an install. Enter opens a
// **bare terminal session and types the command into it** — the shell is the
// user's, with their proxy, mirror and environment already in effect, and it
// outlives the install. Nothing here runs the command itself, and nothing
// waits for it.
//
// A tool that *is* installed needs no row from the catalog: the PATH command
// scan finds the executable itself, which is the thing the user wants to run.

// SetToolCatalog replaces the catalog state the install rows are built from.
func (a *App) SetToolCatalog(states []tools.State) { a.InstallCatalog = states }

// toolInstallItems is the install rows for a query: the tools the catalog
// knows, that are not installed, and whose name or keywords the query matches.
func (a *App) toolInstallItems() []Item {
	if len(a.InstallCatalog) == 0 {
		return nil
	}
	terms := strings.Fields(strings.ToLower(a.Query))
	if len(terms) == 0 {
		return nil
	}
	copy := a.copy()
	out := make([]Item, 0, len(a.InstallCatalog))
	for _, state := range a.InstallCatalog {
		if state.Installed || state.Command == "" || !state.Matches(terms) {
			continue
		}
		state := state
		// The row is searched by the tool's own vocabulary (its keywords and
		// the executable names), which is what a query like 搜索 or rg hits.
		search := strings.Join([]string{state.ID, state.Name,
			strings.Join(state.Keywords, " "), strings.Join(state.Probes, " ")}, " ")
		out = append(out, Item{
			ID:       "install:" + state.ID,
			Title:    copy.InstallTool(state.Name),
			Detail:   state.Command,
			Search:   search,
			Shortcut: state.Manager,
			Run:      func() { a.openInstallSession(state) },
		})
		if len(out) >= toolRowLimit {
			break
		}
	}
	return out
}

// toolInvokeItems is the invoke rows for a query: a tool the catalog knows,
// that *is* installed, and that carries a launch action of its own — the argv
// the launcher starts for it. Discover → install → invoke never leaves the
// launcher: the install row offers the command while the tool is missing, and
// this row starts the tool once it is there.
func (a *App) toolInvokeItems() []Item {
	if len(a.InstallCatalog) == 0 {
		return nil
	}
	terms := strings.Fields(strings.ToLower(a.Query))
	if len(terms) == 0 {
		return nil
	}
	out := make([]Item, 0, len(a.InstallCatalog))
	for _, state := range a.InstallCatalog {
		if !state.Installed || len(state.Launch) == 0 || !state.Matches(terms) {
			continue
		}
		state := state
		search := strings.Join([]string{state.ID, state.Name,
			strings.Join(state.Keywords, " "), strings.Join(state.Probes, " ")}, " ")
		argv := append([]string{}, state.Launch...)
		out = append(out, Item{
			ID: "invoke:" + state.ID,
			// The tool's own name, and the argv the spawn will receive,
			// verbatim: the row says what it will start, not that it starts
			// something.
			Title:  state.Name,
			Detail: strings.Join(argv, " "),
			Search: search,
			// A full-screen TUI needs a real terminal or it exits the moment
			// it starts: the row hands the argv to the user's own shell
			// instead of a detached spawn with nowhere to draw.
			Run: func() {
				if state.NeedsTerminal {
					if a.Actions.RunInTerminal != nil {
						a.Actions.RunInTerminal(argv)
					}
					return
				}
				if a.Actions.SpawnDetached != nil {
					a.Actions.SpawnDetached(argv)
				}
			},
		})
		if len(out) >= toolRowLimit {
			break
		}
	}
	return out
}

// toolRowLimit is the most install (or invoke) rows one query may add: a
// discovery row is not the main match, and a single loose keyword ("git",
// "search") can hit several tools at once. Three keeps the list a launcher
// and not a package index.
const toolRowLimit = 3

// openInstallSession opens a bare terminal session with the install command
// typed into it. The session is the user's own shell and outlives the install;
// the command is typed, never run by floter.
func (a *App) openInstallSession(state tools.State) {
	if a.Actions.OpenInstallSession != nil {
		a.Actions.OpenInstallSession(state.Command)
		return
	}
	a.Hide()
}
