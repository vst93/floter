package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	// The package is where the host keeps its own generated integrations —
	// and it stays there: nothing is copied into the extensions directory, so
	// the generated integration is the same directory the old build's
	// `is_generated_custom_integration` looks for.
	packageDir := filepath.Join(paths.Data, entry.ID, "integration")
	if entry.ManifestPath != filepath.Join(packageDir, manifestFileName) {
		t.Errorf("the entry points at %q, want the data directory's package", entry.ManifestPath)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, entry.ID)); err == nil {
		t.Error("the package was also copied into the extensions directory")
	}
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

// R158 · a script integration writes the host's own internal description to
// both its manifest and its descriptor and never reads the request's own —
// which is why the form hides the description control in script mode. The
// user's words must not leak into a shared identity string that no UI shows.
func TestCustomScriptDescriptionIsInternal(t *testing.T) {
	paths := FromRoot(t.TempDir())
	defer StubInterpreterLookup(func(language string) (string, bool) {
		return "/usr/bin/sh", true
	})()

	entry, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name:           "Weather",
		Command:        "weather",
		Mode:           "script",
		ScriptLanguage: "shell",
		ScriptContent:  "#!/bin/sh\necho sunny\n",
		Description:    "Says the weather",
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	integration, ok := LoadInventory(paths).WithID(entry.ID)
	if !ok {
		t.Fatal("the integration does not load")
	}
	if integration.Manifest.Description != "Local script integration" {
		t.Errorf("the script manifest carried the user's words: %q", integration.Manifest.Description)
	}
	data, readErr := os.ReadFile(filepath.Join(integration.PackageDir(), "provider-description.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	descriptor, parseErr := ParseDescription(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if descriptor.Provider.Description != "Local script integration" {
		t.Errorf("the descriptor provider carried %q", descriptor.Provider.Description)
	}
	if descriptor.Commands[0].Description != "Local script integration" {
		t.Errorf("the descriptor command carried %q", descriptor.Commands[0].Description)
	}
}

// R159 · an executable integration stores the user's own words, trimmed, and a
// blank stays honestly blank. The old build collapsed a blank into a generated
// `Local integration for <exe>` sentence, which landed on disk, was read back
// as the user's content, and therefore could never be cleared.
func TestCustomExecutableDescriptionStaysHonest(t *testing.T) {
	blankPaths, blankProgram := customToolFixture(t)
	blank, err := CreateCustom(context.Background(), blankPaths, CustomRequest{
		Name: "Blank", Command: "blank", ExecutablePath: blankProgram, Description: "   ",
	}, true)
	if err != nil {
		t.Fatalf("create blank: %v", err)
	}
	blankIntegration, ok := LoadInventory(blankPaths).WithID(blank.ID)
	if !ok {
		t.Fatal("the blank integration does not load")
	}
	if blankIntegration.Manifest.Description != "" {
		t.Errorf("a blank description became %q", blankIntegration.Manifest.Description)
	}

	keptPaths, keptProgram := customToolFixture(t)
	kept, err := CreateCustom(context.Background(), keptPaths, CustomRequest{
		Name: "Kept", Command: "kept", ExecutablePath: keptProgram, Description: "  a gadget of my own  ",
	}, true)
	if err != nil {
		t.Fatalf("create kept: %v", err)
	}
	keptIntegration, ok := LoadInventory(keptPaths).WithID(kept.ID)
	if !ok {
		t.Fatal("the kept integration does not load")
	}
	if keptIntegration.Manifest.Description != "a gadget of my own" {
		t.Errorf("the description read %q, want it trimmed", keptIntegration.Manifest.Description)
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

// A connection may declare the tool's inputs: they travel into the manifest,
// and a bad declaration is refused before any package is written.
func TestCreateCustomCarriesParams(t *testing.T) {
	paths, program := customToolFixture(t)
	entry, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name: "My Gadget", Command: "gadget", ExecutablePath: program,
		Params: []ParamDefinition{
			{ID: "target", Label: "Target", Kind: ParamText, Flag: "--target"},
			{ID: "verbose", Kind: ParamBoolean, Flag: "--verbose"},
		},
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	integration, ok := LoadInventory(paths).WithID(entry.ID)
	if !ok {
		t.Fatal("the integration does not load")
	}
	if len(integration.Manifest.Params) != 2 {
		t.Fatalf("params = %+v", integration.Manifest.Params)
	}
	if integration.Manifest.Params[0].Flag != "--target" {
		t.Errorf("the flag did not survive: %+v", integration.Manifest.Params[0])
	}
	// A bad declaration is refused before anything is written.
	if _, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name: "Bad", ExecutablePath: program,
		Params: []ParamDefinition{{ID: "a", Kind: "colour"}},
	}, true); err == nil {
		t.Error("a bad declaration was accepted")
	}
}

// Editing a generated integration keeps its identity, rebuilds the manifest
// from the request, and re-derives the command list; publisher content is
// never rewritten.
func TestUpdateCustomTool(t *testing.T) {
	paths, program := customToolFixture(t)
	entry, err := CreateCustom(context.Background(), paths, CustomRequest{
		Name: "My Gadget", Command: "gadget", ExecutablePath: program,
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Rewrite it: a new name, a new argument prefix, a declared input.
	updated, err := UpdateCustom(context.Background(), paths, entry.ID, CustomRequest{
		Name:           "My Gadget Renamed",
		Command:        "gadget",
		ExecutablePath: program,
		ArgsPrefix:     []string{"--floter"},
		Description:    "renamed",
		Params:         []ParamDefinition{{ID: "target", Kind: ParamText, Flag: "--target"}},
	}, true)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != entry.ID {
		t.Errorf("the identity changed: %q -> %q", entry.ID, updated.ID)
	}
	integration, ok := LoadInventory(paths).WithID(entry.ID)
	if !ok {
		t.Fatal("the integration does not load")
	}
	if integration.Name != "My Gadget Renamed" || len(integration.Manifest.Provider.ArgsPrefix) != 1 {
		t.Errorf("integration = %+v", integration.Manifest)
	}
	if len(integration.Manifest.Params) != 1 {
		t.Errorf("params = %+v", integration.Manifest.Params)
	}
	// The descriptor was re-derived: the plugin the help lists is still there.
	data, readErr := os.ReadFile(filepath.Join(integration.PackageDir(), "provider-description.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	descriptor, parseErr := ParseDescription(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if len(descriptor.Commands) != 2 {
		t.Errorf("commands = %+v", descriptor.Commands)
	}
	// Only a generated integration may be edited.
	repo, _ := LoadRepository(paths.RepositoryFile)
	repoEntry := repo.Extensions[entry.ID]
	repoEntry.PublisherID = "vst"
	repo.Extensions[entry.ID] = repoEntry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateCustom(context.Background(), paths, entry.ID, CustomRequest{
		Name: "Nope", ExecutablePath: program,
	}, true); err == nil {
		t.Error("a publisher integration was edited")
	}
}

// The `output` mode a custom tool is created with follows the schema. There is
// no `output` field on the request's default, and a manifest that keeps the
// default must not spell it out — the same omission a manifest written before
// the field existed has, and the same rule the old host's `skip_serializing_if`
// applied. A declared `terminal` is the only mode that reaches the disk, and it
// is the only one that routes a run through a PTY.
func TestCustomToolOutputSpelling(t *testing.T) {
	background, program := customToolFixture(t)
	entry, err := CreateCustom(context.Background(), background, CustomRequest{
		Name: "Quiet", Command: "quiet", ExecutablePath: program,
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	raw, err := os.ReadFile(entry.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"output"`) {
		t.Errorf("a manifest keeping the default wrote an output field:\n%s", raw)
	}
	manifest, err := LoadManifest(entry.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Route(); got != RouteBackground {
		t.Errorf("default route = %q, want %q", got, RouteBackground)
	}

	terminal, terminalProgram := customToolFixture(t)
	termEntry, err := CreateCustom(context.Background(), terminal, CustomRequest{
		Name: "Loud", Command: "loud", ExecutablePath: terminalProgram, Output: RouteTerminal,
	}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	termManifest, err := LoadManifest(termEntry.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := termManifest.Route(); got != RouteTerminal {
		t.Errorf("declared terminal route = %q, want %q", got, RouteTerminal)
	}
	termRaw, err := os.ReadFile(termEntry.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(termRaw), `"output": "terminal"`) {
		t.Errorf("a declared terminal was not written:\n%s", termRaw)
	}
}
