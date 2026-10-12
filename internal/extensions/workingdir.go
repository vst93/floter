package extensions

import "strings"

// WorkingDirectory resolves a provider descriptor's `execution.workingDirectory`
// to the directory the process starts in.
//
// The descriptor names a behaviour, not a path, and the old host resolved it
// against the launch's own cwd (provider.rs:655-659):
//
//	"current" -> the launch cwd (what the manifest's lifecycle cwd policy resolved)
//	"home"    -> the user's home directory
//	"inherit" -> nothing at all: the host's own cwd stands
//
// Anything else is the schema's default, "current".
//
// The registry used to hand the descriptor's word straight to the process
// (`cmd.Dir = "current"`), which is not a directory: the background route
// failed to start, and the terminal route only escaped by ignoring the
// keyword — which also threw away the home policy. Resolving here keeps one
// answer for both routes.
func WorkingDirectory(mode, launchDir, home string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "home":
		return home
	case "inherit":
		return ""
	default:
		return launchDir
	}
}
