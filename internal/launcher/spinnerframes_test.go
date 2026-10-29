package launcher

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// pagedApp is the launcher a page in flight leaves on screen: the first page of
// a self-paging list has landed, and the fetch for the next one never answers,
// so the footer keeps its loading row — and its ring — for as long as the test
// looks at it.
func pagedApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "paged", Name: "Paged"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{
			Success: true,
			Stdout:  `{"output":` + listOutput(12) + `,"page":{"cursor":"c1","hasMore":true}}`,
		}, nil)
	}
	a.Actions.LoadOutputPage = func(extensions.CommandEntry, []string, string, func(extensions.CapturedRun, error)) {}
	return a
}

// footerRingTurn reads the paging ring off a rendered frame: where its leading
// quarter points (degrees, clockwise from the ring's centre) and how much ink
// the ring drew at all. The centre is the ring's own ink box, which the whole
// circle fixes whatever the quarter's angle; the quarter is the darkest ink in
// it, so the ink's centre of gravity leans its way.
func footerRingTurn(tt *ui.Tester, label ui.Rect) (float32, int) {
	img := tt.Image()
	mid := int(label.Y + label.H/2)
	x0, x1 := int(label.X)-26, int(label.X)-2
	y0, y1 := mid-8, mid+8
	// The row's own surface, sampled left of the ring's slot.
	i := mid*img.Stride + (x0-6)*4
	bg := [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
	var sumX, sumY, weight float64
	ink := 0
	minX, maxX, minY, maxY := x1, x0, y1, y0
	for y := y0; y <= y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			j := y*img.Stride + x*4
			d := absInt(bg[0]-int(img.Pix[j])) + absInt(bg[1]-int(img.Pix[j+1])) + absInt(bg[2]-int(img.Pix[j+2]))
			if d == 0 {
				continue
			}
			ink++
			sumX += float64(x) * float64(d)
			sumY += float64(y) * float64(d)
			weight += float64(d)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if ink == 0 || weight == 0 {
		return 0, ink
	}
	cx, cy := float64(minX+maxX)/2, float64(minY+maxY)/2
	deg := math.Atan2(sumY/weight-cy, sumX/weight-cx) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return float32(deg), ink
}

// swung reports how far a and b are apart on the circle, in degrees.
func swung(a, b float32) float32 {
	d := float32(math.Mod(float64(a-b)+720, 360))
	if d > 180 {
		d = 360 - d
	}
	return d
}

// The paging ring turns while the frame paints, and asks for the frames that
// paint the view again rather than for builds of it (P11-bp). Two things are
// pinned: over a whole turn the view is built not once — before this round
// every refresh of the ring rebuilt the whole launcher — and the quarter's
// angle is read while painting, so a quarter of the period moves it a quarter
// of the circle.
func TestTheFootersRingTurnsWhilePaintingInsteadOfRebuilding(t *testing.T) {
	a := pagedApp(t)
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		a.View(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	a.Query = "paged"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	a.loadMoreOutputPage()
	tt.Frame()

	word := a.Copy().OutputLoading
	if !tt.HasText(word) {
		t.Fatalf("the loading footer is missing: %v", tt.Texts())
	}
	label, ok := tt.Find(word)
	if !ok {
		t.Fatalf("the loading word has no box")
	}
	built := builds
	start, ink := footerRingTurn(tt, label)
	if ink == 0 {
		t.Fatalf("the ring drew no ink beside %q", word)
	}

	tt.Advance(footerSpinnerPerTurn)
	if grew := builds - built; grew != 0 {
		t.Fatalf("the paging ring built the view %d times over one turn: "+
			"the frames are builds, not paints", grew)
	}
	if !tt.FramePending() {
		t.Fatalf("the paging ring asked for no frame at all: nothing is driving it")
	}

	// A quarter of the period: the quarter's ink points a quarter of the way
	// round. The band is wide enough for a frame's worth of lag and for a
	// centre of gravity that is not exactly the quarter's middle, and narrow
	// enough that a ring standing still (or drawn at a fixed angle) fails.
	tt.Advance(footerSpinnerPerTurn / 4)
	later, inkLater := footerRingTurn(tt, label)
	if inkLater == 0 {
		t.Fatalf("the ring drew no ink at the quarter mark")
	}
	if turned := swung(start, later); turned < 45 || turned > 135 {
		t.Errorf("the ring's quarter went from %v° to %v° over a quarter of its period "+
			"(%v° of the circle): the angle is not read while painting", start, later, turned)
	} else {
		t.Logf("the quarter went from %v° to %v° over a quarter of its period (%v° of the circle)", start, later, turned)
	}
}

// The same ring on a desktop that asked for less motion: it stands at the 0%
// keyframe (the old build's `animation: none`) and asks for no frame at all —
// the frames the preference is about are not asked for and then ignored.
func TestTheFootersRingRestsUnderReduceMotion(t *testing.T) {
	a := pagedApp(t)
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		a.View(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	a.Query = "paged"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	a.loadMoreOutputPage()
	tt.Frame()

	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	word := a.Copy().OutputLoading
	label, ok := tt.Find(word)
	if !ok {
		t.Fatalf("the loading word has no box: %v", tt.Texts())
	}
	rest, ink := footerRingTurn(tt, label)
	if ink == 0 {
		t.Fatalf("the ring drew no ink under reduce motion")
	}
	built := builds
	// The frame the preferences changed in is spent; nothing after it may ask.
	tt.Advance(time.Millisecond)
	tt.Frame()
	built = builds
	tt.Advance(footerSpinnerPerTurn)
	if grew := builds - built; grew != 0 {
		t.Errorf("the resting ring built the view %d times", grew)
	}
	if tt.FramePending() {
		t.Errorf("the resting ring is still asking for frames")
	}
	still, inkLater := footerRingTurn(tt, label)
	if inkLater != ink || swung(rest, still) > 1 {
		t.Errorf("the resting ring moved: ink %d → %d, quarter %v° → %v°", ink, inkLater, rest, still)
	}
}

// TestDumpTheFooterSpinnerPNGs writes this round's visual evidence: the loading
// footer's ring at two angles, in both appearances. Opt-in, as every dump here
// is.
func TestDumpTheFooterSpinnerPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := pagedApp(t)
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := renderIn(t, a, dark)
		a.Query = "paged"
		tt.Frame()
		tt.TypeKey(0, ui.KeyEnter, "")
		tt.Frame()
		a.loadMoreOutputPage()
		tt.Frame()
		for _, frame := range []struct {
			name  string
			after time.Duration
		}{
			{"rest", 0},
			{"turn", footerSpinnerPerTurn / 4},
		} {
			if frame.after > 0 {
				tt.Advance(frame.after)
			}
			path := filepath.Join("/tmp", "floter-P11-bp-footer-"+appearance+"-"+frame.name+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s %s: %s", appearance, frame.name, path)
		}
	}
}
