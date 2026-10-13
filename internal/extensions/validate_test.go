package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateRelativePath pins the old host's rule
// (`manifest.rs:606`): a declared path is non-empty, relative, and made only
// of ordinary components.
func TestValidateRelativePath(t *testing.T) {
	valid := []string{
		"tool.sh",
		"sub/dir/tool.sh",
		"a//b", // `Path::components` collapses the empty component
		"a/b/", // a trailing separator is harmless
		"description.json",
		"中文脚本.sh",
	}
	for _, value := range valid {
		if err := ValidateRelativePath(value, "runtime script"); err != nil {
			t.Errorf("ValidateRelativePath(%q) = %v, want nil", value, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"/abs/path",
		`\abs\path`,
		`C:\windows\system32`,
		"C:/windows",
		"../up",
		"a/../b",
		`a\..\b`,
		"./x",
		".",
		"..",
	}
	for _, value := range invalid {
		err := ValidateRelativePath(value, "runtime script")
		if err == nil {
			t.Errorf("ValidateRelativePath(%q) = nil, want an error", value)
			continue
		}
		if !errors.Is(err, ErrBadManifest) {
			t.Errorf("ValidateRelativePath(%q) = %v, want ErrBadManifest", value, err)
		}
	}
}

// TestManifestValidatePaths checks every path a manifest declares is held to
// the rule, and that an omitted optional path is not an error.
func TestManifestValidatePaths(t *testing.T) {
	cases := []struct {
		name     string
		manifest Manifest
		wantErr  bool
	}{
		{
			name: "a clean script manifest",
			manifest: Manifest{
				Icon:     "icon.png",
				Runtime:  Runtime{Type: "script", Path: "scripts/tool.sh"},
				Provider: Provider{Descriptor: "description.json"},
			},
		},
		{
			name: "no optional paths at all",
			manifest: Manifest{
				Runtime: Runtime{Type: "system", ExecutableNames: []string{"v"}},
			},
		},
		{
			name: "an absolute icon",
			manifest: Manifest{
				Icon:    "/tmp/icon.png",
				Runtime: Runtime{Type: "system"},
			},
			wantErr: true,
		},
		{
			name: "a script that climbs out",
			manifest: Manifest{
				Runtime: Runtime{Type: "script", Path: "../../evil.sh"},
			},
			wantErr: true,
		},
		{
			name: "a descriptor that climbs out",
			manifest: Manifest{
				Runtime:  Runtime{Type: "system"},
				Provider: Provider{Descriptor: "../description.json"},
			},
			wantErr: true,
		},
		{
			name: "a script runtime with no path",
			manifest: Manifest{
				Runtime: Runtime{Type: "script"},
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.manifest.ValidatePaths()
			if tc.wantErr && err == nil {
				t.Fatalf("ValidatePaths() = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidatePaths() = %v, want nil", err)
			}
		})
	}
}

// TestLifecycleValidatePaths covers the paths a lifecycle declares: a
// completion's source, a template's source and target, and a launch command's
// program (with `self` exempt).
func TestLifecycleValidatePaths(t *testing.T) {
	clean := Lifecycle{
		Completions:            []ShellCompletion{{Shell: "bash", Source: "completions/floter.bash"}},
		ConfigurationTemplates: []Template{{Source: "config/default.toml", Target: "config.toml"}},
		Launch:                 Launch{Command: LaunchCommand{Program: "self", Args: []string{"serve"}}},
	}
	if err := clean.ValidatePaths(); err != nil {
		t.Fatalf("clean lifecycle = %v, want nil", err)
	}

	bad := []struct {
		name      string
		lifecycle Lifecycle
	}{
		{"completion source climbs out", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "../completion.bash"}}}},
		{"template source is absolute", Lifecycle{ConfigurationTemplates: []Template{{Source: "/etc/hosts", Target: "config.toml"}}}},
		{"template target climbs out", Lifecycle{ConfigurationTemplates: []Template{{Source: "config/default.toml", Target: "../../config.toml"}}}},
		{"launch program climbs out", Lifecycle{Launch: Launch{Command: LaunchCommand{Program: "../bin/tool"}}}},
		{"launch program is absolute", Lifecycle{Launch: Launch{Command: LaunchCommand{Program: "/usr/bin/tool"}}}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.lifecycle.ValidatePaths(); err == nil {
				t.Fatal("ValidatePaths() = nil, want an error")
			}
		})
	}
}

