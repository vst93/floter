package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFile writes a fixture and returns its path.
func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// desktopFixture writes a desktop entry into a directory and returns the
// directory.
func desktopFixture(t *testing.T, directory, id, content string) string {
	t.Helper()
	writeFile(t, filepath.Join(directory, id), content)
	return directory
}

// stubDesktopSeams replaces the three seams the desktop-entry reader goes
// through, so a pin drives fixtures instead of the machine: the program
// resolver, the package-export roots, and the executable judgement.
func stubDesktopSeams(t *testing.T, resolve func(string) (string, bool), flatpak []string, snap string) {
	t.Helper()
	previousProgram, previousRoots, previousExecutable := desktopProgram, linuxExportRoots, executableFile
	desktopProgram = resolve
	linuxExportRoots = func() ([]string, string) { return flatpak, snap }
	executableFile = func(string) bool { return true }
	t.Cleanup(func() {
		desktopProgram, linuxExportRoots, executableFile = previousProgram, previousRoots, previousExecutable
	})
}

// The Exec line is an argv declaration, not a string split on spaces: quotes
// group, a backslash escapes, the field codes are stripped and an unknown code
// or an unterminated quote makes the line unusable.
func TestTheDesktopExecIsAnArgvNotASplitOnSpaces(t *testing.T) {
	for _, testCase := range []struct {
		name string
		exec string
		want []string
	}{
		{"a bare program", "rg %F", []string{"rg"}},
		{"a quoted path with a space", `"/opt/Some App/app" %U`, []string{"/opt/Some App/app"}},
		{"an env prefix stays one argument list", "env A=1 /snap/bin/x %u", []string{"env", "A=1", "/snap/bin/x"}},
		{"a shell command string is quoted", `sh -c "firefox %u"`, []string{"sh", "-c", "firefox "}},
		{"a literal percent", "100%%done %f", []string{"100%done"}},
		{"an unknown field code", "rg %z", nil},
		{"an unterminated quote", `rg "unclosed`, nil},
		{"blank", "   ", nil},
	} {
		got := desktopExecArgv(testCase.exec)
		if strings.Join(got, "\x00") != strings.Join(testCase.want, "\x00") {
			t.Errorf("%s: desktopExecArgv(%q) = %q, want %q", testCase.name, testCase.exec, got, testCase.want)
		}
	}
}

// An entry that is not an application, or that is hidden, is not a tool — the
// guards the reference implementation applies and the launcher's own
// application scan already had.
func TestTheDesktopEntryFiltersWhatIsNotATool(t *testing.T) {
	dir := t.TempDir()
	for _, testCase := range []struct {
		name    string
		id      string
		content string
		want    bool
	}{
		{
			name:    "an application",
			id:      "rg.desktop",
			content: "[Desktop Entry]\nName=Ripgrep\nComment=Fast search\nExec=rg %F\nType=Application\n",
			want:    true,
		},
		{
			name:    "a missing Type defaults to an application",
			id:      "notype.desktop",
			content: "[Desktop Entry]\nName=NoType\nExec=notype\n",
			want:    true,
		},
		{
			name:    "Hidden is the user's deletion marker",
			id:      "hidden.desktop",
			content: "[Desktop Entry]\nName=Hidden\nExec=hidden\nType=Application\nHidden=true\n",
			want:    false,
		},
		{
			name:    "NoDisplay",
			id:      "nodisplay.desktop",
			content: "[Desktop Entry]\nName=NoDisplay\nExec=nodisplay\nType=Application\nNoDisplay=true\n",
			want:    false,
		},
		{
			name:    "a link is not an application",
			id:      "link.desktop",
			content: "[Desktop Entry]\nName=Link\nExec=xdg-open https://example.com\nType=Link\n",
			want:    false,
		},
		{
			name:    "no Name",
			id:      "noname.desktop",
			content: "[Desktop Entry]\nExec=noname\nType=Application\n",
			want:    false,
		},
		{
			name:    "no Exec",
			id:      "noexec.desktop",
			content: "[Desktop Entry]\nName=NoExec\nType=Application\n",
			want:    false,
		},
	} {
		desktopFixture(t, dir, testCase.id, testCase.content)
		if _, ok := parseDesktopEntry(filepath.Join(dir, testCase.id)); ok != testCase.want {
			t.Errorf("%s: parsed = %v, want %v", testCase.name, ok, testCase.want)
		}
	}
	if _, ok := parseDesktopEntry(filepath.Join(dir, "absent.desktop")); ok {
		t.Error("a missing file parsed as an entry")
	}
}

