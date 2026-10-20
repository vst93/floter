package ci

import (
	"encoding/json"
	"strings"
	"testing"
)

// The release channel is a user-facing feature: `releases/latest` is what
// install.sh and every "download" link resolve, and a pre-release is invisible
// to it. 0.3.19 was published as a pre-release while 0.3.18 stayed `latest`, so
// the two reports that round's fixes answered (「搜索页面左右两边各有一大块空
// 白」 and 「在终端中打开」 fixed at the list's first item) were verified green in
// code and still reproduced on the user's machine — the build with the fixes was
// sitting in releases with no path to it.
//
// Three pins, all of them about that one failure:
//
//  1. a version without a pre-release suffix is published as a plain release,
//     whatever the dispatch input says (the input is the way 0.3.19 went wrong);
//  2. a plain release claims `latest` when it is created;
//  3. a plain release that already exists is pulled back to `latest`, which is
//     the repair path for a version already published as a pre-release.
func TestAPlainVersionIsAlwaysTheLatestRelease(t *testing.T) {
	workflow := repoFile(t, ".github/workflows/release-mygo.yml")

	// 1 · the suffix, not the input, decides. The `-alpha|beta|preview|rc`
	// branch is the only way to reach `true` now.
	suffixGate := `if echo "$VERSION" | grep -qiE '-(alpha|beta|preview|rc)'; then`
	if !strings.Contains(workflow, suffixGate) {
		t.Fatalf("release-mygo.yml no longer decides the pre-release flag from the version suffix")
	}
	gate := workflow[strings.Index(workflow, suffixGate):]
	if !strings.Contains(gate, `PRE="false"`) {
		t.Error("a version with no pre-release suffix must be published as a plain release")
	}
	if strings.Contains(gate, `elif [ -z "$PRE" ]`) {
		t.Error("the pre-release flag must not be inherited from the dispatch input for a plain version")
	}

	// 2 · the created release takes `latest` rather than relying on the default
	// (which a later pre-release can take back).
	if !strings.Contains(workflow, `echo "--latest"`) {
		t.Error("a plain release must be created with --latest")
	}

	// 3 · an existing plain release is repaired in place.
	if !strings.Contains(workflow, `gh release edit "mygo-v$VERSION" --prerelease=false --latest`) {
		t.Error("an existing plain release must be pulled back to latest (the 0.3.19 repair path)")
	}
}

// mygo.json's version is what the workflow stamps into every asset name, so a
// suffixed version there ships a preview the channel cannot serve. The file is
// the release's own declaration; the suffix rule above is only as good as it.
func TestTheStampedVersionIsAPlainSemver(t *testing.T) {
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(repoFile(t, "mygo.json")), &manifest); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if manifest.Version == "" {
		t.Fatal("mygo.json carries no version")
	}
	for _, banned := range []string{"-alpha", "-beta", "-preview", "-rc", "+"} {
		if strings.Contains(manifest.Version, banned) {
			t.Errorf("mygo.json version %q carries %q: a preview cannot be the latest release", manifest.Version, banned)
		}
	}
	if parts := strings.Split(manifest.Version, "."); len(parts) != 3 {
		t.Errorf("mygo.json version %q is not major.minor.patch", manifest.Version)
	}
}

// A release renames `## [Unreleased]` into `## [<version>] - <date>`, and the
// workflow does it with `count=1` (release-mygo.yml, the python heredoc): one
// heading, because a changelog has one "next release" section by definition.
//
// The file had **two** of them — a merge left the 0.3.7-era block in place above
// the block the current cycle was appending to. The release would have renamed
// the first (the older one) and left the second standing, so every version
// after it would have shipped the *previous* cycle's notes under its own
// number while the current cycle's entries stayed under `[Unreleased]` forever.
// That is the same failure shape as the pre-release flag in
// TestAPlainVersionIsAlwaysTheLatestRelease: the pipeline is green, the version
// is published, and the notes the user reads are one release behind.
//
// The count the workflow assumes is pinned here rather than the merge that
// caused it, so a second `[Unreleased]` reintroduced by any later merge fails on
// the commit that adds it.
func TestTheChangelogHasOneUnreleasedSection(t *testing.T) {
	changelog := repoFile(t, "CHANGELOG.md")
	if n := strings.Count(changelog, "\n## [Unreleased]"); n != 1 {
		t.Errorf("CHANGELOG.md has %d `## [Unreleased]` sections, want 1: the release renames one (count=1), so a second leaves its notes unfiled forever", n)
	}
	// And it is the *first* section: the workflow's regexp is anchored to a line
	// start and takes the first match, so a released heading above it would make
	// the rename fire on the wrong block entirely.
	first := strings.Index(changelog, "\n## [")
	if first < 0 {
		t.Fatal("CHANGELOG.md has no release heading")
	}
	line, _, _ := strings.Cut(changelog[first+1:], "\n")
	if line != "## [Unreleased]" {
		t.Errorf("the changelog's first section is %q, want `## [Unreleased]`: the release renames the first match", line)
	}
}
