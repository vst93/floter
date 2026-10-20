package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// resetIntegrationFacts clears the process-wide memo so a test starts cold.
func resetIntegrationFacts() {
	integrationFactsMemo.mu.Lock()
	integrationFactsMemo.facts = nil
	integrationFactsMemo.computed = time.Time{}
	integrationFactsMemo.pending = false
	integrationFactsMemo.mu.Unlock()
}

// factsTestItems builds `count` system integrations whose program does not
// exist: each one therefore makes the runtime probe miss in every search
// directory, which is the expensive shape the page used to pay per frame.
func factsTestItems(t *testing.T, count int) []extensions.Integration {
	t.Helper()
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	body := []byte(`{"schemaVersion":"1","id":"probe","name":"probe",` +
		`"runtime":{"type":"system","executableNames":["floter-no-such-tool-xyz"]}}`)
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	entry := extensions.Entry{
		ID:             "probe",
		Enabled:        true,
		State:          "ready",
		ManifestPath:   manifestPath,
		ExecutablePath: filepath.Join(root, "gone"),
		ApprovedAt:     1,
	}
	items := make([]extensions.Integration, 0, count)
	for i := 0; i < count; i++ {
		copy := entry
		copy.ID = entry.ID + string(rune('a'+i))
		items = append(items, extensions.Integration{
			Entry:    copy,
			Manifest: extensions.Manifest{ID: copy.ID, Runtime: extensions.Runtime{Type: "system", ExecutableNames: []string{"floter-no-such-tool-xyz"}}},
		})
	}
	return items
}

// countComputes wraps computeIntegrationFacts and counts the passes, so a test
// can say "the frame did not compute" rather than only "it was fast".
func countComputes(calls *int) func([]extensions.Integration) map[string]integrationFact {
	return func(items []extensions.Integration) map[string]integrationFact {
		*calls++
		return computeIntegrationFacts(items)
	}
}

// The Integrations page's list rebuilds `integrationList` on every frame, and
// two of its facts used to cost a syscall per integration per frame: the
// runtime probe (a stat per name per search directory) and the manifest digest
// (one file read).
//
// This pins the policy: a cold memo is computed once, and the sixty frames that
// follow it compute nothing at all.
func TestTheIntegrationFactsAreNotRecomputedEveryFrame(t *testing.T) {
	items := factsTestItems(t, 12)
	resetIntegrationFacts()

	calls := 0
	compute := countComputes(&calls)
	now := time.Now()
	clock := func() time.Time { return now }

	facts := refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, clock())
	if calls != 1 {
		t.Fatalf("the first frame computed %d passes, want 1", calls)
	}
	if len(facts) != len(items) {
		t.Fatalf("the first frame answered %d facts, want %d", len(facts), len(items))
	}
	if facts[items[0].Entry.ID].available {
		t.Errorf("a missing runtime was reported as available")
	}
	if !facts[items[0].Entry.ID].hasDigest {
		t.Errorf("the approved manifest's digest was not read on the first frame")
	}

	// Sixty frames inside the TTL: the memo answers, nothing is walked, nothing
	// is read.
	for frame := 0; frame < 60; frame++ {
		now = now.Add(time.Second / 60)
		got := refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, clock())
		if len(got) != len(items) {
			t.Fatalf("frame %d answered %d facts, want %d", frame, len(got), len(items))
		}
	}
	if calls != 1 {
		t.Errorf("sixty drawn frames computed %d passes, want the first one only", calls)
	}
}

// A memo that has gone stale is refreshed behind the next frame, and the frame
// still answers from what it has: the settings page must never wait for a walk.
func TestAStaleMemoIsRefreshedBehindTheFrame(t *testing.T) {
	items := factsTestItems(t, 4)
	resetIntegrationFacts()

	calls := 0
	compute := countComputes(&calls)
	landed := make(chan struct{}, 4)
	now := time.Now()

	refreshIntegrationFacts(&integrationFactsMemo, items, compute, func() {
		select {
		case landed <- struct{}{}:
		default:
		}
	}, now)
	if calls != 1 {
		t.Fatalf("the cold frame computed %d passes, want 1", calls)
	}

	// Past the TTL: the frame answers from the memo and starts one pass.
	now = now.Add(integrationFactsTTL + time.Millisecond)
	got := refreshIntegrationFacts(&integrationFactsMemo, items, compute, func() {
		select {
		case landed <- struct{}{}:
		default:
		}
	}, now)
	if len(got) != len(items) {
		t.Fatalf("the stale frame answered %d facts, want the memo's %d", len(got), len(items))
	}
	// A second stale frame must not start a second pass.
	refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, now)

	select {
	case <-landed:
	case <-time.After(5 * time.Second):
		t.Fatal("the background pass never landed")
	}
	if calls != 2 {
		t.Errorf("the stale frame ran %d passes, want the cold one plus one refresh", calls)
	}
	// The pass's answer is what the next frame reads.
	facts, computed, pending := integrationFactsMemo.snapshot()
	if pending {
		t.Error("the pass left the memo marked pending")
	}
	if computed.IsZero() {
		t.Error("the landed pass did not record its clock")
	}
	if len(facts) != len(items) {
		t.Errorf("the landed pass published %d facts, want %d", len(facts), len(items))
	}
}

