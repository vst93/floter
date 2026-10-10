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
	}
	return out
}

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
