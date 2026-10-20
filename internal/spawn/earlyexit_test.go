package spawn

import (
	"runtime"
	"testing"
	"time"
)

// The hand-off's own failure, observed: an emulator that is on the machine but
// dies at once is not a launch. The port counted `Start` returning nothing as
// success, so 「在系统终端中继续」 on a machine whose first candidate could not
// open a window did nothing and said nothing — the report this pins.
func TestAnEmulatorThatDiesAtOnceIsAFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a POSIX shell")
	}
	if err := startCandidate("", []string{"sh", "-c", "exit 3"}, true); err == nil {
		t.Fatal("a launcher that exited 3 inside the watch window was reported as started")
	}
}

// A hand-off that exits **0** is a launch: gnome-terminal, konsole and
// `kitty --single-instance` all return successfully the moment a running
// server has the window. Reading their exit as failure would fall through to
// the next emulator and open a second terminal.
func TestAHandOffThatExitsZeroIsALaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a POSIX shell")
	}
	if err := startCandidate("", []string{"sh", "-c", "exit 0"}, true); err != nil {
		t.Fatalf("a hand-off that exited 0 was reported as a failure: %v", err)
	}
}

// And an emulator that is still there after the window is the window: the
// watch is bounded, so the launch path pays it once and not for the child's
// whole life.
func TestASurvivingLauncherIsStartedAfterTheWindow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a POSIX shell")
	}
	begin := time.Now()
	if err := startCandidate("", []string{"sh", "-c", "sleep 5"}, true); err != nil {
		t.Fatalf("a surviving launcher was reported as a failure: %v", err)
	}
	if elapsed := time.Since(begin); elapsed < earlyExitWindow {
		t.Errorf("the watch returned after %v, before its own window of %v", elapsed, earlyExitWindow)
	}
	if elapsed := time.Since(begin); elapsed > 4*earlyExitWindow {
		t.Errorf("the watch took %v, longer than the window it bounds (%v)", elapsed, earlyExitWindow)
	}
}

// A program that is not there is still the spawn's own error, watched or not.
func TestAMissingProgramIsAnError(t *testing.T) {
	if err := startCandidate("", []string{"floter-no-such-program-xyz"}, true); err == nil {
		t.Fatal("a missing program was reported as started")
	}
}
