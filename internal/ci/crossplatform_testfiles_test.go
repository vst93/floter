package ci

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gatePlatforms are the three operating systems every gate builds and every
// CI matrix leg tests (`.github/workflows/go.yml:20`). A test package has to
// compile on all three: `go vet ./...` compiles `_test.go` files too, so a
// package whose tests do not build makes that matrix leg fail before it runs a
// single assertion.
var gatePlatforms = []string{"linux", "darwin", "windows"}

// TestTestHelpersAreVisibleOnEveryPlatform is the contract that a test helper
// belongs to its *package*, not to the platform it happens to be convenient on.
//
// It exists because two packages shipped exactly that defect: `internal/apps`
// declared `icnsEntry`/`icnsContainer`/`pngBytes`/`writeBytes` in a
// `//go:build !windows` file while the untagged `icnsfallback_test.go` used
// them, and `internal/extensions` declared `testPlatforms` in a
// `//go:build !windows` file while the untagged `commandword_test.go` used it.
// `GOOS=windows go vet ./...` therefore failed with `undefined: pngBytes` and
// `undefined: testPlatforms`, which is why the windows-latest leg of the CI
// matrix never reached `go test` at all — Windows had zero executed tests, and
// nothing on the Linux gate could see it, because a gated file simply does not
// participate in the build it is gated out of.
//
// The scan asks the compiler's own questions without running it: `go/build`
// decides which files a platform compiles (filename suffixes and `//go:build`
// lines both), `go/parser` collects what each file declares, and every
// identifier a platform's test files mention is checked against what that
// platform actually has. Names are collected per directory and merged across
// package names, so a `foo_test` package reaching into `foo` cannot produce a
// false positive; the direction of the check is "this name exists nowhere on
// this platform", which is precisely the compile error.
func TestTestHelpersAreVisibleOnEveryPlatform(t *testing.T) {
	for _, root := range []string{"internal", "cmd"} {
		root := filepath.Join("..", "..", root)
		dirs, err := testDirectories(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs {
			checkDirectoryOnEveryPlatform(t, dir)
		}
	}
}

// testDirectories lists the directories under root that hold at least one
// `_test.go` file, relative to the repository root.
func testDirectories(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		for _, child := range entries {
			if strings.HasSuffix(child.Name(), "_test.go") && !child.IsDir() {
				dirs = append(dirs, path)
				break
			}
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs, err
}

// checkDirectoryOnEveryPlatform reports every test identifier that a platform
// mentions but does not have.
func checkDirectoryOnEveryPlatform(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}

	available := make(map[string]map[string]bool, len(gatePlatforms))
	compiled := make(map[string][]string, len(gatePlatforms))
	every := make(map[string]bool)
	for _, goos := range gatePlatforms {
		context := build.Default
		context.GOOS = goos
		context.GOARCH = "amd64"
		context.CgoEnabled = false
		declared := make(map[string]bool)
		var files []string
		for _, name := range names {
			included, matchErr := context.MatchFile(dir, name)
			if matchErr != nil {
				t.Fatalf("%s: %v", filepath.Join(dir, name), matchErr)
			}
			if !included {
				continue
			}
			files = append(files, name)
			file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if parseErr != nil {
				t.Fatalf("%s: %v", filepath.Join(dir, name), parseErr)
			}
			for _, name := range declaredNames(file) {
				declared[name] = true
				every[name] = true
			}
		}
		available[goos] = declared
		compiled[goos] = files
	}

	for _, goos := range gatePlatforms {
		missing := make(map[string]bool)
		for name := range every {
			if available[goos][name] || types.Universe.Lookup(name) != nil {
				continue
			}
			missing[name] = true
		}
		if len(missing) == 0 {
			continue
		}
		context := build.Default
		context.GOOS = goos
		context.GOARCH = "amd64"
		context.CgoEnabled = false
		for _, name := range compiled[goos] {
			if !strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			set := token.NewFileSet()
			file, parseErr := parser.ParseFile(set, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				t.Fatalf("%s: %v", path, parseErr)
			}
			bound := boundNames(file)
			ast.Inspect(file, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok || !missing[ident.Name] || bound[ident.Name] {
					return true
				}
				position := set.Position(ident.Pos())
				t.Errorf("%s: %s does not exist on %s — it is declared only in a file %s does not compile (helper belongs to the package, not to one platform)",
					position, ident.Name, goos, goos)
				return true
			})
		}
	}
}

// boundNames are the identifiers in a file that do not name a package-level
// symbol: names the file binds itself (`:=`, `var`, parameters, labels) and
// names that are syntax rather than references (a selector's field, a struct
// field, a composite literal's key, an import's alias). Subtracting them is
// what keeps a local `write := func(...)` from being read as a use of the
// package-level `write` helper that another platform happens to own.
func boundNames(file *ast.File) map[string]bool {
	bound := make(map[string]bool)
	fields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				bound[name.Name] = true
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			bound[typed.Sel.Name] = true
		case *ast.KeyValueExpr:
			if ident, ok := typed.Key.(*ast.Ident); ok {
				bound[ident.Name] = true
			}
		case *ast.ImportSpec:
			if typed.Name != nil {
				bound[typed.Name.Name] = true
			}
		case *ast.AssignStmt:
			if typed.Tok == token.DEFINE {
				for _, lhs := range typed.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						bound[ident.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			if typed.Tok == token.DEFINE {
				for _, expr := range []ast.Expr{typed.Key, typed.Value} {
					if ident, ok := expr.(*ast.Ident); ok {
						bound[ident.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				bound[name.Name] = true
			}
		case *ast.TypeSpec:
			bound[typed.Name.Name] = true
		case *ast.FuncDecl:
			fields(typed.Recv)
			fields(typed.Type.Params)
			fields(typed.Type.Results)
		case *ast.FuncLit:
			fields(typed.Type.Params)
			fields(typed.Type.Results)
		case *ast.StructType:
			fields(typed.Fields)
		case *ast.InterfaceType:
			fields(typed.Methods)
		case *ast.LabeledStmt:
			bound[typed.Label.Name] = true
		}
		return true
	})
	return bound
}

// declaredNames are the package-level names a file introduces: functions
// without a receiver, and the names inside every `var`, `const` and `type`
// declaration. Methods are skipped — a method needs its receiver type to be
// available, which is the same check one level down.
func declaredNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil {
				names = append(names, typed.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, typed.Name.Name)
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						names = append(names, name.Name)
					}
				}
			}
		}
	}
	return names
}
