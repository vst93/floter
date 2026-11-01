package launcher

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/motion"
	"floter/internal/theme"
)

// The captured-output view: what a command whose manifest sends its output to
// the background said, shown in the launcher rather than in a terminal.
//
// The old build's plugin text view had a three-line floor, a ten-row ceiling
// and scrolled inside it; this is the same shape, and the two keys are the
// old ones: Enter copies the text, Escape closes the view.

// OutputView is one captured run, ready to show: its rows when the output is
// the list protocol, its text otherwise.
type OutputView struct {
	// Title is the command line that ran.
	Title string
	// Text is what it wrote (standard output, with standard error appended
	// when there is any).
	Text string
	// Empty reports that the run wrote nothing at all, so `Text` is the
	// empty-state sentence rather than output. The distinction cannot be a
	// string comparison: a command that literally prints that sentence wrote
	// output, and it must stay output (selectable, copyable, in the body's
	// register). P13-cd: `Text` keeps the sentence so the height chain
	// (`pluginTextRows`) and the copy path (`PinText`) are unchanged; only the
	// drawing differs (`quietStatement`).
	Empty bool
	// Rows are the list protocol's rows, when the output is a list; a nil
	// slice with IsList false means the output is text.
	Rows []extensions.Row
	// IsList reports whether the output is a list (which may be empty).
	IsList bool
	// Selected is the chosen row in a list, -1 while none is.
	Selected int
	// Status is the one-line summary: the exit code, the duration, and
	// whether the output was cut or the run timed out.
	Status string
	// Verdict is the half of Status that says *how the run ended* — `Finished`,
	// `Exit code 3`, `Timed out` — with no facts appended. A surface that has
	// room for a ladder draws the verdict and the facts at different heights
	// (the plugin window's band, P13-bi / §6.28); the launcher's own strip still
	// draws the one joined sentence, because that is one line under a title.
	Verdict string
	// Facts is the other half: the duration and the truncation mark, in the
	// same order `Status` joins them. Empty when the run carried neither.
	Facts string
	// Failed reports whether this run did not succeed — a non-zero exit, a
	// timeout, or a run that never started. `Status` is the words; this is
	// the fact the strip needs to draw them differently (P13-bg).
	Failed bool
	// FontFamily is the family the text is drawn in: the terminal's own, so
	// a command's output reads the way it would in the terminal.
	FontFamily string
	// Spans are the text's runs with the styles its own ANSI sequences asked
	// for: a CLI writes colour and weight for a terminal, and the text form
	// reads them rather than printing the bytes (see internal/ansi). Text
	// holds the same characters with every sequence removed, for copying.
	Spans []ansi.Span
	// Entry is the command the output came from, when it came from one: the
	// window that can be opened for it re-runs the same command.
	Entry *extensions.CommandEntry
	// Args are the arguments the run was given, kept so a later page of a
	// self-paging list re-runs the same command.
	Args []string
	// AllRows is the full set of rows the emission carried, before any
	// in-memory windowing; Rows is the window actually drawn.
	AllRows []extensions.Row
	// Page is the list's pagination state: the plugin's own block when it
	// pages itself, the window's block when the launcher pages in memory, and
	// nil for a list that is complete in one emission.
	Page *extensions.Page
	// PagesLoaded is how many viewport pages the in-memory window holds.
	PagesLoaded int
	// Declared is true when the plugin pages itself: its cursor is replayed
	// rather than the window being grown.
	Declared bool
	// LoadingMore is true while a self-paging plugin's next page is in flight.
	LoadingMore bool
}

// Copy is the launcher's copy in the stored language, for a caller outside
// the launcher (the shell's notification wording).
func (a *App) Copy() i18n.Launcher { return a.copy() }

// RunStatus is the output view's one-line summary, for a caller that reports
// the same run elsewhere (a notification).
func (a *App) RunStatus(run extensions.CapturedRun, err error) string {
	copy := a.copy()
	if err != nil && !run.Success && !run.TimedOut {
		return a.failureReason(err)
	}
	return a.runStatus(copy, run)
}

// failureReason is a failed run's one-line reason. A keyed failure (a stable
// key plus a JSON payload) is localised; so is a `run_param_*` refusal, which
// carries the parameter id instead of a payload. Anything else keeps its own
// words.
func (a *App) failureReason(err error) string {
	if err == nil {
		return ""
	}
	copy := i18n.For(a.settings().Language).Settings
	if failure, ok := extensions.ParseRunFailure(err.Error()); ok {
		return runErrorMessage(copy, failure)
	}
	if failure, ok := extensions.ParseParamFailure(err.Error()); ok {
		return paramRunMessage(copy, failure)
	}
	return err.Error()
}

// showOutput puts a captured run's output in front of the user: as a list when
// the command printed the list protocol, as text otherwise.
func (a *App) showOutput(run extensions.CapturedRun, err error, entry *extensions.CommandEntry, args []string) {
	view := a.OutputFor(run, err, entry)
	view.Args = args
	// P11-ar：一份新的捕获输出从它自己的开头开始。`Output` 与 `OutputList` 是
	// App 级的滚动/列表状态——滚动容器需要它们跨帧保留偏移——但它们也是**跨视图**
	// 复用的：没有这一步，关掉一份滚到 Y=400 的长输出再跑另一份长输出，新页面就
	// 从第 ~15 行起步（本轮实测）。这与 P11-ao 是同一条规则（换设置页回顶）：一份
	// 新内容从它自己的开头开始。只在换上视图之前重置一次；逐帧重建的路径不动，
	// 否则滚轮每帧被归零。分页续页（`loadMoreOutputPage`）走的是同一份视图，不经过
	// 这里，所以不受影响。
	a.Output = ui.ScrollState{}
	a.OutputList = ui.ListState{}
	// P11-bl：行盒/视口盒也是跨视图复用的（它们是 App 级的，见 launcher.go 的
	// outputRowRects）。上一份输出的盒子若留着，新输出的第一帧就会按上一份的
	// 几何发号——与上面同一条规则：一份新内容从它自己的开头开始。
	clear(a.outputRowRects)
	a.outputListRect = ui.Rect{}
	// P13-x：分组头的盒子也属于上一份输出，跟着一起清。
	clear(a.outputHeadingRects)
	a.output = &view
	a.Selected, a.chosenRow = 0, -1
}

