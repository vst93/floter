package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func TestConfigControlsStayInsideTheirGridColumn(t *testing.T) {
	for _, kind := range []string{"text", "select", "multiselect"} {
		t.Run(kind, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			var box ui.Rect
			value := "quiet"
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				ui.Column(c).FillWidth().Padding(20).Children(func() {
					a.extensionMetrics(func() {
						a.configFieldGrid(c, ConfigField{}, "Mode", "", "", func() ui.Element {
							var e ui.Element
							switch kind {
							case "text":
								e = a.extensionField(c, &value, configFieldMetrics).FillWidth()
							case "select":
								e = a.configSelect(c, &value, []string{"quiet", "verbose"}, "Mode", true)
							case "multiselect":
								e = a.configMultiselectBox(c, nil, false, func() {})
							}
							box = e.Bounds()
							return e
						})
					})
				})
			}, 520, 200)
			tt.Frame()
			if box.X != 150 || box.X+box.W != 500 {
				t.Fatalf("%s control=%v; want second column [150,500], not the whole grid width", kind, box)
			}
		})
	}
}

func TestConfigRequiredMarkerSurvivesLongLabels(t *testing.T) {
	for _, label := range []string{"Authentication endpoint address", "服务认证地址与访问配置"} {
		a := New(newStore(t), Actions{})
		value := "https://localhost"
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).FillWidth().Padding(20).Children(func() {
				a.extensionMetrics(func() {
					a.configFieldGrid(c, ConfigField{Required: true}, label, "Required", "", func() ui.Element {
						return a.extensionField(c, &value, configFieldMetrics).FillWidth()
					})
				})
			})
		}, 520, 120)
		tt.Frame()
		name, star := mustFind(t, tt, label), mustFind(t, tt, "*")
		if star.X+star.W > 138 || star.W == 0 || name.X+name.W > star.X-3 {
			t.Errorf("required label spills from [20,138]: name=%v star=%v", name, star)
		}
	}
}

func TestConfigLongOptionKeepsItsCheckboxClickable(t *testing.T) {
	a := New(newStore(t), Actions{})
	option := strings.Repeat("Long option name ", 8)
	on, changed := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		ui.Column(c).Width(220).Padding(10).Children(func() {
			a.extensionMetrics(func() {
				a.configOptionRow(c, option, &on, &changed, false)
			})
		})
	}, 240, 250)
	tt.Frame()
	word := mustFind(t, tt, option)
	// The rightmost 20px of the option row belong to its checkbox.
	tt.ClickAt(200, word.Y+word.H/2)
	tt.Frame()
	if !on || !changed {
		t.Fatalf("visible trailing checkbox was not clickable: on=%v changed=%v word=%v", on, changed, word)
	}
}

func TestConfigCheckboxRemainsVisibleCheckedAndLocked(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := New(newStore(t), Actions{})
		on, changed, locked := false, false, false
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).Width(220).Padding(10).Children(func() {
				a.extensionMetrics(func() {
					box := ui.Column(c).FillWidth()
					if locked {
						box.Disabled(true)
					}
					box.Children(func() { a.configOptionRow(c, "JSON", &on, &changed, locked) })
				})
			})
		}, 240, 100)
		tt.SetDark(dark)
		tt.Frame()
		before := copyFrame(tt.Image())
		tt.ClickAt(200, 20)
		tt.Move(0, 0)
		tt.Frame()
		if !on || !changed {
			t.Fatal("checkbox click did not select the option")
		}
		delta := 0
		for y := 10; y < 30; y++ {
			for x := 190; x < 210; x++ {
				if before.RGBAAt(x, y) != tt.Image().RGBAAt(x, y) {
					delta++
				}
			}
		}
		if delta < 100 {
			t.Errorf("checked mark has no visible area: changed pixels=%d", delta)
		}
		locked, changed = true, false
		tt.Frame()
		tt.ClickAt(200, 20)
		tt.Key(0, ui.KeySpace)
		if !on || changed {
			t.Fatal("locked checkbox accepted a click or Space")
		}
	}
}

func TestConfigDefaultsClearOnlyRestoredFieldErrors(t *testing.T) {
	en := i18n.For("en").Settings
	fields := []ConfigField{
		{Key: "retries", Label: "Retries", Type: "number", Default: float64(3)},
		{Key: "token", Label: "Token", Type: "text"},
	}
	a, calls := configFormApp(t, fields, map[string]any{"retries": float64(4)})
	integration := a.Integrations()[0]
	draft := a.draft(integration)
	draft.setLive(fields[0], "-")
	a.noteConfigFieldError(integration.ID, "retries", en.ConfigNumber)
	a.noteConfigFieldError(integration.ID, "token", en.ConfigOutOfLength)
	a.selectPage(PageIntegrations)
	tt := renderThemed(t, a, 720, 1100)
	if err := tt.Click(en.ConfigDefaults); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.configFieldProblem(integration, fields[0]); got != "" || tt.HasText(en.ConfigNumber) {
		t.Errorf("restored number still reports its old error: %q", got)
	}
	if got := a.configFieldProblem(integration, fields[1]); got != en.ConfigOutOfLength {
		t.Errorf("unrestored field lost its error: %q", got)
	}
	if draft.values["retries"] != float64(3) || len(calls.saved) != 0 {
		t.Errorf("defaults must update the draft only: values=%v saves=%v", draft.values, calls.saved)
	}
}

