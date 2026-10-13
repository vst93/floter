package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Resolve follows the old host's chains: the session cwd when there is one, a
// project root found by markers, then the tool data dir, then home.
func TestCWDPolicyResolve(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "session")
	data := filepath.Join(root, "data")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		policy CWDPolicy
		ctx    CwdContext
		want   string
	}{
		{"default inherits the session", CWDPolicy{}, CwdContext{SessionCwd: session, DataDir: data, Home: home}, session},
		{"inherit falls back to data", CWDPolicy{Kind: CWDInheritActiveSession}, CwdContext{DataDir: data, Home: home}, data},
		{"inherit falls back to home", CWDPolicy{Kind: CWDInheritActiveSession}, CwdContext{DataDir: filepath.Join(root, "absent"), Home: home}, home},
		{"home", CWDPolicy{Kind: CWDHome}, CwdContext{DataDir: data, Home: home}, home},
		{"fixed", CWDPolicy{Kind: CWDFixed, Path: "/opt/tool"}, CwdContext{DataDir: data, Home: home}, "/opt/tool"},
		{"toolData", CWDPolicy{Kind: CWDToolData}, CwdContext{DataDir: data, Home: home}, data},
		{"projectRoot with no session falls back to data", CWDPolicy{Kind: CWDProjectRoot}, CwdContext{DataDir: data, Home: home}, data},
	}
	for _, c := range cases {
		if got := c.policy.Resolve(c.ctx); got != c.want {
			t.Errorf("%s: Resolve = %q, want %q", c.name, got, c.want)
		}
	}
}

// The projectRoot policy walks up from the session cwd looking for a marker,
// capped by maxDepth; when nothing is found it keeps the session cwd.
func TestProjectRootWalksUpFromSessionCwd(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	nested := filepath.Join(project, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".git"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	policy := CWDPolicy{Kind: CWDProjectRoot}
	ctx := CwdContext{SessionCwd: nested, DataDir: filepath.Join(root, "data"), Home: root}
	if got := policy.Resolve(ctx); got != project {
		t.Errorf("project root = %q, want %q", got, project)
	}

	// A depth cap of one level cannot reach the root, so the session cwd
	// stands.
	policy = CWDPolicy{Kind: CWDProjectRoot, MaxDepth: 1}
	if got := policy.Resolve(ctx); got != nested {
		t.Errorf("shallow project root = %q, want %q", got, nested)
	}

	// A custom marker finds a different root.
	if err := os.WriteFile(filepath.Join(nested, "floter.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	policy = CWDPolicy{Kind: CWDProjectRoot, Markers: []string{"floter.toml"}}
	if got := policy.Resolve(ctx); got != nested {
		t.Errorf("custom marker root = %q, want %q", got, nested)
	}
}

// findProjectRoot walks up at most maxDepth levels and stops at the filesystem
// root.
func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "x", "y", "z")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(deep, []string{".floter-no-such-marker"}, 32); got != "" {
		t.Errorf("found %q with no marker", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(deep, []string{"go.mod"}, 32); got != root {
		t.Errorf("found %q, want %q", got, root)
	}
	if got := findProjectRoot(deep, []string{"go.mod"}, 2); got != "" {
		t.Errorf("found %q under a depth cap of 2", got)
	}
	if got := findProjectRoot("", []string{"go.mod"}, 32); got != "" {
		t.Errorf("empty start found %q", got)
	}
}

// A schema-legal object cwdPolicy loads and refreshes into commands, the
// object `fixed` form resolves to its declared path, and the object
// `projectRoot` form (with no active session cwd) lands on the data dir.
func TestObjectCwdPolicyRefreshes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	fixedDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(fixedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeCwdIntegration(t, paths, "dev.floter.root", `{"policy":"projectRoot","markers":[".git"],"maxDepth":4}`)
	writeCwdIntegration(t, paths, "dev.floter.fixed", `{"policy":"fixed","path":"`+fixedDir+`"}`)

	store := OpenStore(paths)
	store.Refresh(context.Background())
	entries := store.CommandEntries()
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	for _, entry := range entries {
		switch entry.IntegrationID {
		case "dev.floter.root":
			if want := filepath.Join(paths.Data, "dev.floter.root"); entry.Dir != want {
				t.Errorf("projectRoot dir = %q, want %q", entry.Dir, want)
			}
		case "dev.floter.fixed":
			if entry.Dir != fixedDir {
				t.Errorf("fixed dir = %q, want %q", entry.Dir, fixedDir)
			}
		}
	}
}

// An illegal object cwdPolicy is a load-time error, not a manifest that
// silently loses its launch block.
func TestInvalidCwdPolicyFailsTheManifest(t *testing.T) {
	cases := []string{
		`{"policy":"fixed","path":"relative"}`,
		`{"path":"/abs"}`,
		`{"policy":"nonsense"}`,
	}
	for _, cwd := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "floter.extension.json")
		body := `{"schemaVersion":"2.0","id":"dev.floter.bad","name":"Bad","lifecycle":{"launch":{"cwdPolicy":` + cwd + `}}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadManifest(path); err == nil {
			t.Errorf("%s: manifest loaded", cwd)
		}
	}
}

// writeCwdIntegration writes one integration whose lifecycle.launch carries
// the given cwdPolicy JSON and a single command.
func writeCwdIntegration(t *testing.T, paths Paths, id, cwd string) {
	t.Helper()
	dir := filepath.Join(paths.Extensions, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
	  "schemaVersion": "2.0", "id": "` + id + `", "name": "Cwd",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable"},
	  "lifecycle": {"launch": {"cwdPolicy": ` + cwd + `}}
	}`
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat <<'JSONEOF'\n" +
		`{"protocolVersion":"1.0","provider":{"id":"` + id + `","name":"Cwd","version":"1.0.0"},` +
		`"commands":[{"id":"run","name":"Run","execution":{"program":"self","mode":"pty"}}]}` +
		"\nJSONEOF\n"
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// Merge into the repository file, keeping any integration already written.
	repository := `{"schemaVersion":1,"extensions":{}}`
	if data, err := os.ReadFile(paths.RepositoryFile); err == nil && len(data) > 0 {
		repository = string(data)
	}
	var repo struct {
		SchemaVersion int                        `json:"schemaVersion"`
		Extensions    map[string]json.RawMessage `json:"extensions"`
	}
	if err := json.Unmarshal([]byte(repository), &repo); err != nil {
		t.Fatal(err)
	}
	if repo.Extensions == nil {
		repo.Extensions = map[string]json.RawMessage{}
	}
	entry := `{"id":"` + id + `","name":"Cwd","state":"enabled","enabled":true,"manifestPath":"` +
		filepath.Join(dir, manifestFileName) + `","installedAt":1,"updatedAt":1}`
	repo.Extensions[id] = json.RawMessage(entry)
	data, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
