//go:build !windows

package extensions

import "testing"

// A frame that moved the command count upward announces once and carries the
// display facts the drawer needs.
func TestDecideDriftNoticeAnnouncesAChange(t *testing.T) {
	gate := NewDriftNoticeGate()
	frame := ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(3),
		CommandCount:         intPtr(5),
	}
	decision := DecideDriftNotice(frame, "Gadgets", gate)
	if decision.Announce == nil {
		t.Fatalf("a changed count did not announce: %+v", decision)
	}
	if decision.Announce.Delta.Kind != DeltaIncrease || decision.Announce.Delta.Count != 2 {
		t.Errorf("delta = %+v, want increase 2", decision.Announce.Delta)
	}
	if decision.Display.ExtensionID != "dev.floter.tool" || decision.Display.ToolVersion != "1.4.0" {
		t.Errorf("display identity = %+v", decision.Display)
	}
	if decision.Display.CommandCount == nil || *decision.Display.CommandCount != 5 {
		t.Errorf("display command count = %v, want 5", decision.Display.CommandCount)
	}
	if gate.Size() != 1 {
		t.Errorf("gate size = %d, want 1", gate.Size())
	}
}

// A decrease announces too, with a positive magnitude.
func TestDecideDriftNoticeAnnouncesADecrease(t *testing.T) {
	gate := NewDriftNoticeGate()
	decision := DecideDriftNotice(ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(5),
		CommandCount:         intPtr(2),
	}, "Gadgets", gate)
	if decision.Announce == nil {
		t.Fatalf("a decrease did not announce")
	}
	if decision.Announce.Delta.Kind != DeltaDecrease || decision.Announce.Delta.Count != 3 {
		t.Errorf("delta = %+v, want decrease 3", decision.Announce.Delta)
	}
}

// Rule 1: a re-probe that left the count alone has nothing to announce, and
// crucially must not burn the once-only slot — the real change after it still
// speaks.
func TestNoChangeDoesNotBurnTheGate(t *testing.T) {
	gate := NewDriftNoticeGate()
	unchanged := ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(3),
		CommandCount:         intPtr(3),
	}
	if decision := DecideDriftNotice(unchanged, "Gadgets", gate); decision.Announce != nil {
		t.Fatalf("an unchanged count announced: %+v", decision.Announce)
	}
	if gate.Size() != 0 {
		t.Fatalf("a no-op frame consumed a gate slot: size = %d", gate.Size())
	}
	changed := unchanged
	changed.CommandCount = intPtr(4)
	if decision := DecideDriftNotice(changed, "Gadgets", gate); decision.Announce == nil {
		t.Errorf("the change after a no-op was silenced")
	}
}

// Rule 2: without a name the toast would say "an integration", so it waits —
// and like a no-op, it must not burn the slot before the name arrives.
func TestNoNameWaitsWithoutBurningTheGate(t *testing.T) {
	gate := NewDriftNoticeGate()
	frame := ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(3),
		CommandCount:         intPtr(5),
	}
	if decision := DecideDriftNotice(frame, "", gate); decision.Announce != nil {
		t.Fatalf("a nameless frame announced: %+v", decision.Announce)
	}
	if gate.Size() != 0 {
		t.Fatalf("a nameless frame consumed a gate slot: size = %d", gate.Size())
	}
	if decision := DecideDriftNotice(frame, "Gadgets", gate); decision.Announce == nil {
		t.Errorf("the frame was silenced after its name arrived")
	}
}