// TestLoadManifestRefusesAnEscapingPath is the end-to-end half: the rule is
// applied while parsing, so a package that names a file outside itself never
// reaches an export's staging write or a descriptor read.
func TestLoadManifestRefusesAnEscapingPath(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	base := func(extra string) string {
		return `{
  "schemaVersion": "2.0", "id": "dev.floter.evil", "name": "Evil",
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
  "provider": {"type": "executable", "argsPrefix": []}` + extra + `
}`
	}

	// A clean manifest still loads.
	clean := write("clean.json", base(`,"icon": "icon.png"`))
	if _, err := LoadManifest(clean); err != nil {
		t.Fatalf("clean manifest = %v, want nil", err)
	}

	// An escaping script path is refused.
	escaping := write("escaping.json", strings.Replace(base(""),
		`"path": "tool.sh"`, `"path": "../../../../tmp/evil.sh"`, 1))
	if _, err := LoadManifest(escaping); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("escaping script = %v, want ErrBadManifest", err)
	}

	// So is an absolute icon.
	absoluteIcon := write("icon.json", base(`,"icon": "/tmp/icon.png"`))
	if _, err := LoadManifest(absoluteIcon); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("absolute icon = %v, want ErrBadManifest", err)
	}
}

// TestLifecycleValidate pins the old host's lifecycle rules
// (`ToolLifecycle::validate`, `lifecycle.rs:39`): one completion per shell,
// exactly one of source or args, bounded arguments and timeouts, unique
// template targets and probe ids, a safe completion file name, and a launch
// with a known restore policy and NUL-free strings. An omitted timeout or
// restore policy takes the schema's default rather than failing.
func TestLifecycleValidate(t *testing.T) {
	clean := Lifecycle{
		Completions:            []ShellCompletion{{Shell: "bash", Source: "completions/floter.bash", FileName: "floter.bash"}},
		ConfigurationTemplates: []Template{{Source: "config/default.toml", Target: "config.toml"}},
		Probes:                 []Probe{{ID: "version", Args: []string{"--version"}}},
		Launch:                 Launch{Command: LaunchCommand{Program: "self", Args: []string{"serve"}}},
	}
	if err := clean.Validate(); err != nil {
		t.Fatalf("clean lifecycle = %v, want nil", err)
	}

	cases := []struct {
		name      string
		lifecycle Lifecycle
	}{
		{"duplicate shell", Lifecycle{Completions: []ShellCompletion{
			{Shell: "bash", Source: "a.bash"}, {Shell: "bash", Args: []string{"--a"}},
		}}},
		{"unknown shell", Lifecycle{Completions: []ShellCompletion{{Shell: "tcsh", Source: "a.tcsh"}}}},
		{"source and args together", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash", Args: []string{"--a"}}}}},
		{"neither source nor args", Lifecycle{Completions: []ShellCompletion{{Shell: "bash"}}}},
		{"too many completion args", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Args: make([]string, 17)}}}},
		{"completion timeout below the floor", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash", TimeoutMs: 99}}}},
		{"completion timeout above the ceiling", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash", TimeoutMs: 10001}}}},
		{"completion file name with a space", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash", FileName: "bad name.bash"}}}},
		{"completion file name too long", Lifecycle{Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash", FileName: strings.Repeat("a", 129)}}}},
		{"duplicate template target", Lifecycle{ConfigurationTemplates: []Template{
			{Source: "a.toml", Target: "config.toml"}, {Source: "b.toml", Target: "config.toml"},
		}}},
		{"duplicate probe id", Lifecycle{Probes: []Probe{{ID: "up", Args: []string{"--up"}}, {ID: "up", Args: []string{"--up"}}}}},
		{"probe timeout below the floor", Lifecycle{Probes: []Probe{{ID: "up", TimeoutMs: 99}}}},
		{"probe timeout above the ceiling", Lifecycle{Probes: []Probe{{ID: "up", TimeoutMs: 30001}}}},
		{"too many probe args", Lifecycle{Probes: []Probe{{ID: "up", Args: make([]string, 33)}}}},
		{"unknown restore policy", Lifecycle{Launch: Launch{RestorePolicy: "sometimes"}}},
		{"NUL in the launch program", Lifecycle{Launch: Launch{Command: LaunchCommand{Program: "self\x00"}}}},
		{"NUL in the launch args", Lifecycle{Launch: Launch{Command: LaunchCommand{Program: "self", Args: []string{"a\x00b"}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.lifecycle.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !errors.Is(err, ErrBadManifest) {
				t.Fatalf("Validate() = %v, want ErrBadManifest", err)
			}
		})
	}

	// The boundaries themselves are legal.
	bounds := Lifecycle{
		Completions: []ShellCompletion{{Shell: "bash", Args: []string{"--a"}, TimeoutMs: 100}, {Shell: "zsh", Args: []string{"--a"}, TimeoutMs: 10000}},
		Probes:      []Probe{{ID: "a", TimeoutMs: 100}, {ID: "b", TimeoutMs: 30000}},
	}
	if err := bounds.Validate(); err != nil {
		t.Fatalf("boundary lifecycle = %v, want nil", err)
	}

	// The schema defaults fill an omitted timeout and restore policy.
	defaults := Lifecycle{
		Completions: []ShellCompletion{{Shell: "bash", Source: "a.bash"}},
		Probes:      []Probe{{ID: "up"}},
	}
	if err := defaults.Validate(); err != nil {
		t.Fatalf("defaulted lifecycle = %v, want nil", err)
	}
}

// TestEffectiveLifecycleDefaults pins the serde defaults the old host filled
// in: a completion gets 3s, a probe 2s, a launch `reattach`.
func TestEffectiveLifecycleDefaults(t *testing.T) {
	if got := (ShellCompletion{}).EffectiveTimeoutMs(); got != 3000 {
		t.Errorf("completion default = %d, want 3000", got)
	}
	if got := (Probe{}).EffectiveTimeoutMs(); got != 2000 {
		t.Errorf("probe default = %d, want 2000", got)
	}
	if got := (Launch{}).EffectiveRestorePolicy(); got != "reattach" {
		t.Errorf("restore default = %q, want reattach", got)
	}
	if got := (Launch{RestorePolicy: "none"}).EffectiveRestorePolicy(); got != "none" {
		t.Errorf("restore override = %q, want none", got)
	}
	if got := (ShellCompletion{TimeoutMs: 500}).EffectiveTimeoutMs(); got != 500 {
		t.Errorf("completion override = %d, want 500", got)
	}
}

// TestLoadManifestRefusesABadLifecycle is the end-to-end half: the rules are
// applied while parsing, so a package with an unrunnable lifecycle never
// loads. The same file with a clean lifecycle still does.
func TestLoadManifestRefusesABadLifecycle(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	base := func(lifecycle string) string {
		return `{
  "schemaVersion": "2.0", "id": "dev.floter.tool", "name": "Tool",
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
  "provider": {"type": "executable", "argsPrefix": []},
  "lifecycle": ` + lifecycle + `
}`
	}

	clean := write("clean.json", base(`{"probes": [{"id": "up", "args": ["--up"], "required": true}]}`))
	if _, err := LoadManifest(clean); err != nil {
		t.Fatalf("clean lifecycle = %v, want nil", err)
	}

	bad := write("bad.json", base(`{"probes": [{"id": "up"}, {"id": "up"}]}`))
	if _, err := LoadManifest(bad); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("duplicate probe = %v, want ErrBadManifest", err)
	}

	unknownRestore := write("restore.json", base(`{"launch": {"restorePolicy": "sometimes"}}`))
	if _, err := LoadManifest(unknownRestore); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("unknown restore policy = %v, want ErrBadManifest", err)
	}
}