// OutputFor builds the view of one captured run: its text (or the list
// protocol's rows), its status, and the command it came from. The launcher
// shows it in its own band; a caller outside can show the same view in a
// window of its own (see the shell's detached window).
func (a *App) OutputFor(run extensions.CapturedRun, err error, entry *extensions.CommandEntry) OutputView {
	copy := a.copy()
	// The text is read as a terminal reads it: the styles it asked for are
	// kept as spans, and the characters without them are what a copy takes.
	raw := run.Text()
	verdict, facts := a.runStatusParts(copy, run)
	view := OutputView{
		Entry:      entry,
		Title:      strings.Join(run.Command, " "),
		Text:       ansi.Plain(raw),
		Spans:      ansi.Parse(raw),
		Status:     a.runStatus(copy, run),
		Verdict:    verdict,
		Facts:      facts,
		Failed:     run.TimedOut || !run.Success,
		FontFamily: a.settings().FontFamily,
		Selected:   -1,
	}
	// A list is read from standard output alone: standard error beside it is
	// a warning, not part of the protocol.
	if emission, ok := extensions.ParseEmission(run.Stdout); ok {
		view.Rows, view.IsList = emission.Rows, true
		view.AllRows = emission.Rows
		view.PagesLoaded = extensions.InitialPages
		view.Declared = emission.Page != nil
		view.Page = emission.Page
		if !view.Declared {
			// A plugin that hands over its whole list is windowed in memory,
			// one viewport page at a time: a complete list gets no block at all,
			// so a short list draws no footer and every row.
			rows, page := extensions.WindowRows(emission.Rows, view.PagesLoaded)
			view.Rows = rows
			if page.HasMore {
				view.Page = &page
			} else {
				view.Page = nil
			}
		}
		view.Selected = initialOutputSelection(view.Rows)
	}
	if !view.IsList {
		if err != nil && view.Text == "" {
			view.Text = a.failureReason(err)
		}
		if view.Text == "" {
			view.Text = copy.OutputEmpty
			view.Empty = true
		}
	}
	return view
}

// initialOutputSelection is where the list's cursor starts: the first row Enter
// could run, and — when the list has nothing to run at all — its first row.
//
// P13-cg（`docs/rounds/P13-cg-spec.md`）：这两件事曾经是同一件事（没有可运行行
// 就 `-1`），于是一份**全是便条**的列表（报告式插件：`kind:"status"` 的行）连
// 游标都没有——方向键转一圈原地不动、Enter 什么也不做，屏幕上也没有一行是「被
// 选中的」，读起来不是「这一页没有可运行的东西」，而是「这一页没画完」。
//
// 游标（用户在哪儿）与门（这一行能不能进）是两件事：便条被选中时戴**只读游标**
// ——面色到了、灯不点（§3 的受光边是「这块面可以被碰」的记号，见 `drawOutputRow`
// 的 `case selected`）。空列表仍然 `-1`：没有行，就没有游标。
func initialOutputSelection(rows []extensions.Row) int {
	for i, row := range rows {
		if row.Runnable() {
			return i
		}
	}
	if len(rows) == 0 {
		return -1
	}
	return 0
}

// loadMoreOutputPage asks for the next page of an output list: a self-paging
// plugin is re-run with the cursor from its last emission, and an in-memory
// window simply grows by one page. A complete list, or a fetch already in
// flight, is left alone.
func (a *App) loadMoreOutputPage() {
	view := a.output
	if view == nil || !view.IsList || view.Page == nil || !view.Page.HasMore || view.LoadingMore {
		return
	}
	if view.Declared {
		if view.Entry == nil || a.Actions.LoadOutputPage == nil {
			return
		}
		view.LoadingMore = true
		entry := *view.Entry
		args := append([]string{}, view.Args...)
		cursor := view.Page.Cursor
		a.Actions.LoadOutputPage(entry, args, cursor, func(run extensions.CapturedRun, err error) {
			if a.output != view {
				return // the view was closed or replaced while the page ran
			}
			view.LoadingMore = false
			emission, ok := extensions.ParseEmission(run.Stdout)
			if !ok {
				// The plugin stopped speaking the protocol: keep what we have
				// and stop asking, rather than looping on a page we cannot read.
				view.Page = &extensions.Page{}
				return
			}
			view.Rows = extensions.MergeRows(view.Rows, emission.Rows)
			view.AllRows = view.Rows
			if emission.Page != nil {
				view.Page = emission.Page
			} else {
				view.Page = &extensions.Page{}
			}
		})
		return
	}
	view.PagesLoaded++
	rows, page := extensions.WindowRows(view.AllRows, view.PagesLoaded)
	view.Rows = rows
	if page.HasMore {
		view.Page = &page
	} else {
		view.Page = nil
	}
}

// The page footer's states, as the old build's `pluginFooterState` had them:
// `footerMore` while the list still has a page the scroll can reach,
// `footerLoading` while a fetch is in flight, `footerEnd` once every row the
// plugin is going to hand over is drawn. `footerMore` is the quiet state — the
// scroll trigger is the affordance, not a button — so it carries no word, only
// the row's own height.
const (
	footerNone    = ""
	footerMore    = "more"
	footerLoading = "loading"
	footerEnd     = "end"
)

// outputFooterState is the footer's one state for a view. A list that is
// complete in one emission has no page block at all and gets no row; a list
// the launcher windows in memory loses its page block when the window reaches
// the end, so `footerEnd` is only ever drawn for a plugin that pages itself.
func outputFooterState(view *OutputView) string {
	if view == nil || view.Page == nil {
		return footerNone
	}
	switch {
	case view.LoadingMore:
		return footerLoading
	case !view.Page.HasMore:
		return footerEnd
	}
	return footerMore
}

// outputFooter draws the list's pagination row: a quiet row under the last
// result, the spinner and a word while a page is in flight, the end word once
// the plugin has finished. The row is not a result — it takes no selection and
// no shortcut.
//
// P13-bj（`docs/liquid-glass.md` §6.29）：这一行有两个会画字的状态，而它们此前
// **一档墨**——实测（装 app 主题，对地面峰值差）加载词浅 118 / 深 118，到底词
// 浅 118 / 深 118，**一个字节不差**。一件「正在发生的事」和一件「到此为止的边界」
// 用同一个声音说话，换状态时这一行没有任何一档变化，只有字变了。
//
// 两档（§6.26 / §6.28 的同一条阶梯，这一条落在**分页状态**上）：
//
//	在飞   `tokens.TextSecondary` + 500   浅 107 / 深 176   ← 这一行唯一的活物，配一枚转着的环
//	到底了 `t.TextMuted`（不动）           浅 128 / 深 133   ← 列表的地板，退到静息档
//
// 字阶**不动**（两个状态仍是 `t.FontSize - 1`）：改字阶 = 改行盒 = 改高度链，
// 这条线 §6.24 / §6.25 / §6.26 / §6.28 已经撞过五次红。
func (a *App) outputFooter(c *ui.Context, copy i18n.Launcher, view *OutputView) {
	state := outputFooterState(view)
	if state == footerNone {
		return
	}
	t := c.Theme()
	text, ink, weight := footerWord(state, copy, a.tokens(c), t)
	ui.Row(c).FillWidth().Shrink(0).MinHeight(t.Space(5.5)).Gap(t.Space(2)).
		Padding(0, t.Space(3)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if state == footerLoading {
			a.footerSpinner(c)
		}
		if text != "" {
			word := ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(ink)
			// Only the live word carries a weight: `footerEnd` must keep the
			// frame's own default (no `font-weight` set at all), so it stays
			// byte-identical to the row it replaces.
			if weight != 0 {
				word = word.FontWeight(weight)
			}
		}
	})
}

