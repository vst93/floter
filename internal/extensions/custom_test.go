package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// customToolFixture is a runnable tool on this machine: it answers --version
// and --help, and its help lists one plugin.
func customToolFixture(t *testing.T) (Paths, string) {
	t.Helper()
	if os.Getenv("GOOS") == "windows" {
		t.Skip("the fixture is a shell script")
	}
	dir := t.TempDir()
	program := filepath.Join(dir, "mygadget")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "mygadget 2.3.1"
elif [ "$1" = "--help" ]; then
  cat <<'ROOT'
mygadget - Gadgets
Available Plugins
==================================================
📦 one 1.0.0  (aliases: o)
  the first gadget
ROOT
elif [ "$2" = "--help" ]; then
  echo "Options:"
  echo "  -x    do x"
else
  exit 3
fi
`
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := FromRoot(filepath.Join(dir, "root"))
	return paths, program
}

// Connecting a local tool writes its package, derives its commands from its
// own help, records the version it reports, and installs it the same way any
// local package is installed.
func TestCreateCustomTool(t *testing.T) {
	paths, program := customToolFixture(t)
	entry, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name:           "My Gadget",
		Command:        "gadget",
		ExecutablePath: program,
		VersionArgs:    []string{"--version"},
		Description:    "a gadget of my own",
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if entry.ID == "" || entry.Name != "My Gadget" {
		t.Fatalf("entry = %+v", entry)
	}
	// The package is where the host keeps its own generated integrations.
	packageDir := filepath.Join(paths.Data, entry.ID, "integration")
	for _, name := range []string{manifestFileName, "package.json", "provider-description.json"} {
		if _, err := os.Stat(filepath.Join(packageDir, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}
	// The descriptor carries the root command and the plugin the help listed.
	data, err := os.ReadFile(filepath.Join(packageDir, "provider-description.json"))
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := ParseDescription(data)
	if err != nil {
		t.Fatalf("the descriptor does not parse: %v", err)
	}
	if len(descriptor.Commands) != 2 {
		t.Fatalf("commands = %+v", descriptor.Commands)
	}
	if descriptor.Commands[0].ID != "gadget" || descriptor.Commands[0].Name != "My Gadget" {
		t.Errorf("root = %+v", descriptor.Commands[0])
	}
	if sub := descriptor.Commands[1]; sub.ID != "one" || sub.Description != "the first gadget" {
		t.Errorf("sub = %+v", sub)
	}
	// The version the tool reported is recorded, so the drift re-probe has
	// something to compare against.
	if entry.ToolVersion == nil || *entry.ToolVersion != "2.3.1" {
		t.Errorf("tool version = %v", entry.ToolVersion)
	}
	// The integration loads, and its manifest names the program.
	integration, ok := LoadInventory(paths).WithID(entry.ID)
	if !ok {
		t.Fatal("the integration does not load")
	}
	if integration.Manifest.Runtime.Type != "system" || integration.Name != "My Gadget" {
		t.Errorf("integration = %+v", integration.Manifest.Runtime)
	}
	// A second connection mints a different id: two tools never fight over a
	// name.
	second, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name: "My Gadget Again", Command: "gadget2", ExecutablePath: program,
	}, true)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if second.ID == entry.ID {
		t.Errorf("the second connection reused the id %q", second.ID)
	}
}

// A request that cannot be honoured says why and writes nothing.
func TestCreateCustomRefusesBadRequests(t *testing.T) {
	paths, program := customToolFixture(t)
	cases := []struct {
		name    string
		request CustomRequest
	}{
		{"no name", CustomRequest{ExecutablePath: program}},
		{"no program", CustomRequest{Name: "Gadget"}},
		{"a program that is not there", CustomRequest{Name: "Gadget", ExecutablePath: filepath.Join(t.TempDir(), "gone")}},
		{"a script with no language", CustomRequest{Name: "Gadget", Mode: "script", ScriptContent: "echo hi"}},
		{"an empty script", CustomRequest{Name: "Gadget", Mode: "script", ScriptLanguage: "shell"}},
		{"an unknown mode", CustomRequest{Name: "Gadget", Mode: "wormhole", ExecutablePath: program}},
		{"an unknown permission", CustomRequest{Name: "Gadget", ExecutablePath: program, Permissions: []string{"mind-reading"}}},
	}
	for _, c := range cases {
		if _, err := CreateCustom(context.Background(), paths, c.request, true); err == nil {
			t.Errorf("%s: the request was accepted", c.name)
		}
	}
	// Nothing was written.
	entries, err := os.ReadDir(paths.Data)
	if err == nil && len(entries) != 0 {
		// A rejected request may leave the reserved directory removed; any
		// package that stayed would be a leak.
		for _, entry := range entries {
			if _, err := os.Stat(filepath.Join(paths.Data, entry.Name(), "integration", "provider-description.json")); err == nil {
				t.Errorf("%s left a package behind", entry.Name())
			}
		}
	}
}

// A package whose permissions are not approved is not recorded, and the
// refusal is the one a caller can act on.
func TestCreateCustomNeedsApproval(t *testing.T) {
	paths, program := customToolFixture(t)
	_, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name:           "My Gadget",
		ExecutablePath: program,
		Permissions:    []string{"environment"},
	}, false)
	if err == nil {
		t.Fatal("an unapproved package was installed")
	}
	if !errors.Is(err, ErrPermissionApprovalRequired) {
		t.Errorf("error = %v", err)
	}
	// The package files survive, so the review can be repeated.
	entries, readErr := os.ReadDir(paths.Data)
	if readErr != nil || len(entries) != 1 {
		t.Fatalf("entries = %v (%v)", entries, readErr)
	}
}
