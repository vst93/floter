package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestStaticCompletions(t *testing.T) {
	command := Command{
		ID:   "jv",
		Name: "JSON Viewer",
		Arguments: []Argument{
			{Names: []string{"-f", "--format"}, Kind: "flag", Description: "Format JSON"},
			{Names: []string{"-file"}, Kind: "path", TakesValue: true, Description: "Read from file"},
			{Names: []string{"-mode"}, Kind: "enum", TakesValue: true, Description: "Output mode", Values: []string{"pretty", "plain"}},
			{Names: []string{"-dir"}, Kind: "directory", TakesValue: true},
		},
	}

	// A bare fragment offers the argument names.
	got := StaticCompletions(command, []string{"-"}, "")
	labels := completionLabels(got)
	if len(labels) != 5 || labels[0] != "--format" {
		t.Errorf("flags = %v", labels)
	}
	// A prefix narrows them.
	if got := completionLabels(StaticCompletions(command, []string{"--f"}, "")); !reflect.DeepEqual(got, []string{"--format"}) {
		t.Errorf("prefix = %v", got)
	}
	// After an enum argument, its values.
	// Enum values keep the order the provider declared them in.
	if got := completionLabels(StaticCompletions(command, []string{"-mode", "p"}, "")); !reflect.DeepEqual(got, []string{"pretty", "plain"}) {
		t.Errorf("enum values = %v", got)
	}
	if got := completionLabels(StaticCompletions(command, []string{"-mode", "x"}, "")); len(got) != 0 {
		t.Errorf("enum miss = %v", got)
	}
	// A command-valued argument is the provider's business, not static.
	if got := StaticCompletions(command, []string{"-file", "x"}, ""); len(got) != 0 {
		// -file is a path, so it lists the file system; the assertion is
		// about the command kind below.
		_ = got
	}
	command.Arguments = append(command.Arguments, Argument{Names: []string{"-run"}, Kind: "command", TakesValue: true})
	if got := StaticCompletions(command, []string{"-run", ""}, ""); len(got) != 0 {
		t.Errorf("a command argument gave static completions: %v", got)
	}

	// Path completions list the directory's entries.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.json"), "{}")
	writeFile(t, filepath.Join(dir, "beta.json"), "{}")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = StaticCompletions(command, []string{"-file", "a"}, dir)
	if len(got) != 1 || got[0].Label != "alpha.json" {
		t.Errorf("path completions = %v", completionLabels(got))
	}
	got = StaticCompletions(command, []string{"-dir", ""}, dir)
	labels = completionLabels(got)
	if len(labels) != 1 || labels[0] != "nested"+string(filepath.Separator) {
		t.Errorf("directory completions = %v", labels)
	}
	if got[0].Kind != "directory" {
		t.Errorf("kind = %q", got[0].Kind)
	}
	// A path that does not exist gives nothing rather than an error.
	if got := StaticCompletions(command, []string{"-file", ""}, filepath.Join(dir, "missing")); len(got) != 0 {
		t.Errorf("missing directory = %v", completionLabels(got))
	}
}

func completionLabels(items []Completion) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}

func TestMergeCompletions(t *testing.T) {
	static := []Completion{{Label: "-f"}, {Label: "-file"}}
	dynamic := []Completion{{Label: "alpha", Kind: "command"}, {Label: "-f", Kind: "flag"}}

	got := MergeCompletions(static, dynamic)
	labels := completionLabels(got)
	if !reflect.DeepEqual(labels, []string{"alpha", "-f", "-file"}) {
		t.Errorf("merged = %v", labels)
	}
	if got[1].Kind != "flag" {
		t.Errorf("the dynamic entry did not win: %+v", got[1])
	}
	if empty := MergeCompletions(static, nil); len(empty) != 2 {
		t.Errorf("no dynamic entries = %v", completionLabels(empty))
	}
}