// The target is the file the entry actually runs: `env VAR=…` is unwrapped, a
// `flatpak run <app-id>` names the package manager's export wrapper, `snap run
// <name>` names the snap's bin, a shell command string has no target, and a
// bare name is resolved rather than taken as a path relative to the working
// directory.
func TestTheDesktopEntryTargetReadsEnvFlatpakSnapAndShell(t *testing.T) {
	flatpakRoot := filepath.Join(t.TempDir(), "flatpak-exports")
	snapRoot := filepath.Join(t.TempDir(), "snap-bin")
	var resolved []string
	stubDesktopSeams(t, func(program string) (string, bool) {
		resolved = append(resolved, program)
		if strings.HasSuffix(program, "missing-tool") {
			return "", false
		}
		if strings.Contains(program, "flatpak-exports") || strings.Contains(program, "snap-bin") {
			return program, true
		}
		return "/resolved/" + filepath.Base(program), true
	}, []string{flatpakRoot}, snapRoot)

	for _, testCase := range []struct {
		name     string
		exec     string
		tryExec  string
		want     string
		wantCall string
	}{
		{
			name:     "a snap entry wraps its command in env",
			exec:     "env BAMF_DESKTOP_FILE_HINT=/var/lib/snapd/desktop/applications/firefox_firefox.desktop /snap/bin/firefox %u",
			want:     "/resolved/firefox",
			wantCall: "/snap/bin/firefox",
		},
		{
			name:     "a flatpak entry names the export wrapper",
			exec:     "/usr/bin/flatpak run --branch=stable --arch=x86_64 --command=gimp-2.10 org.gimp.GIMP %U",
			want:     filepath.Join(flatpakRoot, "org.gimp.GIMP"),
			wantCall: filepath.Join(flatpakRoot, "org.gimp.GIMP"),
		},
		{
			name:     "a bare flatpak program reads the same",
			exec:     "flatpak run org.gimp.GIMP",
			want:     filepath.Join(flatpakRoot, "org.gimp.GIMP"),
			wantCall: filepath.Join(flatpakRoot, "org.gimp.GIMP"),
		},
		{
			name:     "snap run names the snap's bin",
			exec:     "snap run firefox",
			want:     filepath.Join(snapRoot, "firefox"),
			wantCall: filepath.Join(snapRoot, "firefox"),
		},
		{
			name:     "a shell command string has no structured target",
			exec:     `sh -c "firefox %u"`,
			want:     "",
			wantCall: "",
		},
		{
			name:     "flatpak without run is not a tool",
			exec:     "/usr/bin/flatpak --version",
			want:     "",
			wantCall: "",
		},
		{
			name:     "a quoted program keeps its space",
			exec:     `"/opt/Some App/app" %U`,
			want:     "/resolved/app",
			wantCall: "/opt/Some App/app",
		},
		{
			name:     "an ordinary program",
			exec:     "rg %F",
			want:     "/resolved/rg",
			wantCall: "rg",
		},
		{
			name:     "a TryExec that does not resolve kills the entry",
			exec:     "rg %F",
			tryExec:  "missing-tool",
			want:     "",
			wantCall: "missing-tool",
		},
	} {
		resolved = nil
		entry, ok := parseDesktopEntry(writeFile(t, filepath.Join(t.TempDir(), "entry.desktop"),
			"[Desktop Entry]\nName=Fixture\nType=Application\nExec="+testCase.exec+"\n"+
				tryExecLine(testCase.tryExec)))
		if !ok {
			t.Fatalf("%s: the fixture entry did not parse", testCase.name)
		}
		got, ok := desktopEntryTarget(entry)
		if got != testCase.want || ok != (testCase.want != "") {
			t.Errorf("%s: target = %q ok=%v, want %q", testCase.name, got, ok, testCase.want)
		}
		if testCase.wantCall == "" {
			if len(resolved) != 0 {
				t.Errorf("%s: the resolver was asked for %q, want no call", testCase.name, resolved)
			}
			continue
		}
		if len(resolved) == 0 || resolved[0] != testCase.wantCall {
			t.Errorf("%s: the resolver was asked for %q, want %q", testCase.name, resolved, testCase.wantCall)
		}
		for _, program := range resolved {
			if program == "env" {
				t.Errorf("%s: the env prefix reached the resolver", testCase.name)
			}
		}
	}
}

// tryExecLine is the TryExec line a fixture entry needs, or nothing.
func tryExecLine(program string) string {
	if program == "" {
		return ""
	}
	return "TryExec=" + program + "\n"
}

