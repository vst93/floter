package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestNoDeadCopy is the guard that keeps dead copy from piling up again.
//
// A field declared in Copy is a string the app is allowed to show. When a
// screen is rebuilt and a key stops being read, the field stays behind —
// nothing in the type system notices, and the next reader cannot tell a key
// that is live from one that is only declared. Ten such fields had collected
// by P7-an (BrowserCDP, PowerConfirm*, PagePlugins*, …); this test walks
// Copy's leaves and fails on any whose name is never selected outside this
// package.
//
// The scan is deliberately name-based and conservative: a field that shares
// its name with an unrelated struct's field reads as used. That direction is
// safe — the guard can miss a dead key, never fail a live one.
func TestNoDeadCopy(t *testing.T) {
	root := moduleRoot(t)
	used := selectedNamesOutsideI18n(t, root)
	if len(used) == 0 {
		t.Fatal("scanned no selector expressions; the source tree was not found")
	}
	for _, name := range copyLeafNames(reflect.TypeOf(Copy{})) {
		if !used[name] {
			t.Errorf("i18n.Copy.%s is declared but never read outside internal/i18n", name)
		}
	}
}

// copyLeafNames walks the Copy tree and returns every string and func field's
// name, sorted and de-duplicated. Structs and slices of structs are walked;
// anything else is a leaf.
func copyLeafNames(t reflect.Type) []string {
	seen := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			switch field.Type.Kind() {
			case reflect.String, reflect.Func:
				seen[field.Name] = true
			case reflect.Struct:
				walk(field.Type)
			case reflect.Slice:
				if field.Type.Elem().Kind() == reflect.Struct {
					walk(field.Type.Elem())
				}
			}
		}
	}
	walk(t)
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// selectedNamesOutsideI18n parses every Go file under cmd/ and internal/ —
// except this package, whose own declarations must not count as a reader —
// and collects the right-hand side of every selector expression (the Name in
// x.Name).
func selectedNamesOutsideI18n(t *testing.T, root string) map[string]bool {
	t.Helper()
	used := map[string]bool{}
	fset := token.NewFileSet()
	excluded := filepath.Join(root, "internal", "i18n")
	for _, dir := range []string{filepath.Join(root, "cmd"), filepath.Join(root, "internal")} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path == excluded {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if sel, ok := node.(*ast.SelectorExpr); ok {
					used[sel.Sel.Name] = true
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scanning %s: %v", dir, err)
		}
	}
	return used
}

// moduleRoot walks up from the test's working directory to the go.mod that
// owns this package, so the scan does not depend on the caller's cwd.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the test directory")
		}
		dir = parent
	}
}
