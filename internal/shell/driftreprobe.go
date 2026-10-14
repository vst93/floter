package shell

import (
	"context"
	"log"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// The app's half of the silent drift re-probe: which integrations are worth
// checking, the once-per-session loop that checks them, and the one toast a
// real change earns.
//
// The reference is `src-tauri/src/commands/extensions.rs` —
// `extensions_list` collects the generated custom integrations whose upstream
// tool may have drifted and hands them to `dispatch_drift_reprobe`, which
// fires and forgets a background loop, and that loop raises one completion
// notification when anything moved. The visible half of the same change lives
// in `extensions.DecideDriftNotice`; this file owns the wiring, so a listing
// stays a listing and a hung tool can never hold up the response.

// driftReprobeCandidates is what a listing hands to the background check: every
// installed integration that could have a derived command list worth keeping
// current. The eligibility rule itself lives in `extensions` (it reads the
// repository, the manifest and the program), so this only reads the inventory.
func (a *App) driftReprobeCandidates() []string {
	if a.Integrations == nil {
		return nil
	}
	inventory := a.Integrations.Inventory()
	out := make([]string, 0, len(inventory.Integrations))
	for _, integration := range inventory.Integrations {
		out = append(out, integration.Entry.ID)
	}
	return out
}

// reprobeDriftedIntegrations runs the version-drift check for every candidate,
// on a goroutine of its own: the list request has already been answered with
// the previous descriptor. A loop already in flight is skipped rather than
// stacked (the reference's `try_start_drift_reprobe`), so rapid listings cost
// at most one probe per drift.
func (a *App) reprobeDriftedIntegrations() {
	// Nil callers exist in tests that build the App by hand; the app always
	// has a store.
	if a.Integrations == nil {
		return
	}
	candidates := a.driftReprobeCandidates()
	if len(candidates) == 0 {
		return
	}
	if !a.startDriftReprobe() {
		return
	}
	go func() {
		defer a.finishDriftReprobe()
		changed := false
		for _, id := range candidates {
			if a.reprobeDriftedIntegration(id) {
				changed = true
			}
		}
		if changed {
			// The re-probe rewrote descriptors and recorded versions on disk,
			// so the settings rows (and the launcher's commands) re-read them.
			// This is a plain store refresh rather than RefreshIntegrations:
			// the version is already recorded, so the check would be a no-op,
			// but re-entering it would still stack another loop for nothing.
			a.refreshAfterDrift()
		}
	}()
}

// reprobeDriftedIntegration checks one integration and announces a change when
// it found one. It reports whether a re-probe actually ran.
//
// The rules and their order are the pure decision's (`DecideDriftNotice`): a
// frame with no command-count movement says nothing, a frame whose row cannot
// be named says nothing, and one change speaks once per session. The gate is
// consulted only where the decision does, so a no-op frame can never burn the
// once-only slot.
func (a *App) reprobeDriftedIntegration(id string) bool {
	result, err := extensions.CheckToolVersionDrift(context.Background(), a.Paths, id)
	if err != nil {
		// Fail soft: the old descriptor stays, the entry is never marked
		// broken, and a later listing retries. Best-effort is the contract
		// (see `CheckToolVersionDrift`).
		log.Printf("floter: could not re-probe %s on version drift: %v", id, err)
		return false
	}
	if !result.ReprobeRan {
		return false
	}
	frame := extensions.ReprobeNoticeFrame{
		ExtensionID:          id,
		ToolVersion:          result.ToolVersion,
		PreviousCommandCount: result.Report.PreviousCommandCount,
		CommandCount:         intPointer(result.Report.CommandCount),
	}
	// A frame only ever carries a real count here, so the "no change" rule
	// inside the decision is what keeps an unchanged list silent.
	decision := extensions.DecideDriftNotice(frame, a.integrationName(id), a.driftNoticeGate())
	if decision.Announce != nil {
		a.raiseDriftNotice(id, decision.Announce.Delta)
	}
	return true
}

// driftNoticeGate is the session's "tell the user once" gate, created on first
// use. It is deliberately not persisted: a relaunch re-arms, because "your
// commands just changed" is news, not a read/unread flag.
func (a *App) driftNoticeGate() extensions.DriftGate {
	a.driftMu.Lock()
	defer a.driftMu.Unlock()
	if a.driftGate == nil {
		a.driftGate = extensions.NewDriftNoticeGate()
	}
	return a.driftGate
}

// startDriftReprobe claims the session's one in-flight loop; finishDriftReprobe
// releases it. The pair is what keeps a listing storm from spawning a probe
// storm.
func (a *App) startDriftReprobe() bool {
	a.driftMu.Lock()
	defer a.driftMu.Unlock()
	if a.driftRunning {
		return false
	}
	a.driftRunning = true
	return true
}

func (a *App) finishDriftReprobe() {
	a.driftMu.Lock()
	a.driftRunning = false
	a.driftMu.Unlock()
}

// refreshAfterDrift re-reads the extension state after a re-probe rewrote a
// descriptor, and hands the launcher the fresh command list.
func (a *App) refreshAfterDrift() {
	a.Integrations.Refresh(context.Background())
	a.onMain(func() {
		a.Launcher.SetCommands(a.launcherCommands())
		// The settings rows read freshness from disk, so a frame redraws them
		// with the new timestamp and command count.
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

// integrationName resolves an id to the row's name for the toast's sentence: a
// message that could only say "an integration" is worse than staying silent,
// so the decision refuses to announce without one.
func (a *App) integrationName(id string) string {
	if a.Integrations == nil {
		return ""
	}
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		return ""
	}
	return integration.Name
}

// raiseDriftNotice queues the one toast a real command-list change earns: the
// reference's `onNotify("success", t("settings.extensions.reprobeNotice"), {
// label: t("settings.extensions.viewDetails"), run: … })`
// (`src/ExtensionsPanel.tsx:2118-2132`). Its action takes the user to the
// integrations list — this build has no per-integration drawer, so the card
// that carries the freshness dot is the closest equivalent of "view details".
func (a *App) raiseDriftNotice(id string, delta extensions.DriftDelta) {
	name := a.integrationName(id)
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	// The direction pick is the reference's `reprobeNoticeDeltaIncrease` /
	// `reprobeNoticeDeltaDecrease`: the decision already settled the direction,
	// so the sentence only words it.
	deltaWord := copy.ReprobeDeltaIncrease(delta.Count)
	if delta.Kind == extensions.DeltaDecrease {
		deltaWord = copy.ReprobeDeltaDecrease(delta.Count)
	}
	message := copy.ReprobeNotice(name, deltaWord)
	a.queueToast(ui.Toast{
		ID:       driftNoticeID,
		Title:    message,
		Type:     "success",
		Timeout:  successNotice,
		Action:   copy.ViewDetails,
		OnAction: func() { a.openIntegrationCard(id) },
	})
}

// openIntegrationCard is the drift toast's action: the settings surface on the
// Integrations page, where the card carries the freshness dot the notice just
// changed. The page is set *after* the surface opens, because opening
// re-restores the user's remembered page (the reference's `setSelectedId`
// landed on the panel it was already on, so the order mattered there too).
func (a *App) openIntegrationCard(id string) {
	a.Open(SurfaceSettings)
	a.Settings.Page = settingsui.IntegrationsPage()
}

// driftLoopRunning reports whether the background drift loop holds the claim;
// the tests read it through here so the flag is never touched unsynchronized.
func (a *App) driftLoopRunning() bool {
	a.driftMu.Lock()
	defer a.driftMu.Unlock()
	return a.driftRunning
}

// driftNoticeID is the drift toast's own id, distinct from the refusal
// notices': a change must not replace a refusal (or vice versa), and the two
// can be on screen at once.
const driftNoticeID = "floter-drift-notice"

// intPointer is the address of a copy, for the notice frame's optional count.
func intPointer(value int) *int { return &value }
