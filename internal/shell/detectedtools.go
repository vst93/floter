package shell

import (
	"path/filepath"
	"sync"
	"time"

	"floter/internal/extensions"
	"floter/internal/inventory"
	"floter/internal/settingsui"
	"floter/internal/tools"
)

// The Detected list is a full-disk walk, and the settings page draws it *every
// frame*.
//
// `settingsui.detectedSection` asks `Actions.Detected()` while it builds the
// integrations page, so whatever that function costs is paid once per painted
// frame — sixty times a second while the page is up, and again on every
// keystroke or hover that asks for a new frame. What it used to do is the
// expensive half of the machine's discovery:
//
//   - `inventory.Shared().Candidates()` re-walks every source (PATH, the
//     desktop entries, flatpak/snap/nix, Homebrew, LaunchServices, Scoop,
//     winget) whenever its 5-minute snapshot has gone stale — measured at
//     **1.19s** on this machine's first call, and the walk is longer on the
//     Windows and macOS sources it does not have here;
//   - `tools.DetectManagers(dirs)` stats every manager's candidate names in
//     every search directory — measured at 0.31ms per call, i.e. every frame
//     did the whole managers × names × dirs product again.
//
// Either one is the report 「很多界面经常卡死」: the settings page froze for a
// second the first time it was opened after the snapshot went stale, and paid
// the manager walk on every frame of every visit after that.
//
// The shape of the fix is the app's own, twice over: `apps.Icon` and
// `launcher.clipThumb` both answer from a memo on the drawing thread and run
// the expensive half on a goroutine that asks for a frame when it lands
// (`apps/icons.go`, `launcher/thumb.go`). This file is the third instance:
//
//   - a frame reads the memo and nothing else (no stat, no walk);
//   - a cold or stale memo starts one background pass and answers with what it
//     has — the previous answer, or nothing on the very first frame;
//   - the pass lands off-thread and asks for one frame, which draws the list.
//
// Staleness is the inventory's own clock rather than a second one: the list is
// a function of that snapshot, so it is as old as the snapshot is, and asking
// the inventory whether *it* needs a refresh (a cheap mutex read) is what keeps
// the two from drifting apart. A pass that finds nothing to do is the common
// case and costs one lock acquisition.

// detectedTTL bounds how long a computed list stands even when the inventory
// says its snapshot is current — the inventory's own default, so the list and
// the snapshot it derives from are one freshness rule.
var detectedTTL = inventory.DefaultTTL

// detectedMemo is the process's one Detected list (there is one App per
// process, exactly as there is one inventory).
var detectedMemo struct {
	mu       sync.Mutex
	tools    []settingsui.DetectedTool
	computed time.Time
	pending  bool
}

// detectedInvalidate is asked for a frame when a background pass lands, so a
// list that arrives while the page is up is drawn without waiting for the next
// keystroke. The shell wires it to its window (see New).
var (
	detectedInvalidateMu sync.Mutex
	detectedInvalidate   func()
)

// SetDetectedInvalidate installs the callback that asks for a frame when a
// background Detected pass lands.
func SetDetectedInvalidate(fn func()) {
	detectedInvalidateMu.Lock()
	detectedInvalidate = fn
	detectedInvalidateMu.Unlock()
}

// detectedTools is the programs this machine has that nothing has connected
// yet: the inventory's own ranking, cut to the ones a person would recognise
// and capped so the page stays a list of what is installed.
//
// It is the *drawing thread's* view of that list: it walks nothing. See the
// file's header — the walk is one background pass, and this reads its answer.
func (a *App) detectedTools() []settingsui.DetectedTool {
	detectedMemo.mu.Lock()
	tools, computed, pending := detectedMemo.tools, detectedMemo.computed, detectedMemo.pending
	detectedMemo.mu.Unlock()

	// Anything to do? A missing or old list, or an inventory that has gone
	// stale underneath it. The inventory's own answer is a mutex read, so
	// asking it every frame is the cheap half of this.
	if !pending && (computed.IsZero() || a.now().Sub(computed) >= detectedTTL || inventory.Shared().NeedsRefresh()) {
		detectedMemo.mu.Lock()
		start := !detectedMemo.pending
		detectedMemo.pending = true
		detectedMemo.mu.Unlock()
		if start {
			go a.scanDetectedTools()
		}
	}
	return tools
}

// invalidateDetectedTools drops the memo so the next frame starts a fresh
// pass. The tray's rescan is the "I changed something on this machine" press,
// and a list that kept serving the pre-rescan answer would be the one row the
// press did not reach.
func invalidateDetectedTools() {
	detectedMemo.mu.Lock()
	detectedMemo.computed = time.Time{}
	detectedMemo.mu.Unlock()
}

// scanDetectedTools runs the one background pass and publishes its answer.
func (a *App) scanDetectedTools() {
	found := a.computeDetectedTools()
	detectedMemo.mu.Lock()
	detectedMemo.tools = found
	detectedMemo.computed = time.Now()
	detectedMemo.pending = false
	detectedMemo.mu.Unlock()
	detectedInvalidateMu.Lock()
	invalidate := detectedInvalidate
	detectedInvalidateMu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// computeDetectedTools is the walk itself: every discovered candidate the
// inventory ranked, minus the ones already connected, each with the recipe
// that would install it. It runs on the scan's goroutine and reads only
// mutex-protected snapshots (the inventory, the store's own copy), never the
// window or the launcher.
func (a *App) computeDetectedTools() []settingsui.DetectedTool {
	candidates := inventory.Shared().Candidates()
	connected := map[string]bool{}
	if a.Integrations != nil {
		// Both spellings: the inventory records the resolved path, a candidate
		// may be the symlink that points at it (the same pair `hasExecutable`
		// compares).
		for _, integration := range a.Integrations.Inventory().Integrations {
			path := integration.Entry.ExecutablePath
			if path == "" {
				continue
			}
			connected[path] = true
			connected[filepath.Clean(path)] = true
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				connected[resolved] = true
			}
		}
	}
	out := make([]settingsui.DetectedTool, 0, maxDetectedTools)
	dirs := extensions.SearchDirectories()
	platform := tools.Platform()
	managers := tools.DetectManagers(dirs)
	for _, candidate := range candidates {
		if !candidate.Available || candidate.Path == "" {
			continue
		}
		if connected[candidate.Path] || connected[filepath.Clean(candidate.Path)] {
			continue
		}
		// A GUI bundle's own helper executables are not "tools a person
		// connects": the curated ranking already puts the recognisable names
		// first, so the cap collects them.
		out = append(out, settingsui.DetectedTool{
			Name:             candidate.Name,
			Path:             candidate.Path,
			Description:      candidate.Description,
			RuntimeAvailable: candidate.Available,
			InstallCommand:   detectedInstallCommand(candidate, platform, managers),
		})
		if len(out) >= maxDetectedTools {
			break
		}
	}
	return out
}
