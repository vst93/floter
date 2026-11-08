package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestContentVisibilityCopyRetired(t *testing.T) {
	for _, action := range []string{"command", "plan", "script"} {
		for _, empty := range []bool{false, true} {
			if action == "plan" && empty {
				// An execution plan remains valid JSON even without a program.
				continue
			}
			for _, dark := range []bool{false, true} {
				name := action + map[bool]string{false: "-withdraw", true: "-empty"}[empty]
				t.Run(name+"/"+appearanceName(dark), func(t *testing.T) {
					a := declarationVisibilityApp(t, dark, 5)
					copy := a.copy()
					label, source := copy.LocalToolCopyCommand, copy.LocalToolCommand
					if action == "plan" {
						label, source = copy.LocalToolCopyPlan, copy.LocalToolProgram
					} else if action == "script" {
						a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('sunny')\n"
						label, source = copy.LocalToolCopyScript, copy.LocalToolScript
					}
					calls := 0
					a.Actions.CopyText = func(string) { calls++ }
					var setup func(*ui.Context)
					var target, viewport ui.Rect
					tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
						if setup != nil {
							f := setup
							setup = nil
							f(c)
						}
						target = c.Resolve(a.contentFocus(c, action).source).Bounds()
						viewport = c.Resolve(a.Body.Handle).Bounds()
					})
					setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, action).button).Focus().ScrollIntoView() }
					tt.Frame()
					tt.Key(0, ui.KeyTab)
					tt.Key(ui.Shift, ui.KeyTab)
					if !tt.Focused(label) {
						t.Fatal("copy setup lost action identity")
					}
					a.Body.Y = a.Body.MaxY
					tt.Frame()
					if empty {
						if action == "command" {
							a.toolCommand = ""
						} else {
							a.toolScript = ""
						}
					} else {
						a.Actions.CopyText = nil
					}
					tt.Frame()
					tt.Frame()
					contentVisibilityDump(t, tt, "copy-"+name, dark)
					declarationVisible(t, a, tt, source, target, viewport)
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("corrected")
					tt.Frame()
					value := a.toolCommand
					if action == "plan" {
						value = a.toolProgram
					} else if action == "script" {
						value = a.toolScript
					}
					if value != "corrected" || calls != 0 || a.copiedContent != "" {
						t.Errorf("copy retirement changed draft/action: value=%q calls=%d confirmation=%q", value, calls, a.copiedContent)
					}
				})
			}
		}
	}
}

func TestContentVisibilityLinkRetired(t *testing.T) {
	for index, link := range []string{deepLinkConnectExample, deepLinkRegisterExample} {
		for _, dark := range []bool{false, true} {
			name := []string{"connect", "register"}[index]
			t.Run(name+"/"+appearanceName(dark), func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				a.Page = PageAbout
				calls := 0
				a.Actions.CopyText = func(string) { calls++ }
				var setup func(*ui.Context)
				var target, viewport ui.Rect
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					a.View(c)
					if setup != nil {
						f := setup
						setup = nil
						f(c)
					}
					target = c.Resolve(a.linkFocus(c, link).source).Bounds()
					viewport = c.Resolve(a.Body.Handle).Bounds()
				}, 720, 300)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				setup = func(c *ui.Context) { c.Resolve(a.linkFocus(c, link).button).Focus().ScrollIntoView() }
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Key(ui.Shift, ui.KeyTab)
				if !tt.Focused(a.copy().DeepLinkCopy) {
					t.Fatal("link setup did not reach copy button")
				}
				a.Body.Y = 0
				tt.Frame()
				a.Actions.CopyText = nil
				tt.Frame()
				tt.Frame()
				contentVisibilityDump(t, tt, "link-"+name, dark)
				declarationVisible(t, a, tt, link, target, viewport)
				if calls != 0 || a.copiedLink != "" {
					t.Error("link retirement dispatched or confirmed copy")
				}
				tt.Key(0, ui.KeyTab)
				if tt.Focused(link) {
					t.Error("link selection trapped keyboard traversal")
				}
			})
		}
	}
}

func TestContentVisibilityPreservesOtherEditor(t *testing.T) {
	for _, action := range []string{"choose", "command", "plan", "script", "export"} {
		for _, dark := range []bool{false, true} {
			t.Run(action+"/"+appearanceName(dark), func(t *testing.T) {
				a := declarationVisibilityApp(t, dark, 5)
				a.Actions.ChooseProgram = func() (string, error) { return "", nil }
				a.Actions.CopyText = func(string) {}
				a.Actions.ExportScript = func(string, string) {}
				if action == "script" || action == "export" {
					a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('sunny')\n"
				}
				var setup func(*ui.Context)
				tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
					if setup != nil {
						f := setup
						setup = nil
						f(c)
					}
				})
				setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, action).button).Focus() }
				tt.Frame()
				setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, "name").source).Focus() }
				tt.Frame()
				a.Body.Y = 500
				tt.Frame()
				a.Actions.ChooseProgram, a.Actions.CopyText, a.Actions.ExportScript = nil, nil, nil
				tt.Frame()
				tt.Frame()
				if a.Body.Y != 500 || !tt.Focused(a.copy().LocalToolName) {
					t.Errorf("%s retirement stole current editor or scroll: %v", action, a.Body.Y)
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("新名字")
				tt.Frame()
				if a.toolName != "新名字" {
					t.Error("surviving editor lost typing")
				}
			})
		}
	}
}