// footerWord is the pagination row's one word, its ink and its weight — the
// single place the two states are told apart (P13-bj). `footerMore` draws no
// word at all: the scroll is that state's affordance, not a label.
func footerWord(state string, copy i18n.Launcher, tokens theme.Tokens, t *ui.Theme) (string, ui.Color, int) {
	switch state {
	case footerLoading:
		return copy.OutputLoading, tokens.TextSecondary, footerLiveWeight
	case footerEnd:
		return copy.OutputEnd, t.TextMuted, 0
	}
	return "", t.TextMuted, 0
}

// footerLiveWeight is the live word's weight: it labels a turning ring rather
// than making a statement, and one step of weight is what keeps the pair
// (ring + word) reading as one object above the row's floor.
const footerLiveWeight = 500

// The spinner's ink: the old build's 12-unit ring (`calc(var(--u) * 12)`), so
// it rides the interface scale, in a 1.5px hairline border — physical ink, so
// the stroke does not.
const (
	footerSpinnerStroke  = 1.5
	footerSpinnerTurn    = 360
	footerSpinnerPerTurn = 700 * time.Millisecond
	// footerSpinnerFPS is how often the ring is painted while a page is in
	// flight: a whole turn in 0.7 s needs a smooth angle, so the display's
	// own cadence rather than a slow fade's.
	footerSpinnerFPS = 60
)

// footerSpinner draws the pagination row's indicator: a hairline circle with a
// darker leading quarter, turning once every 0.7 s.
//
// The turn is read through motion, so a desktop that asked for less motion
// draws the ring at rest — the old build's `animation: none` — and registers
// no loop. The framework's own Loop would go on turning whatever the
// preference says; the app is where the preference has a meaning.
//
// P11-bp: **the angle is read while the frame paints, not while the view is
// built.** Before, `Element.Loop` (called at build time) asked for
// `Context.AnimationFrame`, which `runtime.go:744-748` turns into "build the
// whole view again next frame" — so a page in flight rebuilt the whole
// launcher at the display's rate, one frame after another, for a 12u ring.
// `Painter.AnimationFrameRate` asks for the frames that *paint this view
// again* instead, and the phase comes from `Painter.Now`. The start of the
// turn lives on the App (`footerSpin`), because element state is lost with
// every build while the drawing must not be. A plain Box has no `rotate` of
// its own (that field is an icon's), so the turn is a painter transform
// around the ring's centre: a circle's stroke width does not change under a
// rotation, so the ink is the same 1.5px it was.
func (a *App) footerSpinner(c *ui.Context) {
	t := c.Theme()
	tokens := a.tokens(c)
	hairline := tokens.Hairline
	ring := ui.Box(c).Size(t.Space(3), t.Space(3)).Shrink(0)
	ring.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		radius := min(r.W, r.H)/2 - footerSpinnerStroke/2
		angle := footerSpinnerTurn * motion.Value(c, func() float32 {
			if a.footerSpin.IsZero() {
				a.footerSpin = p.Now()
			}
			return float32(p.Now().Sub(a.footerSpin)%footerSpinnerPerTurn) /
				float32(footerSpinnerPerTurn)
		})
		if !c.Preferences().ReduceMotion {
			p.AnimationFrameRate(footerSpinnerFPS)
		}
		p.Transform(ui.Rotation(angle), cx, cy, func() {
			var full ui.Path
			full.Circle(cx, cy, radius)
			p.StrokePath(&full, footerSpinnerStroke, hairline)
			var lead ui.Path
			lead.Circle(cx, cy, radius)
			lead.Trim(0, 0.25)
			// P13-bj: the moving quarter wears the live word's own ink
			// (`footerWord`), so the ring and its label stay one object; the
			// track above stays the hairline, which is the part that does not
			// move and must not get louder.
			p.StrokePath(&lead, footerSpinnerStroke, tokens.TextSecondary)
		})
	})
}

// moveOutputSelection moves the list's choice by one, skipping rows that
// cannot be run.
func (a *App) moveOutputSelection(delta int) {
	if a.output == nil || !a.output.IsList {
		return
	}
	rows := a.output.Rows
	index := a.output.Selected
	for i := 0; i < len(rows); i++ {
		index += delta
		if index < 0 {
			index = len(rows) - 1
		}
		if index >= len(rows) {
			index = 0
		}
		if rows[index].Runnable() {
			a.output.Selected = index
			a.OutputList.ScrollIntoView(index)
			return
		}
	}
}

// runOutputRow does what Enter does on the chosen row: open a page, copy text,
// or put it back in the field. A row with no action does nothing.
func (a *App) runOutputRow() {
	if a.output == nil || !a.output.IsList {
		return
	}
	rows := a.output.Rows
	if a.output.Selected < 0 || a.output.Selected >= len(rows) {
		return
	}
	row := rows[a.output.Selected]
	if !row.Runnable() {
		return
	}
	copy := a.copy()
	switch row.Action.Type {
	case "open":
		if a.Actions.OpenURL != nil {
			if err := a.Actions.OpenURL("", row.Action.URL); err != nil {
				// The old build's `openWithSystem` stayed up on a refusal and
				// reported it; closing would throw away the output the user
				// was looking at (`useLauncherActions.ts:430-445`).
				a.toast = copy.OpenURLFailed
				return
			}
		}
		a.leaveOutput()
		a.Hide()
	case "copy":
		if a.Actions.Copy != nil {
			a.Actions.Copy(row.Action.Text)
			a.toast = copy.Copied
		}
	case "insert":
		// Leaving clears the field, so the text goes in after it.
		a.leaveOutput()
		a.Query = row.Action.Text
		a.pendingCaret = true
	}
}

