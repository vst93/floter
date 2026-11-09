package settingsui

import (
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestLocalActionFeedbackPackage(t *testing.T)  { checkLocalActionFeedback(t, "package") }
func TestLocalActionFeedbackVersion(t *testing.T)  { checkLocalActionFeedback(t, "version") }
func TestLocalActionFeedbackOutput(t *testing.T)   { checkLocalActionFeedback(t, "output") }
func TestLocalActionFeedbackPlatform(t *testing.T) { checkLocalActionFeedback(t, "platform") }

func TestLocalActionFeedbackSubmit(t *testing.T)     { checkLocalActionFeedback(t, "submit") }
func TestLocalActionFeedbackPermission(t *testing.T) { checkLocalActionFeedback(t, "permission") }

func TestLocalActionFeedbackInstall(t *testing.T) { checkLocalActionFeedback(t, "install") }
func TestLocalActionFeedbackFolder(t *testing.T)  { checkLocalActionFeedback(t, "folder") }

func checkLocalActionFeedback(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 1)
			a.toolAdvanced = kind == "permission"
			copy := a.copy()
			permission := extensions.AllPermissions[0]
			label := map[string]string{"submit": copy.LocalToolSave, "output": copy.LocalToolTerminal,
				"platform": customPlatforms[0].Label, "permission": i18n.PermissionLabel(a.settings().Language, permission),
				"package": copy.IntegrationsPackageHint, "version": copy.IntegrationsVersionHint,
				"install": copy.IntegrationsInstall, "folder": copy.InstallFolderChoose}[kind]
			calls := 0
			a.Actions.EditCustomTool = func(id string, tool CustomTool) {
				calls++
				if id != "local.weather" || tool.Command != "weather" {
					t.Error("submit changed tool ownership")
				}
			}
			a.Actions.InstallFromRegistry = func(name, version string) {
				calls++
				if name != "weather" || version != "^1.0.0" {
					t.Errorf("install draft changed: %q / %q", name, version)
				}
			}
			a.Actions.ChooseFolder = func() (string, error) { calls++; return "", nil }
			a.Actions.InstallFromFolder = func(string) { t.Error("cancelled picker installed") }
			if kind == "install" {
				a.installName, a.installVersion = "weather", "^1.0.0"
			}
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			for n := 0; n < 120 && !tt.Focused(label); n++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot reach %s", label)
			}
			// Put the current control just inside the lower safe edge, with
			// real content below it. New preceding feedback must reveal it.
			r, viewport := mustFind(t, tt, label), mustFind(t, tt, "feedback-viewport")
			a.Body.Y += r.Y + r.H + 28 - viewport.Y - viewport.H
			tt.Frame()
			for step, on := range []bool{true, false, true} {
				a.toolIssue = on
				a.toolName = ""
				if kind == "submit" {
					a.toolParams[0].Kind = extensions.ParamSelect
					a.toolParams[0].Options = "北京\n上海"
					if on {
						a.toolParams[0].Options = strings.Repeat("广州\n", 18) + "上海"
					}
				}
				tt.Frame()
				if step == 0 && kind == "package" {
					if tag := os.Getenv("FLOTER_P13LH_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-lh-package-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						err = png.Encode(f, tt.Image())
						f.Close()
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				r, viewport := mustFind(t, tt, label), mustFind(t, tt, "feedback-viewport")
				// Compare raster rows, not float32 accumulation tails at an
				// otherwise exact edge. No extra tolerance or omitted ring.
				if !tt.Focused(label) || r.W <= 0 || r.H <= 0 ||
					math.Round(float64(r.Y-3)) < math.Round(float64(viewport.Y+16)) ||
					math.Round(float64(r.Y+r.H+3)) > math.Round(float64(viewport.Y+viewport.H-16)) {
					t.Errorf("feedback hides %s: rect=%+v viewport=%+v focused=%v", label, r, viewport, tt.Focused(label))
				}
			}
			manual := a.Body.Y
			if a.Body.MaxY > 0 {
				// The options TextArea consumes wheels over its own surface.
				// Scroll in the page gutter to exercise the outer viewport.
				dy := float32(140)
				if a.Body.Y == a.Body.MaxY {
					dy = -dy
				}
				tt.Wheel(710, 280, 0, dy)
				tt.Advance(250 * time.Millisecond)
				tt.Frame()
				if a.Body.Y == manual {
					t.Fatal("page gutter wheel did not move")
				}
				manual = a.Body.Y
			}
			a.toolDescription = "same height"
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Error("ordinary edit reclaimed manual scroll")
			}
			switch kind {
			case "package", "version":
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("next")
				tt.Frame()
				value := a.installName
				if kind == "version" {
					value = a.installVersion
				}
				if value != "next" || calls != 0 {
					t.Errorf("editor ownership: value=%q calls=%d", value, calls)
				}
			case "output":
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.toolOutput != routeBackground || calls != 0 {
					t.Error("output selection lost")
				}
			case "platform":
				before := a.toolPlatforms[customPlatforms[0].ID]
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.toolPlatforms[customPlatforms[0].ID] == before || calls != 0 {
					t.Error("platform toggle lost")
				}
			case "permission":
				before := a.toolPermissions[permission]
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.toolPermissions[permission] == before || calls != 0 {
					t.Error("permission toggle lost")
				}
			default:
				a.toolName = "Weather"
				tt.Frame()
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Errorf("action dispatch=%d", calls)
				}
			}
		})
	}
}
