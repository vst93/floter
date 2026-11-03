package settingsui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// disabledArm is which of `docs/liquid-glass.md` §1's three disabled readings a
// site takes: a face loses its lit crown, a groove and a bare (ink-only) control
// have no material to add, they only take the framework's one fade.
type disabledArm int

const (
	armFace   disabledArm = iota // 面：`DimFace`（底对侧还在、受光边熄了）
	armGroove                    // 槽：不叠材质（内线各留一半）
	armBare                      // 透明面：不叠材质（只有墨）
	armRoot                      // 只在根上挂一道淡；材质由调用点按上面三种读法给
)

func (a disabledArm) String() string {
	return [...]string{"面 (DimFace)", "槽 (no material)", "透明面 (no material)", "根 (flag only)"}[a]
}

// disabledSite is one `Disabled(...)` call site, keyed by the function that
// owns it. The arm is a claim about what the site draws, and
// TestEveryUnavailableControlIsClassified fails when a site exists that nobody
// classified — the whole point of §1's table is that a new site cannot quietly
// invent a fourth depth.
type disabledSite struct {
	file, fn string
	arm      disabledArm
}

var disabledSites = []disabledSite{
	// 面 — the pill family. Every one of these goes through
	// `extensionActionFace`'s disabled arm (P13-s): resting plane, `DimFace`,
	// no lit crown.
	{"uninstall.go", "uninstallActionButton", armFace},
	{"config.go", "configActions", armFace},
	{"rows.go", "disabledActionButton", armFace},
	{"rows.go", "accentPrimaryFace", armFace},
	{"rows.go", "busyActionButton", armFace},
	{"rows.go", "sessionKillConfirm", armFace},
	{"settingsui.go", "configOpenToolRow", armFace},
	{"settingsui.go", "repairAction", armFace},
	// 面 — the cards and the glyph keys that are a face in their own right.
	{"uninstall.go", "uninstallPartRow", armFace},
	// 槽 — no lit edge to turn off; the framework's half is the statement.
	{"rows.go", "disabledSwitch", armGroove},
	{"settingsui.go", "slider", armGroove},
	// 透明面 — only ink; half the ink is the whole statement.
	{"rows.go", "extensionIconButton", armBare},
	{"detach.go", "detachedBar", armBare},
	// 根 — the one shared flag helper: it owns the depth, not the material.
	{"config.go", "lockedControl", armRoot},
}

// TestEveryUnavailableControlIsClassified is the pin behind §1's「禁用的材质」
// table: a `Disabled(...)` in this package or in the detached window's bar must
// be one of the sites the table names. The registry is by function, not by
// line, so a moved call does not turn this red but a new one does.
func TestEveryUnavailableControlIsClassified(t *testing.T) {
	want := map[string]disabledSite{}
	for _, s := range disabledSites {
		want[s.file+":"+s.fn] = s
	}
	seen := map[string]bool{}
	for _, dir := range []string{".", filepath.Join("..", "shell")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				calls := disabledCalls(fn)
				if len(calls) == 0 {
					continue
				}
				key := name + ":" + fn.Name.Name
				seen[key] = true
				if _, ok := want[key]; !ok {
					for _, at := range calls {
						t.Errorf("%s: %s calls Disabled(%s) at %s, and §1's table does not name this site — "+
							"add it to disabledSites with the arm it draws (面/槽/透明面)",
							key, fn.Name.Name, at.arg, fset.Position(at.pos))
					}
				}
			}
		}
	}
	for _, s := range disabledSites {
		if !seen[s.file+":"+s.fn] {
			t.Errorf("disabledSites names %s:%s, which no longer calls Disabled(): drop the row or fix the name",
				s.file, s.fn)
		}
	}
}

type disabledCall struct {
	pos token.Pos
	arg string
}

// disabledCalls returns every `X.Disabled(...)` call inside fn, including the
// ones inside closures — a slider's track is disabled inside a `Children(func()
// {…})`, and that is still the slider row's statement.
func disabledCalls(fn *ast.FuncDecl) []disabledCall {
	var out []disabledCall
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Disabled" || len(call.Args) != 1 {
			return true
		}
		arg := ""
		if id, ok := call.Args[0].(*ast.Ident); ok {
			arg = id.Name
		} else {
			arg = fmt.Sprint("?")
		}
		out = append(out, disabledCall{pos: call.Pos(), arg: arg})
		return true
	})
	return out
}