// outputText is what Enter copies from the view: the text, or the chosen row's
// text in a list.
func (a *App) outputText() string {
	if a.output == nil {
		return ""
	}
	if !a.output.IsList {
		return a.output.Text
	}
	rows := a.output.Rows
	if a.output.Selected < 0 || a.output.Selected >= len(rows) {
		return ""
	}
	row := rows[a.output.Selected]
	// P13-cg：游标现在可以落在便条上（只读游标），而「复制取的那一行」必须是
	// 一行**内容**——这条守卫保住的是本轮之前的行为（那时 `Selected` 到不了
	// 便条），不是新语义。
	if !row.Runnable() {
		return ""
	}
	if row.Action != nil && row.Action.Type != "open" {
		return row.Action.Text
	}
	if row.Subtitle != "" {
		return row.Title + " \u2014 " + row.Subtitle
	}
	return row.Title
}

// runStatus is the output view's summary line: the verdict and the facts, joined
// by the reference's own `·`. It is the one sentence a single line can hold (the
// launcher's strip, §6.26); a surface with a ladder takes the two halves apart
// instead (see runStatusParts).
func (a *App) runStatus(copy i18n.Launcher, run extensions.CapturedRun) string {
	verdict, facts := a.runStatusParts(copy, run)
	parts := []string{verdict}
	if facts != "" {
		parts = append(parts, facts)
	}
	return strings.Join(parts, "  ·  ")
}

// runStatusParts splits the summary into the two things it says: **how the run
// ended** and **what else is worth knowing about it** (its duration, whether its
// output was cut).
//
// P13-bi (`docs/liquid-glass.md` §6.28): the split exists because the plugin
// window has a whole band for this and the launcher has one line. Joining them at
// the string level meant the window could draw only one ink for both — and it
// drew neither, because the sentence never reached the window at all.
func (a *App) runStatusParts(copy i18n.Launcher, run extensions.CapturedRun) (verdict, facts string) {
	switch {
	case run.TimedOut:
		verdict = copy.OutputTimedOut
	case run.Success:
		verdict = copy.OutputSucceeded
	default:
		verdict = fmt.Sprintf(copy.OutputFailed, run.ExitCode)
	}
	parts := []string{}
	if run.Duration > 0 {
		parts = append(parts, extensions.FormatRunDuration(run.Duration.Seconds()*1000))
	}
	if run.Truncated {
		parts = append(parts, copy.OutputTruncated)
	}
	return verdict, strings.Join(parts, "  ·  ")
}

// leaveOutput closes the output view.
func (a *App) leaveOutput() {
	if a.output == nil {
		return
	}
	a.output = nil
	a.outputNumbers = nil
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// InOutputView reports whether a captured run's output is on screen.
func (a *App) InOutputView() bool { return a.output != nil }

// outputBody draws the captured output in place of the result list: the
// status line pinned under the field, and the text or the rows scrolling under
// it.
func (a *App) outputBody(c *ui.Context, copy i18n.Launcher, edge float32) {
	view := a.output
	t := c.Theme()
	header := t.Space(4)
	ui.Column(c).Fill().Children(func() {
		// The strip is the view's own band, not a layer over the text. It sits
		// in the column above the body, as the reference's
		// `.plugin-window__bar` is a sibling of `.plugin-window__body`
		// (`main:src/plugin-window/DetachedPluginApp.tsx:296-329`).
		//
		// P11-aq: this used to be an absolute layer (`Absolute().Top(edge)`)
		// whose height the body only guessed at — it reserved 16u for a 42u
		// strip (title row + status line), so the status line printed straight
		// over the first group heading and over the first line of text, and
		// every run in that 26u band was outside the pointer's reach (a visible
		// line nobody could select). Laid out in flow, the strip is as tall as
		// the text it holds and the body starts below it by construction —
		// there is no reserved height to drift.
		ui.Column(c).FillWidth().Padding(edge, t.Space(3), 0, t.Space(3)).Children(func() {
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, view.Title).FontSize(t.FontSize).TextColor(t.Text).Ellipsis("\u2026").SingleLine().Grow(1)
				// The output's own controls: 28u transparent squares with a
				// mark, the reference's `.plugin-window__bar-button`, never
				// filled pills in the text surface's header.
				tokens := a.tokens(c)
				control := func(mark, label, hint string, run func()) {
					if run == nil {
						return
					}
					button := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
						Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
						Label(label)
					// P13-ay：页头两枚 28u 方键的提示也戴 E2 浮面。
					button = tokens.Tip(c, button, hint)
					// P13-aa（§2 / §3）：悬停的小键是 E1 —— 面 + 顶受光边 + 底对侧暗线。
					// 它此前只有一层 `ControlHover` 洗色：一条线都没有，读起来是「一块颜色」
					// 而不是「一块能被碰的面」，而同一个启动器的 `rowControl`（20u）与
					// `fieldSettingsButton`（28u）自 P13-h 起就戴 `LitFace`。不补 `Border`：
					// 面差就是它的边界（P13-j 的 toast chip 同一条判据）。
					//
					// P13-bw（`docs/liquid-glass.md` §5「按下」/ §6.40 第 40-3 行）：这两枚
					// 键此前只有静息（透明）与悬停（`ControlHover` + 一对线）**两档**——
					// 指针按下去的那一帧与悬停帧逐像素相同（实测浅 226/226、深 56/56）。
					// 第三档按同族规矩：`ControlPress` 的面 + `Scale(0.92)`；一对线
					// **保留**——这一族的面是「一块能被碰的板」，压下去仍在地面之上，
					// 不是沉进凹槽（P13-q / P13-bv 在设置页的同一个取舍）。
					//
					// 面写成变量、一对线提到臂外，是本仓已有的形状（`power.go` 的
					// `if plane.A > 0 { LitFace }`）：一份面，所有状态共用一对线
					// （`internal/ci/litarm_contract_test.go` 的逃生口 ②）。
					plane := ui.Transparent
					switch {
					case button.Pressed():
						plane = tokens.ControlPress
						button.Scale(0.92, 0.92)
					case button.Hovered():
						plane = tokens.ControlHover
					}
					button.Background(plane)
					if plane.A > 0 {
						tokens.LitFace(button)
					}
					button.Children(func() {
						ui.Text(c, mark).FontSize(tokens.Emphasis).TextColor(t.TextMuted)
					})
					if button.Clicked() {
						run()
					}
				}
				// The output can be shown in a window of its own: the old
				// build's own pin, on the surface that holds the text.
				if a.Actions.PinText != nil {
					control("\u2197", copy.HistoryPin, copy.HistoryPin, func() { a.Actions.PinText(view.Title, view.Text) })
				}
				// And a command's page can be detached into a window that
				// stays put: the old build's second window, which never hides
				// on blur and re-runs the command from its own controls.
				if a.Actions.DetachOutput != nil && view.Entry != nil {
					control("\u29c9", copy.OutputDetach, copy.OutputDetachHint, func() { a.Actions.DetachOutput(*view) })
				}
			})
			// P13-bg（`docs/liquid-glass.md` §6.26）：这一行此前是**一件事**——
			// 这一页的结局（`Finished` / `Exit code 3` / `Timed out`）与它自己的
			// 键位图例（`Enter copies · Esc closes`）同一档墨、同一个字重，只隔一个
			// 中点；而且三种结局**逐像素相同**（实测浅 116 / 深 164，三个臂一个字节
			// 不差），一次失败的运行与一次成功的运行在条子上长得一样。现在它是三段
			// 各自的寄存器：结局、中点、图例（见 `outputStatusSpans`）。
			ui.RichText(c, a.outputStatusSpans(c, copy, view)...).FontSize(t.FontSize - 1)
		})
		// The body takes what the strip left: its own inset is the breath below
		// the status line, not the panel's top edge — the strip already spent
		// `edge` getting there.
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
			if view.IsList {
				ui.Column(c).Fill().Children(func() {
					// P11-bl：与搜索列表同一个时机——行盒与视口盒都是上一帧的，而编号在
					// 行被建出来之前就要发下去，所以先清掉上一帧的盒，由本帧建出来的行
					// 重新填（离开列表的行于是不会留下过期盒子骗 viewportRange）。
					clear(a.outputRowRects)
					// P13-x：分组头的盒子与行盒同一时机清掉（见 `headingRects`）。
					clear(a.outputHeadingRects)
					// P13-ce（`docs/liquid-glass.md` §6.48）：条子下面那 4u 的呼吸属于**这一页**，
					// 不属于内容——它留在裁剪之外。此前它是列表自己的 padding（内容侧），
					// 于是滚一行就被内容吃掉了：实测第一行的字尾落在 y=102，而条子最后一行墨在
					// y=100——中间只隔 **2px**，一行被折边切成两半的字贴在页头的结论上。
					// 移到外面之后裁剪边固定在 y=118，条子下面永远有 16px 的空气；静止帧
					// **逐像素不变**（首行仍在 118，视口下沿仍在 466——少掉的 16px 高度与
					// 多出来的 16px 空气是同一笔）。
					ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(header, 0, 0, 0).Children(func() {
						list := ui.List(c.Key("launcher.output.list"), &a.OutputList, len(view.Rows), func(i int) {
							a.outputRow(c, view.Rows[i], i)
						}).Fill().Padding(0, t.Space(2), 0, t.Space(2)).Label(view.Title)
						// The rows are clipped to this box, so it is the viewport the output's
						// numbered family measures against (see outputNumberedViewport).
						a.outputListRect = list.Bounds()
						// The scroll trigger: reaching the end of what is loaded asks for
						// the next page, exactly as the reference's own list did.
						if a.OutputList.AtEnd() {
							a.loadMoreOutputPage()
						}
					})
					a.outputFooter(c, copy, view)
				})
			} else if view.Empty {
				// P13-cd（`docs/liquid-glass.md` §6.47）：一条什么都没写的命令，
				// 说的是**这一页的空态**，不是输出。此前这句话戴着正文的墨
				// （12px `t.Text` + `Selectable()`）、坐在正文第一行上——实测墨盒
				// `{X:12 Y:123 W:188 H:16}`，与页头那行命令行同档墨，与「命令
				// 真的打印了这句话」在界面上无法区分。现在它与结果区的空态走
				// 同一个函数、同一档墨，居中在这条正文带里；`Scroll` 不建了，
				// 一行字没有可滚的东西（`view.Text` 本身不动，高度链与复制不变）。
				ui.Column(c).Fill().Center().Children(func() {
					a.quietStatement(c, view.Text)
				})
			} else {
				// P13-ce（`docs/liquid-glass.md` §6.48）：正文分支读同一条规则——条子下面的
				// 4u 呼吸留在裁剪之外。这一支此前连折边都没有（正文不是行，`foldAlpha` 没有
				// 可量的盒子），所以滚起来时第一行字是**满墨**贴着条子的；现在它至少和列表
				// 分支结束在同一条线上，两端的收口不再一个在页头墨上、一个在窗口最后一行
				// 像素上。正文的高度链、`MaxY`、可选中的墨都不动。
				ui.Column(c).Fill().Padding(header, 0, 0, 0).Children(func() {
					ui.Scroll(c.Key("launcher.output")).TrackScroll(&a.Output).Fill().
						Padding(0, t.Space(3), t.Space(1), t.Space(3)).Children(func() {
						ui.RichText(c, a.outputSpans(c, view)...).Font(view.FontFamily).
							FontSize(t.FontSize).TextColor(t.Text).Selectable()
					})
				})
			}
		})
	})
}

