package shell

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
)

// The user's two named items, checked on the **shipped** frame rather than on
// the launcher's own package.
//
// Both reports (「搜索页面左右两边各有一大块空白」 and 「输入框中有输入内容时出现
// 的「在终端中打开」项：现在固定在第一项…应该有选中样式（当前缺失）」) are about
// what the *app* draws: the launcher's tests render it bare, on the framework's
// stock theme, with no platform inset and no shell card around it. A number that
// matches there can still be wrong here — the card's width is the window less
// `insetFor`, the column's 16u is the shipped `small` step's 14.4px, and the
// action bar only wears its selected face when the *selection* reached it, which
// needs the shell's own frame to have run first. That gap is why every round
// that measured the launcher alone kept reporting green while the user kept
// reporting the same two bugs.
//
// These two tests therefore read the shell's frame: the window the app is given,
// the card inside it, and the bar the field's own row draws.

// The search page's card is the window less the platform's inset, with no band
// of desktop between them: the user's 「左右各有一大块空白」.
//
// `base.css`'s `.collapsed-shell` is `width: 100vw` padding `calc(var(--u) * 10)`
// a side on Linux and `4u 10u 12u 4u` on Windows (`:1503,1591-1607`), and
// `.collapsed-card { width: 100% }` (`launcher.css:8-13`) is 100% of what that
// padding leaves. So the card's content box starts on the inset's own edge and
// ends on its mirror: the only pixels beside it are the inset, which is the
// design's shadow room, never a blank block. A card narrower than the window
// less its inset is exactly the report.
func TestTheSearchCardFillsTheWindowLessTheInset(t *testing.T) {
	a := newApp(t)
	tt := ui.NewTester(a.View, launcher.InputWindowWidth, 420)
	tt.Frame()
	tt.Type("term")
	tt.Frame()

	_, rightUnits, _, leftUnits := a.insetFor(true)
	space := float32(a.Launcher.Spacing)
	left, right := leftUnits*space, rightUnits*space
	// The card's own 1px stroke is painted inside its box, so the content is one
	// pixel in from the card's outer edge on either side.
	wantLeft := int(left) + 1
	wantRight := launcher.InputWindowWidth - int(right) - 2

	// The field is full-bleed across the card (`.collapsed-card__input-row` is
	// the card's whole width), so the run of its own fill *is* the card's content
	// box on a row that holds no text: read the run's first and last pixel.
	img := tt.Image()
	rect, ok := tt.Find("Search")
	if !ok {
		t.Fatalf("the field was not drawn: %q", tt.Texts())
	}
	y := int(rect.Y + rect.H/2)
	edge := img.RGBAAt(wantRight, y)
	first, last := -1, -1
	for x := 0; x < launcher.InputWindowWidth; x++ {
		if img.RGBAAt(x, y) != edge {
			continue
		}
		if first < 0 {
			first = x
		}
		last = x
	}
	if first < 0 {
		t.Fatalf("the field's fill (%v) is nowhere on its own row", edge)
	}
	if absi(first-wantLeft) > 1 {
		t.Errorf("the card's content starts at x=%d, want the inset's own edge x=%d (%v blank)",
			first, wantLeft, first)
	}
	if absi(last-wantRight) > 1 {
		t.Errorf("the card's content ends at x=%d, want the mirrored edge x=%d (%v blank)",
			last, wantRight, launcher.InputWindowWidth-last)
	}
	// And the band beside the card is transparent, not a slice of window fill:
	// a gap of *painted* pixels there would be the extra底色 the user saw.
	if c := img.RGBAAt(1, y); c.A > 8 {
		t.Errorf("the window's margin is painted (alpha %d at x=1): the card does not own the window", c.A)
	}

	// A client wider than the 720 contract is the shape the report is actually
	// made of: the port capped the card at the contract and centred it, so a
	// window the platform made wider than asked painted the desktop *inside* the
	// window as two bands of nothing beside the card. `FillWidth` cannot disagree
	// with the window it fills, so the card's content box still starts on the
	// inset's own edge — wherever the window ended up.
	wideApp := newApp(t)
	wide := ui.NewTester(wideApp.View, 900, 420)
	wide.Frame()
	wide.Type("term")
	wide.Frame()
	wideImg := wide.Image()
	wideRect, ok := wide.Find("Search")
	if !ok {
		t.Fatalf("the field was not drawn in the wider window: %q", wide.Texts())
	}
	wideY := int(wideRect.Y + wideRect.H/2)
	// The field's fill again, read as the run of the card's own edge colour.
	wideEdge := wideImg.RGBAAt(880, wideY)
	wideFirst := -1
	for x := 0; x < 900; x++ {
		if wideImg.RGBAAt(x, wideY) == wideEdge {
			wideFirst = x
			break
		}
	}
	if wideFirst < 0 {
		t.Fatalf("the field's fill (%v) is nowhere in the 900-wide window", wideEdge)
	}
	if absi(wideFirst-wantLeft) > 1 {
		t.Errorf("a 900-wide window leaves %d px of blank beside the card (content starts at x=%d, want %d)",
			wideFirst, wideFirst, wantLeft)
	}
}

