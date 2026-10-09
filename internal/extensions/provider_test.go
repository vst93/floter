package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

const exampleDescription = `{
  "protocolVersion": "1.0",
  "provider": {
    "id": "io.github.vst93.v",
    "name": "V Tools",
    "version": "0.0.12",
    "description": "Gadgets under the terminal"
  },
  "commands": [
    {
      "id": "jv",
      "name": "JSON Viewer",
      "description": "View, format and edit JSON",
      "aliases": [],
      "keywords": ["json", "format"],
      "execution": {
        "program": "self",
        "argsPrefix": ["jv"],
        "mode": "pty",
        "workingDirectory": "current"
      },
      "arguments": [
        { "names": ["-f"], "kind": "flag", "description": "Format JSON" },
        { "names": ["-file"], "kind": "path", "takesValue": true, "description": "Read from file" }
      ]
    }
  ]
}`

func TestParseDescription(t *testing.T) {
	d, err := ParseDescription([]byte(exampleDescription))
	if err != nil {
		t.Fatal(err)
	}
	if d.Provider.ID != "io.github.vst93.v" || d.Provider.Version != "0.0.12" {
		t.Errorf("provider = %+v", d.Provider)
	}
	if len(d.Commands) != 1 {
		t.Fatalf("commands = %+v", d.Commands)
	}
	command := d.Commands[0]
	if command.ID != "jv" || command.Name != "JSON Viewer" {
		t.Errorf("command = %+v", command)
	}
	if command.Execution.NormalizedMode() != "pty" {
		t.Errorf("mode = %q", command.Execution.NormalizedMode())
	}
	if !reflect.DeepEqual(command.Execution.ArgsPrefix, []string{"jv"}) {
		t.Errorf("argsPrefix = %v", command.Execution.ArgsPrefix)
	}
	if len(command.Arguments) != 2 || command.Arguments[1].Kind != "path" || !command.Arguments[1].TakesValue {
		t.Errorf("arguments = %+v", command.Arguments)
	}

	// `capture` is the legacy spelling of `pty`.
	d, err = ParseDescription([]byte(`{"protocolVersion":"1.0","provider":{"id":"a.b","name":"A"},"commands":[{"id":"c","name":"C","execution":{"mode":"capture","argsPrefix":[]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Commands[0].Execution.NormalizedMode(); got != "pty" {
		t.Errorf("capture normalized to %q", got)
	}

	// Wrong protocol, missing identity and malformed JSON are all errors.
	for _, payload := range []string{
		`{"protocolVersion":"2.0","provider":{"id":"a.b","name":"A"},"commands":[]}`,
		`{"protocolVersion":"1.0","provider":{"id":"","name":""},"commands":[]}`,
		`{"protocolVersion":"1.0","provider":{"id":"a.b","name":"A"},"commands":[{"id":"","name":""}]}`,
		`{not json`,
	} {
		if _, err := ParseDescription([]byte(payload)); err == nil {
			t.Errorf("ParseDescription(%s) accepted a bad payload", payload)
		}
	}
}

func TestDescribeStaticDescriptor(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := paths.Extensions + "/io.github.vst93.v"
	writeFile(t, filepath.Join(pkg, manifestFileName), exampleManifest)
	writeFile(t, filepath.Join(pkg, "description.json"), exampleDescription)

	integration := Integration{
		Entry: Entry{ID: "io.github.vst93.v", ManifestPath: filepath.Join(pkg, manifestFileName)},
		Paths: paths,
	}
	manifest, err := LoadManifest(integration.Entry.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Provider.Type = "static-descriptor"
	manifest.Provider.Descriptor = "description.json"
	integration.Manifest = manifest

	d, err := Describe(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 1 || d.Commands[0].ID != "jv" {
		t.Errorf("commands = %+v", d.Commands)
	}

	// A descriptor that points outside the package is refused.
	manifest.Provider.Descriptor = "../outside.json"
	integration.Manifest = manifest
	if _, err := Describe(context.Background(), integration); err == nil {
		t.Error("a descriptor outside the package was read")
	}

	// So is a static provider with no descriptor at all.
	manifest.Provider.Descriptor = ""
	integration.Manifest = manifest
	if _, err := Describe(context.Background(), integration); err == nil {
		t.Error("a static provider with no descriptor loaded")
	}
}

// writeProvider writes a shell script that answers describe with payload (or
// fails) and marks it executable.
func writeProvider(t *testing.T, dir, payload string, exit int, sleep string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	path := filepath.Join(dir, "provider.sh")
	script := "#!/bin/sh\n"
	if sleep != "" {
		script += "sleep " + sleep + "\n"
	}
	if payload != "" {
		script += "cat <<'JSONEOF'\n" + payload + "\nJSONEOF\n"
	}
	if exit != 0 {
		script += "echo 'provider exploded' >&2\n"
	}
	script += "exit " + itoa(exit) + "\n"
	writeFile(t, path, script)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func providerIntegration(t *testing.T, executable string, timeoutMs int) Integration {
	t.Helper()
	dir := t.TempDir()
	paths := FromRoot(dir)
	manifest := Manifest{
		ID:   "io.github.vst93.v",
		Name: "V Tools",
		Runtime: Runtime{
			Type:            "system",
			ExecutableNames: []string{"v-not-installed-anywhere"},
		},
		Provider: Provider{Type: "executable", ArgsPrefix: []string{"--floter"}, DescribeTimeoutMs: timeoutMs},
	}
	return Integration{
		Entry:    Entry{ID: "io.github.vst93.v", ExecutablePath: executable},
		Paths:    paths,
		Manifest: manifest,
	}
}

func TestDescribeRunsTheProvider(t *testing.T) {
	dir := t.TempDir()
	executable := writeProvider(t, dir, exampleDescription, 0, "")
	integration := providerIntegration(t, executable, 5000)

	// The argv reaches the provider: a script that only answers with the
	// protocol flag would prove nothing, so the fake checks it.
	writeFile(t, executable, "#!/bin/sh\n"+
		`case "$*" in
  "--floter describe --protocol 1.0") cat <<'JSONEOF'
`+exampleDescription+`
JSONEOF
    ;;
  *) echo "unexpected args: $*" >&2; exit 3 ;;
esac
`)

	d, err := Describe(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if d.Provider.Name != "V Tools" || len(d.Commands) != 1 {
		t.Errorf("description = %+v", d)
	}
}

func TestDescribeReportsAFailingProvider(t *testing.T) {
	dir := t.TempDir()
	executable := writeProvider(t, dir, "", 2, "")
	integration := providerIntegration(t, executable, 5000)

	_, err := Describe(context.Background(), integration)
	if err == nil {
		t.Fatal("a failing provider was accepted")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %T, want *ProviderError", err)
	}
	if providerErr.ExitCode != 2 || providerErr.Stderr == "" {
		t.Errorf("provider error = %+v", providerErr)
	}
}

func TestDescribeTimesOut(t *testing.T) {
	dir := t.TempDir()
	executable := writeProvider(t, dir, exampleDescription, 0, "5")
	integration := providerIntegration(t, executable, 100)

	start := time.Now()
	_, err := Describe(context.Background(), integration)
	if err == nil {
		t.Fatal("a hanging provider was accepted")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("the timeout took %s", elapsed)
	}
}

func TestResolveRuntime(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "a.script")
	writeFile(t, filepath.Join(pkg, "run.sh"), "#!/bin/sh\n")
	script := filepath.Join(pkg, "run.sh")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}

	integration := Integration{
		Entry: Entry{ID: "a.script"},
		Paths: paths,
		Manifest: Manifest{
			ID:      "a.script",
			Name:    "Script",
			Runtime: Runtime{Type: "script", Language: "shell", Path: "run.sh"},
		},
	}
	if runtime.GOOS == "windows" {
		t.Skip("no sh at the expected place")
	}
	binding, err := ResolveRuntime(integration)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(binding.Program) != "sh" {
		t.Errorf("interpreter = %q", binding.Program)
	}
	if !reflect.DeepEqual(binding.Args, []string{script}) {
		t.Errorf("args = %v", binding.Args)
	}

	// A script path that leaves the package is refused.
	integration.Manifest.Runtime.Path = "../escape.sh"
	if _, err := ResolveRuntime(integration); err == nil {
		t.Error("an escaping script path resolved")
	}

	// A compiled language has no interpreter here.
	integration.Manifest.Runtime.Language = "go"
	if _, err := ResolveRuntime(integration); err == nil {
		t.Error("a go script resolved without an artifact")
	}

	// An unknown runtime type is an error, not a guess.
	integration.Manifest.Runtime = Runtime{Type: "magic"}
	if _, err := ResolveRuntime(integration); err == nil {
		t.Error("an unknown runtime resolved")
	}
}

func TestInterpreterNamesAndLookingUp(t *testing.T) {
	if names := InterpreterNames("python"); names[0] != "python3" {
		t.Errorf("python names = %v", names)
	}
	if names := InterpreterNames("go"); names != nil {
		t.Errorf("go names = %v, want none", names)
	}
	if got := InterpreterArgs("powershell", "x.ps1"); !reflect.DeepEqual(got, []string{"-File", "x.ps1"}) {
		t.Errorf("powershell args = %v", got)
	}
	if got := InterpreterArgs("python", "x.py"); !reflect.DeepEqual(got, []string{"x.py"}) {
		t.Errorf("python args = %v", got)
	}

	dir := t.TempDir()
	tool := filepath.Join(dir, "some-tool")
	writeFile(t, tool, "#!/bin/sh\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	found, ok := lookToolIn([]string{dir}, "nope", "some-tool")
	if !ok || found != tool {
		t.Errorf("lookToolIn = %q, %v", found, ok)
	}
	if _, ok := lookToolIn([]string{dir}, "nope"); ok {
		t.Error("a missing tool was found")
	}

	// The search path always has the process entries first and never
	// duplicates one.
	dirs := SearchDirectories()
	seen := map[string]bool{}
	for _, d := range dirs {
		if seen[d] {
			t.Errorf("duplicate directory %q in the search path", d)
		}
		seen[d] = true
	}
	if len(dirs) == 0 {
		t.Error("the search path is empty")
	}
}