// outputStatusSpans is the strip's one line as the three things it actually
// says: this run's verdict, the reference's muted `·` between them, and the
// key legend under it.
//
// P13-bg（`docs/liquid-glass.md` §6.26）：此前这三件是一件事——同一个墨、同一个
// 字重、只隔一个中点，而且三种结局逐像素相同（实测浅 116 / 深 164，`Finished`、
// `Exit code 3`、`Timed out` 三个臂一个字节不差）。结局是这一页的**结论**，图例是
// 它的**说明书**；它们不该是同一档。
//
// 三档（对启动器的地面，浅 / 深）：
//
//	标题（命令行，`t.Text`，12px）      228 / 221   ← 这一页的名字，不动
//	结局 `TextSecondary` + 580          145 / 133
//	结局（失败）`t.Warning` + 580         139 / 115   ← 色相是语义，不是强调（§6.19）
//	图例 `TextTertiary` + 500            109 /  89   ← 这一页最轻的一件
//
// **为什么图例退到 tertiary 而不是留在 muted**：`TextMuted`（浅 116 / 深 164）与
// `TextSecondary`（108 / 156）只差 **8** 级——那是「差值太小」的原样。三档要真的成阶梯，
// 必须把图例退到 tertiary 那一档（浅 144 / 深 112），结局才第一次比它高 36 / 44 级；
// 失败臂因此也才高过图例（30 / 26），色相之外还有高度。
//
// 行盒一个像素不动：三段同一个 `FontSize - 1`，RichText 只换墨与字重。
func (a *App) outputStatusSpans(c *ui.Context, copy i18n.Launcher, view *OutputView) []ui.Span {
	tokens := a.tokens(c)
	t := c.Theme()
	verdict := tokens.TextSecondary
	if view.Failed {
		verdict = t.Warning
	}
	return []ui.Span{
		{Text: view.Status, Weight: 580, Color: verdict},
		// The reference's own muted `·` between two facts
		// (`extensions.css:2096-2103`); `settingsui`'s fact lines draw it at
		// `TextTertiary` 0.7 and this line keeps that reading.
		{Text: "  \u00b7  ", Color: tokens.TextTertiary.Alpha(0.7)},
		{Text: a.outputHint(copy, view), Weight: 500, Color: tokens.TextTertiary},
	}
}

