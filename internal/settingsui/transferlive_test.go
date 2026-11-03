package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-ff · 集成页的导出/导入行：在飞那一句话，比它自己按灭的两枚胶囊还安静
// （`docs/liquid-glass.md` §6.127）。
//
// 导出/导入在飞时，行里两枚胶囊一起熄灯（P13-aq 的禁用面：熄冠线、墨落一档，
// 实测权重 105），而「哪一个在跑」只剩行尾那一句 `Exporting…` / `Importing…`。
// 这句话此前穿 `t.TextMuted`——与页面上**永远不变**的帮助句同一档墨，也与它自己
// 按灭的那两枚胶囊几乎同档。实测（生产取景 720×1600，深浅两套，量的是这一行的
// 卡面）：
//
//	        在飞句权重   禁用胶囊权重
//	浅色        107            105
//	深色        104            105      ← 唯一说明「有活在跑」的那句话，是整行最安静的字
//
// 本仓对「在飞的词」已经有一条规矩：启动器的翻页页脚（`launcher/output.go` 的
// `footerWord`）在飞 `TextSecondary` + 500、静止 `TextMuted` + 无字重。设置页的
// 配置工具条（`config.go`，P13-el）走的是另一条——在飞与落地同档——因为它有一枚
// 落地后的孪生词；这一句没有孪生词（结束就整句消失），所以走第一条。
//
// P13-ff 重定基线：`TextMuted`（无字重）→ `TextSecondary` + 500。字号仍是
// `tokens.Caption`，盒、位置、行内中线一个字节不动。实测：
//
//	浅色 107 → 130，深色 104 → 127（对同一行的禁用胶囊 105：+25 / +22）。

// transferBusyProbe is one frame of the real Integrations page with a transfer
// in flight: the row's busy word, the two pills it locked, and the card face
// they all sit on.
type transferBusyProbe struct {
	tt   *ui.Tester
	face int
	word ui.Rect
	exp  ui.Rect
	imp  ui.Rect
	hint ui.Rect
}

func transferBusyProbeFor(t *testing.T, dark bool) transferBusyProbe {
	t.Helper()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	en := i18n.For("en").Settings
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{
		ExportIntegrations: func() {},
		ImportIntegrations: func() {},
	})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (%v)", err, tt.Texts())
	}
	tt.Frame()
	a.TransferBusy = "export"
	tt.Frame()

	p := transferBusyProbe{tt: tt}
	p.word = mustFind(t, tt, en.IntegrationsExporting)
	p.exp = mustFind(t, tt, en.IntegrationsExport)
	p.imp = mustFind(t, tt, en.IntegrationsImport)
	p.hint = mustFind(t, tt, en.IntegrationsHint)
	// The card's own face: the strip between the import pill and the word.
	p.face = pageLuma(tt, p.word.X-6, p.word.Y+p.word.H/2)
	return p
}

// transferBusyInk is how heavy a line reads on the card: the distance between
// the line's own ink and the face under it, whichever way round the appearance
// runs (the peak pixel inside the line's own box).
func transferBusyInk(tt *ui.Tester, face int, box ui.Rect) int {
	img := tt.Image()
	worst := 0
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			d := l - face
			if d < 0 {
				d = -d
			}
			if d > worst {
				worst = d
			}
		}
	}
	return worst
}

// The first pin: the one line that says a transfer is running must outrank the
// two pills that just went dead — it is the only signal left in the row.
func TestTheTransferBusyLineOutranksTheButtonsItKilled(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := transferBusyProbeFor(t, dark)
		word := transferBusyInk(p.tt, p.face, p.word)
		pill := transferBusyInk(p.tt, p.face, p.exp)
		importPill := transferBusyInk(p.tt, p.face, p.imp)
		t.Logf("%s busy=%d exportPill=%d importPill=%d (face %d)", appearanceOf(dark), word, pill, importPill, p.face)
		if pill != importPill {
			t.Fatalf("%s the two locked pills read differently: %d vs %d", appearanceOf(dark), pill, importPill)
		}
		// The measured gap is 25 (light) and 22 (dark); before this round it was
		// 2 and -1. Fifteen levels is the floor that still means "one rung
		// louder", not "two rasterized pixels apart".
		if word-pill < 15 {
			t.Errorf("%s the busy word does not outrank the pills it locked: word=%d pill=%d",
				appearanceOf(dark), word, pill)
		}
	}
}

// The second pin: this round changed ink only. The word keeps the caption's
// own line box (the page's hint line is the same step) and stays centred on the
// pills' midline — it is still one row, and the row did not grow.
func TestTheTransferBusyLineKeepsTheCaptionBoxAndTheMidline(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := transferBusyProbeFor(t, dark)
		if p.word.H != p.hint.H {
			t.Errorf("%s the busy word left the caption step: box %v, the page's hint line %v",
				appearanceOf(dark), p.word, p.hint)
		}
		if got, want := p.word.Y+p.word.H/2, p.imp.Y+p.imp.H/2; got != want {
			t.Errorf("%s the busy word left the row's midline: mid %v, pills' mid %v",
				appearanceOf(dark), got, want)
		}
		if p.word.X <= p.imp.X+p.imp.W {
			t.Errorf("%s the busy word moved into the pills: word x=%v, import pill ends at %v",
				appearanceOf(dark), p.word.X, p.imp.X+p.imp.W)
		}
	}
}

// TestDumpP13FFTransferBusyPNGs is the round's own frame: the transfer row in
// both appearances, with the two pills, the word and a strip of card around
// them.
func TestDumpP13FFTransferBusyPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13FF_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13FF_DUMP=1 writes floter-P13-ff-transfer-<appearance>.png")
	}
	dir := os.Getenv("FLOTER_P13FF_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	tag := os.Getenv("FLOTER_P13FF_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		p := transferBusyProbeFor(t, dark)
		box := ui.Rect{X: p.exp.X - 18, Y: p.exp.Y - 18, W: 360, H: p.exp.H + 36}
		img := p.tt.Image()
		r := image.Rect(int(box.X), int(box.Y), int(box.X+box.W), int(box.Y+box.H))
		crop := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c := imagePixel(img, x, y)
				i := crop.PixOffset(x-r.Min.X, y-r.Min.Y)
				crop.Pix[i], crop.Pix[i+1], crop.Pix[i+2], crop.Pix[i+3] = c.R, c.G, c.B, c.A
			}
		}
		name := filepath.Join(out, "floter-P13-ff-transfer-"+appearanceOf(dark)+".png")
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
