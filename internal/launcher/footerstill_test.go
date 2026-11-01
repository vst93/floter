package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-ck（`docs/liquid-glass.md` §6.54）：分页脚注的两种状态此前是「环 + 词」与
// 「词」两套内容，而这一行是**居中**布局——拉取结束的那一帧，词横移 7.5px（实测
// 浅 331 → 323.5，深同值），环整个消失，一行里没有任何东西留在原地。这一组 pin
// 量页面画的像素（`ui.Tester` + app 主题，与 `shell.View` 同一条生产路径）。
//
// 现在轨道两档都画：一圈合上 = 到底了，一圈在转 = 还在拉。词的位置因此逐像素相同，
// 而「到底了」那一档只剩 hairline 轨道，比它自己的词轻一大截。
const (
	// footerWordDrift is how far the word may move between the two states.
	// It is **not** zero and cannot be: the row is centred and the two words
	// are 3px apart in width, so half of that (1.5px) is the layout's own
	// answer. What the ring's box buys is the other 6px: before P13-ck the word
	// slid 7.5px, i.e. half the width difference plus the whole 12u ring and
	// its 2u gap. The ceiling is the width difference alone — a row that loses
	// its ring again lands back at 7.5 and this arm goes red.
	footerWordDrift = 2.0
	// footerStillRingFloor is the settled ring's minimum ink over the plane:
	// the hairline track has to be on the page at all (a closed circle), or
	// the word is the only thing holding the row and it still reads as a jump.
	footerStillRingFloor = 12
	// footerStillRingCeiling is how much *less* than its word the settled ring
	// may peak: it is the track only — no leading quarter, which would make the
	// ended list look like it is still turning.
	footerStillRingGap = 12
)

// TestThePagingFooterHoldsItsWordAcrossStates pins the row's composition: the
// word sits at the same x whether the next page is in flight or the list has
// ended.
func TestThePagingFooterHoldsItsWordAcrossStates(t *testing.T) {
	for _, dark := range []bool{false, true} {
		live, liveApp := footerFrame(t, footerLoading, dark)
		liveWord := footerWordBox(t, live, liveApp, footerLoading)
		end, endApp := footerFrame(t, footerEnd, dark)
		endWord := footerWordBox(t, end, endApp, footerEnd)
		if drift := liveWord.X - endWord.X; drift > footerWordDrift || drift < -footerWordDrift {
			t.Errorf("dark=%v: the word moves %.1fpx between the live and the ended footer (x %.1f → %.1f) — the row is centred, so losing the ring slides the word",
				dark, drift, liveWord.X, endWord.X)
		}
	}
}

// TestThePagingFooterKeepsTheRingWhenTheListEnds pins the second half: the
// ring's slot is still drawn once the list has ended, and it is only the
// track — a closed hairline circle, quieter than its own word.
func TestThePagingFooterKeepsTheRingWhenTheListEnds(t *testing.T) {
	for _, dark := range []bool{false, true} {
		end, endApp := footerFrame(t, footerEnd, dark)
		word := footerWordBox(t, end, endApp, footerEnd)
		ring := ui.Rect{X: word.X - 30, Y: word.Y, W: 24, H: word.H}
		_, ringPeak := footerBand(end, ring)
		_, wordPeak := footerBand(end, word)
		if ringPeak < footerStillRingFloor {
			t.Errorf("dark=%v: the settled ring peaks at %d over the plane, want ≥ %d — the row has nothing at rest where the ring was",
				dark, ringPeak, footerStillRingFloor)
		}
		if wordPeak-ringPeak < footerStillRingGap {
			t.Errorf("dark=%v: the settled ring peaks at %d and its word at %d (gap %d, want ≥ %d) — the ring still wears the live quarter",
				dark, ringPeak, wordPeak, wordPeak-ringPeak, footerStillRingGap)
		}
	}
}

// The live state is unchanged by P13-ck: it still turns, and its quarter still
// shares the word's ink (P13-bj). This arm exists so the new "both states draw
// a ring" rule cannot quietly flatten the live one into a track.
func TestThePagingFooterStillTurnsWhileLoading(t *testing.T) {
	for _, dark := range []bool{false, true} {
		live, liveApp := footerFrame(t, footerLoading, dark)
		word := footerWordBox(t, live, liveApp, footerLoading)
		_, wordPeak := footerBand(live, word)
		ring := ui.Rect{X: word.X - 30, Y: word.Y, W: 24, H: word.H}
		_, ringPeak := footerBand(live, ring)
		if ringPeak < wordPeak-footerRingSlack {
			t.Errorf("dark=%v: the live ring peaks at %d, its word at %d — the quarter lost the live ink",
				dark, ringPeak, wordPeak)
		}
	}
}

// TestDumpPagingFooterStates writes the row's two states in both appearances to
// /tmp for the round's own visual check. Opt-in: the only test here that needs
// a file system and a name the report quotes.
func TestDumpPagingFooterStates(t *testing.T) {
	if os.Getenv("FLOTER_P13CK_DUMP") == "" {
		t.Skip("set FLOTER_P13CK_DUMP=1 to dump the paging footer's states")
	}
	tag := os.Getenv("FLOTER_P13CK_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, state := range []string{footerLoading, footerEnd} {
			tt, _ := footerFrame(t, state, dark)
			name := fmt.Sprintf("/tmp/floter-P13-ck-footer-%s-%s-%s.png", state, appearance, tag)
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