// outputHint is the key line: a list can be walked and run, text can only be
// copied.
func (a *App) outputHint(copy i18n.Launcher, view *OutputView) string {
	if view.IsList {
		return copy.OutputListHint
	}
	return copy.OutputHint
}

// outputRow draws one list row: its group heading when it starts a group, its
// icon, and its two lines. A status row is muted and cannot be chosen.
//
// The heading rule is the old build's `list-sections.ts` one: a row starts a
// group when it carries a name and the row above it does not carry the same
// one. The *first* row of the list starts a group too — there is no row above
// it to continue — so a list whose opening row names a group prints its
// heading, exactly as a later block does.
func (a *App) outputRow(c *ui.Context, row extensions.Row, i int) {
	t := c.Theme()
	if row.Group != "" && (i == 0 || row.Group != a.output.Rows[i-1].Group) {
		// The same register as the launcher list's own section title: the body
		// step at 1.4, weight 620, muted, with the 6u/4u band the rows use.
		// Both surfaces print a group heading, and the reference's
		// `.launcher-section-title` is one element, not two.
		//
		// P11-ac 重定基线：分组头落在**本表面结果行标题的那一列**上。旧值是行
		// 左 padding 12u、没有任何图标列/gap 偏移，于是标签挂在 20u，比它自己的
		// 标题（52u）左挂了 32u；旧注释写着「the 6u/12u/4u inset the rows use」，
		// 而行根本没有 12u 的左 inset。现在左缩进是行自己的那一份：行内 padding
		// 8u（下面的 `Padding(t.Space(1.5), t.Space(2))`）+ 图标列 28u
		// （`iconPlate` 的 `Space(7)`）+ 该行自己的 gap 10u（`Gap(t.Space(2.5))`）
		// = 38u 的 `Margin-left`，加上列表自己的 8u 即 54u。
		//
		// P11-ad 重定基线：这个和里那一项 gap 从 8u 改成 10u，与启动器列表的行
		// 同一条内部节奏——同一张卡里的两种列表只有一条文字列（54u），不是一个
		// 表面 52u、一个表面 54u。旧值 → 新值：`Margin-left` 36u → 38u，标题列
		// 52u → 54u。
		// P13-x · 分组头也是列表的一部分，所以它也参加折边（与 `row` 的分组头臂同一个
		// 规则、同一个纯函数）。它是 `style` 的**兄弟**，折边切到它的时候它保持满墨——
		// 实测 `Group B` 在只剩 8/22u 时墨是 134、12/22u 时 137，与整行可见时一样；
		// 这不是「下面还有」，这是「界面坏了」。
		//
		// 盒子记进 `outputHeadingRects`，不进 `outputRowRects`——后者喂
		// `outputNumberedViewport`，一个标签混进去会占掉一个数字位。
		heading := ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(2), t.Space(1), t.Space(2))
		if a.outputHeadingRects == nil {
			a.outputHeadingRects = make(map[int]ui.Rect, 4)
		}
		a.outputHeadingRects[i] = heading.Bounds()
		if fold := foldAlpha(a.outputListRect, a.outputHeadingRects[i]); fold < 1 {
			heading.Opacity(float32(fold))
		}
		heading.Children(func() {
			ui.Text(c, row.Group).FontSize(t.FontSize).FontWeight(620).
				TextColor(t.TextMuted).Margin(0, 0, 0, t.Space(9.5))
		})
	}
	// P11-ad 重定基线：行的 gap 8u → 10u，与启动器列表的行（`launcher.go` 的
	// `Gap(t.Space(2.5))`）同一份。这一项同时管着图标列与文字列之间、文字列与
	// 行尾控件之间，所以两种列表的整条横向节奏是同一个数：面板/卡片 8u + 行内
	// padding 8u + 图标列 28u + gap 10u = 标题列 54u。
	style := ui.Row(c.Key(row.ID)).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Gap(t.Space(2.5))
	// P13-cg：`selected` 是「游标在这儿」，`chosen` 是「这是一个被选中的门」。
	// 便条（`row.Status` / `row.Disabled`）可以被游标指到，但它不是门——
	// 所以它拿面色不拿灯（见下面 switch 的 `case selected`）。
	selected := i == a.output.Selected
	chosen := selected && row.Runnable()
	// P11-bl：这一行上一帧的盒子，与搜索列表的 rowBody 记的是同一件事（见
	// viewportRange）。首帧还没有盒子，记一个空矩形，viewportRange 会跳过它。
	// P13-w：`Bounds()` 是**上一帧**的矩形，所以折边退后（下面）读的与记的是
	// 同一个盒子——与 `rowBody` 的时序逐字相同。
	if a.outputRowRects == nil {
		a.outputRowRects = make(map[int]ui.Rect, 16)
	}
	a.outputRowRects[i] = style.Bounds()
	// P13-w · 折边之外的那一行**退后**，不是被切（`docs/rounds/P13-w-spec.md`）。
	//
	// P13-v 把这条规则落在了启动器列表上（`rowBody`），捕获输出页是同一张卡里的
	// 另一张列表，却一直没接——滚到中间时实测 `Row 3` 的字母在 y=122 一刀断掉
	// （y=111..121 有 `darkest = 25` 的墨，y=122 起是 235 的板）。同一张卡的两张
	// 列表只有一条折边规则，所以这里调**同一个纯函数** `foldAlpha`，不另写一份。
	//
	// 这一页的读数与启动器那边不同：40u 的行盒里字盒从 y+9 起，折边常常切在字盒的
	// 前导里，可见的那几像素是**图标板的顶边**（实测 y=484..487 `darkest = 235`）。
	// 所以退后在像素上的表现是「板的顶边从 235 退到 248」——不是 P13-v 那种「半个
	// 字变淡」。规则一条，读数两样，两边都只按自己露出的比例退。
	//
	// 静息档这一页是 1（没有启动器那边历史行的 0.92），所以取 min 就是取折边；
	// 写成 min 是为了「退后只会更深，不会把一行退回来」这条规则留在原地。
	opacity := 1.0
	if fold := foldAlpha(a.outputListRect, a.outputRowRects[i]); fold < opacity {
		opacity = fold
	}
	if opacity < 1 {
		// 必须在 `Children` 之前：框架按建树那一刻的 opacity 画这一帧。
		style.Opacity(float32(opacity))
	}
	tokens := a.tokens(c)
	switch {
	case chosen:
		// P8-e 重定基线（§5 禁止清单「列表选中行」）：旧值是
		// `tokens.AccentTintQuiet`——一块带色相的洗。选中行是中性面，与启动器
		// 列表（P8-a）的选中行同一档 `tokens.RowSelected`，两套外观各自的 rung
		// 由 token 自己分。旧值 → 新值：`AccentTintQuiet` → `RowSelected`。
		style.Background(tokens.RowSelected)
		// P12-b（§6.1 第 1-5 行）：捕获输出页的选中行与启动器列表的选中行是同
		// 一条规则、同一个函数——`theme.Tokens.LitFace`（面 + 顶受光边 + 底对侧
		// 暗线）。两张列表是同一张卡里的同一件东西，选中块不许一个「站起来」一个
		// 「平躺」。
		style = tokens.LitFace(style)
	case selected:
		// P13-cg · 只读游标：游标指着一行**便条**。它拿选中行的面（`RowSelected`，
		// 与门那一支同一个 token——「选中」是一种状态，不是一种权限），但**不点灯**：
		// `LitFace` 的顶受光边是 §3 说的「这块面可以被碰」的记号，而便条按定义
		// 碰不动（`extensions/rows.go` 的 `Status`：Enter 什么也不做）。
		//
		// 改前：全便条的列表 `Selected = -1`，整页没有任何一行被画成选中的——
		// 用户看不到游标，也看不到「这一页没有可运行的东西」这句话（本轮实测见
		// 报告：便条行的面色对地面 **0** 级）。改后：面色到了（浅色 +12 / 深色
		// +13，与门那一支逐字节相同），冠线仍等于面色（差 **0**）。
		style.Background(tokens.RowSelected)
	case row.Status || row.Disabled:
		// A note is not a door: it stays muted and never highlights.
	default:
		if style.Hovered() {
			style.Background(t.SurfaceHover)
		}
	}
	style.Children(func() {
		dim := row.Status || row.Disabled
		plate := a.iconPlate(c, false, dim)
		if dim {
			plate.Opacity(0.62)
		}
		plate.Children(func() {
			if glyph, ok := outputGlyph(row.Icon); ok {
				ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			color := t.Text
			if dim {
				color = t.TextMuted
			}
			// P13-ci · 同一张卡里的两张列表只有一套排法：启动器的结果列表给
			// 标题 580、副标题 510（`launcher.go` 的 `rowBody`），而这一页一直是
			// 400 / 400。便条行因此**同墨同重**（实测浅色 113 / 113）：两行读起来
			// 是两句等价的话，标题没有说它是标题。
			//
			// 字重是墨级差看不见的那一半（P13-ch §6.51）：`TextMuted` 一个字节
			// 没动，分档全靠这一行。`chosen` 的 700 仍是「门被选中」的记号
			// （P13-cg），与启动器列表的 700 同一档。
			//
			// **副标题不动**（仍是框架的 400）：本轮先量过「照抄启动器的 510」——
			// 便条行标题/副标题的墨量差从 3669 掉到 1769（比 1.26 → 1.10），
			// 因为 12px 那行的 110 档加在字少的一行上更显眼。启动器那边是
			// `TextStrong 580 / TextMuted 510` 两个旋钮一起给；这一页两行同墨，
			// 只有一个旋钮，副标题必须留在静息档，差才拉得开。**rank 是规则，
			// 数字是每个面的。**
			title := ui.Text(c, row.Title).FontSize(t.FontSize).TextColor(color).FontWeight(580)
			// P13-cg：加粗与提墨是「门被选中」的记号，不是「游标在这儿」的记号——
			// 一句便条不该因为被游标指到就变成标题。
			if chosen {
				title.FontWeight(700)
			}
			if row.Subtitle != "" {
				subtitle := ui.Text(c, row.Subtitle).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				if chosen {
					subtitle.TextColor(t.Text)
				}
			}
		})
		// The numbered badge sits at the row's trailing edge, exactly as the
		// launcher's own rows print it. Only rows the family can reach carry
		// one, so a status note stays unnumbered.
		if number, ok := a.outputNumbers[i]; ok {
			a.rowActionText(c, a.resultNumber(number), chosen)
		}
	})
	if style.Clicked() && row.Runnable() {
		a.output.Selected = i
		a.runOutputRow()
	}
}

