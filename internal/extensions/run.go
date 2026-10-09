package extensions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// RunCaptured runs a command headless and captures its output.
//
// The argv is the entry's own (the runtime, the manifest's argsPrefix, the
// caller's arguments), so there is no shell anywhere on the path and no string
// is re-parsed.
func RunCaptured(ctx context.Context, entry CommandEntry, args []string) (CapturedRun, error) {
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

	cmd := exec.CommandContext(runCtx, entry.Program, argv...)
	cmd.Stdin = nil
	if entry.Dir != "" {
		cmd.Dir = entry.Dir
	}
	if len(entry.Env) > 0 {
		cmd.Env = append(providerEnv(Manifest{}), entry.Env...)
	}
	// A program that ignores the context must not hold the app: after the
	// deadline, Wait gives the I/O half a moment and then returns.
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)

	run := CapturedRun{
		Command:   append([]string{entry.Program}, argv...),
		Duration:  duration,
		ExitCode:  -1,
		Truncated: stdout.Len() > MaxRunOutputBytes || stderr.Len() > MaxRunOutputBytes,
	}
	run.Stdout = clip(stdout.Bytes())
	run.Stderr = clip(stderr.Bytes())

	if runCtx.Err() == context.DeadlineExceeded {
		run.TimedOut = true
		return run, fmt.Errorf("extensions: %s timed out after %s", entry.Command.ID, timeout)
	}
	if ctx.Err() != nil {
		return run, ctx.Err()
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			run.ExitCode = exitErr.ExitCode()
			return run, nil // a non-zero exit is an answer, not a failure
		}
		return run, &ProviderError{Op: "run", Command: strings.Join(run.Command, " "), Err: runErr}
	}
	run.ExitCode = 0
	run.Success = true
	return run, nil
}

// clip cuts a stream at the cap, on a rune boundary.
func clip(data []byte) string {
	if len(data) <= MaxRunOutputBytes {
		return string(data)
	}
	cut := data[:MaxRunOutputBytes]
	for len(cut) > 0 && !utf8Start(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}
	return string(cut)
}

// utf8Start reports whether a byte begins a UTF-8 sequence.
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }
