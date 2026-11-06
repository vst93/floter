package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func conditionDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HX_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hx-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// The integration stays installed throughout. Only the facts that made its
// question meaningful change; opening and answering still use real UI events.
func checkNoticeCondition(t *testing.T, kind, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, escape := range []bool{false, true} {
			name := kind + "-" + change
			t.Run(appearanceName(dark)+"/"+map[bool]string{false: "return", true: "escape"}[escape], func(t *testing.T) {
				a := noticeApp(t, dark)
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				if kind == "discard" {
					in.Config = []ConfigField{{Key: "token", Label: "Token", Type: "text"}}
					in.ConfigValues = map[string]any{"token": "stored"}
				}
				if kind == "uninstall" {
					in.DistributionSource = "npm"
				}
				original := in
				a.Integrations = func() []Integration { return []Integration{in} }
				dispatched, closed := 0, 0
				a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { dispatched++ }
				a.Actions.UninstallIntegration = func(string, string, string, func(error)) { dispatched++ }
				a.Actions.UninstallComponents = func(string, string, string, UninstallComponents, func(error)) { dispatched++ }
				a.Actions.OpenConfiguration = func(string, []string) { dispatched++ }
				a.Actions.Close = func() { closed++ }
				copy := i18n.For("en").Settings
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsEnable
				if kind == "discard" {
					opener = copy.ConfigOpen
					if err := tt.Click(opener); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
					word := mustFind(t, tt, "Token")
					tt.ClickAt(word.X+240, word.Y+word.H/2)
					tt.Frame()
					if !tt.Focused("Token") {
						t.Fatal("Token input did not take focus")
					}
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("edited")
					tt.Frame()
				}
				if kind == "remove" || kind == "uninstall" {
					opener = in.removalLabel(copy)
				}
				if err := tt.Click(opener); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if kind == "uninstall" {
					tt.Key(0, ui.KeyTab)
					tt.Frame()
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					if !tt.HasText(copy.UninstallHostConfig) {
						t.Fatal("component dialog did not open")
					}
				}
				if !a.anyConfirmOpen() {
					t.Fatal("question did not open")
				}
				switch change {
				case "clean":
					in.ConfigValues = map[string]any{"token": "edited"}
				case "schema":
					in.Config = nil
				case "owner":
					in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
				case "orphan":
					in.Orphan = true
				case "disabled":
					in.Enabled, in.Running = false, false
				case "broken":
					in.Broken = true
				case "kind":
					in.DistributionSource = "local"
				default:
					t.Fatalf("unknown change %q", change)
				}
				tt.Frame()
				tt.Frame()
				if a.anyConfirmOpen() {
					t.Errorf("%s question survived changed %s condition", kind, change)
				}
				if escape {
					tt.Key(0, ui.KeyEscape)
					tt.Frame()
					if closed != 1 {
						t.Errorf("invalid %s question swallowed Escape: close=%d", kind, closed)
					}
				} else {
					if change == "clean" {
						if !tt.Focused(copy.ConfigOpen) {
							t.Error("clean draft lost its configuration opener focus")
						}
						word := mustFind(t, tt, "Token")
						tt.ClickAt(word.X+240, word.Y+word.H/2)
						tt.Frame()
						if !tt.Focused("Token") {
							t.Fatal("Token input did not take focus")
						}
						tt.Key(ui.Cmd, ui.KeyA)
						tt.Type("second edit")
						tt.Frame()
					} else {
						in = original
						tt.Frame()
						tt.Frame()
					}
					conditionDump(t, tt, name, dark)
					if a.anyConfirmOpen() {
						t.Errorf("%s question revived when %s condition returned", kind, change)
					}
					if kind == "discard" {
						want := "edited"
						if change == "clean" {
							want = "second edit"
						}
						if !a.configOpen[in.ID] || a.draft(in).values["token"] != want {
							t.Error("retirement lost the editable draft")
						}
					}
				}
				if dispatched != 0 {
					t.Errorf("condition refresh dispatched %d actions", dispatched)
				}
			})
		}
	}
}

func TestNoticeConditionDiscardClean(t *testing.T)  { checkNoticeCondition(t, "discard", "clean") }
func TestNoticeConditionDiscardSchema(t *testing.T) { checkNoticeCondition(t, "discard", "schema") }