// outputSpans maps a command's own ANSI output to the spans the text element
// draws: the sixteen program colours of the current appearance, the weights
// and the decorations, with the surface's own colour for whatever the command
// left unsaid.
func (a *App) outputSpans(c *ui.Context, view *OutputView) []ui.Span {
	appearance := a.tokens(c).Appearance
	base := c.Theme().Text
	out := make([]ui.Span, 0, len(view.Spans))
	for _, span := range view.Spans {
		if span.Text == "" {
			continue
		}
		out = append(out, resolveOutputSpan(appearance, span.Style, base, c.Theme().FontSize, span.Text))
	}
	if len(out) == 0 {
		out = append(out, ui.Span{Text: view.Text, Size: c.Theme().FontSize, Color: base})
	}
	return out
}

// dimAlpha is the opacity of a dim run: `spanStyle`'s `opacity: 0.62`, the
// figure a terminal's half-intensity text is drawn with.
const dimAlpha = 0.62

// resolveOutputSpan turns one parsed ANSI run into the span the text surface
// draws, over the surface's own foreground (`base`). It is the Go reading of
// `main:src/launcher/PluginTextView.tsx:46-63`'s `spanStyle`.
//
// Inverse swaps the two colours only where the command named them: a side left
// at the default keeps the surface's own value — the default foreground, a
// transparent background — instead of gaining a filled-in colour. `spanStyle`
// paints a side only when the swap actually produced one (`if (fg)`/`if (bg)`),
// so an unnamed side is not a colour to paint with.
//
// Dim is half-intensity text: the whole run, background included, is drawn at
// 62% opacity, which is what CSS `opacity` does to the span and not a lighter
// version of the foreground alone.
func resolveOutputSpan(appearance theme.Appearance, style ansi.Style, base ui.Color, size float32, text string) ui.Span {
	fg := ansiColor(appearance, style.FG, base)
	bg := ansiColor(appearance, style.BG, ui.Transparent)
	if style.Inverse {
		fg = ansiColor(appearance, style.BG, base)
		bg = ansiColor(appearance, style.FG, ui.Transparent)
	}
	if style.Dim {
		fg = fg.Alpha(dimAlpha)
		bg = bg.Alpha(dimAlpha)
	}
	weight := 400
	if style.Bold {
		weight = 700
	}
	return ui.Span{
		Text:          text,
		Size:          size,
		Weight:        weight,
		Italic:        style.Italic,
		Color:         fg,
		Underline:     style.Underline,
		Strikethrough: style.Strike,
		Background:    bg,
	}
}

