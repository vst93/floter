package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reprobeFixture installs a generated custom integration: the publisher is
// the local user, the provider a static descriptor the host wrote, and the
// package dir the connect flow's `integration` dir. The tool answers
// --help with one plugin whose own help names one flag.
func reprobeFixture(t *testing.T, toolScript string) (Paths, string) {
	t.Helper()
	if hasWindowsShell(t) {
		t.Skip("the fixture is a shell script")
	}
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.tool", "integration")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.tool", "name": "Tool",
	  "runtime": {"type": "system", "executableNames": ["tool.sh"]},
	  "provider": {"type": "static-descriptor", "descriptor": "provider-description.json"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(pkg, "tool.sh")
	if err := os.WriteFile(executable, []byte(toolScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "provider-description.json"), []byte(`{
	  "protocolVersion": "1.0",
	  "provider": {"id": "dev.floter.tool", "name": "Tool", "version": "1.0.0"},
	  "commands": [{"id": "tool", "name": "Tool", "description": "root",
	    "execution": {"program": "self", "argsPrefix": [], "mode": "pty"}}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := `{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.tool": {"id": "dev.floter.tool", "name": "Tool",
	    "publisherId": "local-user", "providerKind": "static-descriptor",
	    "state": "enabled", "enabled": true,
	    "manifestPath": "` + filepath.Join(pkg, manifestFileName) + `",
	    "executablePath": "` + executable + `",
	    "installedAt": 1, "updatedAt": 1}}
	}`
	if err := os.WriteFile(paths.RepositoryFile, []byte(repository), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, pkg
}

// hasWindowsShell skips a script fixture where no shell answers.
func hasWindowsShell(t *testing.T) bool {
	t.Helper()
	return os.Getenv("GOOS") == "windows"
}

// The re-probe rebuilds the command list from the tool's own help: the
// root keeps its identity, the subcommand joins as a runnable command with
// its probed flags, and the sidecar records the sizes.
func TestReprobeCommandsRebuildsTheDescriptor(t *testing.T) {
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  cat <<'ROOT'
tool - Gadgets
Available Plugins
==================================================
📦 json2excel 0.0.1 👤 vst  (aliases: j2e)
  convert json data to excel file
ROOT
elif [ "$2" = "--help" ]; then
  cat <<'SUB'
Options:
  -o, --output <FILE>    Write result to FILE
SUB
else
  exit 3
fi
`
	paths, pkg := reprobeFixture(t, script)
	report, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool")
	if err != nil {
		t.Fatalf("reprobe: %v", err)
	}
	if report.CommandCount != 2 || report.RootArguments != 0 || report.Subcommands != 1 {
		t.Errorf("report = %+v", report)
	}
	// The descriptor carries both commands.
	data, err := os.ReadFile(filepath.Join(pkg, "provider-description.json"))
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := ParseDescription(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(descriptor.Commands) != 2 || descriptor.Commands[0].ID != "tool" {
		t.Fatalf("commands = %+v", descriptor.Commands)
	}
	sub := descriptor.Commands[1]
	if sub.ID != "json2excel" || sub.Description != "convert json data to excel file" {
		t.Errorf("sub = %+v", sub)
	}
	if len(sub.Aliases) != 1 || sub.Aliases[0] != "j2e" {
		t.Errorf("aliases = %v", sub.Aliases)
	}
	if len(sub.Arguments) != 1 || sub.Arguments[0].Names[0] != "-o" || sub.Arguments[0].ValueHint != "FILE" {
		t.Errorf("arguments = %+v", sub.Arguments)
	}
	if len(sub.Execution.ArgsPrefix) != 1 || sub.Execution.ArgsPrefix[0] != "json2excel" {
		t.Errorf("args prefix = %v", sub.Execution.ArgsPrefix)
	}
	// The sidecar records the derivation.
	record := readHelpProbeRecord(pkg)
	if record == nil || record.CommandCount == nil || *record.CommandCount != 2 {
		t.Errorf("record = %+v", record)
	}
	if record.PreviousCommandCount != nil {
		t.Errorf("a first probe has no previous count: %+v", record)
	}
}

// A second re-probe compares against the sidecar it replaces.
func TestReprobeCommandsComparesWithThePreviousSidecar(t *testing.T) {
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "tool - Gadgets"
  echo "Available Plugins"
  echo "📦 one 1.0"
  echo "📦 two 1.0"
elif [ "$2" = "--help" ]; then
  echo "Options:"
  echo "  -x    do x"
else
  exit 3
fi
`
	paths, _ := reprobeFixture(t, script)
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err != nil {
		t.Fatal(err)
	}
	report, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool")
	if err != nil {
		t.Fatal(err)
	}
	if report.PreviousCommandCount == nil || *report.PreviousCommandCount != 3 {
		t.Errorf("previous = %v, want 3 (root + two)", report.PreviousCommandCount)
	}
	if report.CommandCount != 3 {
		t.Errorf("count = %d", report.CommandCount)
	}
}

// A tool that answers nothing usable keeps the previous descriptor: the
// re-probe errors and the file is untouched.
func TestReprobeCommandsKeepsTheDescriptorWhenTheToolGoesQuiet(t *testing.T) {
	script := `#!/bin/sh
exit 0
`
	paths, pkg := reprobeFixture(t, script)
	_ = pkg
	before, err := os.ReadFile(filepath.Join(pkg, "provider-description.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err == nil {
		t.Fatal("a quiet tool re-probed cleanly")
	}
	after, err := os.ReadFile(filepath.Join(pkg, "provider-description.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the descriptor changed behind a failed re-probe")
	}
	if _, err := os.Stat(filepath.Join(pkg, ".provider-description.reprobing.tmp")); !os.IsNotExist(err) {
		t.Error("the staged descriptor was left behind")
	}
}

// A publisher's integration is never re-probed: the host would overwrite
// shipped content with its own derivation.
func TestReprobeCommandsRefusesAPublisherIntegration(t *testing.T) {
	paths, _ := reprobeFixture(t, "#!/bin/sh\necho hi\n")
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions["dev.floter.tool"]
	entry.PublisherID = "vst"
	repo.Extensions["dev.floter.tool"] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err == nil {
		t.Fatal("a publisher integration was re-probed")
	} else if !strings.Contains(err.Error(), "not generated") {
		t.Errorf("error = %v", err)
	}
}