// TestDumpP13FIDisabledSheet is the round's visual evidence: the same six
// controls, usable on the left and unusable on the right, off the real page
// ground, both appearances. Run it with FLOTER_P13FI_DUMP=1.
func TestDumpP13FIDisabledSheet(t *testing.T) {
	if os.Getenv("FLOTER_P13FI_DUMP") == "" {
		t.Skip("set FLOTER_P13FI_DUMP=1 to write the round's visual evidence")
	}
	on, val := true, true
	type pair struct {
		name string
		draw func(a *App, c *ui.Context, disabled bool) ui.Element
	}
	pairs := []pair{
		{"action pill", func(a *App, c *ui.Context, d bool) ui.Element {
			if d {
				return a.disabledActionButton(c, "Check")
			}
			return a.actionButton(c, "Check")
		}},
		{"accent primary", func(a *App, c *ui.Context, d bool) ui.Element {
			return a.extensionPrimaryButton(c, "Save", d)
		}},
		{"icon key", func(a *App, c *ui.Context, d bool) ui.Element {
			return a.extensionIconButton(c, "trash", "Remove", d)
		}},
		{"switch", func(a *App, c *ui.Context, d bool) ui.Element {
			sw := a.switchControl(c, &on)
			if d {
				return disabledSwitch(sw)
			}
			return sw
		}},
		{"component card (fixed)", func(a *App, c *ui.Context, d bool) ui.Element {
			if d {
				return a.uninstallPartRow(c, uninstallPart{Label: "floter", Hint: "Always removed", Fixed: true})
			}
			return a.uninstallPartRow(c, uninstallPart{Label: "hosts", Hint: "Leaves a file", Value: &val})
		}},
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Row(c).Padding(20).Gap(24).AlignItems(ui.Start).Children(func() {
				for _, usable := range []bool{true, false} {
					ui.Column(c).Width(240).Gap(14).Children(func() {
						for _, p := range pairs {
							p.draw(a, c, !usable)
						}
					})
				}
			})
		}, 560, 320)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		out := filepath.Join(os.TempDir(), "floter-P13-fi-disabled-"+appearanceName(dark)+".png")
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", out)
	}
}

// The pointer's half of the same statement. §1 answers「这个控件现在不能用」
// with material — a face goes unlit — so the pointer is *not* where this app
// says it; sixteen sites leave it at the arrow, and one changes it. That one is
// measured first, because a second shape appearing anywhere else would be a
// second vocabulary for one state.
//
// `not-allowed` is the reference's own arm for the uninstall dialog's locked
// program row (`ComponentizedUninstallDialog.css:60-83`, `:has(input:disabled)
// { cursor: not-allowed }`) — a card that *contains* a locked check box, which
// is a form set rather than a key. Every key in the app says `default`.
func TestAnUnavailableControlPointsAtItselfWithOneOfTwoShapes(t *testing.T) {
	ground := realPageGround(t, false)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(false) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	on, val := true, true
	cases := []struct {
		what string
		want ui.Cursor
		draw func(c *ui.Context) ui.Element
	}{
		{"the row-locked action pill", ui.CursorDefault, func(c *ui.Context) ui.Element {
			return a.disabledActionButton(c, "Check")
		}},
		{"the accent primary while it writes", ui.CursorDefault, func(c *ui.Context) ui.Element {
			return a.extensionPrimaryButton(c, "Save", true)
		}},
		{"the config toolbar's icon key", ui.CursorDefault, func(c *ui.Context) ui.Element {
			return a.extensionIconButton(c, "trash", "Remove", true)
		}},
		{"the locked switch", ui.CursorDefault, func(c *ui.Context) ui.Element {
			return disabledSwitch(a.switchControl(c, &on))
		}},
		{"the uninstall dialog's fixed component card", ui.CursorNotAllowed, func(c *ui.Context) ui.Element {
			return a.uninstallPartRow(c, uninstallPart{Label: "floter", Hint: "Always removed", Fixed: true})
		}},
		{"the same card on a row the user chooses", ui.CursorDefault, func(c *ui.Context) ui.Element {
			return a.uninstallPartRow(c, uninstallPart{Label: "hosts", Hint: "Leaves a file", Value: &val})
		}},
	}
	for _, tc := range cases {
		var box ui.Rect
		var ref ui.Handle
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(24).Width(320).Children(func() {
				tc.draw(c).Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
		}, 400, 200)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Move(box.X+box.W/2, box.Y+box.H/2)
		tt.Frame()
		if got := tt.Cursor(); got != tc.want {
			t.Errorf("hovering %s shows %v, want %v", tc.what, got, tc.want)
		}
	}
}
