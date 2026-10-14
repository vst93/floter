package settingsui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"floter/internal/i18n"
)

// The reference's custom-tool drawer carries a content-action row under the
// fields (`CustomIntegrationDrawer.tsx:145-146`): in script mode one "Copy
// script", in program mode "Copy command" and "Copy execution plan". This
// build's form had none, so a script the user was editing could only be copied
// by selecting it by hand, and the command word had no one-press copy at all.
func TestLocalToolFormCopiesItsContent(t *testing.T) {
	copy := i18n.For("en").Settings
	var copied []string
	a := New(newStore(t), Actions{
		CopyText:         func(text string) { copied = append(copied, text) },
		ConnectLocalTool: func(CustomTool) {},
		ChooseProgram:    func() (string, error) { return "/usr/bin/rg", nil },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Nothing copied before a press, and the two program-mode buttons are drawn
	// once the fields carry values.
	a.toolProgram, a.toolCommand, a.toolArgs = "/usr/bin/rg", "rg", "--floter -n"
	tt.Frame()
	if !tt.HasText(copy.LocalToolCopyCommand) || !tt.HasText(copy.LocalToolCopyPlan) {
		t.Fatalf("the program form is missing its content actions: %q", tt.Texts())
	}
	if tt.HasText(copy.LocalToolCopyScript) {
		t.Errorf("the script button is drawn in program mode: %q", tt.Texts())
	}

	// "Copy command" puts the command word on the clipboard and confirms on
	// its own button.
	if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(copied) != 1 || copied[0] != "rg" {
		t.Fatalf("copied = %v", copied)
	}
	if !tt.HasText(copy.LocalToolCommandCopied) {
		t.Errorf("the command button did not confirm: %q", tt.Texts())
	}
	for _, text := range tt.Texts() {
		if text == copy.LocalToolCopyCommand {
			t.Errorf("the command button stayed on its resting label: %q", tt.Texts())
		}
	}
	// The plan is the reference's own JSON: the program, the default arguments
	// split the way the host splits them, and the run mode.
	if err := tt.Click(copy.LocalToolCopyPlan); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(copied) != 2 {
		t.Fatalf("copied = %v", copied)
	}
	var plan struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Mode    string   `json:"mode"`
	}
	if err := json.Unmarshal([]byte(copied[1]), &plan); err != nil {
		t.Fatalf("the plan is not JSON: %q (%v)", copied[1], err)
	}
	if plan.Program != "/usr/bin/rg" || plan.Mode != "pty" || strings.Join(plan.Args, "|") != "--floter|-n" {
		t.Errorf("the plan = %+v", plan)
	}
	if !tt.HasText(copy.LocalToolPlanCopied) {
		t.Errorf("the plan button did not confirm: %q", tt.Texts())
	}
	// Only the last press confirms: the command button went back to rest.
	for _, text := range tt.Texts() {
		if text == copy.LocalToolCommandCopied {
			t.Errorf("a second press left the first confirmation up: %q", tt.Texts())
		}
	}
	// The window is the app's own state, testable without the frame clock.
	if !a.contentCopied("plan", time.Now()) {
		t.Error("the plan press is not marked copied")
	}
	if a.contentCopied("plan", time.Now().Add(2*time.Second)) {
		t.Error("the confirmation outlived its window")
	}
}

// In script mode the one button is "Copy script", and it copies the text the
// editor holds — the same draft the connect submits. The script button and the
// program buttons never appear together, because they are the two kinds of the
// same form.
func TestLocalToolFormCopiesTheScript(t *testing.T) {
	copy := i18n.For("en").Settings
	var copied []string
	a := New(newStore(t), Actions{
		CopyText:         func(text string) { copied = append(copied, text) },
		ConnectLocalTool: func(CustomTool) {},
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.LocalToolCopyScript) {
		t.Fatalf("the script form is missing its copy button: %q", tt.Texts())
	}
	if tt.HasText(copy.LocalToolCopyCommand) || tt.HasText(copy.LocalToolCopyPlan) {
		t.Errorf("a program-mode button is drawn in script mode: %q", tt.Texts())
	}
	// A fresh script form is seeded with the language's template, so there is
	// something to copy.
	if err := tt.Click(copy.LocalToolCopyScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(copied) != 1 || strings.TrimSpace(copied[0]) == "" {
		t.Fatalf("copied = %v", copied)
	}
	if !strings.Contains(copied[0], "print") && !strings.Contains(copied[0], "print(") {
		t.Logf("copied script = %q", copied[0])
	}
	if !tt.HasText(copy.LocalToolScriptCopied) {
		t.Errorf("the script button did not confirm: %q", tt.Texts())
	}
}

// The plan helper is a pure projection of the two fields the host turns into
// argv: an absent arguments field reads as an empty list (never null), and the
// mode is the pty run the reference named.
func TestExecutionPlanJSON(t *testing.T) {
	var plan struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Mode    string   `json:"mode"`
	}
	if err := json.Unmarshal([]byte(executionPlanJSON("/bin/tool", "  --a   --b  ")), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Program != "/bin/tool" || plan.Mode != "pty" || strings.Join(plan.Args, "|") != "--a|--b" {
		t.Errorf("plan = %+v", plan)
	}
	if err := json.Unmarshal([]byte(executionPlanJSON("/bin/tool", "")), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Args == nil || len(plan.Args) != 0 {
		t.Errorf("an empty arguments field is %+v, want an empty list", plan.Args)
	}
	if strings.Contains(executionPlanJSON("/bin/tool", ""), "null") {
		t.Errorf("the plan carries null: %q", executionPlanJSON("/bin/tool", ""))
	}
}
