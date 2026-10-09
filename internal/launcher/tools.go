package launcher

import (
	"strings"

	"floter/internal/tools"
)

// The install rows: a tool the catalog knows, which this machine does not have,
// matched by what the user typed.
//
// Floter is not a package manager and never runs an install: the row **copies**
// the command for the user's own shell, where their proxy, mirror and
// environment are already in effect. Nothing here spawns anything.
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
			Run:      func() { a.copyInstallCommand(state) },
		})
	}
	return out
}

// copyInstallCommand puts a tool's install command on the clipboard, and says
// so. The command is never run: it belongs in the user's own shell.
func (a *App) copyInstallCommand(state tools.State) {
	if a.Actions.Copy != nil {
		a.Actions.Copy(state.Command)
		a.toast = a.copy().Copied
	}
}
