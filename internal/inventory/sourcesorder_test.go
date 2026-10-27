package inventory

import (
	"reflect"
	"testing"
)

// The vocabulary's order is the reference's declaration order, not ours. The
// reference sorts a candidate's `sources` on every push
// (`extensions/inventory.rs:996-998`), so the list it hands a reader is in the
// enum's own order (`inventory.rs:24-40`: … `Nix`, `Brew`, `LaunchServices`,
// `Registry`, `Scoop`, `Chocolatey`, `WinGet`, `Wsl`). `scoop` therefore sorts
// before `chocolatey`; the table used to have them the other way round while
// its own comment claimed it was the reference's order.
func TestTheVocabularyIsInTheReferenceOrder(t *testing.T) {
	want := []DiscoverySource{
		SourcePath,
		SourceDesktop,
		SourceFlatpak,
		SourceSnap,
		SourceNix,
		SourceBrew,
		SourceLaunchService,
		SourceScoop,
		SourceChocolatey,
		SourceWinGet,
	}
	if !reflect.DeepEqual(discoverySources, want) {
		t.Fatalf("discoverySources = %v\nwant the reference's declaration order %v", discoverySources, want)
	}
	if sourceRank(SourceScoop) >= sourceRank(SourceChocolatey) {
		t.Errorf("sourceRank(scoop) = %d, sourceRank(chocolatey) = %d; scoop sorts first in the reference",
			sourceRank(SourceScoop), sourceRank(SourceChocolatey))
	}
	if sourceRank(DiscoverySource("elsewhere")) != len(discoverySources) {
		t.Errorf("sourceRank(unknown) = %d, want %d (last)", sourceRank(DiscoverySource("elsewhere")), len(discoverySources))
	}
}

// A candidate's `Sources` is kept in the vocabulary's order, not in the order
// the walks happened to find it. The merge is driven here with sources in
// reverse declaration order, so an `append`-only merge (the pre-P10-r shape)
// would answer `[winget flatpak desktop path]` and fail.
func TestSourcesAreListedInTheReferenceOrder(t *testing.T) {
	byPath := map[string]*Candidate{}
	merge := mergeInto(byPath)
	const path = "/opt/floter-fixture/tool"
	merge(path, "tool", "", SourceWinGet, QualityAutoDetected)
	merge(path, "tool", "", SourceFlatpak, QualityAutoDetected)
	merge(path, "tool", "", SourceDesktop, QualityAutoDetected)
	merge(path, "tool", "", SourcePath, QualityAutoDetected)
	// A second sighting of a source already recorded must not duplicate it.
	merge(path, "tool", "", SourceDesktop, QualityNativeSupport)

	candidate, ok := byPath[normalized(path)]
	if !ok {
		t.Fatalf("the merge table has no entry for %q", path)
	}
	want := []DiscoverySource{SourcePath, SourceDesktop, SourceFlatpak, SourceWinGet}
	if !reflect.DeepEqual(candidate.Sources, want) {
		t.Fatalf("Sources = %v, want the declaration order %v", candidate.Sources, want)
	}
}
