package extensions

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// The background route: a command the manifest sends away from the terminal
// runs headless, with both streams captured, and the launcher shows what it
// said. The run is time-boxed and its output is capped, so a hung or chatty
// program cannot pin the app.

// RunTimeout is how long a background run may take before it is killed: long
// enough for a real script, short enough that a hung one does not pin the host.
const RunTimeout = 300 * time.Second

// MaxRunOutputBytes is the cap on each captured stream. The old host returned
// the streams whole and left its `truncated` flag false; a cap is the honest
// version of the same promise, and the flag says when it bit.
const MaxRunOutputBytes = 1 << 20

// CapturedRun is one background run's outcome.
type CapturedRun struct {
	// Command is the argv that ran, for the output view's title.
	Command []string
	// Stdout and Stderr are what the program wrote; Truncated says a stream
	// was cut at the cap.
	Stdout    string
	Stderr    string
	Truncated bool
	// ExitCode is the process's exit code (-1 when it never ran or was
	// killed), Success whether it exited zero, and TimedOut whether the
	// timeout killed it.
	ExitCode int
	Success  bool
	TimedOut bool
	Duration time.Duration
}

// Text is what the output view shows: standard output, with standard error
// appended when there is any (so a program that only complains still shows
// something).
func (r CapturedRun) Text() string {
	stdout := strings.TrimRight(r.Stdout, "\n")
	stderr := strings.TrimRight(r.Stderr, "\n")
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stderr == "":
		return stdout
	case stdout == "":
		return stderr
	default:
		return stdout + "\n" + stderr
	}
}

// PageCursorEnv is the environment variable a self-paging plugin reads to
// continue from the cursor its last emission named. The launcher re-runs the
// command for the next page with this set; a plugin that does not page itself
// never sees it.
const PageCursorEnv = "FLOTER_PAGE_CURSOR"

// RunCaptured runs a command headless and captures its output.
//
// The argv is the entry's own (the runtime, the manifest's argsPrefix, the
// caller's arguments), so there is no shell anywhere on the path and no string
// is re-parsed.
func RunCaptured(ctx context.Context, entry CommandEntry, args []string) (CapturedRun, error) {
	return runCaptured(ctx, entry, args, "")
}

// RunCapturedPage runs a command for a later page of a list it pages itself:
// the cursor from its last emission is handed over in FLOTER_PAGE_CURSOR, so
// the plugin can continue where it stopped.
func RunCapturedPage(ctx context.Context, entry CommandEntry, args []string, cursor string) (CapturedRun, error) {
	return runCaptured(ctx, entry, args, cursor)
}

func runCaptured(ctx context.Context, entry CommandEntry, args []string, cursor string) (CapturedRun, error) {
	argv := append(append([]string{}, entry.Args...), args...)
	if len(argv) == 0 {
		return CapturedRun{}, errors.New("extensions: the command has no program to run")
	}
	timeout := RunTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// A Windows `.cmd`/`.bat` program is not startable on its own: the probe
	// layer happily matches those shims, so the one place that spawns a
	// program has to spell out the cmd.exe wrap (R74). The gate is the same
	// platform-aware check the parameter rules use; off Windows nothing
	// changes, argv included.
	program, argv := cmdShimWrap(ProgramIsCmdScript(entry.Program), entry.Program, argv)

	cmd := newExtensionCommand(runCtx, program, argv...)
	cmd.Stdin = nil
	if entry.Dir != "" {
		cmd.Dir = entry.Dir
	}
	if len(entry.Env) > 0 || cursor != "" {
		env := append(providerEnv(Manifest{}), entry.Env...)
		if cursor != "" {
			env = append(env, PageCursorEnv+"="+cursor)
		}
		cmd.Env = env
	}
	// A program that ignores the context must not hold the app: after the
	// deadline, Wait gives the I/O half a moment and then returns.
	cmd.WaitDelay = 2 * time.Second

	// Each stream keeps only its leading prefix, so a program that never stops
	// talking cannot pin the host's memory before the timeout fires.
	var stdout, stderr capturedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	started := time.Now()
	runErr := runExtensionCommand(cmd)
	duration := time.Since(started)

	run := CapturedRun{
		Command:   append([]string{program}, argv...),
		Duration:  duration,
		ExitCode:  -1,
		Truncated: stdout.truncated || stderr.truncated,
	}
	run.Stdout = stdout.String()
	run.Stderr = stderr.String()

	if runCtx.Err() == context.DeadlineExceeded {
		run.TimedOut = true
		return run, RunTimeoutFailure(int(timeout / time.Second))
	}
	if ctx.Err() != nil {
		// The caller cancelled the run. In this build the only cancel source
		// is the run registry, which fires when the integration is disabled or
		// uninstalled, so the entry's own id names the cause; a cancel with no
		// id to name stays the plain context error.
		if entry.IntegrationID != "" {
			return run, RunKilledFailure(entry.IntegrationID)
		}
		return run, ctx.Err()
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			run.ExitCode = exitErr.ExitCode()
			return run, nil // a non-zero exit is an answer, not a failure
		}
		return run, RunSpawnFailure(entry.Program, runErr)
	}
	run.ExitCode = 0
	run.Success = true
	return run, nil
}