// A bare `Exec=firefox` is resolved through the search path; it is never taken
// as a path relative to whatever directory the process happens to be in. Before
// this, every such entry produced a second, unavailable candidate at
// `<working directory>/firefox` next to the real one.
func TestABareExecNameIsResolvedNotTakenRelativeToTheWorkingDirectory(t *testing.T) {
	previousLookup := desktopLookup
	t.Cleanup(func() { desktopLookup = previousLookup })
	var asked []string
	desktopLookup = func(names ...string) (string, bool) {
		asked = append(asked, names...)
		return filepath.Join("/resolved", names[0]), true
	}
	got, ok := desktopProgram("firefox")
	if !ok || got != filepath.Join("/resolved", "firefox") {
		t.Fatalf("desktopProgram(firefox) = %q ok=%v, want the search path's answer", got, ok)
	}
	if len(asked) != 1 || asked[0] != "firefox" {
		t.Errorf("the search path was asked for %q, want [firefox]", asked)
	}
	// A path with a separator is not a search: it has to be executable as
	// written.
	missing := filepath.Join(t.TempDir(), "absent")
	if got, ok := desktopProgram(missing); ok {
		t.Errorf("desktopProgram(%q) = %q, want no answer", missing, got)
	}
	if len(asked) != 1 {
		t.Errorf("a path with a separator reached the search path: %q", asked)
	}
	// And a real fixture that is executable as written resolves to itself.
	name := "rg"
	if runtime.GOOS == "windows" {
		name = "rg.exe"
	}
	fixture := writeFile(t, filepath.Join(t.TempDir(), name), "#!/bin/sh\n")
	if got, ok := desktopProgram(fixture); !ok || got != fixture {
		t.Errorf("desktopProgram(%q) = %q ok=%v", fixture, got, ok)
	}
}

// The flatpak desktop entry and the export wrapper are one candidate, not a
// wrapper row next to a `/usr/bin/flatpak` row: the entry's Name and Comment
// land on the wrapper, which is the path a connect would use.
func TestTheFlatpakDesktopEntryAndTheExportWrapperAreOneRow(t *testing.T) {
	flatpakRoot := filepath.Join(t.TempDir(), "flatpak-exports")
	wrapper := writeFile(t, filepath.Join(flatpakRoot, "org.gimp.GIMP"), "#!/bin/sh\n")
	desktopRoot := desktopFixture(t, t.TempDir(), "org.gimp.GIMP.desktop", `[Desktop Entry]
Name=GIMP
Comment=Image editor
Exec=/usr/bin/flatpak run --branch=stable --arch=x86_64 --command=gimp-2.10 --file-forwarding org.gimp.GIMP @@u %U @@
Type=Application
`)
	var resolved []string
	stubDesktopSeams(t, func(program string) (string, bool) {
		resolved = append(resolved, program)
		if program == wrapper {
			return wrapper, true
		}
		return "", false
	}, []string{flatpakRoot}, filepath.Join(t.TempDir(), "snap-bin"))

	byPath := map[string]*Candidate{}
	merge := mergeInto(byPath)
	mergeEntries(merge, []string{desktopRoot}, SourceDesktop, QualityNativeSupport)
	for _, root := range []string{flatpakRoot} {
		discoverExecutableDirectory(merge, root, SourceFlatpak, QualityNativeSupport)
	}
	out := ranked(byPath)
	if len(out) != 1 {
		t.Fatalf("%d candidates, want 1: %+v", len(out), out)
	}
	got := out[0]
	if got.Path != wrapper {
		t.Errorf("path = %q, want the export wrapper %q", got.Path, wrapper)
	}
	if got.Name != "GIMP" || got.Description != "Image editor" {
		t.Errorf("name/description = %q / %q", got.Name, got.Description)
	}
	if !got.Available {
		t.Error("the wrapper is not marked available")
	}
	if !hasSource(got.Sources, SourceDesktop) || !hasSource(got.Sources, SourceFlatpak) {
		t.Errorf("sources = %v, want both the desktop entry and the export", got.Sources)
	}
	for _, program := range resolved {
		if program == "/usr/bin/flatpak" {
			t.Error("the flatpak package manager itself was resolved as the tool")
		}
	}
}

// A desktop-file id is unique across the search path: the user's copy shadows
// the system copy, so one application is one row even when the two entries
// name different commands.
func TestTheUserDesktopEntryShadowsTheSystemCopyOfTheSameID(t *testing.T) {
	userRoot := desktopFixture(t, t.TempDir(), "firefox.desktop", "[Desktop Entry]\nName=Firefox (mine)\nExec=/opt/mine/firefox %u\nType=Application\n")
	systemRoot := desktopFixture(t, t.TempDir(), "firefox.desktop", "[Desktop Entry]\nName=Firefox\nExec=/usr/bin/firefox %u\nType=Application\n")
	stubDesktopSeams(t, func(program string) (string, bool) { return program, true },
		[]string{filepath.Join(t.TempDir(), "flatpak-exports")}, filepath.Join(t.TempDir(), "snap-bin"))

	byPath := map[string]*Candidate{}
	merge := mergeInto(byPath)
	mergeEntries(merge, []string{userRoot, systemRoot}, SourceDesktop, QualityNativeSupport)
	out := ranked(byPath)
	if len(out) != 1 {
		t.Fatalf("%d candidates, want the user's one: %+v", len(out), out)
	}
	if filepath.Base(out[0].Path) != "firefox" || !strings.Contains(filepath.ToSlash(out[0].Path), "/opt/mine/") {
		t.Errorf("path = %q, want the user's override", out[0].Path)
	}
	if out[0].Name != "Firefox (mine)" {
		t.Errorf("name = %q, want the user's entry", out[0].Name)
	}
}
