package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/terminalui"
)

// A script tool is connected like any other: the host writes the package, the
// script becomes the provider file, and the interpreter this machine has is
// what runs it. The draft's own fields travel through one mapping, so the
// connect path and the edit path cannot disagree.
func TestConnectAScriptTool(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	restoreInterpreter := extensions.StubInterpreterLookup(func(language string) (string, bool) {
		if language == "python" {
			return "/usr/bin/python3", true
		}
		return "", false
	})
	defer restoreInterpreter()

	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: paths,
	})
	tool := settingsui.CustomTool{
		Name:           "Weather",
		Mode:           "script",
		ScriptLanguage: "python",
		ScriptContent:  "#!/usr/bin/env python3\nprint('sunny')\n",
		Command:        "weather",
		Description:    "Says the weather",
		Permissions:    []string{extensions.PermissionNetworkFetch},
	}
	// A script carries no program and no argument prefix, whatever the form's
	// other fields held.
	tool.Program = "/usr/bin/ls"
	tool.Args = "--floter"
	request := customRequest(tool)
	if request.Mode != "script" {
		t.Fatalf("mode = %q", request.Mode)
	}
	if request.ExecutablePath != "" || len(request.ArgsPrefix) != 0 {
		t.Errorf("a script carried a program: %+v", request)
	}
	if request.ScriptLanguage != "python" || request.ScriptContent != tool.ScriptContent {
		t.Errorf("the script was lost: %+v", request)
	}

	// Prepare and commit the same way the shell does.
	prepared, err := extensions.PrepareCustom(context.Background(), paths, request)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	entry, err := prepared.Commit(true)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if entry.Name != "Weather" {
		t.Errorf("name = %q", entry.Name)
	}
	// The package holds the script as its provider file, and the manifest says
	// how to run it.
	integration, ok := extensions.LoadInventory(paths).WithID(entry.ID)
	if !ok {
		t.Fatalf("the integration is not in the inventory: %s", entry.ID)
	}
	if integration.Manifest.Runtime.Type != "script" || integration.Manifest.Runtime.Language != "python" {
		t.Errorf("runtime = %+v", integration.Manifest.Runtime)
	}
	script := integration.Manifest.Runtime.Path
	if !filepath.IsAbs(script) {
		script = filepath.Join(integration.PackageDir(), script)
	}
	data, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("the script is not in the package: %v", err)
	}
	if string(data) != tool.ScriptContent {
		t.Errorf("the script reads %q", data)
	}
	if !strings.HasSuffix(script, ".py") {
		t.Errorf("the script is named %q", filepath.Base(script))
	}

	// And the form can read it back: the edit path shows the script it wrote.
	a.RefreshIntegrations(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	var edited settingsui.CustomTool
	for {
		var found bool
		edited, found = a.toolForEdit(entry.ID)
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the form could not read the script back")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if edited.Mode != "script" || edited.ScriptLanguage != "python" {
		t.Errorf("the form read %+v", edited)
	}
	if edited.ScriptContent != tool.ScriptContent {
		t.Errorf("the form read the script as %q", edited.ScriptContent)
	}
}

// The sessions list reports every session the terminal holds, with what each
// one is doing: the one on screen, one running behind it, one that has ended.
// The page's own controls hand an index back, so a click names which session.
func TestSessionsListReportsEverySession(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	// The terminals are markers: the list reads their state, and a terminal
	// the plugin never started has no program to run.
	a.Terminal.Sessions = []*terminalui.Session{
		{Title: "zsh", Term: &terminal.Terminal{}, Created: time.Now()},
		{Title: "vim", Term: &terminal.Terminal{}, Created: time.Now()},
		{Title: "build", Term: &terminal.Terminal{}, Resident: true, ExitCode: 1, Created: time.Now()},
	}
	a.Terminal.Active = 1

	sessions := a.Settings.Sessions()
	if len(sessions) != 3 {
		t.Fatalf("sessions = %+v", sessions)
	}
	if !sessions[1].Active || sessions[1].Title != "vim" || !sessions[1].Running {
		t.Errorf("the session on screen = %+v", sessions[1])
	}
	if sessions[0].Active || !sessions[0].Running {
		t.Errorf("the session behind it = %+v", sessions[0])
	}
	if sessions[2].Running {
		t.Errorf("the ended session reads as running: %+v", sessions[2])
	}
	for i, session := range sessions {
		if session.Index != i {
			t.Errorf("session %d has index %d", i, session.Index)
		}
	}
	// An unnamed session is labelled by the dictionary, not left blank.
	a.Terminal.Sessions[0].Title = ""
	if got := a.Settings.Sessions()[0].Title; got == "" {
		t.Error("an unnamed session has no label")
	}
}
