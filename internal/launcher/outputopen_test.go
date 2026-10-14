package launcher

import (
	"errors"
	"testing"

	"floter/internal/extensions"
)

// A refused `open` action stays in the output view and says so, the old
// build's `openWithSystem` rule: the view is the surface holding the text the
// user was reading, so a refusal must not close it silently
// (`useLauncherActions.ts:430-445`).
func TestOutputOpenRefusesWithoutClosing(t *testing.T) {
	a := testApp()
	refused := ""
	a.Actions.OpenURL = func(browserID, url string) error {
		refused = browserID + "|" + url
		return errors.New("refused")
	}
	a.output = &OutputView{
		IsList:   true,
		Selected: 0,
		Rows: []extensions.Row{{
			ID:     "bad",
			Title:  "Bad link",
			Action: &extensions.RowAction{Type: "open", URL: "javascript:alert(1)"},
		}},
	}

	a.runOutputRow()

	if refused != "|javascript:alert(1)" {
		t.Errorf("opened %q", refused)
	}
	if !a.InOutputView() {
		t.Error("a refused open closed the output view")
	}
	if want := a.Copy().OpenURLFailed; a.toast != want {
		t.Errorf("toast = %q, want %q", a.toast, want)
	}
}
