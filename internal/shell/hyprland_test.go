package shell

import (
	"errors"
	"testing"
)

func TestHyprlandFloatingIsASetNotAToggle(t *testing.T) {
	var calls [][]string
	run := func(program string, args ...string) error {
		call := append([]string{program}, args...)
		calls = append(calls, call)
		return nil
	}

	runHyprlandFloating(true, run)

	want := [][]string{
		{"hyprctl", "dispatch", "setfloating"},
		{"hyprctl", "dispatch", "centerwindow"},
	}
	if len(calls) != len(want) {
		t.Fatalf("dispatches = %v, want %v", calls, want)
	}
	for i := range want {
		if len(calls[i]) != len(want[i]) {
			t.Fatalf("dispatch %d = %v, want %v", i, calls[i], want[i])
		}
		for j := range want[i] {
			if calls[i][j] != want[i][j] {
				t.Errorf("dispatch %d[%d] = %q, want %q", i, j, calls[i][j], want[i][j])
			}
		}
	}
	// `togglefloating` would flip an already floating window back into the
	// tiling layout on a second reveal; the dispatches must never carry it.
	for _, call := range calls {
		for _, arg := range call {
			if arg == "togglefloating" {
				t.Fatalf("a togglefloating dispatch slipped in: %v", calls)
			}
		}
	}
}

func TestHyprlandFloatingSkipsOtherSessions(t *testing.T) {
	ran := false
	run := func(string, ...string) error {
		ran = true
		return nil
	}
	runHyprlandFloating(false, run)
	if ran {
		t.Error("a non-Hyprland session ran the floating dispatches")
	}
}

func TestHyprlandFloatingIgnoresAFailedDispatch(t *testing.T) {
	// A machine without hyprctl on PATH fails the first dispatch; the second
	// still runs and the failure is swallowed, so the reveal is never broken.
	ran := 0
	run := func(string, ...string) error {
		ran++
		return errors.New("no hyprctl")
	}
	runHyprlandFloating(true, run)
	if ran != len(hyprlandFloatingDispatches) {
		t.Errorf("dispatches run = %d, want %d", ran, len(hyprlandFloatingDispatches))
	}
}
