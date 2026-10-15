package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The reference's one empty region is `SettingsEmpty` itself
// (`SettingsRows.tsx:218-244`; `settings.css:633-676`): one quiet 22px glyph,
// the state in one line in `--text-secondary` at 580, the grey why underneath
// in `--text-caption`, an 8u gap between them, 24u of padding, and a floor.
// The Sessions page is the one place that overrides the floor, because there
// the empty region is the whole page body (`settings.css:1393`).
//
// Rendering the region on its own reads its box and the ink above the title
// without the surrounding page; the old port drew a single bare muted line,
// so the floor, the glyph and the title/hint pair had all drifted.
func TestTheEmptyRegionWearsTheReferenceSettingsEmptyMetrics(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})

	var ref ui.Handle
	var box ui.Rect
	var floor, gap, icon float32
	tt := ui.NewTester(func(c *ui.Context) {
		// The region sizes itself from the theme the app draws in; capture that
		// same theme here rather than a second one, which could be scaled
		// differently.
		floor, gap, icon = c.Theme().Space(40), c.Theme().Space(2), c.Theme().Space(5.5)
		a.emptyState(c, "square-terminal", en.SessionsEmpty, en.SessionsEmptyHint,
			floor, nil).Bind(&ref)
		box = c.Resolve(ref).Bounds()
	}, 720, 300)
	tt.Frame()

	if box.H < floor {
		t.Errorf("empty region height = %v, want the page-body floor 160u (%v)", box.H, floor)
	}
	if box.H > floor+1 {
		t.Errorf("empty region height = %v, want the 160u floor, not a taller box", box.H)
	}

	title, ok := tt.Find(en.SessionsEmpty)
	if !ok {
		t.Fatalf("the empty title is missing: %q", tt.Texts())
	}
	hint, ok := tt.Find(en.SessionsEmptyHint)
	if !ok {
		t.Fatalf("the empty hint is missing: %q", tt.Texts())
	}
	if d := hint.Y - (title.Y + title.H); absFloat(d-gap) > 1.5 {
		t.Errorf("hint sits %v under the title, want the reference's 8u gap (%v)", d, gap)
	}

	// The glyph is the mark above the title: 22u, drawn from the shared
	// vocabulary. Removing it leaves the band empty, which the pixels catch.
	midX := box.X + box.W/2
	bg := sample(tt, midX, box.Y+1)
	top, bottom := inkBand(tt, midX, box.Y, title.Y-1, bg, 24)
	if top < 0 {
		t.Fatalf("no glyph ink above the empty title: the 22u mark is missing")
	}
	if h := bottom - top + 1; h < icon*0.5 || h > icon+2 {
		t.Errorf("glyph ink height = %v, want the 22u mark (%v)", h, icon)
	}
}

// The live Sessions page wears that region with the reference's resting words
// and its own way out: `terminal.sessionsEmpty` / `sessionsEmptyHint`, not the
// old self-made sentence, and the New session control still starts a session
// (`SessionsPage.tsx:142-167`).
func TestTheSessionsEmptyStateIsTheReferenceEmptyRegion(t *testing.T) {
	en := i18n.For("en").Settings
	started := 0
	a := New(newStore(t), Actions{NewSession: func() { started++ }})
	tt := render(t, a, 720, 760)

	if err := tt.Click(en.PageSessions); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{en.SessionsEmpty, en.SessionsEmptyHint, en.SessionsNew} {
		if !tt.HasText(want) {
			t.Errorf("the sessions empty region is missing %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("No sessions are running.") {
		t.Errorf("the old self-made empty sentence is still drawn: %q", tt.Texts())
	}
	if err := tt.Click(en.SessionsNew); err != nil {
		t.Fatalf("the empty region's way out is missing: %v", err)
	}
	tt.Frame()
	if started != 1 {
		t.Errorf("the empty region's way out ran %d times, want 1", started)
	}
}

// The three-way choice is the reference's (`SessionsPage.tsx:142-167`): a
// failed list leads with AlertCircle and the retry hint, a list being fetched
// with RefreshCw and no hint, and the resting state with SquareTerminal and
// the first-session hint. This build can only reach the resting arm, but the
// mapping is the one place the marks and words are chosen, so all three are
// pinned here.
func TestSessionsEmptyFaceFollowsTheReferenceThreeWay(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	for _, tc := range []struct {
		name string
		got  sessionsEmptyFace
		want sessionsEmptyFace
	}{
		{"resting", sessionsEmptyFaceFor(en, false, false),
			sessionsEmptyFace{Icon: "square-terminal", Title: en.SessionsEmpty, Hint: en.SessionsEmptyHint}},
		{"loading", sessionsEmptyFaceFor(en, false, true),
			sessionsEmptyFace{Icon: "refresh", Title: en.SessionsLoading}},
		{"failed", sessionsEmptyFaceFor(en, true, false),
			sessionsEmptyFace{Icon: "alert", Title: en.SessionsError, Hint: en.SessionsRefreshHint}},
		{"failure wins over loading", sessionsEmptyFaceFor(en, true, true),
			sessionsEmptyFace{Icon: "alert", Title: en.SessionsError, Hint: en.SessionsRefreshHint}},
		{"zh resting", sessionsEmptyFaceFor(zh, false, false),
			sessionsEmptyFace{Icon: "square-terminal", Title: "暂无持久化会话", Hint: "启动终端会话后，会在此处显示"}},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
}

// inkBand returns the top and bottom rows of ink in the column at x between y0
// and y1: rows whose colour differs from the background by more than tol on
// any channel. (-1, -1) when the column holds only background.
func inkBand(tt *ui.Tester, x, y0, y1 float32, bg ui.Color, tol int) (float32, float32) {
	top, bottom := float32(-1), float32(-1)
	for y := int(y0); y <= int(y1); y++ {
		if channelDelta(sample(tt, x, float32(y)), bg) > tol {
			if top < 0 {
				top = float32(y)
			}
			bottom = float32(y)
		}
	}
	return top, bottom
}
