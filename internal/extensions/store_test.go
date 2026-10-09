package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fixture builds a config root with two integrations: one whose provider is
// a script that answers describe, and one that ships a static descriptor.
// The repository records both.
func fixture(t *testing.T) (Paths, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := FromRoot(dir)

	// The executable provider.
	pkg := filepath.Join(paths.Extensions, "io.github.vst93.v")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0",
  "id": "io.github.vst93.v",
  "name": "V Tools",
  "description": "Developer tools running in the terminal",
  "publisher": {"id": "vst93", "name": "vst"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["v"]},
  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
}`)
	executable := filepath.Join(pkg, "v.sh")
	writeFile(t, executable, "#!/bin/sh\ncat <<'JSONEOF'\n"+exampleDescription+"\nJSONEOF\n")
	if err := os.Chmod(executable, 0o755); err != nil {
		t.Fatal(err)
	}

	// The static-descriptor provider, disabled.
	staticPkg := filepath.Join(paths.Extensions, "dev.floter.static")
	writeFile(t, filepath.Join(staticPkg, manifestFileName), `{
  "schemaVersion": "2.0",
  "id": "dev.floter.static",
  "name": "Static Tools",
  "publisher": {"id": "floter", "name": "floter"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "built-in"},
  "runtime": {"type": "system", "executableNames": ["static"]},
  "provider": {"type": "static-descriptor", "descriptor": "description.json", "argsPrefix": []}
}`)
	writeFile(t, filepath.Join(staticPkg, "description.json"), `{
  "protocolVersion": "1.0",
  "provider": {"id": "dev.floter.static", "name": "Static Tools", "version": "1.0.0"},
  "commands": [{"id": "ping", "name": "Ping", "description": "Ping something", "execution": {"program": "self", "argsPrefix": ["ping"], "mode": "external"}}]
}`)

	writeFile(t, paths.RepositoryFile, `{
  "schemaVersion": 1,
  "extensions": {
    "io.github.vst93.v": {
      "id": "io.github.vst93.v", "name": "V Tools", "publisherName": "vst",
      "distributionSource": "local", "runtimeOwnership": "system", "providerKind": "executable",
      "state": "enabled", "enabled": true, "packageVersion": "0.0.12",
      "manifestPath": "", "executablePath": "`+executable+`", "channel": "stable",
      "installedAt": 1, "updatedAt": 1
    },
    "dev.floter.static": {
      "id": "dev.floter.static", "name": "Static Tools", "publisherName": "floter",
      "distributionSource": "built-in", "runtimeOwnership": "bundled", "providerKind": "static-descriptor",
      "state": "disabled", "enabled": false, "packageVersion": "1.0.0",
      "manifestPath": "`+filepath.Join(staticPkg, manifestFileName)+`", "executablePath": "", "channel": "stable",
      "installedAt": 1, "updatedAt": 1
    }
  }
}`)
	return paths, executable
}

func TestStoreRefreshGathersCommands(t *testing.T) {
	paths, _ := fixture(t)
	store := OpenStore(paths)
	if store.Loaded() {
		t.Error("a fresh store claims to be loaded")
	}

	notified := 0
	store.OnChange(func() { notified++ })

	store.Refresh(context.Background())
	if !store.Loaded() {
		t.Error("the store is not loaded after a refresh")
	}
	if notified != 1 {
		t.Errorf("listeners ran %d times, want 1", notified)
	}

	inventory := store.Inventory()
	if len(inventory.Integrations) != 2 {
		t.Fatalf("integrations = %+v", inventory.Integrations)
	}

	// Only the enabled integration's provider is asked.
	if _, ok := store.Description("io.github.vst93.v"); !ok {
		t.Error("the enabled provider's description is missing")
	}
	if _, ok := store.Description("dev.floter.static"); ok {
		t.Error("a disabled integration's provider was asked")
	}

	entries := store.CommandEntries()
	if len(entries) != 1 {
		t.Fatalf("commands = %+v", entries)
	}
	entry := entries[0]
	if entry.IntegrationID != "io.github.vst93.v" || entry.Command.ID != "jv" {
		t.Errorf("entry = %+v", entry)
	}
	if filepath.Base(entry.Program) != "v.sh" {
		t.Errorf("program = %q", entry.Program)
	}
	// The provider protocol prefix is not part of a launch: only the
	// execution's own prefix is.
	if len(entry.Args) != 1 || entry.Args[0] != "jv" {
		t.Errorf("args = %v", entry.Args)
	}
	if entry.Mode != "pty" || entry.Dir != "current" {
		t.Errorf("mode/dir = %q/%q", entry.Mode, entry.Dir)
	}
}

func TestStoreEnableDisableWritesAndReloads(t *testing.T) {
	paths, _ := fixture(t)
	store := OpenStore(paths)
	store.Refresh(context.Background())

	if err := store.SetEnabled("io.github.vst93.v", false); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEnabled("dev.floter.static", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEnabled("nope", true); !errors.Is(err, ErrNoIntegration) {
		t.Errorf("unknown id = %v, want ErrNoIntegration", err)
	}

	// The file says what we did, and the in-memory inventory followed.
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Extensions["io.github.vst93.v"].Enabled {
		t.Error("the disabled integration is still enabled in the file")
	}
	if !repo.Extensions["dev.floter.static"].Enabled {
		t.Error("the enabled integration is still disabled in the file")
	}
	integration, _ := store.Inventory().WithID("dev.floter.static")
	if !integration.Entry.Enabled || integration.Entry.State != "enabled" {
		t.Errorf("in-memory entry = %+v", integration.Entry)
	}
	// A disabled command is gone from the catalog.
	for _, entry := range store.CommandEntries() {
		if entry.IntegrationID == "io.github.vst93.v" {
			t.Error("a disabled integration still contributes commands")
		}
	}
}

func TestStoreRecordsProviderFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "bad.tool")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "bad.tool", "name": "Bad",
  "publisher": {"id": "x", "name": "x"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["bad"]},
  "provider": {"type": "executable", "argsPrefix": []}
}`)
	executable := filepath.Join(pkg, "bad.sh")
	writeFile(t, executable, "#!/bin/sh\necho 'no' >&2\nexit 2\n")
	os.Chmod(executable, 0o755)
	writeFile(t, paths.RepositoryFile, `{
  "schemaVersion": 1,
  "extensions": {
    "bad.tool": {
      "id": "bad.tool", "name": "Bad", "state": "enabled", "enabled": true,
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+executable+`",
      "channel": "stable", "installedAt": 1, "updatedAt": 1
    }
  }
}`)

	store := OpenStore(paths)
	store.Refresh(context.Background())
	if err := store.DescribeError("bad.tool"); err == nil {
		t.Error("a failing provider was not recorded")
	}
	if entries := store.CommandEntries(); len(entries) != 0 {
		t.Errorf("a failing provider contributed %+v", entries)
	}
}

// TestCommandEntriesCarryTheConfiguration covers the host-owned
// configuration: the stored values reach the command as environment and
// arguments, and a password reaches only the environment.
func TestCommandEntriesCarryTheConfiguration(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "dev.floter.configured", "name": "Configured",
  "publisher": {"id": "floter", "name": "floter"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["configured"]},
  "provider": {"type": "static-descriptor", "descriptor": "description.json", "argsPrefix": []}
}`)
	writeFile(t, filepath.Join(pkg, "description.json"), `{
  "protocolVersion": "1.0",
  "provider": {"id": "dev.floter.configured", "name": "Configured", "version": "1.0.0"},
  "commands": [{"id": "run", "name": "Run", "description": "Run it", "execution": {"program": "self", "argsPrefix": [], "mode": "pty"}}],
  "configuration": {
    "configVersion": 1,
    "owner": "host",
    "environmentMapping": {"endpoint": "TOOL_ENDPOINT"},
    "schema": [
      {"key": "endpoint", "type": "text"},
      {"key": "mode", "type": "select", "argument": "--mode", "options": ["pretty", "plain"]},
      {"key": "token", "type": "password", "envVar": "TOOL_TOKEN", "argument": "--token"}
    ]
  }
}`)
	tool := filepath.Join(dir, "configured-tool")
	writeFile(t, tool, "#!/bin/sh\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(paths.Data, "dev.floter.configured", "config.json"), `{
  "configVersion": 1,
  "values": {"endpoint": "https://example.com", "mode": "pretty", "token": "s3cret"}
}`)
	writeFile(t, paths.RepositoryFile, `{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.configured": {
      "id": "dev.floter.configured", "name": "Configured", "state": "enabled", "enabled": true,
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+tool+`", "channel": "stable",
      "installedAt": 1, "updatedAt": 1
    }
  }
}`)

	store := OpenStore(paths)
	store.Refresh(context.Background())
	entries := store.CommandEntries()
	if len(entries) != 1 {
		t.Fatalf("commands = %+v", entries)
	}
	entry := entries[0]
	wantEnv := []string{"TOOL_ENDPOINT=https://example.com", "TOOL_TOKEN=s3cret"}
	if len(entry.Env) != len(wantEnv) {
		t.Fatalf("env = %v, want %v", entry.Env, wantEnv)
	}
	for i, want := range wantEnv {
		if entry.Env[i] != want {
			t.Errorf("env[%d] = %q, want %q", i, entry.Env[i], want)
		}
	}
	if len(entry.Args) != 2 || entry.Args[0] != "--mode" || entry.Args[1] != "pretty" {
		t.Errorf("args = %v, want the configured mode only", entry.Args)
	}

	// A tool-owned configuration contributes a runnable Configuration
	// command instead.
	writeFile(t, filepath.Join(pkg, "description.json"), `{
  "protocolVersion": "1.0",
  "provider": {"id": "dev.floter.configured", "name": "Configured", "version": "1.0.0"},
  "commands": [{"id": "run", "name": "Run", "description": "Run it", "execution": {"program": "self", "argsPrefix": [], "mode": "pty"}}],
  "configuration": {"configVersion": 1, "owner": "tool", "openCommand": ["config", "edit"]}
}`)
	store.Refresh(context.Background())
	entries = store.CommandEntries()
	if len(entries) != 2 {
		t.Fatalf("commands = %+v", entries)
	}
	found := false
	for _, entry := range entries {
		if entry.Command.ID == "configuration" {
			found = true
			if len(entry.Args) != 2 || entry.Args[0] != "config" {
				t.Errorf("configuration command args = %v", entry.Args)
			}
		}
	}
	if !found {
		t.Error("the tool-owned configuration command is missing")
	}
}
