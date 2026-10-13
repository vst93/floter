package extensions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The provider protocol operations and the argument that asks for a version.
const (
	OpDescribe = "describe"
	OpComplete = "complete"
	OpDiagnose = "diagnose"

	protocolFlag = "--protocol"
)

// RuntimeBinding is how an integration's program is started: the program to
// run and the leading arguments before the provider protocol's own.
type RuntimeBinding struct {
	// Program is the executable (or interpreter) to run.
	Program string
	// Args are the leading arguments: the script path or interpreter flags,
	// before the manifest's argsPrefix.
	Args []string
}

// ResolveRuntime finds the program that runs an integration's provider: the
// executable the repository recorded, an executable on the search path for a
// system runtime, or the interpreter plus script for a script runtime.
//
// A failure is a *BindingFailure* wherever the cause is one of the named
// binding facts, so a caller can record the code and the keyed detail on the
// repository entry (R11). The structural failures that have no code of their
// own are classified `binding-check-failed` with the underlying prose, the old
// host's `failure_code` fallback.
func ResolveRuntime(integration Integration) (RuntimeBinding, error) {
	manifest := integration.Manifest
	if integration.ManifestErr != nil {
		return RuntimeBinding{}, fmt.Errorf("extensions: %s: %w", integration.Entry.ID, integration.ManifestErr)
	}
	switch manifest.Runtime.Type {
	case "system":
		if path := integration.Entry.ExecutablePath; path != "" {
			if isExecutableFile(path) {
				return RuntimeBinding{Program: path}, nil
			}
			return RuntimeBinding{}, NewBindingFailure(BindingCodeMissing, BindingMissingDetail(path))
		}
		names := manifest.Runtime.ExecutableNames
		if len(names) == 0 {
			names = []string{manifest.ID}
		}
		if path, ok := lookTool(names...); ok {
			return RuntimeBinding{Program: path}, nil
		}
		return RuntimeBinding{}, NewBindingFailure(BindingCodeRuntimeUnavailable, RuntimeUnavailableDetail(names))

	case "script":
		language := manifest.Runtime.Language
		interpreter, ok := findInterpreter(language)
		if !ok {
			names := InterpreterNames(language)
			if len(names) == 0 {
				names = []string{language}
			}
			return RuntimeBinding{}, NewBindingFailure(BindingCodeRuntimeUnavailable, RuntimeUnavailableDetail(names))
		}
		dir := integration.PackageDir()
		script := manifest.Runtime.Path
		if dir != "" && !filepath.IsAbs(script) {
			joined, ok := joinWithin(dir, script)
			if !ok {
				return RuntimeBinding{}, NewBindingFailure(BindingCodeCheckFailed,
					fmt.Sprintf("the script path %q leaves the package", script))
			}
			script = joined
		}
		return RuntimeBinding{Program: interpreter, Args: InterpreterArgs(language, script)}, nil

	default:
		return RuntimeBinding{}, NewBindingFailure(BindingCodeCheckFailed,
			fmt.Sprintf("unknown runtime %q", manifest.Runtime.Type))
	}
}

// ProtocolArgs is the argv of a protocol operation: the runtime, then the
// manifest's argsPrefix, then the operation and the protocol version. The
// argsPrefix is the one resolved for this host's platform, so a manifest's
// `platformOverrides` reach the provider call.
func ProtocolArgs(binding RuntimeBinding, manifest Manifest, op string) []string {
	provider := manifest.Provider
	if len(manifest.PlatformOverrides) > 0 {
		provider = manifest.EffectiveProvider(CurrentPlatform())
	}
	args := make([]string, 0, len(binding.Args)+len(provider.ArgsPrefix)+3)
	args = append(args, binding.Args...)
	args = append(args, provider.ArgsPrefix...)
	args = append(args, op, protocolFlag, ProtocolVersion)
	return args
}

// ProviderError is a describe (or complete) failure with what the provider
// said: its exit code and stderr, for the details drawer.
type ProviderError struct {
	Op       string
	Command  string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *ProviderError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	switch {
	case detail != "":
		return fmt.Sprintf("extensions: %s failed (exit %d): %s", e.Op, e.ExitCode, detail)
	case e.Err != nil:
		return fmt.Sprintf("extensions: %s failed: %v", e.Op, e.Err)
	default:
		return fmt.Sprintf("extensions: %s failed (exit %d)", e.Op, e.ExitCode)
	}
}

func (e *ProviderError) Unwrap() error { return e.Err }

// Describe obtains an integration's provider description: the static
// descriptor the package ships, or the executable's `describe` answer.
//
// The host probes the provider with the environment the manifest declares,
// under the manifest's describe timeout, and fails cleanly rather than
// hanging the app.
func Describe(ctx context.Context, integration Integration) (Description, error) {
	if integration.ManifestErr != nil {
		return Description{}, integration.ManifestErr
	}
	switch integration.Manifest.Provider.Type {
	case "static-descriptor", "bundled-static":
		path, ok := StaticDescriptionPath(integration)
		if !ok {
			return Description{}, fmt.Errorf("extensions: %s: the provider ships no descriptor", integration.Entry.ID)
		}
		return LoadStaticDescription(path)
	}

	binding, err := ResolveRuntime(integration)
	if err != nil {
		return Description{}, err
	}
	args := ProtocolArgs(binding, integration.Manifest, OpDescribe)

	timeout := time.Duration(integration.Manifest.DescribeTimeout()) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binding.Program, args...)
	cmd.Env = providerEnv(integration.Manifest)
	// A provider that hangs (or whose child keeps the pipes open) must not
	// hold the app: after the context is done, Wait gives the I/O half a
	// second and then returns.
	cmd.WaitDelay = 500 * time.Millisecond
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	command := strings.Join(append([]string{binding.Program}, args...), " ")
	if ctx.Err() == context.DeadlineExceeded {
		return Description{}, &ProviderError{Op: OpDescribe, Command: command, Err: fmt.Errorf("timed out after %s", timeout)}
	}
	if runErr != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return Description{}, &ProviderError{Op: OpDescribe, Command: command, ExitCode: exitCode, Stderr: stderr.String(), Err: runErr}
	}
	description, err := ParseDescription(stdout.Bytes())
	if err != nil {
		return Description{}, &ProviderError{Op: OpDescribe, Command: command, Stderr: err.Error()}
	}
	return description, nil
}

// providerEnv is the environment a provider runs with: the app's own plus
// the manifest's declared variables, with this platform's `platformOverrides`
// merged on top.
func providerEnv(manifest Manifest) []string {
	env := os.Environ()
	provider := manifest.Provider
	if len(manifest.PlatformOverrides) > 0 {
		provider = manifest.EffectiveProvider(CurrentPlatform())
	}
	for key, value := range provider.Environment {
		env = append(env, key+"="+value)
	}
	return env
}