// ansiColor resolves one parsed colour, falling back to the surface's own for
// a colour the command did not name.
func ansiColor(appearance theme.Appearance, color ansi.Color, fallback ui.Color) ui.Color {
	switch color.Kind {
	case ansi.Named:
		return theme.ANSIColor(appearance, 1, color.N, 0, 0, 0)
	case ansi.Indexed:
		if color.N < 16 {
			return theme.ANSIColor(appearance, 1, color.N, 0, 0, 0)
		}
		return theme.ANSIColor(appearance, 2, color.N, 0, 0, 0)
	case ansi.RGB:
		return theme.ANSIColor(appearance, 3, 0, color.R, color.G, color.B)
	}
	return fallback
}

// windowRows is the row table the window's height is charged from: the search
// results the list draws, or — while a captured run's output owns the band —
// the output view's own rows. The old build priced the plugin view, not the
// page behind it: `launcherListRows = pluginView ? pluginViewRows(pluginView)
// : displayedResults.length` (`main:src/App.tsx:2133-2137`). An output view in
// front of a two-row query is therefore as tall as its output, not its query.
//
// `sections` is the number of section-title bands a captured-output list
// prints; the ordinary list carries its headings as `RowInfo.Heading` rows
// inside the budget, so it reports none here.
func (a *App) windowRows(results []Item) (rows []RowInfo, held, sections int) {
	if a.output != nil {
		return a.outputRows()
	}
	rows = make([]RowInfo, len(results))
	for i, item := range results {
		if rowSubtitle(item) != "" {
			rows[i].Lines = 2
		} else {
			rows[i].Lines = 1
		}
		rows[i].Heading = item.heading
		rows[i].Status = item.status
		rows[i].Bar = strings.HasPrefix(item.ID, "bar:")
	}
	return rows, len(results), 0
}

// The captured-output text block's own measurements, in the old build's units
// (`main:src/launcher/plugin-mode.ts:238-247`). A text line is 24u; the block
// never draws shorter than three lines (the user's 「一个最小高度也要限制」) and
// never taller than the ten-row list ceiling, scrolling inside itself past it.
const (
	pluginTextLineUnits = 6   // 24u
	pluginTextMinUnits  = 18  // 72u, three lines
	pluginTextMaxUnits  = 100 // 400u, MAX_RESULTS × ROW_HEIGHT_TWO_LINE
)

// pluginTextRows is the text form's band row count: its explicit lines at
// `PLUGIN_TEXT_LINE_UNITS` each, clamped into the floor and the ceiling, then
// rounded up to whole `ROW_HEIGHT_TWO_LINE` band rows — the old build's
// `pluginTextMetrics(...).rows`, which is what `pluginViewRows` returned for
// the text form. An empty string has no lines and takes the floor.
func pluginTextRows(text string) int {
	lines := 0
	if text != "" {
		lines = strings.Count(text, "\n") + 1
	}
	height := math.Max(pluginTextMinUnits, math.Min(float64(lines*pluginTextLineUnits), pluginTextMaxUnits))
	return int(math.Ceil(height / rowUnits))
}

// outputRows is the band table for the captured-output view in front of the
// user. The text form is `pluginTextRows` band rows, each charged the full
// `ROW_HEIGHT_TWO_LINE` (the block is one body, not rows); the list form is its
// first `MAX_RESULTS` rows at their own heights (a status note 30u, a result
// 40u) with a section-title band per block. See
// `main:src/launcher/plugin-mode.ts:647-651` (`pluginViewRows`) and
// `main:src/App.tsx:2185-2200`.
func (a *App) outputRows() (rows []RowInfo, held, sections int) {
	view := a.output
	if view == nil {
		return nil, 0, 0
	}
	if !view.IsList {
		n := pluginTextRows(view.Text)
		rows = make([]RowInfo, n)
		for i := range rows {
			rows[i].Band = rowUnits
		}
		return rows, n, 0
	}
	visible := view.Rows
	if len(visible) > rowBudget {
		visible = visible[:rowBudget]
	}
	rows = make([]RowInfo, 0, len(visible))
	for i, row := range visible {
		if row.Group != "" && (i == 0 || row.Group != visible[i-1].Group) {
			sections++
		}
		if row.Status || row.Disabled {
			rows = append(rows, RowInfo{Status: true})
			continue
		}
		lines := 1.0
		if row.Subtitle != "" {
			lines = 2
		}
		rows = append(rows, RowInfo{Lines: lines})
	}
	return rows, len(rows), sections
}

// outputNumberedViewport is the captured-output list's numbered viewport: its
// own row boxes against its own clip, read by the one rule in viewportRange
// (P11-bl).
func (a *App) outputNumberedViewport(count int) (first, last int, ok bool) {
	return viewportRange(count, a.outputRowRects, a.outputListRect)
}

// assignOutputNumbers gives the captured-output list its numbered family: the
// same `⌘1`-`⌘0` slots the launcher's results carry, over the output's own
// rows. The old build fed the plugin view's items through the ordinary
// `resultShortcutSlots` (`main:src/App.tsx:1602-1611`), so the rule is the
// list's own: only a row with something to run is numbered (a status note or a
// disabled row is information, not a door), `1`-`9` are the first nine runnable
// rows on screen and `0` the tenth, and the family follows the scroll viewport
// (R34/R72).
//
// The viewport is read for **every** list, not only one past the ten-slot
// budget (P11-bl). The old gate `len(rows) > maxResultSlots` assumed a list
// that fits cannot scroll; here it can — the body charges the status strip
// (42u) above the list, so exactly ten rows scroll under it, and the gate then
// skipped the viewport and pinned the badges to the top. The viewport is also
// measured from the row boxes rather than `OutputList.Visible()`, which answers
// "a pixel": a row scrolled nine tenths away kept a badge nobody could see
// (P11-bb's report, on this surface). Both surfaces read the one rule in
// `viewportRange`.
func (a *App) assignOutputNumbers() map[int]int {
	numbers := map[int]int{}
	a.outputNumbers = numbers
	if a.output == nil || !a.output.IsList {
		return numbers
	}
	rows := a.output.Rows
	// The family follows the viewport whenever the last frame left geometry to
	// read; without geometry it falls back to the whole list, and the loop's cap
	// still bounds it at ten (see assignNumbers).
	first, last := 0, len(rows)-1
	if visibleFirst, visibleLast, ok := a.outputNumberedViewport(len(rows)); ok {
		first, last = visibleFirst, visibleLast
	}
	count := 0
	for i, row := range rows {
		if !row.Runnable() {
			continue
		}
		if i < first || i > last {
			continue
		}
		count++
		if count > maxResultSlots {
			break
		}
		numbers[i] = count % 10
	}
	return numbers
}
