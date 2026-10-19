//go:build !windows

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The manifest's `output` mode is the route a command's run takes, and the
// route has to reach the launcher on the CommandEntry — the launcher's row
// reads it for the direct-output nudge and `runCommandWith` reads it to decide
// between the terminal surface and the captured output view. A manifest that
// declares `terminal` must therefore produce a terminal entry, and one that
// declares nothing must land on the schema default, `background`.
func TestCommandRouteFollowsTheManifestOutput(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	// Two integrations, one per spelling: a declared `terminal`, and the
	// background default (no `output` field at all).
	writeIntegration := func(id, output string) {
		dir := filepath.Join(paths.Extensions, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		declared := ""
		if output != "" {
			declared = `"output": "` + output + `",`
		}
		manifest := `{
		  "schemaVersion": "2.0", "id": "` + id + `", "name": "` + id + `",
		  ` + declared + `
		  "distribution": {"type": "local"},
		  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
		  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
		}`
		if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\ncat <<'JSONEOF'\n" +
			`{"protocolVersion":"1.0","provider":{"id":"` + id + `","name":"` + id + `","version":"1.0.0"},` +
			`"commands":[{"id":"run","name":"Run","execution":{"program":"self","argsPrefix":["run"],"mode":"pty"}}]}` +
			"\nJSONEOF\n"
		if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeIntegration("dev.floter.term", RouteTerminal)
	writeIntegration("dev.floter.bg", "")

	repository := `{
	  "schemaVersion": 1,
	  "extensions": {
	    "dev.floter.bg": {"id": "dev.floter.bg", "name": "bg", "state": "enabled", "enabled": true,
	      "manifestPath": "` + filepath.Join(paths.Extensions, "dev.floter.bg", manifestFileName) + `",
	      "installedAt": 1, "updatedAt": 1},
	    "dev.floter.term": {"id": "dev.floter.term", "name": "term", "state": "enabled", "enabled": true,
	      "manifestPath": "` + filepath.Join(paths.Extensions, "dev.floter.term", manifestFileName) + `",
	      "installedAt": 1, "updatedAt": 1}
	  }
	}`
	if err := os.WriteFile(paths.RepositoryFile, []byte(repository), 0o644); err != nil {
		t.Fatal(err)
	}

	store := OpenStore(paths)
	store.Refresh(context.Background())

	routes := map[string]string{}
	for _, entry := range store.CommandEntries() {
		routes[entry.IntegrationID] = entry.Route
	}
	if got := routes["dev.floter.term"]; got != RouteTerminal {
		t.Errorf("a declared terminal command has route %q, want %q", got, RouteTerminal)
	}
	if got := routes["dev.floter.bg"]; got != RouteBackground {
		t.Errorf("an undeclared command has route %q, want %q (the schema default)", got, RouteBackground)
	}
}
