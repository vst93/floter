package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CompletionRequest is what the provider's complete operation is asked, as
// catalog.rs sent it: the command, the tokens typed so far, and where.
type CompletionRequest struct {
	Command string   `json:"command"`
	Tokens  []string `json:"tokens"`
	CWD     string   `json:"cwd,omitempty"`
}

// Completion is one suggestion the command mode can offer.
type Completion struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Completer is a value a provider answered with.
type completionResponse struct {
	Completions []Completion `json:"completions"`
}

// Diagnosis is a provider's diagnose answer: its status and what it checked.
type Diagnosis struct {
	Status string           `json:"status"`
	Checks []DiagnosisCheck `json:"checks"`
}

// DiagnosisCheck is one thing a provider checked.
type DiagnosisCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Complete asks a provider for completions of the tokens typed so far. A
// provider that does not implement the operation (or answers too slowly) is
// an error the caller degrades from, never a failure of the command mode.
func Complete(ctx context.Context, integration Integration, request CompletionRequest) ([]Completion, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(integration.Manifest.CompleteTimeout()) * time.Millisecond
	out, err := runOperation(ctx, integration, OpComplete, payload, timeout)
	if err != nil {
		return nil, err
	}
	var response completionResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, fmt.Errorf("extensions: invalid completion answer: %w", err)
	}
	return response.Completions, nil
}

// Diagnose runs a provider's diagnose operation: what it says about its own
// health, for the details list.
func Diagnose(ctx context.Context, integration Integration) (Diagnosis, error) {
	out, err := runOperation(ctx, integration, OpDiagnose, nil, 5*time.Second)
	if err != nil {
		return Diagnosis{}, err
	}
	var diagnosis Diagnosis
	if err := json.Unmarshal(out, &diagnosis); err != nil {
		return Diagnosis{}, fmt.Errorf("extensions: invalid diagnose answer: %w", err)
	}
	return diagnosis, nil
}

// runOperation runs one provider protocol operation: the runtime, the
// argsPrefix, the operation and the protocol version, with payload on stdin,
// and returns stdout.
func runOperation(ctx context.Context, integration Integration, op string, payload []byte, timeout time.Duration) ([]byte, error) {
	if integration.ManifestErr != nil {
		return nil, integration.ManifestErr
	}
	binding, err := ResolveRuntime(integration)
	if err != nil {
		return nil, err
	}
	args := ProtocolArgs(binding, integration.Manifest, op)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binding.Program, args...)
	cmd.Env = providerEnv(integration.Manifest)
	// A provider that hangs (or whose child keeps the pipes open) must not
	// hold the app.
	cmd.WaitDelay = 500 * time.Millisecond
	if payload != nil {
		cmd.Stdin = bytes.NewReader(payload)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	runErr := cmd.Run()
	command := strings.Join(append([]string{binding.Program}, args...), " ")
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &ProviderError{Op: op, Command: command, Err: fmt.Errorf("timed out after %s", timeout)}
	}
	if runErr != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		}
		return nil, &ProviderError{Op: op, Command: command, ExitCode: code, Stderr: stderr.String(), Err: runErr}
	}
	return stdout.Bytes(), nil
}

// StaticCompletions is what a command's declared arguments offer for the
// tokens typed so far, as catalog.rs built it: after an argument that takes a
// value, that argument's enum values or path names; otherwise the argument
// names themselves.
func StaticCompletions(command Command, tokens []string, cwd string) []Completion {
	fragment := ""
	if len(tokens) > 0 {
		fragment = tokens[len(tokens)-1]
	}
	var previous string
	if len(tokens) >= 2 {
		previous = tokens[len(tokens)-2]
	}

	if previous != "" {
		if argument, ok := valueArgument(command, previous); ok {
			switch argument.Kind {
			case "enum":
				return enumCompletions(argument, fragment)
			case "path", "directory":
				return pathCompletions(fragment, cwd, argument.Kind == "directory")
			case "command":
				// The provider's complete operation answers these.
				return nil
			}
		}
	}

	seen := map[string]bool{}
	var items []Completion
	for _, argument := range command.Arguments {
		for _, name := range argument.Names {
			if strings.HasPrefix(name, fragment) && !seen[name] {
				seen[name] = true
				items = append(items, Completion{Label: name, Kind: argument.Kind, Detail: argument.Description})
			}
		}
	}
	sortCompletions(items)
	return items
}

// valueArgument is the declared argument a name belongs to, when it takes a
// value.
func valueArgument(command Command, name string) (Argument, bool) {
	for _, argument := range command.Arguments {
		if !argument.TakesValue {
			continue
		}
		for _, candidate := range argument.Names {
			if candidate == name {
				return argument, true
			}
		}
	}
	return Argument{}, false
}

func enumCompletions(argument Argument, fragment string) []Completion {
	var items []Completion
	for _, value := range argument.Values {
		if strings.HasPrefix(value, fragment) {
			items = append(items, Completion{Label: value, Kind: "enum", Detail: argument.Description})
		}
	}
	return items
}

// pathCompletions lists the file names a fragment prefixes, as the old
// host's path completion did: directories keep a trailing separator.
func pathCompletions(fragment, cwd string, directoriesOnly bool) []Completion {
	// The text before the last separator names the directory to list; what
	// follows it is the prefix to match.
	dir, base := "", fragment
	if index := strings.LastIndexAny(fragment, "/\\"); index >= 0 {
		dir, base = fragment[:index+1], fragment[index+1:]
	}
	lookup := dir
	if lookup == "" {
		lookup = "."
	}
	if !filepath.IsAbs(lookup) && cwd != "" {
		lookup = filepath.Join(cwd, lookup)
	}
	entries, err := os.ReadDir(lookup)
	if err != nil {
		return nil
	}
	var items []Completion
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		isDir := entry.IsDir()
		if directoriesOnly && !isDir {
			continue
		}
		value := dir + name
		if isDir {
			value += string(filepath.Separator)
		}
		kind := "path"
		if isDir {
			kind = "directory"
		}
		items = append(items, Completion{Label: value, Kind: kind})
	}
	sortCompletions(items)
	return items
}

// NeedsDynamicCompletion reports whether the tokens are a value of an
// argument the provider itself completes (kind "command"): only then is the
// complete operation worth running.
func NeedsDynamicCompletion(command Command, tokens []string) bool {
	if len(tokens) < 2 {
		return false
	}
	argument, ok := valueArgument(command, tokens[len(tokens)-2])
	return ok && argument.Kind == "command"
}

// MergeCompletions puts dynamic completions first and appends the static
// ones they do not already name, as the old host merged them.
func MergeCompletions(static, dynamic []Completion) []Completion {
	if len(dynamic) == 0 {
		return append([]Completion{}, static...)
	}
	seen := map[string]bool{}
	out := make([]Completion, 0, len(dynamic)+len(static))
	for _, item := range dynamic {
		if item.Label == "" || seen[item.Label] {
			continue
		}
		seen[item.Label] = true
		out = append(out, item)
	}
	for _, item := range static {
		if item.Label == "" || seen[item.Label] {
			continue
		}
		seen[item.Label] = true
		out = append(out, item)
	}
	return out
}

// sortCompletions orders completions by label, so a list is stable.
func sortCompletions(items []Completion) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Label < items[j].Label })
}
