package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// runParamKindLabel names every declared kind with the same five words the
// parameter editor's picker offers — the reference's `customParamType.*`
// labels. A kind the manifest invents (or an empty one) reads as text, the
// default arm `RunParamForm.tsx` had.
func TestRunParamKindLabelNamesEveryKind(t *testing.T) {
	en := i18n.For("en").Settings
	cases := []struct {
		kind string
		want string
	}{
		{extensions.ParamText, en.ParamsKindText},
		{extensions.ParamNumber, en.ParamsKindNumber},
		{extensions.ParamBoolean, en.ParamsKindBoolean},
		{extensions.ParamSelect, en.ParamsKindSelect},
		{extensions.ParamPath, en.ParamsKindPath},
		{"", en.ParamsKindText},
		{"made-up", en.ParamsKindText},
	}
	for _, tc := range cases {
		if got := runParamKindLabel(en, tc.kind); got != tc.want {
			t.Errorf("runParamKindLabel(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
	zh := i18n.For("zh").Settings
	if got := runParamKindLabel(zh, extensions.ParamSelect); got != zh.ParamsKindSelect {
		t.Errorf("zh choice label = %q, want %q", got, zh.ParamsKindSelect)
	}
}

// The run form's own frame, which the reference drew and this build used to
// drop: the heading and hint above the inputs (`customRunValues` /
// `customRunValuesHint`), each row's *kind* label with the required marker
// beside it (`customParamType.*` + `customParamRequired`, never the
// placeholder), and at the foot the injection note plus the route the manifest
// declares (`customParamInjectionNote` + one of `customRunOutputHint*`).
func TestRunFormDrawsTheReferenceFrame(t *testing.T) {
	en := i18n.For("en").Settings
	for _, tc := range []struct {
		name   string
		output string
		hint   string
	}{
		{"background", extensions.RouteBackground, en.RunOutputHintBackground},
		{"terminal", extensions.RouteTerminal, en.RunOutputHintTerminal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := New(settings.NewStore(settings.Default()), Actions{
				RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
					return nil
				},
			})
			a.Integrations = func() []Integration {
				return []Integration{{
					ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
					OutputMode: tc.output,
					Params: []Param{
						{ID: "target", Label: "Target", Kind: extensions.ParamText, Required: true, Placeholder: "where"},
						{ID: "verbose", Label: "Verbose", Kind: extensions.ParamBoolean},
					},
				}}
			}
			tt := render(t, a, 720, 900)
			if err := tt.Click("Integrations"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click("Run"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()

			for _, want := range []string{en.RunValues, en.RunValuesHint, en.RunInjectionNote, tc.hint} {
				if !tt.HasText(want) {
					t.Errorf("the form is missing %q: %q", want, tt.Texts())
				}
			}
			// The row's second line is the kind, with the required marker; the
			// placeholder stays on the input where the reference put it.
			if !tt.HasText(en.ParamsKindText + "  ·  " + en.RunRequired) {
				t.Errorf("the required text row's kind marker is missing: %q", tt.Texts())
			}
			if !tt.HasText(en.ParamsKindBoolean) {
				t.Errorf("the toggle row's kind label is missing: %q", tt.Texts())
			}
			// The route the other manifest declares is never shown: the hint
			// follows the integration's own output mode.
			other := en.RunOutputHintTerminal
			if tc.output == extensions.RouteTerminal {
				other = en.RunOutputHintBackground
			}
			if tt.HasText(other) {
				t.Errorf("the form stated the wrong route (%q): %q", other, tt.Texts())
			}
		})
	}
}

// The row's Run button (the reference's `customRun`): a runnable integration
// with no inputs runs at once from the settings page — which this build could
// not do at all before — while one that declares inputs does not wear its form
// until the same button unfolds it, and Cancel (the reference's
// `customRunCancel`) folds it back.
func TestRunButtonRunsOrOpensTheForm(t *testing.T) {
	en := i18n.For("en").Settings

	t.Run("no inputs run at once", func(t *testing.T) {
		type run struct {
			id     string
			values map[string]string
		}
		runs := []run{}
		a := New(settings.NewStore(settings.Default()), Actions{
			RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
				runs = append(runs, run{id, values})
				return nil
			},
		})
		a.Integrations = func() []Integration {
			return []Integration{{ID: "plain", Name: "Plain", Enabled: true, Running: true}}
		}
		tt := render(t, a, 720, 900)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click(en.RunAction); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if len(runs) != 1 || runs[0].id != "plain" || runs[0].values != nil {
			t.Fatalf("runs = %v", runs)
		}
	})

	t.Run("inputs open the form and Cancel folds it", func(t *testing.T) {
		a := New(settings.NewStore(settings.Default()), Actions{
			RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
				return nil
			},
		})
		a.Integrations = func() []Integration {
			return []Integration{{
				ID: "param", Name: "Param", Enabled: true, Running: true,
				Params: []Param{{ID: "target", Label: "Target", Kind: extensions.ParamText}},
			}}
		}
		tt := render(t, a, 720, 900)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		// The form is not resident: neither its heading nor its Run button is
		// drawn before the row's Run button is pressed.
		if tt.HasText(en.RunValues) || tt.HasText(en.RunLabel) {
			t.Fatalf("the form is resident before the press: %q", tt.Texts())
		}
		if err := tt.Click(en.RunAction); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !tt.HasText(en.RunValues) || !tt.HasText(en.RunLabel) {
			t.Fatalf("the Run button did not unfold the form: %q", tt.Texts())
		}
		if err := tt.Click(en.RunCancel); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if tt.HasText(en.RunValues) || tt.HasText(en.RunLabel) {
			t.Fatalf("Cancel did not fold the form: %q", tt.Texts())
		}
	})
}
