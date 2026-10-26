package shell

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The reference's manifest matrix (`main:src-tauri/src/deep_link.rs:270-338`), row
// for row, with this build's verdict next to it. Most rows are the same verdict
// and are simply ported; the rest are the two documented divergences pinned: a
// relative path (and `~`) is accepted here because this build's link also
// arrives from the user's own command line, and a `https` remote manifest is
// refused here because there is no remote install path to land on
// (`linkManifestValue`, linkmanifest.go).
func TestAManifestValueIsAPathThisBuildCanStat(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	cases := []struct {
		value string
		want  string // "" means refused
		why   string
	}{
		{"/home/u/tool.json", "/home/u/tool.json", "absolute file: same verdict as the reference"},
		{"/home/u/..notes.json", "/home/u/..notes.json", "a filename starting with two dots is not a traversal (reference :1597-1600)"},
		{"/home/u/../../etc/tool.json", "", "traversal: ManifestTraversal (:328-333)"},
		{"/home/u/.config/floter/tools/../tools/tool.json", "", "an interior .. is refused even when it cancels out (:1592-1595)"},
		{`..\tool.json`, "", "the backslash spelling is a traversal on all three platforms, not just Windows"},
		{"tool.json", filepath.Join(home, "tool.json"), "divergence: relative resolves against home (plan:690-695)"},
		{"~/tools/tool.json", filepath.Join(home, "tools", "tool.json"), "divergence: ~ is this build's documented shorthand"},
		{"~/../tool.json", "", "traversal still wins over the ~ shorthand"},
		{"https://example.com/tool.json", "", "divergence: no remote manifest install path in this build"},
		{"file:///home/u/tool.json", "", "same verdict: only https could ever be a manifest (:301-317)"},
		{"http://example.com/tool.json", "", "same verdict: plain http is refused"},
		{"https://user:pw@example.com/tool.json", "", "same verdict: embedded credentials are refused"},
		{"", "", "MissingManifest (:290-292)"},
		{"   ", "", "trimmed to nothing is still missing"},
		{"/home/u/to\nol.json", "", "a control character is ManifestInsecure (:296-299)"},
		{"/home/u/to\x00ol.json", "", "a NUL would truncate the value on the way to the filesystem"},
		{"/home/u/tool.txt", "/home/u/tool.txt", "the suffix is the file form's rule, not the value's (manifestFileFits)"},
	}
	for _, c := range cases {
		got, ok := linkManifestValue(c.value)
		if c.want == "" {
			if ok {
				t.Errorf("%q was accepted as %q, want a refusal: %s", c.value, got, c.why)
			}
			continue
		}
		if !ok {
			t.Errorf("%q was refused, want %q: %s", c.value, c.want, c.why)
			continue
		}
		if got != c.want {
			t.Errorf("%q resolved to %q, want %q: %s", c.value, got, c.want, c.why)
		}
	}
}

// The reference's table lists `C:\tools\tool.json` as "local", and that is true
// on Windows only: on POSIX `Path::is_absolute` says no and the reference
// refuses it (`:1815-1825`). This build accepts it on both, because its
// home-relative rule turns the value into a (probably absent) file under home —
// the stat refuses it a step later. Pinned rather than aligned: the divergence is
// the documented relative-path rule, not this spelling.
func TestAWindowsSpelledPathIsStillHomeRelativeHere(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	got, ok := linkManifestValue(`C:\tools\tool.json`)
	if !ok {
		t.Fatalf("a windows-spelled path was refused; the home-relative rule accepts it on every platform")
	}
	want := `C:\tools\tool.json`
	if runtime.GOOS != "windows" {
		want = filepath.Join(home, `C:\tools\tool.json`)
	}
	if got != want {
		t.Errorf("resolved to %q, want %q", got, want)
	}
}

// The suffix rule is the *file* form's, because this build's connect value may
// also name the package directory (documented, and three tests hand it a
// directory) and a directory has no suffix to check. The reference has no
// directory form, so this is not a divergence it can answer.
func TestOnlyTheFileFormNeedsTheJSONSuffix(t *testing.T) {
	for _, c := range []struct {
		path string
		want bool
	}{
		{"/home/u/tool.json", true},
		{"/home/u/floter.extension.json", true},
		{"/home/u/tool.txt", false},
		{"/home/u/tool", false},
		{"/home/u/TOOL.JSON", false}, // case-sensitive, like str::ends_with
		{"/home/u/tool.json.txt", false},
	} {
		if got := manifestFileFits(c.path); got != c.want {
			t.Errorf("manifestFileFits(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// A value the value rule refuses never reaches the page: the refusal is a
// sentence, not a trip to the integrations page. The same place discipline the
// parameter table (P10-h) and the args hint (P10-i) already keep.
func TestTheConnectLinkRefusesAManifestThatIsNotAPath(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, value := range []string{
		"/home/u/../../etc/tool.json",
		"~/../tool.json",
		"https://example.com/tool.json",
		"file:///home/u/tool.json",
		"/home/u/to\nol.json",
		"",
	} {
		link := "floter://connect?manifest=" + url.QueryEscape(value)
		a := newApp(t)
		surface, page := a.Surf, a.Settings.Page
		a.HandleURL(link)
		if a.Surf != surface {
			t.Errorf("%q switched surface to %v", value, a.Surf)
		}
		if a.Settings.Page != page {
			t.Errorf("%q landed on page %v, want the page it started on", value, a.Settings.Page)
		}
		if got := pendingNoticeText(a); got != want {
			t.Errorf("%q reads %q, want %q", value, got, want)
		}
	}
}

// The other half of the file form, on the live path: a link that names a real
// file in a real package is refused when the file is not a `.json` one — without
// the rule that value connects whatever package happens to live in the same
// directory — and the documented `.json` spelling of the same package still
// connects. The gate is a gate, not a wall.
func TestAConnectLinkTakesAJSONFileOrThePackageDirectory(t *testing.T) {
	requirePOSIXShell(t)
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := writeLinkedPackage(t, paths, "dev.floter.jsononly", `{
	  "schemaVersion": "2.0", "id": "dev.floter.jsononly", "name": "JSON only",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "permissions": ["environment"]
	}`)
	for name, body := range map[string]string{"tool.sh": "#!/bin/sh\n", "notes.txt": "not a manifest\n"} {
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	a.HandleURL("floter://connect?manifest=" + url.QueryEscape(filepath.Join(pkg, "notes.txt")))
	// The refusal is raised off the main thread (the connect flow runs in the
	// background), so the sentence arrives a moment later.
	deadline := time.Now().Add(5 * time.Second)
	for pendingNotices(a) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the refusal never arrived")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got, want := pendingNoticeText(a), i18n.For("en").Settings.DeepLinkRejected; got != want {
		t.Fatalf("a non-.json file reads %q, want the refusal %q", got, want)
	}
	if _, ok := a.Integrations.Inventory().WithID("dev.floter.jsononly"); ok {
		t.Fatal("a non-.json file connected the package next to it")
	}

	// The documented spelling of the same package still lands.
	a.HandleURL("floter://connect?manifest=" + url.QueryEscape(filepath.Join(pkg, "floter.extension.json")))
	deadline = time.Now().Add(10 * time.Second)
	for {
		if _, ok := a.Integrations.Inventory().WithID("dev.floter.jsononly"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the .json spelling of the same package never connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
