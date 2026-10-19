package ci

import (
	"strings"
	"testing"
)

// The Windows setup asset is named `windows-x86_64-setup.exe` in the release
// notes table, in docs/release.md and in the assets README users read — and the
// workflow used to copy mygo's `amd64` directory name straight into the file it
// uploaded (`windows-amd64-setup.exe`), so every link the docs published was a
// 404 and the download the install instructions describe did not exist.
//
// Pins both halves: the workflow publishes the x86_64 spelling, and it keeps
// publishing the ARM64 one beside it. A tidy-up that loops `for A in amd64
// arm64` again fails here rather than at the next release.
func TestTheWindowsSetupAssetIsNamedLikeTheDocs(t *testing.T) {
	workflow := repoFile(t, ".github/workflows/release-mygo.yml")
	docs := repoFile(t, "docs/release.md")

	for _, want := range []string{
		"floter-<v>-windows-x86_64-setup.exe",
		"floter-<v>-windows-arm64-setup.exe",
	} {
		if !strings.Contains(docs, want) {
			t.Errorf("docs/release.md no longer documents %q", want)
		}
	}

	if !strings.Contains(workflow, `"amd64:x86_64"`) {
		t.Error("release-mygo.yml must map the amd64 build to the x86_64 asset name")
	}
	if !strings.Contains(workflow, `windows-$N-setup.exe`) {
		t.Error("release-mygo.yml must name the setup asset from the mapped arch, not the directory")
	}
	if strings.Contains(workflow, `windows-$A-setup.exe`) {
		t.Error("release-mygo.yml still names the setup asset from the build directory (amd64)")
	}
	// The failures this guards are the two published names, so the release notes
	// table the workflow writes must agree with the copy above it.
	notes := workflow[strings.Index(workflow, "| \\`floter-$VERSION"):]
	for _, want := range []string{
		"floter-$VERSION-windows-x86_64-setup.exe",
		"floter-$VERSION-windows-arm64-setup.exe",
	} {
		if !strings.Contains(notes, want) {
			t.Errorf("the release notes table does not list %q", want)
		}
	}
}
