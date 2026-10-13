package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A manifest's lifecycle.launch is how the integration as a whole runs: its
// program and leading arguments replace each command's own execution, and its
// cwd policy and terminal environment come with it.
func TestLaunchOverrides(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.launched")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.launched", "name": "Launched",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "lifecycle": {"launch": {"command": {"program": "self", "args": ["--launch"]}, "cwdPolicy": "toolData"}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\ncat <<'JSONEOF'\n"+
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.launched","name":"Launched","version":"1.0.0"},
		  "commands":[{"id":"run","name":"Run","execution":{"program":"self","argsPrefix":["run"],"mode":"pty"}}]}`+
		"\nJSONEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.launched": {"id": "dev.floter.launched", "name": "Launched",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := OpenStore(paths)
	store.Refresh(context.Background())
	entries := store.CommandEntries()
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	entry := entries[0]
	// The launch program is `self`: the integration's own executable, which is
	// the interpreter the runtime binding resolved (its arguments, the script,
	// come first), with the launch arguments before the command's prefix.
	if got := strings.Join(entry.Args, " "); !strings.Contains(got, "tool.sh --launch run") {
		t.Errorf("program = %q argv = %q", entry.Program, got)
	}
	// The toolData policy points at the integration's own data directory.
	if entry.Dir != filepath.Join(paths.Data, "dev.floter.launched") {
		t.Errorf("dir = %q", entry.Dir)
	}
	// The terminal environment is named for floter.
	found := map[string]string{}
	for _, pair := range entry.Env {
		key, value, _ := strings.Cut(pair, "=")
		found[key] = value
	}
	if found["TERM"] != "floter-256color" || found["TERM_PROGRAM"] != "floter" {
		t.Errorf("env = %v", found)
	}
}

// A fixed cwd policy names an absolute directory in either spelling, and the
// object forms of the other policies parse too. Illegal objects fail at load
// time rather than making the whole manifest unreadable.
func TestLaunchFixedPolicy(t *testing.T) {
	decode := func(t *testing.T, raw string) (CWDPolicy, error) {
		t.Helper()
		var policy CWDPolicy
		err := json.Unmarshal([]byte(raw), &policy)
		return policy, err
	}

	valid := []struct {
		raw  string
		kind string
		path string
	}{
		{`"fixed:/opt/tool/data"`, CWDFixed, "/opt/tool/data"},
		{`{"policy":"fixed","path":"/opt/tool/data"}`, CWDFixed, "/opt/tool/data"},
		{`"home"`, CWDHome, ""},
		{`"toolData"`, CWDToolData, ""},
		{`"inheritActiveSession"`, CWDInheritActiveSession, ""},
		{`{"policy":"home"}`, CWDHome, ""},
		{`{"policy":"projectRoot"}`, CWDProjectRoot, ""},
	}
	for _, c := range valid {
		policy, err := decode(t, c.raw)
		if err != nil {
			t.Errorf("%s: %v", c.raw, err)
			continue
		}
		if policy.Kind != c.kind || policy.Path != c.path {
			t.Errorf("%s: policy = %+v, want kind %q path %q", c.raw, policy, c.kind, c.path)
		}
	}

	// A projectRoot object without markers or depth gets the old host's
	// defaults.
	policy, err := decode(t, `{"policy":"projectRoot"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Markers) != len(defaultProjectMarkers) || policy.MaxDepth != defaultProjectMaxDepth {
		t.Errorf("defaults = %+v", policy)
	}
	policy, err = decode(t, `{"policy":"projectRoot","markers":[".floter"],"maxDepth":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Markers) != 1 || policy.Markers[0] != ".floter" || policy.MaxDepth != 3 {
		t.Errorf("declared = %+v", policy)
	}

	// A relative fixed path, a missing policy and an unknown word are all
	// load-time errors.
	invalid := []string{
		`"fixed:relative/path"`,
		`{"policy":"fixed","path":"relative/path"}`,
		`{"policy":"fixed"}`,
		`{"path":"/opt/tool/data"}`,
		`{}`,
		`"nonsense"`,
		`123`,
	}
	for _, raw := range invalid {
		if _, err := decode(t, raw); err == nil {
			t.Errorf("%s: no error", raw)
		}
	}
}

// A manifest without a lifecycle leaves the commands' own execution in place.
func TestNoLaunchOverrides(t *testing.T) {
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.plain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.plain", "name": "Plain",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\ncat <<'JSONEOF'\n"+
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.plain","name":"Plain","version":"1.0.0"},
		  "commands":[{"id":"run","name":"Run","execution":{"program":"self","argsPrefix":["run"],"mode":"pty"}}]}`+
		"\nJSONEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.plain": {"id": "dev.floter.plain", "name": "Plain",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := OpenStore(paths)
	store.Refresh(context.Background())
	entries := store.CommandEntries()
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	// No launch program override: the runtime binding runs the command's own
	// prefix, and no terminal environment is added.
	for _, pair := range entries[0].Env {
		if strings.HasPrefix(pair, "TERM_PROGRAM=") {
			t.Errorf("a launch environment appeared without a launch: %v", entries[0].Env)
		}
	}
	// The default cwd policy still resolves: the tool data directory, which is
	// where the old host ran a command with no lifecycle.
	if want := filepath.Join(paths.Data, "dev.floter.plain"); entries[0].Dir != want {
		t.Errorf("dir = %q, want %q", entries[0].Dir, want)
	}
}
