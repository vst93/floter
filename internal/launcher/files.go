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

// filesWords are the trigger words for the mode.
var filesWords = []string{"files", "file", "drops", "drop", "文件"}

// EnterFiles opens the files mode over the paths the desktop dropped.
func (a *App) EnterFiles() { a.enterFiles() }

// InFilesMode reports whether the files mode is open.
func (a *App) InFilesMode() bool { return a.files }

// SetDropped replaces the dropped files the mode lists.
func (a *App) SetDropped(files []drops.File) { a.Dropped = files }

// enterFiles starts the files mode.
func (a *App) enterFiles() {
	a.mode = nil
	a.clipboard = false
	a.browser = false
	a.calculatorMode = false
	a.files = true
	a.Query = filesWord + " "
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
}

// leaveFiles returns to the search.
func (a *App) leaveFiles() {
	if !a.files {
		return
	}
	a.files = false
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// filesQuery is what the user typed after the mode word.
func (a *App) filesQuery() string {
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return ""
	}
	return strings.Join(words[1:], " ")
}

// filesItems is the files mode's list: one row per dropped file per action,
// filtered by the needle.
func (a *App) filesItems() []Item {
	copy := a.copy()
	terms := strings.Fields(strings.ToLower(a.filesQuery()))
	out := make([]Item, 0, len(a.Dropped)*3+1)
	// The block's own heading: a dropped file's rows are one group, and the
	// list says so above them (the old build's `list-sections`).
	if len(a.Dropped) > 0 {
		out = append(out, Item{ID: "section:dropped", heading: true, Title: copy.FilesSection})
	}
	for _, file := range a.Dropped {
		file := file
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
