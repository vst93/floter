package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// barTitle is a long session name, so the reference's body step and the one
// below it land far enough apart to read off the ink.
const barTitle = "the quick brown fox session name"

// barFrame renders just the terminal bar over a black root with the pointer
// resting on it, so the identity line is revealed: the title, and the status
// dot on its left. The app's own theme is set on the context as the shell sets
// it, so `c.Theme()` and the tokens agree — the bar mixes both.
func barFrame(t *testing.T, a *App, reduceMotion bool) (*ui.Tester, theme.Tokens, float32) {
	t.Helper()
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	var tok theme.Tokens
	var body float32
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		body = c.Theme().FontSize
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.titleBar(c, copy)
	}, 900, 40)
	tt.SetPreferences(ui.Preferences{ReduceMotion: reduceMotion})
	tt.Move(200, 14) // over the bar: the identity row is revealed
	tt.Frame()
	return tt, tok, body
}

// barInkBox returns the region's ink box and mass.
func barInkBox(tt *ui.Tester, x0, x1, y1 int) (minX, maxX, mass int) {
	img := tt.Image()
	minX, maxX, mass = 1<<30, -1, 0
	for y := 0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			if r > 20 || g > 20 || b > 20 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
			mass += r + g + b
		}
	}
	return minX, maxX, mass
}

// sample reads one pixel of the last frame, at a logical coordinate.
func sample(tt *ui.Tester, x, y int) ui.Color {
	img := tt.Image()
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// refTitle renders the same words at one size and colour over the same black
// root, and returns the ink's width and mass — the two rulers a register and a
// colour are read against.
func refTitle(tok theme.Tokens, size float32, col ui.Color) (int, int) {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Text(c, barTitle).FontSize(size).FontWeight(580).TextColor(col)
	}, 900, 40)
	tt.Frame()
	minX, maxX, mass := barInkBox(tt, 0, 900, 40)
	return maxX - minX + 1, mass
}

// flatColor is what a solid fill of col at the given opacity paints over the
// black root: the composite a dot's centre pixel can be held against exactly.
func flatColor(tok theme.Tokens, col ui.Color, opacity float32) ui.Color {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Box(c).Size(40, 40).Background(col).Opacity(opacity)
	}, 60, 60)
	tt.Frame()
	return sample(tt, 20, 20)
}

// `.terminal-bar__title` (`src/styles/terminal.css:150-161`) is the body step in
// `--text-secondary`, not the muted step one size down the old port drew (and
// not the muted step at all). The bar's identity is the name of the session on
// screen, so it is read off the live bar: its ink's width against the body and
// body−1 references, its ink's mass against `--text-secondary` and
// `--text-muted` — the two registers the reference names.
func TestTheTerminalBarTitleIsTheReferenceSecondaryBody(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	tt, tok, body := barFrame(t, a, true)

	// Skip the status dot: it starts on the bar's 3u inset and is 7u wide, so
	// the title's own ink starts past 23.
	minX, maxX, mass := barInkBox(tt, 23, 900, 40)
	if maxX < 0 {
		t.Fatalf("the bar drew no title ink: %q", tt.Texts())
	}
	gotW := maxX - minX + 1

	bodyW, _ := refTitle(tok, body, tok.TextSecondary)
	smallW, _ := refTitle(tok, body-1, tok.Theme.TextMuted)
	if absFloat32(float32(gotW-bodyW)) > absFloat32(float32(gotW-smallW)) {
		t.Errorf("the bar title is not the body step (ink %d, body %d, body−1 %d)", gotW, bodyW, smallW)
	}

	_, secondary := refTitle(tok, body, tok.TextSecondary)
	_, muted := refTitle(tok, body, tok.Theme.TextMuted)
	if absInt(mass-secondary) > absInt(mass-muted) {
		t.Errorf("the bar title is not --text-secondary (mass %d, secondary %d, muted %d)", mass, secondary, muted)
	}
}

