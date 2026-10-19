//go:build !windows

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// The wrap is a pure function of its two inputs: shim says the program is a
// Windows command script, and only then is the argv rewritten onto cmd.exe.
func TestCmdShimWrapTruthTable(t *testing.T) {
	t.Run("without the shim the pair is handed back untouched", func(t *testing.T) {
		args := []string{"one", "two"}
		program, argv := cmdShimWrap(false, "runner.cmd", args)
		if program != "runner.cmd" {
			t.Errorf("program = %q, want the caller's own", program)
		}
		if !reflect.DeepEqual(argv, args) {
			t.Errorf("argv = %v, want %v", argv, args)
		}
		// A byte-for-byte no-op: not a copy, the very same slice.
		if len(argv) > 0 && &argv[0] != &args[0] {
			t.Error("the skipped wrap copied the argv")
		}
		// And an empty argv stays empty, not a one-element /C.
		if _, empty := cmdShimWrap(false, "runner", nil); empty != nil {
			t.Errorf("nil argv became %v", empty)
		}
	})

	t.Run("with the shim the script is run through cmd.exe", func(t *testing.T) {
		program, argv := cmdShimWrap(true, `C:\tools\runner.cmd`, []string{"build", "-v"})
		if program != "cmd.exe" {
			t.Errorf("program = %q, want cmd.exe", program)
		}
		want := []string{"/D", "/S", "/C", `C:\tools\runner.cmd`, "build", "-v"}
		if !reflect.DeepEqual(argv, want) {
			t.Errorf("argv = %v, want %v", argv, want)
		}
	})

	t.Run("a script with no arguments still names itself after /C", func(t *testing.T) {
		_, argv := cmdShimWrap(true, "runner.bat", nil)
		want := []string{"/D", "/S", "/C", "runner.bat"}
		if !reflect.DeepEqual(argv, want) {
			t.Errorf("argv = %v, want %v", argv, want)
		}
	})
}

// The platform gate, not the pure function, decides whether a `.cmd` name is
// wrapped: off Windows a program whose name ends in `.cmd` is an ordinary
// executable and its argv must not be touched.
func TestCmdScriptWrapFollowsThePlatformGate(t *testing.T) {
	args := []string{"--flag"}
	program, argv := cmdShimWrap(ProgramIsCmdScript("runner.cmd"), "runner.cmd", args)
	if runtime.GOOS == "windows" {
		if program != "cmd.exe" || argv[3] != "runner.cmd" {
			t.Errorf("on Windows the script was not wrapped: %q %v", program, argv)
		}
		return
	}
	if program != "runner.cmd" || !reflect.DeepEqual(argv, args) {
		t.Errorf("off Windows the script was wrapped: %q %v", program, argv)
	}
}

// Off Windows nothing changes at the spawn either: a file that merely *spells*
// its name like a Windows command script is executed directly, and the run
// records the caller's own argv. If the gate ever grew a wrap off Windows this
// fixture would try to launch a cmd.exe that does not exist.
func TestRunCapturedDoesNotWrapCommandScriptsOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a POSIX script that happens to be named like a command script")
	}
	script := filepath.Join(t.TempDir(), "runner.cmd")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'ran-directly'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := CommandEntry{
		IntegrationID: "test.ext",
		Command:       Command{ID: "run", Name: "Run"},
		Program:       script,
		Args:          []string{"ignored"},
		Route:         RouteBackground,
	}
	run, err := RunCaptured(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	if strings.TrimSpace(run.Stdout) != "ran-directly" {
		t.Errorf("stdout = %q", run.Stdout)
	}
	if !reflect.DeepEqual(run.Command, []string{script, "ignored"}) {
		t.Errorf("the run altered the argv: %v", run.Command)
	}
}

// On Windows the spawn really does go through cmd.exe: a `.cmd` file cannot be
// started on its own, so a run that succeeds here proves the wiring, and the
// recorded command names the wrap that made it possible.
func TestRunCapturedWrapsWindowsCommandScripts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the wrap only happens on Windows")
	}
	dir := t.TempDir()
	if strings.ContainsRune(dir, ' ') {
		t.Skip("cmd /S /C quoting needs a path without spaces")
	}
	script := filepath.Join(dir, "runner.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\necho shim-ok %1\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := CommandEntry{
		IntegrationID: "test.ext",
		Command:       Command{ID: "run", Name: "Run"},
		Program:       script,
		Args:          []string{"first"},
		Route:         RouteBackground,
	}
	run, err := RunCaptured(context.Background(), entry, nil)
	if err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	if strings.TrimSpace(run.Stdout) != "shim-ok first" {
		t.Errorf("stdout = %q, stderr = %q", run.Stdout, run.Stderr)
	}
	if len(run.Command) < 5 || run.Command[0] != "cmd.exe" || run.Command[1] != "/D" ||
		run.Command[2] != "/S" || run.Command[3] != "/C" || run.Command[4] != script {
		t.Errorf("the run was not wrapped: %v", run.Command)
	}
}