// Rule 3: the same change announces once; a genuinely different change still
// speaks.
func TestSameChangeAnnouncesOncePerSession(t *testing.T) {
	gate := NewDriftNoticeGate()
	frame := ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(3),
		CommandCount:         intPtr(5),
	}
	if decision := DecideDriftNotice(frame, "Gadgets", gate); decision.Announce == nil {
		t.Fatalf("the first offer did not announce")
	}
	if decision := DecideDriftNotice(frame, "Gadgets", gate); decision.Announce != nil {
		t.Errorf("the identical frame announced twice")
	}
	// A different magnitude is different news.
	bigger := frame
	bigger.CommandCount = intPtr(6)
	if decision := DecideDriftNotice(bigger, "Gadgets", gate); decision.Announce == nil {
		t.Errorf("a larger change was silenced")
	}
	// So is a different tool version, even with the same counts.
	reversioned := frame
	reversioned.ToolVersion = "1.5.0"
	if decision := DecideDriftNotice(reversioned, "Gadgets", gate); decision.Announce == nil {
		t.Errorf("a change under a new version was silenced")
	}
	if gate.Size() != 3 {
		t.Errorf("gate size = %d, want 3", gate.Size())
	}
}

// A frame with a missing count says nothing and fabricates nothing: the
// display keeps nil on either side rather than inventing a zero, and no toast
// (or gate slot) is spent on an incomparable change.
func TestMissingCountsFabricateNothing(t *testing.T) {
	gate := NewDriftNoticeGate()
	for _, frame := range []ReprobeNoticeFrame{
		{ExtensionID: "a", ToolVersion: "1", PreviousCommandCount: nil, CommandCount: intPtr(5)},
		{ExtensionID: "a", ToolVersion: "1", PreviousCommandCount: intPtr(3), CommandCount: nil},
		{ExtensionID: "a", ToolVersion: "1", PreviousCommandCount: nil, CommandCount: nil},
	} {
		decision := DecideDriftNotice(frame, "Gadgets", gate)
		if decision.Announce != nil {
			t.Errorf("an incomparable frame announced: %+v", decision.Announce)
		}
		if decision.Display.CommandCount != frame.CommandCount {
			t.Errorf("display fabricated a count: %+v", decision.Display)
		}
		if decision.Display.PreviousCommandCount != frame.PreviousCommandCount {
			t.Errorf("display fabricated a previous count: %+v", decision.Display)
		}
	}
	if gate.Size() != 0 {
		t.Errorf("incomparable frames consumed gate slots: %d", gate.Size())
	}
}

// The key is stable across identical frames and distinct across any of its
// three components.
func TestReprobeNoticeKeyIdentity(t *testing.T) {
	delta := DriftDelta{Kind: DeltaIncrease, Count: 2}
	base := ReprobeNoticeKey("dev.floter.tool", "1.4.0", delta)
	if base != ReprobeNoticeKey("dev.floter.tool", "1.4.0", delta) {
		t.Errorf("the same change produced two keys")
	}
	for name, key := range map[string]string{
		"extension": ReprobeNoticeKey("dev.floter.other", "1.4.0", delta),
		"version":   ReprobeNoticeKey("dev.floter.tool", "1.5.0", delta),
		"kind":      ReprobeNoticeKey("dev.floter.tool", "1.4.0", DriftDelta{Kind: DeltaDecrease, Count: 2}),
		"count":     ReprobeNoticeKey("dev.floter.tool", "1.4.0", DriftDelta{Kind: DeltaIncrease, Count: 3}),
	} {
		if key == base {
			t.Errorf("a different %s produced the same key", name)
		}
	}
}

// The gate is session-scoped: a fresh gate re-arms the same change.
func TestFreshGateRearms(t *testing.T) {
	frame := ReprobeNoticeFrame{
		ExtensionID:          "dev.floter.tool",
		ToolVersion:          "1.4.0",
		PreviousCommandCount: intPtr(3),
		CommandCount:         intPtr(5),
	}
	first := NewDriftNoticeGate()
	if DecideDriftNotice(frame, "Gadgets", first).Announce == nil {
		t.Fatalf("the first session did not announce")
	}
	second := NewDriftNoticeGate()
	if DecideDriftNotice(frame, "Gadgets", second).Announce == nil {
		t.Errorf("a fresh session did not re-arm")
	}
}
