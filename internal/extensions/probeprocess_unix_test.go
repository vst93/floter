//go:build !windows

package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestProbeProcessHelper(t *testing.T) {
	if os.Getenv("FLOTER_PROBE_PROCESS_HELPER") != "1" {
		return
	}
	// A private process group contains a tool's group-directed signals;
	// no controlling terminal prevents it from stealing the host's foreground.
	if syscall.Getpgrp() != os.Getpid() {
		fmt.Print("shared process group")
		os.Exit(1)
	}
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		tty.Close()
		fmt.Print("inherited controlling terminal")
		os.Exit(1)
	}
	switch os.Getenv("FLOTER_PROBE_PROCESS_OUTPUT") {
	case "path":
		fmt.Print("__floter_path__/isolated")
	case "version":
		fmt.Print("1.2.3")
	default:
		fmt.Print("isolated")
	}
	os.Exit(0)
}

func TestBackgroundProbesAreIsolatedFromHostTerminal(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLOTER_PROBE_PROCESS_HELPER", "1")
	binding := RuntimeBinding{Program: program, Args: []string{"-test.run=^TestProbeProcessHelper$"}}
	t.Run("help and version", func(t *testing.T) {
		if got := ProbeHelpText(context.Background(), binding, Integration{}); strings.TrimSpace(got) != "isolated" {
			t.Fatalf("probe process: %q", got)
		}
	})
	script := filepath.Join(t.TempDir(), "probe")
	t.Setenv("FLOTER_PROBE_PROCESS_PROGRAM", program)
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec \"$FLOTER_PROBE_PROCESS_PROGRAM\" -test.run=^TestProbeProcessHelper$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Run("script runtime version", func(t *testing.T) {
		t.Setenv("FLOTER_PROBE_PROCESS_OUTPUT", "version")
		if got := probeProgramVersion(context.Background(), script); got != "1.2.3" {
			t.Fatalf("isolated version probe: %q", got)
		}
	})
	t.Run("login shell PATH", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("login shell PATH discovery is macOS-only")
		}
		t.Setenv("SHELL", script)
		t.Setenv("FLOTER_PROBE_PROCESS_OUTPUT", "path")
		if got := loginShellDirs(); len(got) != 1 || got[0] != "/isolated" {
			t.Fatalf("isolated login shell: %v", got)
		}
	})
	t.Run("health", func(t *testing.T) {
		record := runProbe(context.Background(), binding, Integration{}, Probe{ID: "terminal-isolation"})
		if !record.Passed {
			t.Fatalf("probe was not isolated: %+v", record)
		}
	})
}