// TestCompleteAndDiagnose runs a fake provider: the protocol argv, the
// request on stdin, the answers, and the failure that degrades.
func TestCompleteAndDiagnose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "dev.floter.completer")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "dev.floter.completer", "name": "Completer",
  "runtime": {"type": "system", "executableNames": ["completer"]},
  "provider": {"type": "executable", "argsPrefix": ["--floter"], "completeTimeoutMs": 500}
}`)
	executable := filepath.Join(pkg, "completer.sh")
	writeFile(t, executable, `#!/bin/sh
case "$*" in
  "--floter complete --protocol 1.0")
    request=$(cat)
    case "$request" in
      *'"-run"'*'"alp"'*) echo '{"completions":[{"label":"alpha","kind":"command","detail":"Alpha"}]}' ;;
      *) echo '{"completions":[]}' ;;
    esac
    ;;
  "--floter diagnose --protocol 1.0")
    echo '{"status":"ok","checks":[{"id":"tool","status":"ok","message":"found"}]}'
    ;;
  *)
    echo "unexpected: $*" >&2
    exit 9
    ;;
esac
`)
	if err := os.Chmod(executable, 0o755); err != nil {
		t.Fatal(err)
	}

	integration := Integration{
		Entry:    Entry{ID: "dev.floter.completer", ExecutablePath: executable},
		Paths:    paths,
		Manifest: mustManifest(t, filepath.Join(pkg, manifestFileName)),
	}

	got, err := Complete(context.Background(), integration, CompletionRequest{
		Command: "run", Tokens: []string{"-run", "alp"}, CWD: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "alpha" || got[0].Kind != "command" {
		t.Errorf("completions = %+v", got)
	}

	// The other request shape answers with nothing rather than an error.
	got, err = Complete(context.Background(), integration, CompletionRequest{Command: "run", Tokens: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("unexpected completions = %+v", got)
	}

	diagnosis, err := Diagnose(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if diagnosis.Status != "ok" || len(diagnosis.Checks) != 1 || diagnosis.Checks[0].ID != "tool" {
		t.Errorf("diagnosis = %+v", diagnosis)
	}

	// A provider that does not implement the operation reports it, and the
	// caller degrades.
	writeFile(t, executable, "#!/bin/sh\necho 'complete unsupported' >&2\nexit 7\n")
	os.Chmod(executable, 0o755)
	if _, err := Complete(context.Background(), integration, CompletionRequest{Command: "x"}); err == nil {
		t.Error("a failing provider was accepted")
	} else {
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) || providerErr.ExitCode != 7 {
			t.Errorf("error = %v", err)
		}
	}
	// A malformed answer is an error too.
	writeFile(t, executable, "#!/bin/sh\nprintf 'not json'\n")
	os.Chmod(executable, 0o755)
	if _, err := Complete(context.Background(), integration, CompletionRequest{Command: "x"}); err == nil {
		t.Error("a malformed answer was accepted")
	}
}

func TestCompleteTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "dev.floter.slow")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "dev.floter.slow", "name": "Slow",
  "runtime": {"type": "system", "executableNames": ["slow"]},
  "provider": {"type": "executable", "argsPrefix": [], "completeTimeoutMs": 100}
}`)
	executable := filepath.Join(pkg, "slow.sh")
	writeFile(t, executable, "#!/bin/sh\nsleep 5\necho '{\"completions\":[]}'\n")
	os.Chmod(executable, 0o755)

	integration := Integration{
		Entry:    Entry{ID: "dev.floter.slow", ExecutablePath: executable},
		Paths:    paths,
		Manifest: mustManifest(t, filepath.Join(pkg, manifestFileName)),
	}
	if _, err := Complete(context.Background(), integration, CompletionRequest{Command: "x"}); err == nil {
		t.Error("a hanging provider was accepted")
	}
}

func mustManifest(t *testing.T, path string) Manifest {
	t.Helper()
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