// The bar's status dot (`src/styles/terminal.css:150-168`): the live one is the
// accent, and once the program ends it takes the *tertiary* step, not the muted
// one the old port read (`--text-tertiary`). Read off the live bar: the dot's
// centre pixel against the exact composite of a solid fill, and its width
// against the same dot on a held session.
//
// P8-c 重定基线（§6.2 第 5 条 / §1）：旧测试的名字与第三条断言是「live dot
// **有** halo」——`--elev-halo` 的 `0 0 0 2px var(--accent-halo)` 把 1.75u 的
// 点撑成了一圈 2u 的红晕。合约把光泽压到 alpha ≤ 0.04，而 `AccentHalo` 是
// 0.2（上限的 5 倍），所以 halo 整圈去掉，让那 1.75u 的点自己说话。
//
// 旧值 → 新值：`dot.Shadow(..., AccentHalo)` 的 2u 环 → 无环。断言从
// 「live 比 held 宽 ≥3px」翻成「live 与 held 同宽，且点周围那圈像素就是笔杆
// 自己的底色」——旧条文的反证。
func TestTheTerminalBarDotDropsItsHaloAndWearsTertiary(t *testing.T) {
	// The held session's dot is still: no halo, and the tertiary step.
	held := New(newStore(), Actions{}, nil)
	withSession(held, &terminal.Terminal{})
	held.Current().Resident, held.Current().ExitCode = true, 0
	ht, tok, _ := barFrame(t, held, true)
	hMinX, hMaxX, _ := barInkBox(ht, 0, 22, 40)
	if hMaxX < 0 {
		t.Fatalf("the held session's bar drew no status dot: %q", ht.Texts())
	}
	hInk := dotCenter(ht, hMinX, hMaxX)
	if got, want := hInk, flatColor(tok, tok.TextTertiary, 0.7); !sameRGB(got, want) {
		t.Errorf("the exited dot's centre = %v, want --text-tertiary at 0.7 (%v)", got, want)
	}
	if muted := flatColor(tok, tok.Theme.TextMuted, 0.7); sameRGB(hInk, muted) {
		t.Errorf("the exited dot still reads the muted step, not --text-tertiary")
	}

	// The live dot, at rest: reduce motion pins the breath at its 0% keyframe,
	// so the ink is the full-opacity accent and its ring.
	live := New(newStore(), Actions{}, nil)
	withSession(live, &terminal.Terminal{})
	lt, tok, _ := barFrame(t, live, true)
	lMinX, lMaxX, _ := barInkBox(lt, 0, 22, 40)
	if lMaxX < 0 {
		t.Fatalf("the live session's bar drew no status dot: %q", lt.Texts())
	}
	if got, want := dotCenter(lt, lMinX, lMaxX), tok.Theme.Accent; !sameRGB(got, want) {
		t.Errorf("the live dot's centre = %v, want the accent (%v)", got, want)
	}
	// No halo: the ink box is the bare 1.75u dot, the same width a held
	// session's dot has. A 2u ring added three columns on each side.
	if liveW, heldW := lMaxX-lMinX+1, hMaxX-hMinX+1; liveW > heldW+1 {
		t.Errorf("the live dot is %d wide and the bare dot %d: the dropped 2u halo is back", liveW, heldW)
	}
	// And the pixels just outside the dot are the bar's own page: a halo is a
	// *ring*, so the first column past the ink has to be the backdrop again.
	// The bar itself is transparent, so the row reads as the root's black.
	midY := (dotTop(lt, (lMinX+lMaxX)/2) + dotBottom(lt, (lMinX+lMaxX)/2)) / 2
	for _, x := range []int{lMinX - 2, lMaxX + 2} {
		if p := sample(lt, x, midY); channelDelta(p, ui.RGBA(0, 0, 0, 1)) > 6 {
			t.Errorf("the pixel %d right of the live dot is %v, not the bar's own backdrop: the halo is back", x, p)
		}
	}
}

// dotTop and dotBottom are the first and last rows of a dot's own ink in the
// bar, at one column: the midline a halo check samples beside.
func dotTop(tt *ui.Tester, cx int) int {
	img := tt.Image()
	for y := 0; y < 40; y++ {
		i := y*img.Stride + cx*4
		if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) > 20 {
			return y
		}
	}
	return 0
}

func dotBottom(tt *ui.Tester, cx int) int {
	img := tt.Image()
	for y := 39; y >= 0; y-- {
		i := y*img.Stride + cx*4
		if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) > 20 {
			return y
		}
	}
	return 0
}

// channelDelta is the largest per-channel difference between two colours: the
// same ruler `internal/settingsui`'s pixel pins use.
func channelDelta(a, b ui.Color) int {
	d := absInt(int(a.R) - int(b.R))
	if x := absInt(int(a.G) - int(b.G)); x > d {
		d = x
	}
	if x := absInt(int(a.B) - int(b.B)); x > d {
		d = x
	}
	return d
}

// dotCenter is the middle pixel of a dot's ink box in the bar: inside the
// circle, clear of the antialiased rim.
func dotCenter(tt *ui.Tester, minX, maxX int) ui.Color {
	cx := (minX + maxX) / 2
	img := tt.Image()
	top, bottom := -1, -1
	for y := 0; y < 40; y++ {
		i := y*img.Stride + cx*4
		if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) > 20 {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	return sample(tt, cx, (top+bottom)/2)
}

// sameRGB compares two colours by the pixels the frame actually shows.
func sameRGB(a, b ui.Color) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
