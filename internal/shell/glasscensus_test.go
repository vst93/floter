package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reference ships its own census of the controls the liquid glass must
// reach, and this is that census mapped onto this build's draw points —
// `main:tests/glass-reaxis.test.ts:47-90`, `every named control draws a lens
// rung, not a bare ring`. The user's third hard requirement was
// 「当前的液态玻璃效果还是不够，特别是各种控件按钮」, and the failure mode the
// census guards is silent: a control that regressed to a flat pane (or lost its
// lens in a rewrite) keeps drawing, so only a list like this makes it loud.
//
// Each entry either points at a landing (a file and the lens call that must
// still be in it, plus the pixel test that grades it) or records, honestly,
// why this build has no distinct landing (`pending`). The file check is the
// same shape as the reference's own CSS check: the named call must be present,
// or the census fails.
type lensLanding struct {
	selector string // the reference's selector
	file     string // this build's draw point, relative to internal/shell
	marker   string // the lens call that must still be in file
	test     string // the pixel test that grades it, when one exists
	pending  string // why there is no landing yet ("" when landed)
}

func TestEveryNamedControlDrawsALensRung(t *testing.T) {
	landings := []lensLanding{
		// Buttons (raised lens).
		{
			selector: ".extensions-action-button",
			file:     "../settingsui/rows.go",
			marker:   "tokens.Lens(button)",
			test:     "settingsui.TestTheSettingsCardControlsCarryTheStopLens/.extensions-action-button",
		},
		{
			selector: ".app-toast__action",
			file:     "notices.go",
			marker:   "tokens.Lens(action)",
			// source-pinned: the toast is a floating overlay and this build has no
			// isolated render for it, so the census pins the call and not a frame.
		},
		{
			selector: ".settings-save-alert button",
			file:     "../settingsui/savealert.go",
			marker:   "tokens.Lens(button)",
			// hover-only here (see the entry's comment); no isolated frame yet.
		},
		{
			selector: ".settings-option--active",
			file:     "../control/control.go",
			marker:   "tokens.Lens(segment)",
			test:     "control.TestTheSegmentedControlCarriesTheStopLens",
		},
		{
			selector: ".settings-sidebar__item:hover",
			file:     "../settingsui/settingsui.go",
			marker:   "tokens.Lens(row)",
			// hover-only; the sidebar row also wears this lens when chosen.
		},
		{
			selector: ".settings-section__card",
			file:     "../settingsui/rows.go",
			marker:   "tokens.Lens(card)",
			test:     "settingsui.TestTheSettingsCardControlsCarryTheStopLens/.settings-section__card",
		},
		{
			selector: ".launcher-result__icon",
			file:     "../launcher/glyphs.go",
			marker:   "tokens.Lens(plate)",
			test:     "launcher.TestTheResultPlateCarriesTheStopLens",
		},
		{
			// The action bar's rows are result rows in this build, so they lead
			// with the same plate: one draw point serves both names.
			selector: ".launcher-action-bar__icon",
			file:     "../launcher/glyphs.go",
			marker:   "tokens.Lens(plate)",
			test:     "launcher.TestTheResultPlateCarriesTheStopLens",
		},
		// Segmented controls (recessed track + raised chosen slot).
		{
			selector: ".settings-options--inline",
			file:     "../control/control.go",
			marker:   "tokens.LensInset(track)",
			test:     "control.TestTheSegmentedControlCarriesTheStopLens",
		},
		{
			selector: ".extension-custom-mode",
			file:     "../settingsui/rows.go",
			marker:   "a.tokens(c).LensInset(ui.Segmented(c, index, labels...))",
			test:     "settingsui.TestTheCustomModeControlCarriesTheStopLens",
		},
		// Switch track (recessed lens).
		{
			selector: ".settings-switch",
			file:     "../settingsui/rows.go",
			marker:   "a.tokens(c).LensInset(ui.Switch(c, on))",
			test:     "settingsui.TestTheSettingsSwitchCarriesTheRecessedStopLens",
		},
		// Two controls are deliberately transparent at rest and light their lens
		// only under the pointer.
		{
			selector: ".extensions-icon-button:hover",
			// This build has no separate extensions-panel icon button: the
			// integration card's actions are all `.extensions-action-button`, so
			// their icon lives inside the same raised-lens button. There is
			// nothing left that is transparent at rest to grade.
			pending: "this build draws the extensions panel's actions as .extensions-action-button (icon inside the lensed button); no resting-transparent icon button exists",
		},
		{
			selector: ".toolbar-button:hover",
			file:     "../terminalui/terminalui.go",
			marker:   "tokensOf(a, c).Lens(button)",
			// hover-only; source-pinned (the terminal's bar needs a full session
			// to render).
		},
	}

	// The reference's own list, in its order: the census cannot silently lose a
	// selector, and a newly named control must be added here with its landing.
	want := []string{
		".extensions-action-button", ".app-toast__action", ".settings-save-alert button",
		".settings-option--active", ".settings-sidebar__item:hover", ".settings-section__card",
		".launcher-result__icon", ".launcher-action-bar__icon",
		".settings-options--inline", ".extension-custom-mode", ".settings-switch",
		".extensions-icon-button:hover", ".toolbar-button:hover",
	}

	if len(landings) != len(want) {
		t.Fatalf("the census lists %d controls, the reference names %d", len(landings), len(want))
	}
	for i, selector := range want {
		if landings[i].selector != selector {
			t.Errorf("census %d = %q, want the reference's %q", i, landings[i].selector, selector)
		}
	}

	root := repoRoot(t)
	for _, landing := range landings {
		landing := landing
		t.Run(landing.selector, func(t *testing.T) {
			if landing.pending != "" {
				if landing.file != "" || landing.marker != "" {
					t.Errorf("a pending control must not claim a landing: %+v", landing)
				}
				return
			}
			if landing.file == "" || landing.marker == "" {
				t.Fatalf("a landed control needs a file and a marker: %+v", landing)
			}
			// The named draw point must still call the lens. This is the
			// source-level half of the census; the pixel tests are the other.
			source, err := os.ReadFile(filepath.Join(root, "internal/shell", landing.file))
			if err != nil {
				t.Fatalf("read %s: %v", landing.file, err)
			}
			if !strings.Contains(string(source), landing.marker) {
				t.Errorf("%s no longer draws its lens rung: %s is gone (graded by %q)",
					landing.file, landing.marker, landing.test)
			}
		})
	}
}

// repoRoot walks up from the test's own directory to the module root (the one
// holding go.mod), so the census can read the source files it names.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
