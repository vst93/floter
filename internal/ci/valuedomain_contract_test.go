package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/settings"
	"floter/internal/valuedomain"
)

// TestValueDomainsHaveOneAnswer is the contract `internal/valuedomain` states:
// every setting stored as a name has one id list, and every face reads it.
//
// Before P11-w the same ids were written in five packages: the picker's tables
// in `internal/i18n` (twice per domain, once per language), the loader's slices
// and factor map in `internal/settings`, the colour and effect tables'
// keys in `internal/terminalui` and `internal/glassmap`, and the app icon's
// `== "light"` in `internal/shell` and `internal/settingsui`. They agreed, so
// nothing was broken — the change that breaks them is adding one option, which
// is a change someone will make on purpose.
//
// The banned shapes are the *lists*, not the ids: `internal/i18n`'s label maps
// name the ids as keys (that is the one part that is per language), and a
// renderer's own table may be keyed by them. What may not exist outside
// `internal/valuedomain` is a second list of which ids exist.
func TestValueDomainsHaveOneAnswer(t *testing.T) {
	needles := []string{
		`[]string{"dark", "light"}`,         // app icons
		`[]string{"auto", "light", "dark"}`, // themes, picker order
		`[]string{"dark", "light", "auto"}`, // …and the loader's old order
		`[]string{"off", "frosted"}`,        // glass steps
		`[]string{"tiny", "small"}`,         // interface sizes
		`[]string{"beam", "block"}`,         // cursor shapes
		`[]string{"compact", "regular"}`,    // paddings
		`[]string{"inherit", "contrast"}`,   // palettes
		`[]string{"font", "bright"}`,        // bold modes
		`== "light"`,                        // a second app-icon rule
	}

	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single list lives here.
				if d.Name() == "valuedomain" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			for index, line := range strings.Split(string(data), "\n") {
				code := line
				if comment := strings.Index(code, "//"); comment >= 0 {
					code = code[:comment]
				}
				for _, needle := range needles {
					if strings.Contains(code, needle) {
						t.Errorf("%s:%d spells a value domain again (%s): read the ids from valuedomain (see internal/valuedomain/valuedomain.go)",
							rel, index+1, needle)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}

// TestTheLoaderAndThePickableDomainsAgree is the behavioral half: whatever the
// picker offers must survive the loader untouched, and whatever the loader
// accepts must be something the picker can show.
//
// It walks the domains the settings package normalizes rather than naming them
// by hand, so a domain added to `valuedomain` without being wired into the
// loader (or the other way round) is visible here.
func TestTheLoaderAndThePickableDomainsAgree(t *testing.T) {
	domains := []struct {
		name   string
		ids    []string
		accept func(string) string
	}{
		{"themes", valuedomain.Themes, settings.NormalizeTheme},
		{"glass steps", valuedomain.GlassSteps, settings.NormalizeGlassStep},
		{"scales", valuedomain.Scales, settings.NormalizeUIScale},
	}

	for _, domain := range domains {
		if len(domain.ids) == 0 {
			t.Fatalf("%s: empty domain", domain.name)
		}
		for _, id := range domain.ids {
			if got := domain.accept(id); got != id {
				t.Errorf("%s: the loader turns the offered id %q into %q", domain.name, id, got)
			}
		}
		// One value the domain does not ship, to prove the loader is not just
		// echoing its input back.
		if got := domain.accept("not-a-shipped-id"); got == "not-a-shipped-id" {
			t.Errorf("%s: the loader accepted an id the domain does not ship", domain.name)
		}
	}

	// The terminal's three string domains are normalized through `Pick`, which
	// is the loader's shared membership check; a stored value in each must come
	// back as itself.
	for _, test := range []struct {
		name  string
		ids   []string
		apply func(*settings.Settings, string)
		read  func(settings.Settings) string
	}{
		{"cursor shapes", valuedomain.CursorShapes,
			func(s *settings.Settings, v string) { s.CursorShape = v },
			func(s settings.Settings) string { return s.CursorShape }},
		{"paddings", valuedomain.Paddings,
			func(s *settings.Settings, v string) { s.TerminalPadding = v },
			func(s settings.Settings) string { return s.TerminalPadding }},
		{"palettes", valuedomain.Palettes,
			func(s *settings.Settings, v string) { s.TerminalTheme = v },
			func(s settings.Settings) string { return s.TerminalTheme }},
		{"bold modes", valuedomain.BoldModes,
			func(s *settings.Settings, v string) { s.TerminalBold = v },
			func(s settings.Settings) string { return s.TerminalBold }},
	} {
		for _, id := range test.ids {
			s := settings.Default()
			test.apply(&s, id)
			if got := test.read(settings.Normalize(s)); got != id {
				t.Errorf("%s: the loader turns the offered id %q into %q", test.name, id, got)
			}
		}
	}

	// The interface size is the one domain whose ids carry a value.
	for _, step := range valuedomain.Scales {
		if factor := settings.UIScaleFactor(step); factor <= 0 {
			t.Errorf("scale %q has factor %v, want > 0", step, factor)
		}
	}
}
