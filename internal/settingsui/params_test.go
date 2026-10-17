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

// The declared-inputs editor is the reference's record plate, not a stack of
// settings rows (`CustomIntegrationDrawer.tsx:81-104`,
// `extensions.css:1854-1883`): one hairline plate per parameter, the fields
// side by side in an auto-fit grid (label, id, kind, placeholder, default,
// flag), the required checkbox and the remove key on one controls line.
func TestTheParamEditorDrawsTheReferencePlates(t *testing.T) {
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
	// The empty state is the argument editor's dashed box, not a bare line.
	if !hasExact(tt.Texts(), copy.ParamsEmpty) {
		t.Fatalf("the empty state is missing: %q", tt.Texts())
	}
	add, ok := tt.Find(copy.ParamsAdd)
	if !ok {
		t.Fatalf("the add key is missing: %q", tt.Texts())
	}
	// `.extension-argument-editor__add` is a 28px square (`extensions.css:1856`),
	// not the panel's 30px icon button.
	if add.W != 28 || add.H != 28 {
		t.Errorf("the add key is %.0fx%.0f, want the reference's 28x28", add.W, add.H)
	}
	if err := tt.Click(copy.ParamsAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The reference's six fields are all there.
	for _, want := range []string{
		copy.ParamsLabel, copy.ParamsID, copy.ParamsKind,
		copy.ParamsPlaceholder, copy.ParamsDefault, copy.ParamsFlag,
	} {
		if !hasExact(tt.Texts(), want) {
			t.Fatalf("the field %q is missing: %q", want, tt.Texts())
		}
	}
	// The record shape: the fields share a line instead of each taking a full
	// settings row. The first two labels of the grid start at the same height.
	first, ok := tt.Find(copy.ParamsLabel)
	if !ok {
		t.Fatal("the first field label was not drawn")
	}
	second, ok := tt.Find(copy.ParamsID)
	if !ok {
		t.Fatal("the second field label was not drawn")
	}
	if first.Y != second.Y {
		t.Errorf("the fields are stacked, not gridded: label y=%.1f, id y=%.1f", first.Y, second.Y)
	}
	if second.X <= first.X {
		t.Errorf("the second field did not follow the first on the line: %.1f then %.1f", first.X, second.X)
	}
	// The plate is a hairline box, so the pixel just inside its leading edge is
	// the rule rather than the page's fill.
	img := tt.Image()
	y := int(first.Y + first.H/2)
	if got := img.RGBAAt(int(first.X)-9, y); got == img.RGBAAt(2, y) {
		t.Errorf("the plate has no hairline edge: %v", got)
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

	// The required checkbox is the controls line's own control: a 14px box
	// with a 4px gap to its word (`extensions.css:1881-1882`), and its face
	// follows the tick.
	req, ok := tt.Find(copy.ParamsRequired)
	if !ok {
		t.Fatalf("the required toggle is missing: %q", tt.Texts())
	}
	// The toggle's own box leads its row: 14px wide, then the 4px gap and the
	// word (`extensions.css:1881`).
	boxX, boxY := int(req.X)+7, int(req.Y+req.H/2)
	before := tt.Image().RGBAAt(boxX, boxY)
	if err := tt.Click(copy.ParamsRequired); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.toolParams[0].Required {
		t.Errorf("the required checkbox did not tick the parameter")
	}
	if after := tt.Image().RGBAAt(boxX, boxY); after == before {
		t.Errorf("the checkbox face did not follow the tick: %v", after)
	}

	// Placeholder and default survive the editor and reach the request.
	a.toolParams[0].ID = "target"
	a.toolParams[0].Placeholder = "search installed tools"
	a.toolParams[0].Default = "app"
	params := a.declaredParams()
	if len(params) != 1 || params[0].Placeholder != "search installed tools" || params[0].Default != "app" {
		t.Fatalf("declared = %+v", params)
	}

	// The remove key takes the row away again, back to the empty state.
	if err := tt.Click(copy.ParamsRemove); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.toolParams) != 0 {
		t.Fatalf("the remove key left the row: %+v", a.toolParams)
	}
	if !hasExact(tt.Texts(), copy.ParamsEmpty) {
		t.Errorf("the empty state did not come back: %q", tt.Texts())
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