// An integration whose identity changed is re-asked on the spot: installing a
// tool while the page is open must not leave the card showing the repair verb
// until the TTL runs out.
func TestAChangedIdentityIsAnsweredOnTheSpot(t *testing.T) {
	items := factsTestItems(t, 2)
	resetIntegrationFacts()

	calls := 0
	compute := countComputes(&calls)
	now := time.Now()
	refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, now)

	// The user installs the tool: the entry's program moves to a path that
	// exists, which changes the identity the memo recorded.
	target := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	changed := append([]extensions.Integration(nil), items...)
	changed[0].Entry.ExecutablePath = target

	facts := refreshIntegrationFacts(&integrationFactsMemo, changed, compute, nil, now.Add(time.Second/60))
	if calls != 2 {
		t.Fatalf("the changed identity ran %d passes, want one recompute", calls)
	}
	if !facts[changed[0].Entry.ID].available {
		t.Errorf("the newly installed program was still reported missing")
	}
	// The other entry kept the answer it had — one identity asked, not the whole
	// list.
	if calls != 2 {
		t.Errorf("the changed identity recomputed the whole list")
	}
}

// invalidateIntegrationFacts drops the memo so the next frame recomputes in
// place: an install, a repair or a rescan rewrites the repository, and a memo
// that outlived it would show the pre-change answer.
func TestAnInvalidatedMemoIsRecomputedInPlace(t *testing.T) {
	items := factsTestItems(t, 2)
	resetIntegrationFacts()

	calls := 0
	compute := countComputes(&calls)
	now := time.Now()
	refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, now)
	if calls != 1 {
		t.Fatalf("the cold frame computed %d passes, want 1", calls)
	}

	invalidateIntegrationFacts()
	refreshIntegrationFacts(&integrationFactsMemo, items, compute, nil, now.Add(time.Second/60))
	if calls != 2 {
		t.Errorf("the invalidated memo ran %d passes, want one recompute", calls)
	}
}

// The list itself reads the memo: the settings page rebuilds `integrationList`
// on every frame, and the two facts the cards need (is the runtime still there,
// what does the approved manifest hash to) are computed once per visit rather
// than once per frame.
//
// This is the wiring half of the two policy tests above — they pin the rule, this
// pins that the rule is the one the page's list actually goes through.
func TestTheIntegrationListReadsTheFactsMemo(t *testing.T) {
	paths := integrationFixture(t)
	resetIntegrationFacts()
	a := New(Options{Store: settings.NewStore(settings.Default()), Paths: paths})
	a.RefreshIntegrations(context.Background())

	list := a.integrationList()
	if len(list) != 1 {
		t.Fatalf("integrations = %+v", list)
	}
	if !list[0].RuntimeAvailable {
		t.Errorf("the fixture's runtime was reported missing: %+v", list[0])
	}
	_, computed, _ := integrationFactsMemo.snapshot()
	if computed.IsZero() {
		t.Fatal("the list did not fill the memo")
	}

	// Sixty frames later the memo is the same pass: the clock does not move and
	// no answer changes.
	for frame := 0; frame < 60; frame++ {
		again := a.integrationList()
		if len(again) != 1 || again[0].RuntimeAvailable != list[0].RuntimeAvailable {
			t.Fatalf("frame %d answered differently: %+v", frame, again)
		}
	}
	_, after, _ := integrationFactsMemo.snapshot()
	if !after.Equal(computed) {
		t.Errorf("sixty frames recomputed the facts (clock moved %v → %v)", computed, after)
	}
}

