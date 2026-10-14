package settingsui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"floter/internal/i18n"
	"floter/internal/scriptlang"
)

// scriptEditor is the local-tool form's script box, kept on the App across
// frames: a *editor.State built every frame (or a NewBuffer every frame) would
// drop the buffer's undo history, carets and scroll position as the form
// rebuilds on each keystroke. It is one state, not one per language, because
// the old TextArea held one draft and a language switch only swaps an untouched
// template for the new language's (see isTemplate) — a written script carries
// over exactly as it did before.
//
// `language` is the language the buffer is highlighted as, and `written` is the
// text the last frame handed back to the form's local draft. The latter is how
// an *outside* write to the draft (a language switch's template, beginEdit, a
// caller's seed) is told from the user's own typing: the user's keystrokes edit
// the buffer directly and leave the draft at `written`, so they are not pushed
// back into the buffer.
type scriptEditor struct {
	state    *editor.State
	language string
	written  string
}

// scriptEditorView draws the local-tool form's script editor: the language's
// syntax, the gutter's line numbers, and the buffer the form's *script draft
// reads. The language picker has already run by the time this is called, so a
// switch shows as *language differing from the state's own.
//
// The editor owns the text, so the flow is: push an outside draft change into
// the buffer, draw, then hand the buffer back through *script. The buffer is
// the source of truth the connect submits.
//
// Note on the input boundary (internal/inputtext): a script is a provider file
// the host writes, not a PTY byte stream, so its edits never travel through
// inputtext's Zs→U+0020 / trailing-newline rules — those exist because terminal
// bytes must match what a keyboard would send. The editor's own paste path
// (mygo ui/editor) is the right one here, and that is deliberate, not a gap.
func (a *App) scriptEditorView(c *ui.Context, copy i18n.Settings, language, script *string) ui.Element {
	t := c.Theme()
	if a.scriptEditor == nil {
		a.scriptEditor = &scriptEditor{}
	}
	se := a.scriptEditor
	if se.state == nil {
		se.state = &editor.State{Buffer: editor.NewBuffer("")}
	}
	st := se.state

	// The editor's look is resolved every frame, so a changed appearance or UI
	// scale lands without dropping the buffer. Label names the box for
	// assistive technology, as the old form's own field did.
	st.Font = ui.Font{Family: "monospace", Size: t.FontSize}
	st.LineHeight = scriptLineHeight(t)
	st.Theme = editor.DefaultTheme(t)
	st.Label = copy.LocalToolScript

	// A language switch re-highlights the buffer; an untouched draft (empty, or
	// still one of the shipped templates) takes the new language's template.
	if se.language != *language {
		st.Buffer.SetLanguage(editorLanguage(*language))
		if text := st.Buffer.String(); text == "" || isTemplate(text) {
			st.Buffer.Set(scriptlang.Template(*language))
		}
		se.language = *language
	}

	// A draft changed from outside the editor becomes the buffer; the user's
	// own typing already edited the buffer, so the draft still equals `written`
	// and is left alone.
	if *script != se.written {
		st.Buffer.Set(*script)
	}

	e := editor.View(c, st).Height(t.Space(18)).Grow(1)

	// Hand the buffer back to the form's draft: a keystroke this frame, or the
	// template the switch above seeded, is what the form will submit.
	*script = st.Buffer.String()
	se.written = *script
	return e
}

// scriptLineHeight is the editor's line height as a multiple of the font size
// that reproduces the old TextArea's `FixedLineHeight(t.Space(2.25))` DIPs: the
// editor's State.LineHeight is a multiple of the font's size (mygo
// ui/editor/state.go), while the old fixed height was DIPs outright.
func scriptLineHeight(t *ui.Theme) float32 {
	size := t.FontSize
	if size <= 0 {
		size = 13
	}
	return t.Space(2.25) / size
}

// editorLanguage is the editor's highlighter for a scriptlang id: by the file
// extension, so the picker's ids and the editor's languages cannot drift. Every
// shipped language is known by its extension except PowerShell — mygo's
// LanguageFor answers nil for "main.ps1", so its grammar is the one described
// here.
func editorLanguage(id string) *editor.Language {
	if id == "powershell" {
		return &powershellLanguage
	}
	if l := editor.LanguageFor("main." + scriptlang.Extension(id)); l != nil {
		return l
	}
	return nil
}

// powershellLanguage is the description mygo ships no grammar for: PowerShell's
// line comments and its two bracket pairs, which is all the form needs (matching
// and indentation), without a tokenizer to color it.
var powershellLanguage = editor.Language{
	Name:        "PowerShell",
	LineComment: "#",
	Brackets: []editor.BracketPair{
		{Open: "{", Close: "}"},
		{Open: "(", Close: ")"},
	},
}
