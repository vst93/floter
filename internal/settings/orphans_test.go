package settings_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEverySettingHasAReader guards the class of bug the settings page kept
// shipping: a control that records a value, and nothing on the surface it
// belongs to asking for it. It happened twice — `copy_selection`/`paste`
// (P7-iw) and the terminal's appearance switches (P7-iz) — and both times the
// user's report was the same: "the switch does nothing".
//
// A field of settings.Settings counts as answered when its name appears in any
// non-test source outside this package: the settings page only stores it, so a
// reader elsewhere is the surface that makes it true. A setting with no reader
// yet has to be named in settingGaps, with the reason.
func TestEverySettingHasAReader(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	readers, err := readersOutsideSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range settingFields(t, filepath.Join(root, "internal/settings/settings.go")) {
		if readers[name] > 0 {
			continue
		}
		if reason, known := settingGaps[name]; known {
			t.Logf("%s has no reader yet: %s", name, reason)
			continue
		}
		t.Errorf("%s is stored by the settings page and read by nothing: "+
			"either wire it to the surface it belongs to, or name it in settingGaps", name)
	}
}

// settingGaps are the settings this build cannot answer yet, each with what it
// is waiting on. They are the terminal's three appearance options the plugin
// does not expose (mygo v0.5.5, the newest release): see .pending-bugs.md's
// upstream proposal. A field that gains a reader is caught by the test above,
// which then reports it here — remove the entry when that happens.
var settingGaps = map[string]string{
	"TerminalBold": "terminal.Options has no bold mode: the plugin always draws " +
		"bold as a heavier face, and \"bright\" (regular weight, lightened colour) " +
		"needs a per-cell switch it does not have",
	"TerminalScrollbar": "terminal.Options has no scrollbar switch: the view's " +
		"drawScrollbar has no option behind it",
	"TerminalWheelLines": "terminal.Options has no wheel amount: a notch scrolls " +
		"three rows, hard-coded in view.handleWheel",
}

// settingFields lists the exported fields of settings.Settings, in source order.
func settingFields(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Settings" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatal("settings.Settings is not a struct")
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.IsExported() {
						names = append(names, name.Name)
					}
				}
			}
		}
	}
	if len(names) == 0 {
		t.Fatal("settings.Settings has no exported fields: the parse found nothing")
	}
	return names
}

// readersOutsideSettings counts, for each exported field of settings.Settings,
// how many non-test files outside internal/settings mention it. Comments are
// dropped first: a comment that names a setting is not a surface that reads it.
func readersOutsideSettings(root string) (map[string]int, error) {
	fields, err := fieldNames(root)
	if err != nil {
		return nil, err
	}
	patterns := make(map[string]*regexp.Regexp, len(fields))
	for _, name := range fields {
		patterns[name] = regexp.MustCompile(`\b` + name + `\b`)
	}
	counts := make(map[string]int, len(fields))
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path == filepath.Join(root, "internal/settings") {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			code := withoutComments(string(source))
			for name, pattern := range patterns {
				if pattern.MatchString(code) {
					counts[name]++
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// fieldNames reads the field list without a *testing.T, for the counter above.
func fieldNames(root string) ([]string, error) {
	path := filepath.Join(root, "internal/settings/settings.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Settings" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.IsExported() {
						names = append(names, name.Name)
					}
				}
			}
		}
	}
	return names, nil
}

var (
	lineComment  = regexp.MustCompile(`(?m)//.*$`)
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// withoutComments drops Go comments, so that a mention in prose does not count
// as a reader.
func withoutComments(source string) string {
	return lineComment.ReplaceAllString(blockComment.ReplaceAllString(source, ""), "")
}
