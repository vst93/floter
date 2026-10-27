package ci

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// mygo v0.7.0 named the NSIS installer after the app's slug —
// `<slug>-setup-<version>-<arch>.exe`, as `floter-setup-0.3.24-amd64.exe` —
// where v0.6.4 wrote `<Name> Setup <version> <arch>.exe`. The release workflow
// still globbed `*Setup*.exe`, which matches nothing after the upgrade: the
// copy is guarded by `[ -n "$SETUP" ]`, so the release would have shipped no
// Windows installer and said nothing, while the notes table and docs/release.md
// kept linking to `floter-<v>-windows-x86_64-setup.exe`.
//
// The pin reads the glob out of the workflow and matches it against the name
// the build really makes, so neither half can drift alone.
func TestTheWindowsSetupGlobMatchesMygoNames(t *testing.T) {
	workflow := repoFile(t, ".github/workflows/release-mygo.yml")

	if strings.Contains(workflow, "*Setup*") {
		t.Error("release-mygo.yml still globs `*Setup*`, which mygo >= v0.7.0 no longer produces")
	}

	// SETUP=$(ls "$D"/*-setup-*.exe 2>/dev/null | head -1 || true)
	re := regexp.MustCompile(`SETUP=\$\(ls "\$D"/([^\s"]+)`)
	m := re.FindStringSubmatch(workflow)
	if m == nil {
		t.Fatal("release-mygo.yml no longer picks the setup asset with `SETUP=$(ls \"$D\"/<glob>)`")
	}
	glob := m[1]

	// The installer v0.7.0 writes for this app: slug, "-setup-", version, arch.
	for _, name := range []string{
		"floter-setup-0.3.24-amd64.exe",
		"floter-setup-0.3.24-arm64.exe",
	} {
		ok, err := filepath.Match(glob, name)
		if err != nil {
			t.Fatalf("glob %q: %v", glob, err)
		}
		if !ok {
			t.Errorf("the setup glob %q does not match the installer the build makes, %q", glob, name)
		}
	}
}

// TestTheAppNameKeepsArtifactNamesStable pins what mygo v0.7.0 made load-bearing:
// every artifact name (dmg, NSIS installer, deb, tarball, the Linux executable
// itself) is the app's slug, not its display name. A CJK name now takes a
// pinyin slug by default, and a `slug` key overrides it outright — either would
// move the names users already have installed, so the app name stays ASCII and
// the slug stays the one the workflow and the docs spell: `floter`.
func TestTheAppNameKeepsArtifactNamesStable(t *testing.T) {
	var cfg struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal([]byte(repoFile(t, "mygo.json")), &cfg); err != nil {
		t.Fatalf("parse mygo.json: %v", err)
	}
	if cfg.Name == "" {
		t.Fatal("mygo.json has no name")
	}
	for _, r := range cfg.Name {
		if r > unicode.MaxASCII {
			t.Fatalf("mygo.json name %q is not ASCII: mygo >= v0.7.0 slugs it through pinyin, moving every artifact name", cfg.Name)
		}
	}

	// The slug mygo derives without a `slug` key: lower case, non-alphanumerics
	// collapsed to single hyphens. Only the shape this app needs is modelled.
	slug := cfg.Slug
	if slug == "" {
		var b strings.Builder
		gap := false
		for _, r := range strings.ToLower(cfg.Name) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if gap && b.Len() > 0 {
					b.WriteByte('-')
				}
				gap = false
				b.WriteRune(r)
				continue
			}
			gap = true
		}
		slug = b.String()
	}
	if slug != "floter" {
		t.Errorf("mygo.json slugs to %q, but the release assets, install paths and docs are all named floter-*", slug)
	}
}
