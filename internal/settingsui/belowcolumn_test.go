package settingsui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-bf: a card row's `below` slot had no horizontal column of its own.
//
// Everything drawn there — an unfolded permission audit, an open configuration
// form, an approval record's permission tiers — was a sibling of the row inside
// the card's column, so it started at the card's own edge. The row's labels
// start one rule slot plus the row's padding in, which is exactly the card's
// inset (`rowInset`: 8u on the integration card). Two of those blocks had
// noticed and re-applied the inset themselves; every other one had not, so the
// same question had two answers and the blocks disagreed by one inset.
//
// The fix is one answer: `cardRow` puts the whole `below` slot on the row's
// content column, and the blocks that used to pad themselves no longer do.
// These pins hold both halves down.

// belowBlockBodies reads the named methods out of the package's own source, so
// the contract arm can assert what a block does *not* do (pad itself) without
// guessing at line numbers.
func belowBlockBodies(t *testing.T, file string, names ...string) map[string]string {
	t.Helper()
	source, err := os.ReadFile(file) // #nosec G304 -- the package's own source
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	out := make(map[string]string, len(names))
	for _, name := range names {
		start := strings.Index(text, "func (a *App) "+name+"(")
		if start < 0 {
			t.Fatalf("%s is not declared in %s", name, file)
		}
		rest := text[start+1:]
		end := strings.Index(rest, "\nfunc ")
		if end < 0 {
			end = len(rest)
		}
		out[name] = rest[:end]
	}
	return out
}

// TestNoBelowBlockReappliesTheCardsInset is the source arm: the card's content
// column is `cardRow`'s answer, so a block drawn in the `below` slot must not
// ask `rowInset` again. Asking twice is not a style nit — it put the permission
// audit's tier labels one whole inset *right* of the labels they explain (the
// regression this round had to fix on the way in).
func TestNoBelowBlockReappliesTheCardsInset(t *testing.T) {
	bodies := belowBlockBodies(t, "settingsui.go", "permissionAudit")
	bodies["configFields"] = belowBlockBodies(t, "config.go", "configFields")["configFields"]
	for name, body := range bodies {
		if strings.Contains(body, "rowInset(") {
			t.Errorf("%s asks rowInset for the card's column: the `below` slot is already inset by "+
				"cardRow, so the block is padded twice and lands one inset right of the row labels", name)
		}
	}
}

// TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn is the geometry arm
// for the approval record: the tier captions, the pills under them and the
// changed-manifest banner all belong on the column the card's row labels use.
//
// Before P11-bf the captions and the banner started at the card's own edge,
// 8u left of the label they explain, and the pill chips sat a pill's padding
// right of that — three different left edges inside one block.
func TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn(t *testing.T) {
	en := i18n.For("en").Settings
	fixture := auditFixture()
	// A changed manifest, so the stale banner is on screen too.
	fixture[0].Approval.CurrentDigest = "sha256-bbbb"
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration { return fixture }
	tt := integrationPage(t, a, 720, 900)

	label := mustFind(t, tt, en.ApprovalRecordPermissions)
	for _, want := range []string{en.PermissionsEnforcedLabel, en.PermissionsDisclosureLabel} {
		got := mustFind(t, tt, want)
		if got.X != label.X {
			t.Errorf("the tier caption %q starts at x=%v but the row label at x=%v (delta %v): "+
				"the block is not on the card's text column", want, got.X, label.X, got.X-label.X)
		}
	}
	// A pill is a chip on that column: its own padding sits inside the column,
	// never left of it.
	pill := mustFind(t, tt, "Write the clipboard")
	if pill.X <= label.X {
		t.Errorf("the pill's text starts at x=%v, not inside the column x=%v: the chip lost its padding",
			pill.X, label.X)
	}
	// The banner is a face, so its *sentence* starts one banner padding in; what
	// matters is that the face itself begins on the column, i.e. its text is
	// right of the label rather than left of it.
	banner := mustFind(t, tt, en.ApprovalRecordChanged)
	if banner.X < label.X {
		t.Errorf("the changed-manifest banner's sentence starts at x=%v, left of the column x=%v: "+
			"the banner's face hangs off the card's edge", banner.X, label.X)
	}
}

// TestTheConfigFormSitsOnTheCardsTextColumn is the same arm for the other block
// that used to pad itself: the open configuration form's fields.
func TestTheConfigFormSitsOnTheCardsTextColumn(t *testing.T) {
	en := i18n.For("en").Settings
	a, _ := configFormApp(t,
		[]ConfigField{{Key: "token", Label: "Token", Type: "text"}},
		map[string]any{"token": "abc"},
	)
	tt := integrationPage(t, a, 900, 1400)
	label := mustFind(t, tt, en.ConfigSection)
	field := mustFind(t, tt, "Token")
	if field.X != label.X {
		t.Errorf("the form's first field label starts at x=%v but the row label at x=%v (delta %v): "+
			"the form is not on the card's text column", field.X, label.X, field.X-label.X)
	}
}

// The P11-bf visual evidence: the two blocks that live in a row's `below` slot,
// in both appearances — the approval record's permission tiers (with the
// changed-manifest banner under them) and the open configuration form.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11BFBelowColumnPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		// ① The approval record: its tier captions, the pills and the
		// changed-manifest banner under them.
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		fixture := auditFixture()
		fixture[0].Approval.CurrentDigest = "sha256-bbbb"
		a := New(store, Actions{})
		a.Integrations = func() []Integration { return fixture }
		a.selectPage(PageIntegrations)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 720, 1000)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P11-bf-approval-"+appearance+".png"), tt)

		// ② The open configuration form, whose fields are the other `below`
		// block that used to carry its own inset.
		configStore := settings.NewStore(settings.Default())
		if err := configStore.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		ca := oneIntegrationApp(Integration{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Config: []ConfigField{
				{Key: "token", Label: "Token", Type: "text"},
				{Key: "endpoint", Label: "Endpoint", Type: "text"},
			},
			ConfigValues: map[string]any{"token": "abc", "endpoint": "https://example.test"},
		}, Actions{})
		ca.Store = configStore
		ca.configOpen = map[string]bool{"dev.floter.tool": true}
		ca.selectPage(PageIntegrations)
		ctt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(ca.tokens(c).Theme)
			ca.View(c)
		}, 900, 1400)
		ctt.SetDark(dark)
		ctt.Frame()
		ctt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P11-bf-config-"+appearance+".png"), ctt)
	}
}
