package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func outletDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HZ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hz-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkNoticeOutlet(t *testing.T, kind, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, elsewhere := range []bool{false, true} {
			name := kind + "-" + change
			suffix := "answer"
			if elsewhere {
				suffix = "elsewhere"
			}
			t.Run(appearanceName(dark)+"/"+suffix, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				if kind == "discard" {
					in.Config = []ConfigField{{Key: "token", Label: "Token", Type: "text"}}
					in.ConfigValues = map[string]any{"token": "stored"}
				}
				if kind == "uninstall" {
					in.DistributionSource = "npm"
				}
				a.Integrations = func() []Integration { return []Integration{in} }
				destructive, saved, opened, checked, adopted, closed := 0, 0, 0, 0, 0, 0
				a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { destructive++ }
				a.Actions.UninstallIntegration = func(string, string, string, func(error)) { destructive++ }
				a.Actions.UninstallComponents = func(string, string, string, UninstallComponents, func(error)) { destructive++ }
				a.Actions.DeleteOrphan = func(string) { destructive++ }
				a.Actions.SaveConfiguration = func(string, map[string]any, func(error)) { saved++ }
				a.Actions.OpenConfiguration = func(id string, command []string) {
					if id != "local" || len(command) != 1 || command[0] != "settings" {
						t.Errorf("configuration outlet ran wrong target: %s %v", id, command)
					}
					opened++
				}
				a.Actions.DiagnoseIntegration = func(string) { checked++ }
				a.Actions.AdoptIntegration = func(string) { adopted++ }
				a.Actions.Close = func() { closed++ }
				copy := a.copy()
				a.Page, a.entered = PageIntegrations, true
				navigationFocused := false
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					c.Root().Background(c.Theme().Background)
					a.View(c)
					navigationFocused = a.Sidebar.Handle.Focused(c)
				}, 720, 900)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				a.Now = tt.Now
				tt.Frame()
				tt.Frame()
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
						t.Fatal("Token input did not focus")
					}
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("edited")
					tt.Frame()
				} else if kind != "disable" {
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
						t.Fatal("uninstall dialog did not open")
					}
				}
				if !a.anyConfirmOpen() {
					t.Fatal("question did not open")
				}
				// A modal traps focus; its control comparison keeps focus on a
				// component checkbox. Inline questions permit leaving for navigation.
				if elsewhere {
					if kind == "uninstall" {
						if err := tt.Click(copy.UninstallToolData); err != nil {
							t.Fatal(err)
						}
					} else {
						for i := 0; i < 30 && !tt.Focused(copy.Close); i++ {
							tt.Key(ui.Shift, ui.KeyTab)
							tt.Frame()
							tt.Frame()
						}
					}
					tt.Frame()
					tt.Frame()
					if kind != "uninstall" && !tt.Focused(copy.Close) {
						t.Fatal("header exit did not receive user focus")
					}
					tt.HoldModifiers(0)
				}
				want := copy.IntegrationsCheck
				switch change {
				case "owner", "unopenable":
					in.ConfigOwner = "tool"
					if change == "owner" {
						in.ConfigOpenCommand = []string{"settings"}
						want = copy.ConfigOpenConfiguration
					}
				case "schema":
					in.Config = nil
				case "orphan":
					in.Orphan = true
					want = copy.IntegrationsAdopt
				case "no-adopt":
					in.Orphan = true
					a.Actions.AdoptIntegration = nil
					want = a.page(PageIntegrations).Title
				case "no-check":
					in.Broken = true
					a.Actions.DiagnoseIntegration = nil
					want = a.page(PageIntegrations).Title
				default:
					t.Fatal("unknown outlet change")
				}
				if elsewhere && kind != "uninstall" {
					want = copy.Close
				}
				tt.Frame()
				tt.Frame()
				if !elsewhere {
					outletDump(t, tt, name, dark)
				}
				focused := tt.Focused(want)
				if want == a.page(PageIntegrations).Title {
					focused = navigationFocused
				}
				if !focused {
					t.Errorf("retired %s lost available keyboard outlet %q", name, want)
				}
				if a.anyConfirmOpen() || destructive != 0 || saved != 0 || opened != 0 || checked != 0 || adopted != 0 {
					t.Error("refresh retained a question or dispatched an action")
				}
				if kind == "discard" && change != "schema" && a.configDrafts[in.ID].values["token"] != "edited" {
					t.Error("focus recovery changed the local draft")
				}
				if kind == "uninstall" {
					if _, exists := a.uninstallDraft[in.ID]; exists {
						t.Error("retired dialog kept abandoned choices")
					}
				}
				if !elsewhere && want != a.page(PageIntegrations).Title {
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					if opened+checked+adopted != 1 || destructive != 0 || saved != 0 {
						t.Errorf("keyboard outlet dispatched wrong actions: open=%d check=%d adopt=%d destructive=%d save=%d", opened, checked, adopted, destructive, saved)
					}
				}
				if elsewhere && kind != "uninstall" {
					// Activate the header the user chose; its tooltip can own
					// the first Escape, independently of confirmation recovery.
					tt.Key(0, ui.KeySpace)
				} else {
					tt.Key(0, ui.KeyEscape)
				}
				tt.Frame()
				if closed != 1 {
					t.Errorf("retired question swallowed Escape: %d", closed)
				}
			})
		}
	}
}

func TestNoticeOutletDiscardOwner(t *testing.T)  { checkNoticeOutlet(t, "discard", "owner") }
func TestNoticeOutletDiscardOrphan(t *testing.T) { checkNoticeOutlet(t, "discard", "orphan") }