// The action bar closes the list, and it wears the selected face when it holds
// the selection.
//
// The reference draws `.launcher-action-bar` as a sibling *below* the scroller
// (`main:src/launcher/LauncherResults.tsx:1043-1073`), so the field's own
// hand-off is the panel's last band, never its first row — the user's
// 「现在固定在第一项」. Its selected face is `--glass-raised-quiet` plus the
// raised pane's shadow (`launcher.css:1579-1583`), which the launcher's own row
// code paints only when the *index* is the selection; the shell's frame is what
// puts the choice there (`defaultsToShell` on a command line), so this is the
// only place the user's two reports can be read together.
func TestTheTerminalRowIsLastAndWearsTheSelection(t *testing.T) {
	a := newApp(t)
	tt := ui.NewTester(a.View, launcher.InputWindowWidth, 560)
	tt.Frame()
	// A command line: the catalogue matches nothing that can run, so Enter
	// belongs to the bar (`shouldDefaultToActionBar`'s shell arm).
	tt.Type("ls")
	tt.Frame()

	results := a.Launcher.Results()
	if len(results) == 0 || !strings.HasPrefix(results[len(results)-1].ID, "bar:") {
		t.Fatalf("the terminal row is not the list's last: %v", resultIDs(results))
	}
	if strings.HasPrefix(results[0].ID, "bar:") {
		t.Fatalf("the terminal row heads the list: %v", resultIDs(results))
	}
	if a.Launcher.Selected != len(results)-1 {
		t.Errorf("the selection is on row %d, want the terminal row %d (%v)",
			a.Launcher.Selected, len(results)-1, resultIDs(results))
	}

	// The selected face is painted: the bar's band differs from the panel fill
	// one step to its left, on the bar's own row.
	rect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the terminal row was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	if got, want := img.RGBAAt(40, y), img.RGBAAt(3, y-80); got == want {
		t.Errorf("the terminal row wears no selected face: %v in its band, %v on the panel", got, want)
	}
	// It is the last band drawn: nothing but its own leftover sits under it.
	if gap := 560 - int(rect.Y+rect.H); gap >= int(rect.H) {
		t.Errorf("the terminal row is not the panel's last band: %d px below it", gap)
	}

	// Releasing nothing, it stays: this row is the field's, not the modifier's.
	// Clearing the field takes it away with the query, which is what makes it
	// "on the way out" rather than a fixed item.
	for range 2 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()
	for _, item := range a.Launcher.Results() {
		if strings.HasPrefix(item.ID, "bar:") {
			t.Errorf("an empty field still draws the terminal row: %v", resultIDs(a.Launcher.Results()))
			break
		}
	}
}

// resultIDs is the list's ids, for a failure message.
func resultIDs(items []launcher.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
