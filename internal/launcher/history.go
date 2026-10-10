package launcher

import (
	"strings"
)

// The query history: the command lines the user has run, newest first, which
// the ↑ and ↓ keys walk.
//
// The draft the user was typing is kept while they are in the history, so
// coming back down to the present restores it rather than losing it. The
// history is deduplicated on entry (a re-run promotes the line) and bounded at
// twenty, exactly as the old build's remembered it.

// maxHistory is how many lines the history keeps.
const maxHistory = 20

// historyRemember records a command line in the history: it goes to the front,
// a previous copy of it is promoted rather than stacked, and the list is
// capped.
func (a *App) historyRemember(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	a.historyIndex = -1
	a.draftBeforeHistory = ""
	kept := make([]string, 0, len(a.History)+1)
	kept = append(kept, command)
	for _, entry := range a.History {
		if entry == command || len(kept) >= maxHistory {
			continue
		}
		kept = append(kept, entry)
	}
	a.History = kept
}

// historyUp moves one line into the past: the first ↑ keeps the draft the user
// was typing, so ↓ can bring it back.
func (a *App) historyUp() bool {
	if len(a.History) == 0 {
		return false
	}
	if a.historyIndex == -1 {
		a.draftBeforeHistory = a.Query
		a.historyIndex = 0
	} else if a.historyIndex < len(a.History)-1 {
		a.historyIndex++
	} else {
		return false
	}
	a.Query = a.History[a.historyIndex]
	a.pendingCaret = true
	return true
}

// historyDown moves one line into the future: past the newest entry, the draft
// the user was typing comes back.
func (a *App) historyDown() bool {
	if a.historyIndex == -1 {
		return false
	}
	a.historyIndex--
	if a.historyIndex < 0 {
		a.Query = a.draftBeforeHistory
		a.pendingCaret = true
		return true
	}
	a.Query = a.History[a.historyIndex]
	a.pendingCaret = true
	return true
}
