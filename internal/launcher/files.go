package launcher

import (
	"strings"

	"floter/internal/drops"
)

// The files mode: what a drop on the launcher offers.
//
// A dropped file becomes *rows*, never an execution: nothing is opened,
// spawned or copied when the files land. The three actions are the safe ones —
// hand the path to the system's own opener, open a terminal in the containing
// directory, or put the absolute path on the clipboard. None of them runs the
// file.
//
// The old build put the action on an action bar beside one row per file; this
// launcher has no action bar, so each file offers one row per action, which is
// the same three choices written down where the rows already are.

// filesWord is what the field starts with in the mode; the other spellings
// enter it too.
const filesWord = "files"

// MaxVisibleDroppedFiles is how many dropped files the mode lists before the
// rest collapse into one "and N more" row (the reference's
// `MAX_VISIBLE_DROPPED_FILES = 5`). A drop of a folder's worth of files would
// otherwise bury the launcher under rows nobody asked to read.
const MaxVisibleDroppedFiles = 5

// filesWords are the trigger words for the mode.
var filesWords = []string{"files", "file", "drops", "drop", "文件"}

// EnterFiles opens the files mode over the paths the desktop dropped.
func (a *App) EnterFiles() { a.enterFiles() }

// InFilesMode reports whether the files mode is open.
func (a *App) InFilesMode() bool { return a.files }

// SetDropped replaces the dropped files the mode lists. A new drop starts
// collapsed: the expander is the previous drop's state, and the file a bar was
// describing is gone.
func (a *App) SetDropped(files []drops.File) {
	a.Dropped = files
	a.dropsExpanded = false
}

// ClearDropped forgets the dropped files. A drop belonged to the previous
// interaction; a summon that starts a fresh launcher takes its rows with it
// (the old build's clearDrops).
func (a *App) ClearDropped() { a.Dropped = nil }

// enterFiles starts the files mode.
func (a *App) enterFiles() {
	a.mode = nil
	a.clipboard = false
	a.browser = false
	a.calculatorMode = false
	a.files = true
	a.dropsExpanded = false
	a.modeWord = filesWord
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveFiles returns to the search.
func (a *App) leaveFiles() {
	if !a.files {
		return
	}
	a.files = false
	a.modeWord = ""
	a.Selected, a.chosenRow = 0, -1
}

// filesQuery is what the user typed: the field holds the needle alone.
func (a *App) filesQuery() string {
	return a.Query
}

// filesItems is the files mode's list: one row per dropped file per action,
// filtered by the needle. Only the first MaxVisibleDroppedFiles files are
// listed; the rest are behind one "and N more" row, which is an ordinary
// result — the arrows and the numbered shortcuts reach it, and running it
// reveals the drop instead of touching a file (the reference's
// `fileDropRows`).
func (a *App) filesItems() []Item {
	copy := a.copy()
	terms := strings.Fields(strings.ToLower(a.filesQuery()))
	// A file is listed when at least one of its actions matches the needle;
	// the cap counts files, not rows, so "and N more" always means files.
	matching := make([][]Item, 0, len(a.Dropped))
	for _, file := range a.Dropped {
		if rows := a.fileRows(file, terms, copy); len(rows) > 0 {
			matching = append(matching, rows)
		}
	}
	visible := matching
	if !a.dropsExpanded && len(visible) > MaxVisibleDroppedFiles {
		visible = visible[:MaxVisibleDroppedFiles]
	}
	out := make([]Item, 0, len(visible)*3+2)
	// The block's own heading: a dropped file's rows are one group, and the
	// list says so above them (the old build's `list-sections`).
	if len(matching) > 0 {
		out = append(out, Item{ID: "section:dropped", heading: true, Title: copy.FilesSection})
	}
	for _, rows := range visible {
		out = append(out, rows...)
	}
	if hidden := len(matching) - len(visible); hidden > 0 {
		out = append(out, Item{
			ID:     "drop:more",
			Title:  copy.FilesMore(hidden),
			Detail: copy.FilesMoreHint,
			Run:    func() { a.dropsExpanded = true },
		})
	}
	return out
}

// fileRows is one dropped file's three actions, the ones the needle matches.
func (a *App) fileRows(file drops.File, terms []string, copy i18nLauncher) []Item {
	detail := file.Directory
	if file.IsDirectory {
		detail = copy.FileFolder + "  ·  " + file.Directory
	}
	rows := []struct {
		id     string
		action string
		run    func()
	}{
		{"open", copy.FileOpen, func() { a.openDropped(file) }},
		{"cd", copy.FileCd, func() { a.cdDropped(file) }},
		{"copy", copy.FileCopyPath, func() { a.copyDropped(file) }},
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		if !matchesFields(terms, file.Name+" "+file.Path+" "+row.action) {
			continue
		}
		out = append(out, Item{
			ID:     "drop:" + row.id + ":" + file.Path,
			Title:  row.action + "  " + file.Name,
			Detail: detail,
			Run:    row.run,
		})
	}
	return out
}

// openDropped hands a dropped path to the system's opener: what a double-click
// does, and what a user dropping a file most often means.
func (a *App) openDropped(file drops.File) {
	if a.Actions.OpenPath != nil {
		a.Actions.OpenPath(file.Path)
	}
	a.leaveFiles()
	a.Hide()
}

// cdDropped opens a terminal in the dropped file's directory: for a folder,
// entering it; for a file, standing next to it.
func (a *App) cdDropped(file drops.File) {
	if a.Actions.OpenInTerminal != nil {
		a.Actions.OpenInTerminal(drops.DirectoryForCD(file))
	}
	a.leaveFiles()
	a.Hide()
}

// copyDropped puts the absolute path on the clipboard.
func (a *App) copyDropped(file drops.File) {
	if a.Actions.Copy != nil {
		a.Actions.Copy(file.Path)
		a.toast = a.copy().Copied
	}
	a.leaveFiles()
	a.Hide()
}

// matchesFields reports whether every term appears in the given text.
func matchesFields(terms []string, text string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(text)
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
