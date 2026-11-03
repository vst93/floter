package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// P13-fh（`docs/liquid-glass.md` §6.129）：结果格最长的那句话，量在它**真正会被画的
// 那个宽度**上——**先量**，量出为放得下，于是这一轮不加换行、不加堆叠、不缩字号。
//
// 起点是 P13-fg 留下的下一格：`Succeeded with warnings`（en，最长的一个答案词）与
// `metadataCell` 里那句 `SingleLine()` 的边界。实测（720u 设置面板，见 §6.129 的表）：
//
//	宽 1600u（自然宽）：墨 129 列
//	宽  860u：格子 313.5u，墨 129 列 —— 整句
//	宽  720u：格子 243.5u，墨 129 列 —— 整句，余 114u
//	宽  560u：格子 163.5u，墨 129 列 —— **被切**
//	宽  460u：格子 113.5u，墨  98 列 —— **被切**
//
// 560/460 是**不可达**的：设置面板是固定面板（`shell.go:4743-4745` 的
// `targetSize` 对 `SurfaceSettings` 一律返回 `launcher.InputWindowWidth`，
// 非终端面不开 resize），宽度只有一个值；而界面语言只有 en / zh 两种
// （`i18n.For` 的 switch），zh 的那句 `成功但有警告` 更短。
//
// 所以这条 pin 钉的不是「加一层换行规则」（那会为一件永远不发生的事给整块
// 多加一行、多一层高度），而是**这个余量本身**：以后谁把句子写长、把字号提一档、
// 把面板收窄，这里会红，并告诉他是哪一格先撑不住。宽度取 `launcher.InputWindowWidth`
// 而不是字面 720，是为了让这条尺子跟着那个常量走。

// freshnessAnswerGrid renders the integration drawer with one result state at
// one width, in one language and one appearance.
func freshnessAnswerGrid(t *testing.T, lang string, dark bool, width int,
	result extensions.FreshnessResult, count int) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	freshness := extensions.Freshness{
		AtSeconds: 1700000000, Source: extensions.FreshnessProbe,
		Result: result, CommandCount: &count, Delta: extensions.DeltaUnchanged,
	}
	a := New(store, Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			return freshness, id == "dev.floter.tool"
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
		}}
	}
	tt, _ := renderIn(t, a, width, 900, dark)
	a.selectPage(PageIntegrations)
	tt.Frame()
	tt.Frame()
	return tt
}

// TestTheFreshnessAnswerFitsTheCellItIsDrawnIn pins the Result cell's answer
// against the width the settings surface is actually drawn at: every one of the
// five words, in both languages, at the panel's one width, runs to its natural
// end rather than stopping at an ellipsis.
func TestTheFreshnessAnswerFitsTheCellItIsDrawnIn(t *testing.T) {
	count := 6
	states := []struct {
		name   string
		result extensions.FreshnessResult
	}{
		{"success", extensions.FreshnessSuccess},
		{"degraded", extensions.FreshnessDegraded},
		{"failed", extensions.FreshnessFailed},
		{"running", extensions.FreshnessRunning},
		{"unknown", extensions.FreshnessUnknown},
	}
	for _, lang := range []string{"en", "zh"} {
		copy := i18n.For(lang).Settings
		words := map[extensions.FreshnessResult]string{
			extensions.FreshnessSuccess:  copy.FreshnessResultSuccess,
			extensions.FreshnessDegraded: copy.FreshnessResultDegraded,
			extensions.FreshnessFailed:   copy.FreshnessResultFailed,
			extensions.FreshnessRunning:  copy.FreshnessResultRunning,
			extensions.FreshnessUnknown:  copy.FreshnessResultUnknown,
		}
		for _, dark := range []bool{false, true} {
			for _, state := range states {
				word := words[state.result]
				// The word's own width, measured where nothing can clip it.
				natural := lineSpan(t,
					freshnessAnswerGrid(t, lang, dark, 1600, state.result, count), word)
				tt := freshnessAnswerGrid(t, lang, dark, launcher.InputWindowWidth, state.result, count)
				box, ok := tt.Find(word)
				if !ok {
					t.Fatalf("[%s dark=%v] the %s word %q is missing: %q",
						lang, dark, state.name, word, tt.Texts())
				}
				if box.W < natural {
					t.Errorf("[%s dark=%v] the %s cell is %.1fu wide but its answer %q needs %.1fu: "+
						"the settings panel is a fixed %du and this is the width it is drawn at",
						lang, dark, state.name, box.W, word, natural, launcher.InputWindowWidth)
					continue
				}
				cols := lineInk(tt.Image(), box)
				if len(cols) == 0 {
					t.Fatalf("[%s dark=%v] the %s word paints nothing: %q",
						lang, dark, state.name, tt.Texts())
				}
				span := float32(cols[len(cols)-1].X) - float32(cols[0].X) + 1
				if span != natural {
					t.Errorf("[%s dark=%v] the %s word %q paints %.0fu of its natural %.0fu in a "+
						"%.1fu cell: the answer is being cut where it is actually drawn",
						lang, dark, state.name, word, span, natural, box.W)
				}
			}
		}
	}
}

// TestTheFreshnessGridGivesItsAnswerRoom: the two cells are the reference's
// 1fr/1fr pair, so the answer cell is exactly half the grid minus the gap. This
// pins the *margin* the fit above relies on: the longest answer must keep at
// least a quarter of its cell free, so a longer translation or one step up the
// type ladder is caught here rather than in a screenshot.
func TestTheFreshnessGridGivesItsAnswerRoom(t *testing.T) {
	count := 6
	en := i18n.For("en").Settings
	word := en.FreshnessResultDegraded
	natural := lineSpan(t, freshnessAnswerGrid(t, "en", false, 1600, extensions.FreshnessDegraded, count), word)
	for _, dark := range []bool{false, true} {
		tt := freshnessAnswerGrid(t, "en", dark, launcher.InputWindowWidth, extensions.FreshnessDegraded, count)
		box, ok := tt.Find(word)
		if !ok {
			t.Fatalf("[dark=%v] the longest answer is missing: %q", dark, tt.Texts())
		}
		slack := box.W - natural
		if slack < natural/4 {
			t.Errorf("[dark=%v] %q leaves only %.1fu of its %.1fu cell free (needs a quarter "+
				"of the word's %.0fu): the next longer sentence would be chopped at the "+
				"panel's only width", dark, word, slack, box.W, natural)
		}
		// The label and the value are one cell: the label is on the line above,
		// left-aligned with the value it names.
		label, lok := tt.Find(en.FreshnessResult)
		if !lok {
			t.Fatalf("[dark=%v] the Result label is missing: %q", dark, tt.Texts())
		}
		if label.X != box.X {
			t.Errorf("[dark=%v] the label starts at x=%.1f but its value at x=%.1f",
				dark, label.X, box.X)
		}
		if box.Y < label.Y+label.H-1 {
			t.Errorf("[dark=%v] the value at y=%.1f overlaps the label ending at y=%.1f",
				dark, box.Y, label.Y+label.H)
		}
	}
}