// Restore defaults leaves a field with no default exactly as the reference left
// it (`ExtensionsPanel.tsx:2013-2016`), including the text the user is still
// typing. Before P13-gr the reset dropped *every* field's live text
// (`draft.clearLive()`), so a no-default number mid-edit ("1e") snapped its
// display back to empty even though its value was untouched — a field that
// reads as if the keystrokes never happened, while the schema-bound field beside
// it correctly showed its restored default. The reset now clears the live text
// only for the fields it actually restores (`clearLiveField`).
func TestConfigDefaultsKeepsANoDefaultFieldHalfTyped(t *testing.T) {
	en := i18n.For("en").Settings
	minimum, maximum := 1.0, 99.0
	fields := []ConfigField{
		{Key: "size", Label: "Size", Type: "number", Minimum: &minimum, Maximum: &maximum, Default: float64(5)},
		{Key: "retries", Label: "Retries", Type: "number", Minimum: &minimum, Maximum: &maximum},
	}
	a, _ := configFormApp(t, fields, map[string]any{"size": float64(2)})
	integration := a.Integrations()[0]
	draft := a.draft(integration)
	a.selectPage(PageIntegrations)
	tt := renderThemed(t, a, 900, 1400)

	// Type a half-parseable value into the no-default field, no Enter.
	label := mustFind(t, tt, "Retries")
	tt.ClickAt(label.X+240, label.Y+label.H/2)
	tt.Frame()
	tt.Type("1e")
	tt.Frame()
	if got := draft.liveText(fields[1], ""); got != "1e" {
		t.Fatalf("the half-typed text did not survive the redraw: live = %q, want %q", got, "1e")
	}

	if err := tt.Click(en.ConfigDefaults); err != nil {
		t.Fatalf("press %q: %v", en.ConfigDefaults, err)
	}
	tt.Frame()

	// The no-default field the reset did not touch keeps its half-typed display;
	// its value stays what the unparsable edit left (nil), untouched by the reset.
	if got := draft.liveText(fields[1], ""); got != "1e" {
		t.Errorf("Restore defaults wiped a no-default field's half-typed text: live = %q, want %q", got, "1e")
	}
	if draft.values["retries"] != nil {
		t.Errorf("Restore defaults moved a no-default field's value: %v, want nil", draft.values["retries"])
	}
	// The field it *did* restore carries its default, and has no stale live text
	// shadowing it: a defaulted field that was mid-edit must show the default, so
	// its live entry is dropped and the display falls back to the value.
	if draft.values["size"] != float64(5) {
		t.Errorf("Restore defaults did not restore the defaulted field: size = %v, want 5", draft.values["size"])
	}
	if _, stale := draft.live["size"]; stale {
		t.Errorf("Restore defaults left stale live text on the field it restored: %v", draft.live)
	}
	// A defaulted field that *was* mid-edit also drops its text. Set one and
	// confirm the reset clears it, so the default is not hidden behind an edit.
	draft.setLive(fields[0], "7")
	if err := tt.Click(en.ConfigDefaults); err != nil {
		t.Fatalf("press %q again: %v", en.ConfigDefaults, err)
	}
	tt.Frame()
	if _, stale := draft.live["size"]; stale {
		t.Errorf("Restore defaults kept a restored field's half-typed text: %v", draft.live)
	}
}

func boundaryConfigPage(t *testing.T, dark bool, language string) *ui.Tester {
	t.Helper()
	labels := []string{"Service authentication endpoint", "Access token", "Working directory", "Result limit", "Mode", "Output formats"}
	if language == "zh" {
		labels = []string{"服务认证地址与访问配置", "访问令牌", "工作目录", "结果上限", "运行模式", "输出格式"}
	}
	fields := []ConfigField{
		{Key: "endpoint", Label: labels[0], Type: "text", Required: true},
		{Key: "token", Label: labels[1], Type: "password"},
		{Key: "path", Label: labels[2], Type: "path"},
		{Key: "limit", Label: labels[3], Type: "number", Default: float64(100)},
		{Key: "mode", Label: labels[4], Type: "select", Options: []string{"quiet", "verbose"}},
		{Key: "formats", Label: labels[5], Type: "multiselect", Options: []string{"JSON documents with complete source metadata", "Plain text", "Markdown"}},
	}
	a, _ := configFormApp(t, fields, map[string]any{
		"endpoint": "https://localhost:8080/search", "token": "local-token", "path": "/workspace/search", "limit": float64(100), "mode": "quiet",
	})
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme, s.Language = appearanceName(dark), language }); err != nil {
		t.Fatal(err)
	}
	a.selectPage(PageIntegrations)
	tt := renderThemed(t, a, 720, 1000)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func TestDumpP13GqConfiguration(t *testing.T) {
	tag := os.Getenv("FLOTER_P13GQ_DUMP")
	if tag == "" {
		t.Skip("opt-in screenshot dump: FLOTER_P13GQ_DUMP=before or after")
	}
	for _, dark := range []bool{false, true} {
		tt := boundaryConfigPage(t, dark, "zh")
		name := fmt.Sprintf("/tmp/floter-P13-gq-settings-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log(name)
	}
}
