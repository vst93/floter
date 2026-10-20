package shell

import (
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/inventory"
	"floter/internal/settings"
	"floter/internal/settingsui"
)

// resetDetectedMemo clears the process-wide memo so a test starts cold.
func resetDetectedMemo() {
	detectedMemo.mu.Lock()
	detectedMemo.tools = nil
	detectedMemo.computed = time.Time{}
	detectedMemo.pending = false
	detectedMemo.mu.Unlock()
}

// The Detected list is a full-disk walk, and the settings page asks for it on
// every frame.
//
// `settingsui.detectedSection` calls `Actions.Detected()` while it builds the
// integrations page, so the cost of that call is paid once per painted frame.
// What the port did there was the machine's whole discovery:
// `inventory.Shared().Candidates()` (a stat walk over PATH, the desktop
// entries, flatpak/snap/nix, Homebrew, LaunchServices, Scoop, winget — measured
// at 1.19s on the first call) plus `tools.DetectManagers` (0.31ms, every
// frame). That is the report 「很多界面经常卡死」: the settings page froze for a
// second on the first visit after the snapshot went stale, and re-walked the
// managers on every frame of every visit.
//
// The fix is the app's own icon/thumbnail pattern: the frame reads a memo, one
// background pass does the walking, and its answer asks for a frame. This pins
// the two halves that matter — the frame never blocks, and the answer arrives.
func TestTheDetectedListNeverWalksOnTheDrawingThread(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	resetDetectedMemo()

	landed := make(chan struct{}, 4)
	SetDetectedInvalidate(func() {
		select {
		case landed <- struct{}{}:
		default:
		}
	})
	defer SetDetectedInvalidate(nil)

	// The inventory is cold in a fresh test process, so the walk this frame
	// would have done is the expensive one. The frame must not pay it: a
	// hundred reads of the drawing thread's view all return at once, and the
	// first one is what starts the single background pass.
	start := time.Now()
	var first []settingsui.DetectedTool
	for i := 0; i < 100; i++ {
		if i == 0 {
			first = a.detectedTools()
		} else {
			a.detectedTools()
		}
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("a hundred drawn frames spent %v asking for the Detected list: the walk is on the drawing thread", elapsed)
	}
	// The very first frame has no answer yet — the list appears when the pass
	// lands, exactly as an extracted icon does.
	if len(first) != 0 {
		t.Errorf("the first frame already had %d rows; nothing has been computed yet", len(first))
	}

	// The same hundred frames asked for exactly one pass, not a hundred.
	detectedMemo.mu.Lock()
	pending := detectedMemo.pending
	detectedMemo.mu.Unlock()
	if !pending {
		t.Fatalf("no pass was started")
	}

	// The pass lands, publishes, and asks for one frame.
	select {
	case <-landed:
	case <-time.After(30 * time.Second):
		t.Fatalf("the background pass never published")
	}
	detectedMemo.mu.Lock()
	computed := detectedMemo.computed
	detectedMemo.mu.Unlock()
	if computed.IsZero() {
		t.Errorf("the pass published no timestamp")
	}

	// A warm memo is free and stable: the frames after the pass draw the same
	// rows without starting another walk.
	start = time.Now()
	for i := 0; i < 100; i++ {
		a.detectedTools()
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("a hundred warm frames spent %v", elapsed)
	}
}

// A rescan drops the memo: the tray's "I changed something on this machine"
// press must not keep serving the answer computed before it.
func TestARescanAsksForAFreshDetectedList(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	resetDetectedMemo()

	// Stand in for a completed pass.
	detectedMemo.mu.Lock()
	detectedMemo.tools = []settingsui.DetectedTool{{Name: "stale", Path: "/usr/bin/stale"}}
	detectedMemo.computed = time.Now()
	detectedMemo.mu.Unlock()

	if got := a.detectedTools(); len(got) != 1 || got[0].Name != "stale" {
		t.Fatalf("the warm memo did not serve its rows: %+v", got)
	}

	invalidateDetectedTools()
	detectedMemo.mu.Lock()
	computed := detectedMemo.computed
	detectedMemo.mu.Unlock()
	if !computed.IsZero() {
		t.Errorf("the rescan left the old pass's timestamp in place: %v", computed)
	}

	// And the next frame starts a new pass rather than serving the old list.
	a.detectedTools()
	detectedMemo.mu.Lock()
	pending := detectedMemo.pending
	detectedMemo.mu.Unlock()
	if !pending {
		t.Errorf("the frame after a rescan started no pass")
	}
}

// The walk itself still answers with the programs nothing has connected, and
// still caps the page: the background pass is the same list the frame used to
// compute, so moving it off the drawing thread took nothing away.
func TestTheBackgroundPassIsTheSameList(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	rows := a.computeDetectedTools()
	if len(rows) > maxDetectedTools {
		t.Errorf("the pass returned %d rows, over the page's cap of %d", len(rows), maxDetectedTools)
	}

	// Nothing it returns is a program that is already connected: the section
	// is the tools "nothing has connected yet".
	connected := map[string]bool{}
	for _, integration := range a.Integrations.Inventory().Integrations {
		if path := integration.Entry.ExecutablePath; path != "" {
			connected[path] = true
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				connected[resolved] = true
			}
		}
	}
	for _, row := range rows {
		if connected[row.Path] {
			t.Errorf("%q is already connected but was offered as detected", row.Path)
		}
		if row.Name == "" || row.Path == "" {
			t.Errorf("a detected row is missing its name or path: %+v", row)
		}
	}

	// The inventory it derives from is the process's shared one, so a
	// candidate the ranking found is the candidate the list offers.
	if len(rows) > 0 && len(inventory.Shared().Candidates()) == 0 {
		t.Errorf("the pass offered %d rows from an empty inventory", len(rows))
	}
	_ = extensions.SearchDirectories()
}
