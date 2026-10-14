package settingsui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/scriptlang"
)

// gutterMarks counts the pixels in a strip that differ from the strip's own
// background — the marks a code editor's gutter leaves behind it. The strip
// is the editor's left gutter (line numbers) under the script row's hint.
func gutterMarks(tt *ui.Tester, x0, y0, x1, y1 int) int {
	img := tt.Image()
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > img.Rect.Dx() {
		x1 = img.Rect.Dx()
	}
	if y1 > img.Rect.Dy() {
		y1 = img.Rect.Dy()
	}
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	// The gutter's background is the editor's own fill, read at the strip's
	// first pixel: the marks are whatever differs from it.
	bi := y0*img.Stride + x0*4
	bg := (int(img.Pix[bi]) + int(img.Pix[bi+1]) + int(img.Pix[bi+2])) / 3
	marks := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			v := (int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])) / 3
			if v > bg+25 || v < bg-25 {
				marks++
			}
		}
	}
	return marks
}

// The script box is a code editor, so a script of four lines numbers them in
// the gutter — a bare ui.TextArea (the old control) drew no numbers at all.
// The tester does not record the gutter's digits as text (they are painted
// as the editor's own decorations, like placeholders through a field), so
// the numbers are counted as pixels: the strip under the hint, at the
// editor's left edge, holds four digits' worth of marks against the
// background. The strip is kept narrow so the script text itself — which
// differs from the background too — stays out of the count.
func TestScriptEditorDrawsLineNumbers(t *testing.T) {
	a := New(newStore(t), Actions{ConnectLocalTool: func(tool CustomTool) {}})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Seed four lines from outside the editor: they are what the numbers
	// count, and the seed itself proves an outside draft reaches the buffer.
	a.toolScript = "AAA\nBBB\nCCC\nDDD"
	tt.Frame()

	hint, ok := tt.Find(copy.LocalToolScriptHint)
	if !ok {
		t.Fatalf("the script row's hint is missing: %q", tt.Texts())
	}
	// The gutter sits under the hint, at the editor's left edge: a narrow
	// strip that holds digits and nothing else.
	x0 := int(hint.X) + 8
	y0 := int(hint.Y+hint.H) + 2
	marks := gutterMarks(tt, x0, y0, x0+48, y0+110)
	if marks < 30 {
		t.Errorf("the gutter holds %d content pixels, want the four line numbers' worth (≥30)", marks)
	}
}

// Every language the picker offers resolves to an editor grammar, PowerShell
// included: mygo's LanguageFor answers nil for ".ps1", so its grammar is the
// one described in scripteditor.go. A missing one would render the box
// unhighlighted, silently.
func TestEveryScriptLanguageHasAnEditorLanguage(t *testing.T) {
	for _, language := range scriptlang.Languages {
		if got := editorLanguage(language.ID); got == nil {
			t.Errorf("language %q has no editor grammar", language.ID)
		}
	}
}

// A draft written from outside the editor (the form's own seed, a saved tool
// reopened for editing) becomes the buffer's text: connect then submits it.
func TestScriptEditorTakesAnOutsideDraft(t *testing.T) {
	connected := []CustomTool{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.toolScript = "print('hi')\n"
	tt.Frame()

	a.toolName = "Weather"
	a.toolCommand = "weather"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	if connected[0].ScriptContent != "print('hi')\n" {
		t.Errorf("the editor did not hand the script on: %q", connected[0].ScriptContent)
	}
}

// Switching the language highlights the buffer as the new one: the state's
// grammar follows the picker, so a saved JavaScript script opened under
// Python (or the template swap after a switch) is colored by the right
// language rather than the old one. The claim is the grammar name itself.
func TestScriptEditorFollowsTheLanguageSwitch(t *testing.T) {
	a := New(newStore(t), Actions{ConnectLocalTool: func(tool CustomTool) {}})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.scriptEditor == nil || a.scriptEditor.state == nil {
		t.Fatal("the editor state was never built")
	}
	// The form starts as JavaScript, so the buffer's grammar is JavaScript.
	if got := a.scriptEditor.state.Buffer.Language(); got == nil || got.Name != "JavaScript" {
		t.Errorf("the opening grammar is %v, want JavaScript", got)
	}
	// A switch to Python re-highlights the buffer: open the picker (its
	// current value is the trigger), then take the option.
	if err := tt.Click("JavaScript"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Python"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.scriptEditor == nil || a.scriptEditor.state == nil {
		t.Fatal("the editor state was never built")
	}
	if got := a.scriptEditor.state.Buffer.Language(); got == nil || got.Name != "Python" {
		t.Errorf("the switched grammar is %v, want Python", got)
	}
	// An untouched draft takes the new language's template, as the old
	// TextArea form did; the state hands it back through the form.
	if a.toolScript != scriptlang.Template("python") {
		t.Errorf("the draft is %q, want the Python template", a.toolScript)
	}
	if !strings.Contains(a.toolScript, "python3") {
		t.Errorf("the Python template does not name its toolchain: %q", a.toolScript)
	}
}

// The editor owns the text: a keystroke goes into its buffer and the form
// submits exactly that, so an author's typing survives the per-frame rebuild
// rather than only the outside seeds doing so. This is the hand-back at the
// end of scriptEditorView (the buffer → the form's *script draft); dropping
// it leaves the submitted script as the template, without the typed text.
func TestScriptEditorHandsTheUsersTypingOn(t *testing.T) {
	connected := []CustomTool{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Put the keyboard in the editor's box (its label owns the element, so
	// Focused is how the test knows the click landed) and type a marker.
	hint, ok := tt.Find(copy.LocalToolScriptHint)
	if !ok {
		t.Fatalf("the script row's hint is missing: %q", tt.Texts())
	}
	tt.ClickAt(hint.X+100, hint.Y+hint.H+15)
	tt.Frame()
	if !tt.Focused(copy.LocalToolScript) {
		t.Fatal("the editor did not take the keyboard")
	}
	tt.Type("MARKER")
	tt.Frame()
	a.toolName = "Weather"
	a.toolCommand = "weather"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	if !strings.Contains(connected[0].ScriptContent, "MARKER") {
		t.Errorf("the typed text did not reach the submitted script: %q", connected[0].ScriptContent)
	}
}
