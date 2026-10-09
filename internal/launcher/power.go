package launcher

import (
	"strings"
)

// The power rows: restart and shut down, offered when the query *is* one of
// their words.
//
// The old build claimed these queries before any other match, and so does this
// one: a user who types "restart" means the machine, not an application whose
// name contains it. The action itself is the shell's — it confirms with a
// native dialog before running anything, because both of these end the session.

// The words that claim the power rows, in both languages.
var (
	restartWords  = []string{"restart", "reboot", "重启", "重新启动"}
	shutdownWords = []string{"shutdown", "shut down", "power off", "poweroff", "关机", "关闭电脑"}
)

// PowerRestart and PowerShutdown are the action ids the shell acts on.
const (
	PowerRestart  = "restart"
	PowerShutdown = "shutdown"
)

// powerItems is the power rows for a query that is exactly one of their words.
func (a *App) powerItems() []Item {
	query := strings.ToLower(strings.TrimSpace(a.Query))
	if query == "" {
		return nil
	}
	copy := a.copy()
	var out []Item
	if matchesWord(query, restartWords) {
		out = append(out, Item{
			ID:     "power:restart",
			Title:  copy.PowerRestart,
			Detail: copy.PowerRestartHint,
			Run:    func() { a.runPower(PowerRestart) },
		})
	}
	if matchesWord(query, shutdownWords) {
		out = append(out, Item{
			ID:     "power:shutdown",
			Title:  copy.PowerShutdown,
			Detail: copy.PowerShutdownHint,
			Run:    func() { a.runPower(PowerShutdown) },
		})
	}
	return out
}

// matchesWord reports whether a query is one of a word list, exactly.
func matchesWord(query string, words []string) bool {
	for _, word := range words {
		if query == word {
			return true
		}
	}
	return false
}

// runPower hands the action to the shell, which confirms it first.
func (a *App) runPower(action string) {
	if a.Actions.Power != nil {
		a.Actions.Power(action)
	}
}
