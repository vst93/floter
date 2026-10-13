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
