package settingsui

import (
	"testing"

	"floter/internal/i18n"
)

// The detected row's one button has the reference's three arms
// (`src/extensions/ExtensionRow.tsx:296-302`): a runnable program connects,
// one that is not runnable but has a recipe opens the install command in a
// terminal, and one with nothing to offer draws its word disabled. This is the
// pure mapping; the pixel and behaviour acceptance follows below.
func TestTheDetectedActionFollowsTheRow(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings

	cases := []struct {
		name     string
		tool     DetectedTool
		kind     detectedActionKind
		disabled bool
		en       string
		zh       string
	}{
		{
			name: "a runnable program is connected",
			tool: DetectedTool{RuntimeAvailable: true},
			kind: detectedConnect, disabled: false,
			en: en.IntegrationsConnect, zh: zh.IntegrationsConnect,
		},
		{
			// A recipe the machine can install with turns the arm into a
			// terminal hand-off, even though the program is not runnable.
			name: "a missing program with a recipe installs in a terminal",
			tool: DetectedTool{InstallCommand: "sudo apt install jq"},
			kind: detectedInstallInTerminal, disabled: false,
			en: en.IntegrationsInstallInTerminal, zh: zh.IntegrationsInstallInTerminal,
		},
		{
			name: "a missing program with nothing to offer says so disabled",
			tool: DetectedTool{},
			kind: detectedInstallFirst, disabled: true,
			en: en.IntegrationsInstallTool, zh: zh.IntegrationsInstallTool,
		},
		{
			// RuntimeAvailable wins: a program that runs is connected, never
			// reinstalled over itself.
			name: "runnable wins over a recipe",
			tool: DetectedTool{RuntimeAvailable: true, InstallCommand: "sudo apt install jq"},
			kind: detectedConnect, disabled: false,
			en: en.IntegrationsConnect, zh: zh.IntegrationsConnect,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			kind, disabled := detectedActionFor(tc.tool)
			if kind != tc.kind || disabled != tc.disabled {
				t.Errorf("detectedActionFor() = (%v, %v), want (%v, %v)", kind, disabled, tc.kind, tc.disabled)
			}
			if got := detectedLabel(en, kind); got != tc.en {
				t.Errorf("detectedLabel(en) = %q, want %q", got, tc.en)
			}
			if got := detectedLabel(zh, kind); got != tc.zh {
				t.Errorf("detectedLabel(zh) = %q, want %q", got, tc.zh)
			}
		})
	}
}

// The same arms reach the row, and each one runs only the action it chose: a
// pure mapping can be right while the row still hard-codes Connect. The pixel
// half pins the three words on screen; the click half pins that pressing the
// install word hands over the command, and that the disabled word does nothing.
func TestTheDetectedRowsCarryTheirThreeActions(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	var connected, installed string
	a.Actions = Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{
				{Name: "Runnable", Path: "/usr/bin/runnable", RuntimeAvailable: true},
				{Name: "Installable", Path: "/usr/bin/installable", InstallCommand: "sudo apt install installable"},
				{Name: "Hopeless", Path: "/usr/bin/hopeless"},
			}
		},
		ConnectDetected: func(tool DetectedTool) { connected = tool.Name },
		InstallDetected: func(command string) { installed = command },
	}
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	for _, want := range []string{
		en.IntegrationsConnect,
		en.IntegrationsInstallInTerminal,
		en.IntegrationsInstallTool,
	} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q in %v", want, tt.Texts())
		}
	}

	if err := tt.Click(en.IntegrationsInstallInTerminal); err != nil {
		t.Fatalf("click install: %v", err)
	}
	tt.Frame()
	if installed != "sudo apt install installable" {
		t.Errorf("the install arm handed over %q, want the row's command", installed)
	}
	if connected != "" {
		t.Errorf("the install arm also connected %q", connected)
	}

	if err := tt.Click(en.IntegrationsConnect); err != nil {
		t.Fatalf("click connect: %v", err)
	}
	tt.Frame()
	if connected != "Runnable" {
		t.Errorf("the connect arm connected %q, want Runnable", connected)
	}

	// The word with nothing behind it is drawn disabled, so pressing it is not
	// an action. It must leave both actions untouched.
	if err := tt.Click(en.IntegrationsInstallTool); err != nil {
		t.Fatalf("click disabled: %v", err)
	}
	tt.Frame()
	if connected != "Runnable" || installed != "sudo apt install installable" {
		t.Errorf("the disabled row acted: connected=%q installed=%q", connected, installed)
	}
}
