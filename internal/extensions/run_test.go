package extensions

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func runEntry(script string) CommandEntry {
	return CommandEntry{
		IntegrationID: "test.ext",
		Command:       Command{ID: "run", Name: "Run"},
		Program:       "/bin/sh",
		Args:          []string{"-c", script},
		Route:         RouteBackground,
	}
}

func TestRunCapturedCapturesBothStreams(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	run, err := RunCaptured(context.Background(), runEntry("echo out; echo err >&2; exit 0"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Success || run.ExitCode != 0 || run.TimedOut {
		t.Fatalf("run = %+v", run)
	}
	if strings.TrimSpace(run.Stdout) != "out" || strings.TrimSpace(run.Stderr) != "err" {
		t.Errorf("streams = %q / %q", run.Stdout, run.Stderr)
	}
	if got := run.Text(); got != "out\nerr" {
		t.Errorf("Text = %q", got)
	}
	if run.Duration <= 0 {
		t.Error("the run took no time")
	}
	if len(run.Command) != 3 || run.Command[0] != "/bin/sh" {
		t.Errorf("command = %v", run.Command)
	}

	// A non-zero exit is an answer, not an error.
	run, err = RunCaptured(context.Background(), runEntry("exit 3"), nil)
	if err != nil {
		t.Fatalf("a failing command reported %v", err)
	}
	if run.Success || run.ExitCode != 3 {
		t.Errorf("run = %+v", run)
	}

	// A stream with no output at all is an empty text.
	run, err = RunCaptured(context.Background(), runEntry("true"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Text() != "" {
		t.Errorf("Text = %q", run.Text())
	}
}

func TestRunCapturedTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	run, err := RunCaptured(ctx, runEntry("sleep 5"), nil)
	if err == nil {
		t.Fatal("a hung command reported success")
	}
	if !run.TimedOut {
		t.Errorf("run = %+v", run)
	}
	if time.Since(started) > 3*time.Second {
		t.Errorf("the timeout took %s", time.Since(started))
	}
}

func TestRunCapturedCapsTheOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	// A stream past the cap is cut, and the flag says so.
	run, err := RunCaptured(context.Background(), runEntry("head -c 2000000 /dev/zero | tr '\\0' 'a'"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Truncated {
		t.Errorf("a huge stream was not marked truncated: %d bytes", len(run.Stdout))
	}
	if len(run.Stdout) > MaxRunOutputBytes {
		t.Errorf("the cap did not hold: %d bytes", len(run.Stdout))
	}
}

func TestManifestRoute(t *testing.T) {
	cases := map[string]string{
		"":           RouteTerminal,
		"terminal":   RouteTerminal,
		"background": RouteBackground,
		"BACKGROUND": RouteBackground,
		" nonsense ": RouteTerminal,
	}
	for output, want := range cases {
		manifest := Manifest{Output: output}
		if got := manifest.Route(); got != want {
			t.Errorf("Route(%q) = %q, want %q", output, got, want)
		}
	}
}
