package shell

import (
	"testing"

	"floter/internal/inventory"
	"floter/internal/tools"
)

// A discovered candidate carries no catalog id, so the detected row finds its
// install command by the executable's own name against the catalog's probes —
// the same names the catalog uses to detect the tool. The recipe is the one a
// manager this machine actually has renders, and nothing is invented for a
// tool the catalog does not know (`src/extensions/tool-rows.ts`'s
// `installCommandForId`).
func TestTheDetectedRowKnowsItsInstallCommand(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		platform string
		managers []string
		want     string
	}{
		{
			name: "a known tool and a manager this machine has",
			path: "/usr/bin/flameshot", platform: tools.PlatformLinux,
			managers: []string{"apt"}, want: "sudo apt install flameshot",
		},
		{
			// No manager present still renders the platform's first recipe, the
			// same fallback the launcher's install rows use.
			name: "a known tool and no manager",
			path: "/usr/bin/flameshot", platform: tools.PlatformLinux,
			want: "sudo pacman -S flameshot",
		},
		{
			// The probe is the executable's name without its launcher suffix.
			name: "a Windows-style executable name",
			path: "/c/bin/flameshot.exe", platform: tools.PlatformWindows,
			managers: []string{"winget"}, want: "winget install --id Flameshot.Flameshot -e",
		},
		{
			name: "a tool the catalog does not know",
			path: "/usr/bin/mystery", platform: tools.PlatformLinux,
			managers: []string{"apt"}, want: "",
		},
		{
			name: "a platform the tool has no recipe for",
			path: "/usr/bin/flameshot", platform: "plan9",
			managers: []string{"apt"}, want: "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := detectedInstallCommand(
				inventory.Candidate{Name: "whatever", Path: tc.path},
				tc.platform, tc.managers,
			)
			if got != tc.want {
				t.Errorf("detectedInstallCommand(%q, %s) = %q, want %q", tc.path, tc.platform, got, tc.want)
			}
		})
	}
}
