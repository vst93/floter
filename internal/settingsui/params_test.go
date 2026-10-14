package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// textAt is the index of want in texts at or after from, or -1.
func textAt(texts []string, from int, want string) int {
	for i := from; i < len(texts); i++ {
		if texts[i] == want {
			return i
		}
	}
	return -1
}

// hasExact reports whether texts holds want as a whole label.
func hasExact(texts []string, want string) bool {
	return textAt(texts, 0, want) >= 0
}

// The declared-inputs editor keeps the reference's field order and the two
// fields this build had dropped (`CustomIntegrationDrawer.tsx:83-104`): each
// row's fields run label, id, kind, placeholder, default, flag, and the
// required toggle plus the remove button close the row. A boolean hides the
// placeholder field — its value is its flag, so a field hint means nothing —
// while every other kind shows it. `beginEdit` and the request both carry the
// two fields, so editing an integration no longer silently loses them.
func TestConnectFormKeepsTheReferenceParamFields(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 3200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.ParamsAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The reference's order, found as a subsequence after the row's own head.
	order := []string{
		copy.ParamsLabel, copy.ParamsID, copy.ParamsKind,
		copy.ParamsPlaceholder, copy.ParamsDefault, copy.ParamsFlag,
	}
	texts := tt.Texts()
	at := textAt(texts, 0, copy.ParamsRow(1))
	if at < 0 {
		t.Fatalf("the param row is missing: %q", texts)
	}
	for _, want := range order {
		at = textAt(texts, at+1, want)
		if at < 0 {
			t.Fatalf("the field %q is missing or out of the reference's order: %q", want, texts)
		}
	}

	// A boolean hides the placeholder field; its default and flag stay.
	a.toolParams[0].Kind = extensions.ParamBoolean
	tt.Frame()
	if hasExact(tt.Texts(), copy.ParamsPlaceholder) {
		t.Errorf("a boolean drew a placeholder field: %q", tt.Texts())
	}
	if !hasExact(tt.Texts(), copy.ParamsDefault) || !hasExact(tt.Texts(), copy.ParamsFlag) {
		t.Errorf("a boolean dropped a field it should keep: %q", tt.Texts())
	}

	// Placeholder and default survive the editor and reach the request.
	a.toolParams[0].ID = "target"
	a.toolParams[0].Placeholder = "search installed tools"
	a.toolParams[0].Default = "app"
	params := a.declaredParams()
	if len(params) != 1 || params[0].Placeholder != "search installed tools" || params[0].Default != "app" {
		t.Fatalf("declared = %+v", params)
	}

	// beginEdit restores both fields from the stored declaration.
	a.beginEdit("local.abc", CustomTool{
		Name: "Gadget", Program: "/usr/bin/rg",
		Params: []extensions.ParamDefinition{{
			ID: "target", Kind: extensions.ParamText,
			Placeholder: "search installed tools", Default: "app",
		}},
	})
	if got := a.toolParams[0]; got.Placeholder != "search installed tools" || got.Default != "app" {
		t.Fatalf("beginEdit dropped a field: %+v", got)
	}
}
