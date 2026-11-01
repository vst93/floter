package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

// freshnessPage renders one integration card carrying both a freshness block
// and a health block, so the block's column can be compared with the card's
// own row labels on the same frame.
func freshnessPage(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	return freshnessPageWith(t, dark, extensions.Freshness{
		AtSeconds:            time.Now().Add(-90 * time.Second).Unix(),
		Source:               extensions.FreshnessProbe,
		Result:               extensions.FreshnessSuccess,
		CommandCount:         intPtr(6),
		PreviousCommandCount: intPtr(4),
		Delta:                extensions.DeltaIncrease,
	})
}

// freshnessHealthPage is the same card where the time comes from the health
// report instead of a probe: the block then ends with its secondary note line,
// which wraps and therefore owns the block's right edge.
func freshnessHealthPage(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	return freshnessPageWith(t, dark, extensions.Freshness{
		Source: extensions.FreshnessHealth,
		Result: extensions.FreshnessUnknown,
	})
}

func freshnessPageWith(t *testing.T, dark bool, freshness extensions.Freshness) *ui.Tester {
	t.Helper()
	store := newStore(t)
	if dark {
		if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
			t.Fatal(err)
		}
	}
	a := New(store, Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			if id != "dev.floter.probed" {
				return extensions.Freshness{}, false
			}
			return freshness, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true,
			Health: "Degraded", HealthStatus: "degraded",
			HealthReport: testHealthReport(),
		}}
	}
	tt := render(t, a, 720, 1700)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// cardFaceAt reads the card's own face extent off one horizontal line that
// carries no ink: the first and last pixel inside the frame equal to the face
// colour sampled at x=400.
func cardFaceAt(img *image.RGBA, y int) (left, right int, face [4]uint8) {
	return cardFace(img, y)
}

// TestTheFreshnessBlockSitsInTheCardsLabelColumn pins the column: the block's
// label, its two metadata labels and its count line all start where the card's
// own row labels start, and the metadata grid ends where the card's content box
// ends — not on the card's frame.
//
// P13-cx（`docs/liquid-glass.md` §6.67）：改前整块左端 192 / 右端 701，行标签 200 /
// 卡框 701——块挂在卡框上，比它自己的标题行左 8px。
func TestTheFreshnessBlockSitsInTheCardsLabelColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt := freshnessPage(t, dark)
		img := tt.Image()
		// The block's label line: find the card face on that line, which carries
		// no ink besides the label itself.
		freshness, ok := tt.Find("Freshness")
		if !ok {
			t.Fatalf("%s: the freshness label is missing: %q", name, tt.Texts())
		}
		left, right, face := cardFaceAt(img, int(freshness.Y+freshness.H/2))
		inset := float32(0)
		rowLabel, ok := tt.Find("Health")
		if !ok {
			t.Fatalf("%s: the health row label is missing: %q", name, tt.Texts())
		}
		inset = rowLabel.X - float32(left)
		if inset <= 0 {
			t.Fatalf("%s: the card's own rows start at x=%v, the face at x=%d: no inset to compare with (%v)", name, rowLabel.X, left, face)
		}
		if got := freshness.X - float32(left); got != inset {
			t.Errorf("%s: the freshness block starts %vpx inside the card's face (x=%v), the row labels %vpx (x=%v): the block hangs on the frame",
				name, got, freshness.X, inset, rowLabel.X)
		}
		// Every label of the block lives on that same column.
		for _, label := range []string{"Last probe", "Commands"} {
			rect, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s: the block is missing %q: %q", name, label, tt.Texts())
			}
			if rect.X != freshness.X {
				t.Errorf("%s: %q starts at x=%v, the block label at x=%v: the block's lines are not one column",
					name, label, rect.X, freshness.X)
			}
		}
		// The block is a tight stack: the column wrapper adds no gap of its own, so
		// the metadata grid's labels start exactly where the block label's line box
		// ends. This is the half of the round that must not move (P13-cx: horizontal
		// inset only).
		metaLabel, ok := tt.Find("Last probe")
		if !ok {
			t.Fatalf("%s: the metadata label is missing: %q", name, tt.Texts())
		}
		if metaLabel.Y != freshness.Y+freshness.H {
			t.Errorf("%s: the metadata labels start at y=%v, the block label ends at y=%v: the block grew a gap of %vpx",
				name, metaLabel.Y, freshness.Y+freshness.H, metaLabel.Y-(freshness.Y+freshness.H))
		}
		// The metadata grid's second cell ends on the card's content edge, one
		// inset inside the frame: the block must not reach the frame.
		result, ok := tt.Find("Result")
		if !ok {
			t.Fatalf("%s: the result cell is missing: %q", name, tt.Texts())
		}
		wantRight := float32(right+1) - inset
		if result.X+result.W != wantRight {
			t.Errorf("%s: the metadata grid ends at x=%v, want %v (the card's content edge, face %d..%d, inset %v)",
				name, result.X+result.W, wantRight, left, right, inset)
		}
	}
}

// TestTheFreshnessNoteWrapsInsideTheColumn pins the block's wrapping line: the
// health-source note owns the block's right edge when it is present, so a
// column that is too wide wraps it on the card's frame and a column that is too
// narrow wraps it short of the content box.
func TestTheFreshnessNoteWrapsInsideTheColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt := freshnessHealthPage(t, dark)
		img := tt.Image()
		note, ok := findPrefixed(tt, "from the health report")
		if !ok {
			t.Fatalf("%s: the health-source note is missing: %q", name, tt.Texts())
		}
		freshness, ok := tt.Find("Freshness")
		if !ok {
			t.Fatalf("%s: the freshness label is missing: %q", name, tt.Texts())
		}
		left, right, _ := cardFaceAt(img, int(note.Y+note.H/2))
		rowLabel, ok := tt.Find("Health")
		if !ok {
			t.Fatalf("%s: the health row label is missing: %q", name, tt.Texts())
		}
		inset := rowLabel.X - float32(left)
		if note.X != freshness.X {
			t.Errorf("%s: the note starts at x=%v, the block label at x=%v", name, note.X, freshness.X)
		}
		if end := note.X + note.W; end > float32(right+1)-inset+0.5 {
			t.Errorf("%s: the note runs to x=%v, past the card's content edge %v (face %d..%d)",
				name, end, float32(right+1)-inset, left, right)
		}
	}
}

// TestDumpP13cxFreshness writes the integration card's freshness block, light
// and dark. FLOTER_P13CX_DUMP=1 enables it; FLOTER_P13CX_TAG names the file.
func TestDumpP13cxFreshness(t *testing.T) {
	if os.Getenv("FLOTER_P13CX_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13CX_DUMP=1 to write /tmp/floter-P13-cx-freshness-*.png")
	}
	tag := os.Getenv("FLOTER_P13CX_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := freshnessPage(t, dark)
		name := "/tmp/floter-P13-cx-freshness-" + appearance + "-" + tag + ".png"
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatalf("encoding %s: %v", name, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("closing %s: %v", name, err)
		}
	}
}