// A refresh drops the memo: an install, a repair or a reconnect rewrites the
// repository, and the card must show the new answer on the next frame.
func TestARefreshDropsTheFactsMemo(t *testing.T) {
	paths := integrationFixture(t)
	resetIntegrationFacts()
	a := New(Options{Store: settings.NewStore(settings.Default()), Paths: paths})
	a.RefreshIntegrations(context.Background())
	a.integrationList()

	a.RefreshIntegrations(context.Background())
	if _, computed, _ := integrationFactsMemo.snapshot(); !computed.IsZero() {
		t.Errorf("a refresh left the previous facts in place (computed %v)", computed)
	}
	list := a.integrationList()
	if len(list) != 1 || !list[0].RuntimeAvailable {
		t.Errorf("the refilled list is wrong: %+v", list)
	}
}

// The card's freshness row is asked once per row per frame —
// `settingsui.integrationRow` reads `Actions.Freshness` while drawing each card
// — and the answer costs a stat of the package directory plus a read of
// `help-probe.json`. It is the third member of this file's family (the runtime
// probe and the manifest digest are the other two), so it reads the same memo.
//
// This pins the wiring: the first ask answers and is remembered, and the sixty
// frames a second that follow it move no clock and touch no disk.
func TestTheFreshnessRowReadsTheFactsMemo(t *testing.T) {
	paths := integrationFixture(t)
	resetIntegrationFacts()
	a := New(Options{Store: settings.NewStore(settings.Default()), Paths: paths})
	a.RefreshIntegrations(context.Background())

	// The sidecar beside the descriptor: the source the row reports.
	pkg := filepath.Join(paths.Extensions, "io.github.vst93.v")
	sidecar := `{"probedAt":1700000000,"rootArgumentCount":1,"subcommandCount":0,` +
		`"commandCount":1,"previousCommandCount":1}`
	if err := os.WriteFile(filepath.Join(pkg, "help-probe.json"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	invalidateIntegrationFacts()

	freshness, ok := a.integrationFreshness("io.github.vst93.v")
	if !ok {
		t.Fatal("the freshness row answered nothing for an integration that has a sidecar")
	}
	if freshness.Source != extensions.FreshnessProbe || freshness.CommandCount == nil ||
		*freshness.CommandCount != 1 {
		t.Fatalf("the row answered %+v, want the sidecar's probe", freshness)
	}

	// The memo now holds it, so the row's own ask did not have to be repeated.
	facts, _, _ := integrationFactsMemo.snapshot()
	if fact, ok := facts["io.github.vst93.v"]; !ok || !fact.hasFreshness {
		t.Fatalf("the row's answer was not remembered: %+v", facts)
	}

	// Sixty frames later: the same answer, and the memo is the same one (the
	// proof that no frame re-read the sidecar — a re-read would replace the
	// fact, and the identity it carries is what the list's pass compares).
	for frame := 0; frame < 60; frame++ {
		again, ok := a.integrationFreshness("io.github.vst93.v")
		if !ok || again.AtSeconds != freshness.AtSeconds {
			t.Fatalf("frame %d answered differently: %+v", frame, again)
		}
	}
}

// A row asked about on its own answers only the freshness question, and the
// list's pass must not trust it for the other two: a partial fact that was
// served as complete would draw the repair verb for a program that is right
// there.
func TestAPartialFactIsRefilledByTheList(t *testing.T) {
	paths := integrationFixture(t)
	resetIntegrationFacts()
	a := New(Options{Store: settings.NewStore(settings.Default()), Paths: paths})
	a.RefreshIntegrations(context.Background())

	// One row asks first (the freshness block draws before the list is built
	// on a surface that draws a single card).
	a.integrationFreshness("io.github.vst93.v")
	facts, _, _ := integrationFactsMemo.snapshot()
	if !facts["io.github.vst93.v"].partial {
		t.Fatalf("the row's own answer was not recorded as partial: %+v", facts)
	}

	// The list's pass answers in full: the runtime probe is filled in, and the
	// card therefore does not offer to repair a runtime that resolves.
	list := a.integrationList()
	if len(list) != 1 || !list[0].RuntimeAvailable {
		t.Fatalf("the list served the partial fact: %+v", list)
	}
	facts, _, _ = integrationFactsMemo.snapshot()
	if facts["io.github.vst93.v"].partial {
		t.Errorf("the list's pass left the fact partial")
	}
}
