package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/drops"
	"floter/internal/settings"
)

// P11-bh · **「还没有拖入文件」不等于「没有匹配项」。**
//
// The files page (`files` / `file` / `drop` / `drops` / `文件`) is the one page a
// user can open without having done the thing that fills it: entering the mode
// with no drop on the hand-over list. That page said `NoResults` — "No results"
// / "没有匹配项" — which reads as a query that came back empty. Nothing was
// asked. The clipboard page (`ClipboardEmpty`), the calculator page
// (`CalculatorEmpty`) and the browser page (`BrowserEmpty` / `BrowserDisabled`
// / `BrowserNoProfile`) each name their own reason; this page was the one that
// borrowed the ordinary search's sentence.
//
// A typed needle that matches none of the dropped files *is* a failed query and
// keeps `NoResults`: the two states are different, so the page has two
// sentences.
func TestTheFilesPageSaysWhyItIsEmpty(t *testing.T) {
	sentences := map[string][2]string{
		// lang: empty page, failed query
		"en": {"No files dropped yet", "No results"},
		"zh": {"还没有拖入文件", "没有匹配项"},
	}
	for _, lang := range []string{"en", "zh"} {
		want := sentences[lang]
		// The page with nothing dropped on it.
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
			t.Fatal(err)
		}
		a.EnterFiles()
		tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), false)
		if !tt.HasText(want[0]) {
			t.Fatalf("%s: the empty files page does not say %q; it drew %q", lang, want[0], tt.Texts())
		}
		if tt.HasText(want[1]) {
			t.Errorf("%s: the empty files page still borrows the failed query's sentence %q", lang, want[1])
		}

		// The same page with a drop, and a needle that matches none of it.
		b := testApp()
		if err := b.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
			t.Fatal(err)
		}
		b.SetDropped([]drops.File{{
			Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp",
		}})
		b.EnterFiles()
		b.Query = "zzzz"
		bt := renderAt(t, b, InputWindowWidth, frameHeight(maxResultSlots), false)
		if !bt.HasText(want[1]) {
			t.Fatalf("%s: a needle that matches no dropped file does not say %q; it drew %q",
				lang, want[1], bt.Texts())
		}
		if bt.HasText(want[0]) {
			t.Errorf("%s: a failed query on a page that has drops still says %q", lang, want[0])
		}
	}
}

// TestDumpP11BHFilesPNGs is the round's visual evidence: the empty files page
// and the failed-query page in both appearances. Opt-in, as the other dumps are.
func TestDumpP11BHFilesPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a.EnterFiles()
		tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), dark)
		img := tt.Image()
		if img == nil {
			t.Fatalf("%s: no frame", appearance)
		}
		path := filepath.Join("/tmp", "floter-P11-bh-files-"+appearance+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
