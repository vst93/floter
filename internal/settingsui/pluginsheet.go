package settingsui

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/theme"
)

// The generic plugin configuration sheet: the old build's R29 overlay, in the
// settings pages' own card language (R59).
//
// 「配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的
// 完全独立的页面。」 A plugin ships a `plugincfg.Schema` — an ordered list of
// fields — and this renders it: the same `SettingsCard` and `SettingsRow`
// language the pages use, the same controls, the same tokens. Nothing here
// knows a plugin's name: every label, bound and option comes from the schema,
// and every change goes back through the callbacks the launcher owns.
//
// The one field kind that is not a value is the action: a button that arms on
// the first press and runs on the second, so a plugin can offer "clear
// history" without this file growing a plugin-specific branch.

// PluginSheet is one frame of the overlay: what to draw, and what to report.
type PluginSheet struct {
	// Schema is the plugin's configuration, nil when there is none.
	Schema *plugincfg.Schema
	// Values are the values as the plugin's settings hold them.
	Values plugincfg.Values
	// Armed is the action key whose button has been pressed once, empty for
	// none.
	Armed string
	// Failed shows the one line under the fields: the last write was refused.
	Failed bool
	// ActionFailed is the action key whose command was refused, empty for
	// none: that row prints the field's own failure line, the way the old
	// build's `ActionControl` kept its own `failed` state beside the button.
	ActionFailed string
	// Change writes one field; the sheet paints the value either way and the
	// caller reports whether the plugin took it.
	Change func(key string, value any)
	// Arm and Run are the action's two presses.
	Arm func(key string)
	Run func(key string)
	// Unarm is the way back from an armed button.
	Unarm func(key string)
}

// PluginSheet draws a plugin's configuration: the title, the failure line,
// and each section's heading and card of rows. It is drawn where the result
// list would be — the launcher gives it the band and scrolls it.
// ClosePluginSheet retires the editor state that belongs to an open sheet.
// The shell keeps one settings App for the whole run and draws every plugin's
// sheet through it, so the drafts and the scroll offset would otherwise wait
// for the next open: a text field's abandoned edit would come back as if it
// were the stored value, and a long sheet would reopen where the last one was
// left. Called when the overlay closes, not between frames of one open.
func (a *App) ClosePluginSheet() {
	a.pluginTextDrafts = nil
	a.pluginNumberDrafts = nil
	a.pluginNumberObserved = nil
	a.PluginScroll.Y = 0
	a.sheetPlugin = ""
}

func (a *App) PluginSheet(c *ui.Context, sheet PluginSheet) {
	if sheet.Schema == nil {
		return
	}
	// Removed fields cannot run their blur cleanup. Prune editor drafts before
	// painting, including an empty schema. Other plugins retain their own edits.
	active := make(map[string]plugincfg.Kind)
	prefix := sheet.Schema.Plugin + "\x00"
	for _, field := range sheet.Schema.Fields {
		active[prefix+field.Key] = field.Kind
	}
	for key := range a.pluginTextDrafts {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if kind, exists := active[key]; !exists || kind != plugincfg.Text {
			delete(a.pluginTextDrafts, key)
		}
	}
	for key := range a.pluginNumberDrafts {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if kind, exists := active[key]; !exists || kind != plugincfg.Number {
			delete(a.pluginNumberDrafts, key)
		}
	}
	for key := range a.pluginNumberObserved {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if kind, exists := active[key]; !exists || kind != plugincfg.Number {
			delete(a.pluginNumberObserved, key)
		}
	}
	// A submit waiting for the host's answer is retired with its field: a
	// record left behind would rebase whatever takes the same key next.
	for key := range a.pluginTextCommits {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if kind, exists := active[key]; !exists || kind != plugincfg.Text {
			delete(a.pluginTextCommits, key)
		}
	}
	t := c.Theme()
	tokens := a.tokens(c)

	// P13-gu（`docs/liquid-glass.md` §6.163）：这块浮层的滚动位置**跨插件串味**。
	// `a.PluginScroll` 是 App 上的一格，而 `ui.Scroll` 的 key 是常量
	// (`"plugin.config"`)——一个 App 依次画每个插件的浮层（`shell/pluginconfig.go`），
	// 于是插件 A 滚到底之后切到插件 B，B 一开帧就落在 A 的偏移上（内容短的那张
	// 直接顶到底部，页头被切在带子上沿）。与 P13-gs 的数字草稿是同一族：
	// **一格全局状态，被 N 个插件轮流用**。
	//
	// 处置取最小的一手：插件变了就把偏移归零（换一张面 = 从头读），而不是把
	// `ui.ScrollState` 拆成每插件一格——后者要把 `PluginScroll` 从 App 上搬进
	// map，六个既有量尺（`sheetcutedge` / `sheetpitch` / `sheetnamerung` / …）
	// 都直接读 `a.PluginScroll.MaxY`，那是结构真变（S），换不来多一个像素。
	if sheet.Schema.Plugin != a.sheetPlugin {
		a.sheetPlugin = sheet.Schema.Plugin
		a.PluginScroll.Y = 0
	}

	// The sheet's own scroller: a schema longer than the band scrolls inside
	// it rather than growing the window past the band it was given.
	//
	// It is wrapped so its two scroll edges can ride the viewport (§6.19), the
	// way the settings body's own scroller is (§6.16): the wrapper fills the
	// band it was given and the scroller fills the wrapper, so neither the
	// height chain nor the scroll range moves.
	//
	// P13-er（`docs/liquid-glass.md` §6.113）：下内边距是 **`Space(3)` → `Space(5)`**。
	// 这块面的底边软边（下面 `Height(t.Space(3))`）正好 10.8px 高，而它给自己的
	// 下留白也是 10.8px——滚到底时**软边整好吃掉了那一格**：实测带子最下 11 行是空白，
	// 第 12 行就是最后一张卡的下边，卡的边离软边上沿只剩 **1.2px**。满态于是读成
	// 「被切开」，不是「到头了」。
	//
	// 同一套滚动收口在设置体上一直是对的：`Padding(edge, 0, t.Space(6), 0)`，软边
	// `edge` = `Space(4)` = 14.4，下留白 21.6 = **软边 + 一格 `Space(2)`**
	// （`settingsui.go:1559`）。规矩是「软边 + 一格呼吸」，每张面代入自己的软边高：
	// 浮层 10.8 + 7.2 = **18.0 = `Space(5)`**。改后带子最下 18 行是空白，卡的边离
	// 软边上沿 7.2px。顶端的 `Space(1)` 与页头的墨一个字节不动（那条 11 行的余量
	// 见 §6.72）。
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c.Key("plugin.config")).TrackScroll(&a.PluginScroll).Fill().
			Padding(t.Space(1), t.Space(4), t.Space(5), t.Space(4)).Children(func() {
			ui.Column(c).FillWidth().Gap(t.Space(3)).Children(func() {
				// The header: the plugin's own name — the sheet's name, so it hangs
				// on the sheet's own column (the scroller's 4u padding), the way a
				// settings page's header hangs on its page column (`settingsui.go`
				// 的页头行同样是 4u)。
				//
				// P13-ei（`docs/liquid-glass.md` §6.104）：这一行此前是
				// `600 / TextMuted`——**整块浮层上最淡的一条墨**（实测浅色档最暗
				// 像素 128，而它下面每一条字段 help 是 57、它自己带的四个分组标题
				// 是 27）。于是唯一说明「你在改哪个插件」的那一行读起来像脚注，
				// 而文件里原本的注释还写着「name > section > field > help」——
				// 像素给的顺序是反的，名字排最后。
				//
				// 同一个角色（一页的名字）在设置页是 `Display / 650 / Text`
				// （`settingsui.go:1572`）。浮层里**字号不许跟**：这一行是
				// `Height(Space(7))` = 25.2px 的行盒，名字的墨从视口顶第 11 行起，
				// 而 §6.72 的顶边高正好 11 行（3u = 10.8px），余量 0.2px——字号
				// 一旦长大，墨顶就抬进边里（`TestThePluginSheetEdgesSitOnBlankAtBothEnds`
				// 会红）。所以只把**墨与字重**穿成页头那一档：旧值 → 新值
				// `600 / TextMuted` → `650 / Text`，字号仍是 body，比分组标题
				// （Title 12.6）低一级，但不再比它下面任何一行淡。
				// P13-gu：没有名字的 schema 不再留一格 25.2px 的**空行盒**。
				// `Title` 是插件自己声明的字段，空串是合法的（`plugincfg.Schema`），
				// 而这一行此前无条件画：浮层顶上于是先空出 25.2px + `Space(3)` 的
				// 空气，第一组标题看起来被推下去了——「什么都没有」被画成「有一行」
				// （与 §6.104 那一轮修的是同一行的另一半：有名字时它的墨必须够重）。
				// 名字在时一个字节不动（`Height(Space(7))` 是顶边余量的唯一来源）。
				if sheet.Schema.Title != "" {
					ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).Children(func() {
						ui.Text(c, sheet.Schema.Title).FontSize(t.FontSize).FontWeight(650).
							TextColor(t.Text).SingleLine()
					})
				}
				if sheet.Failed {
					// The status line is the caption step in the warning colour
					// (`.plugin-config-action__status`): a muted line, never a
					// red banner.
					//
					// P13-db（`docs/liquid-glass.md` §6.71）：它此前挂在**浮层的列**上
					// （scroller 的 4u，实测墨左沿 `14.4`），而它说的是下面那些**字段**
					// 写不进去——同一块面上，动作自己的失败线是坐在字段那一行里的
					// （实测 `438.6`，行标签从 `28.0` 起）。同一句话在这儿有两个说法：
					// 一条挂框、一条挂行。这一条收进字段列，与它下一行的分组标题同
					// 一个 3.5u 缩进（实测 `14.4 → 27.0`，量宽 `691.2 → 664.0`，正好
					// 是它讲的那批字段的宽度）。标题留在浮层的列上：它是这块面的
					// 名字，不是某张卡的话。
					ui.Text(c, i18n.For(a.language()).Settings.SaveFailed).
						FontSize(tokens.Caption).TextColor(t.Warning).
						Padding(0, t.Space(3.5))
				}
				// P13-en（`docs/liquid-glass.md` §6.109）：从第二组起，每一组在**自己
				// 身上**多带一条 `Space(4)` 的上内边距——**不是**把外层这一列的 `Gap` 抬高。
				//
				// 病：外层 `Gap` 是 `Space(3)`（10.8px），而 `sheetSection` 里「分组标题 →
				// 它自己的卡」是 `Space(4)`（14.4px，P13-ej）——同一张浮层上「换一组」比
				// 「组内标题与卡」**还紧**。盒对盒实测：第一张卡底 → 第二组标题盒顶
				// **12.2px**，而标题盒底 → 它自己那张卡顶 **14.8px**。第二组标题于是读成
				// 第一张卡的最后一行小字。
				//
				// 为什么不动 `Gap`（`Space(3)` → `Space(6)` 是一个更短的手）：那一列的
				// 孩子是「页头 / 第一组 / 第二组」，`Gap` 是**三对共用**的一个数——抬它会
				// 把「页头 → 第一组」一起抬（实测 22.2 → 25.7px），而那一对不是「换一组」。
				// 这正是 §6.106 已经立过的教训：**空气属于那一对，就加在那一对的元素上**。
				// 所以第一组不动，从第二组起每组自带一条 `Space(4)` 的上内边距
				// （14.4px = 一整档 4u）：组边界 12.2 → **22.6px**，落在 §4.3 那条
				// 「一个可以按的新东西」的 20px 档上（与 §6.69 会话页的 22px 同一读数），
				// 组内 14.8px 留在「同一段的下一行」档。页头 → 第一组一个字节不动。
				for i, section := range plugincfg.Sections(sheet.Schema) {
					section := section
					// Inserting the write-failure sentence must not replace every
					// field's identity and drop the focused edit before a retry.
					a.sheetSection(c.Key("plugin.section."+strconv.Itoa(i)), sheet, section, i > 0)
				}
			})
		})
		// P13-aw（`docs/liquid-glass.md` §6.19）：这块浮层是「滚动收口」这一族的
		// 最后一块面（§6.16 设置体 / §6.17 输出凹槽 / §6.18 多选箱之后）。改前
		// 它的内容在带子的两条边上被 `.Clip()` 平刀切。
		//
		// 门不是条件判断，是**几何**——与 §6.16 同一条机制，而不是 §6.17 / §6.18
		// 的 `ScrollState.Y` 门：这道边的两条都落在留白上。
		//
		// P13-dc（`docs/liquid-glass.md` §6.72）在**装好设计主题**的取景里重量了一遍
		// （此前 `pluginSheetFrame` 少了 `c.SetTheme`，量的是库存字的 14 / 4）：
		// 停在开头时带子最上 11 行是空白（墨从第 12 行开始，那是 2u 底衬 + 1u 顶留白
		// 与标题行的行盒上身）、滚到底时最下 11 行也是空白；边高 3u = 10.8px，
		// **余量 0.2px**。也就是说这条几何仍然成立，但没有余量——标题行一旦变高、
		// 顶留白一旦变窄，边就会盖进墨里，那时该换成 §6.17 那条 `Y` 门，而不是
		// 调小窗口（`TestThePluginSheetEdgesSitOnBlankAtBothEnds` 钉的就是这个前提）。
		//
		// 另外两条取舍：边高 3u 小于一个行盒，最多软化一行；边的底色是浮层自己的面、
		// 压成不透明（`sheetEdgeFace`）——`ui.Color.Alpha` 是**乘**，把 0.17 的
		// `Sunken` 直接交给 `ScrollEdge` 会把 0.85 乘成 0.14，边淡了个寂寞。
		face := sheetEdgeFace(a.tokens(c), t)
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(t.Space(3)).PassThrough().
			Material(glass.ScrollEdge{Background: face})
		ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(t.Space(3)).PassThrough().
			Material(glass.ScrollEdge{Bottom: true, Background: face})
	})
}

// sheetSection draws one group: its heading (when it has one) and the card
// holding its fields.
//
// P13-en：`opensGroup` says this group is not the sheet's first, so it carries the
// extra `Space(4)` above itself (§6.109). The air rides the *group*, not the
// column's `Gap`, because that column's gap is shared by three pairs — the same
// rule §6.106 settled for the settings pages.
func (a *App) sheetSection(c *ui.Context, sheet PluginSheet, section plugincfg.Section, opensGroup bool) {
	t := c.Theme()
	tokens := a.tokens(c)
	// The section is the settings pages' own: the title at the title step
	// (620, strong) — the old build's `.plugin-config__section` /
	// `__section-title`.
	//
	// P13-ej（`docs/liquid-glass.md` §6.105）：标题与它自己那张卡之间的空气此前是
	// `Space(2)`（7.2px，参考的 8u），量出来是**标题墨 → 第一行标签墨 25px**，而同一张
	// 卡里**上一行的 help 墨 → 下一行标签墨 23px**——分组只比两个邻居多 2px，等于没有
	// 分组。§4.3 的规矩是「一个新的东西抬开一整档」，所以这里走 Space(2) → **Space(4)**
	// （7.2 → 14.4），抬的是一整档 8u；改后 25 → 32px，与 23px 拉开 9px。
	group := ui.Column(c).FillWidth().Gap(t.Space(4))
	if opensGroup {
		group = group.Padding(t.Space(4), 0, 0, 0)
	}
	group.Children(func() {
		if section.Title != "" {
			// 分组标题坐在它下面那些行标签的列上：卡自己的内边距，加上 scroller 的。
			//
			// P13-dc（`docs/liquid-glass.md` §6.72）：这一格的字号此前是 body 的
			// **偏移量** `t.FontSize+2`（0.9 步上 12.8），阶梯上是 `tokens.Title`
			// （12.6）。偏移量在默认步上恰好看不出区别（同一个行盒），但它**不随
			// 界面大小缩放**：用户换一步，这一格就慢慢离开阶梯；阶梯上的档才是
			// 用户的放大。P13-db 量过的「改为零像素差」是在一条**没装设计主题**的
			// 夹具里量的，那条夹具本轮才修好（`pluginSheetFrame` 现在 `c.SetTheme`）。
			ui.Text(c, section.Title).FontSize(tokens.Title).FontWeight(620).
				TextColor(tokens.TextStrong).Padding(0, t.Space(3.5))
		}
		// An optional heading must not become part of the fields' identity.
		a.card(c.Key("plugin.section.fields"), func() {
			for _, field := range section.Fields {
				field := field
				a.sheetField(c, sheet, field)
			}
		})
	})
}

// sheetField draws one field with the control its kind names.
func (a *App) sheetField(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	// Field state follows its plugin and schema key, never its sibling index.
	key := sheet.Schema.Plugin + "\x00" + field.Key
	// A replaced control has no blur frame. Retire its edit here so returning
	// to that kind reads the host value immediately, without a stale flash.
	if field.Kind != plugincfg.Text {
		delete(a.pluginTextDrafts, key)
	}
	if field.Kind != plugincfg.Number {
		delete(a.pluginNumberDrafts, key)
	}
	c = c.Key("plugin.field." + key + "\x00" + strconv.Itoa(int(field.Kind)))
	// P13-gu：没有标签的字段回落到它的 key，与集成配置表单同一条规则
	// (`config.go` 的 `configField`：`if label == "" { label = field.Key }`)。
	// `Label` 是 schema 里的自由文本，空串合法：此前浮层给这种字段画一格
	// **空的标签列**（行高照旧、右边控件照旧），而同一个应用的另一张表单给
	// 同一个字段画的是 key。同一个字段两种说法，是这一族里最便宜的一条。
	//
	// 回落发生在**这一份副本**上，所以下面每一种 kind 与 `sheetAction` 的按钮
	// 文字都一起接住（action 的胶囊此前是一枚零宽的空药丸）。
	if field.Label == "" {
		field.Label = field.Key
	}
	value := sheet.Values[field.Key]
	switch field.Kind {
	case plugincfg.Toggle:
		on, _ := value.(bool)
		a.checkbox(c, field.Label, field.Help, on, func(next bool) { sheet.Change(field.Key, next) })
	case plugincfg.Select:
		current, _ := value.(string)
		a.choose(c, field.Label, field.Help, optionsOf(field.Options, a.copy().ConfigNone), current,
			func(next string) { sheet.Change(field.Key, next) })
	case plugincfg.Radio:
		current, _ := value.(string)
		a.sheetRadio(c, sheet, field, current)
	case plugincfg.Checkboxes:
		chosen := stringsOf(value)
		a.sheetCheckboxes(c, sheet, field, chosen)
	case plugincfg.Slider:
		number, _ := value.(float64)
		a.sheetSlider(c, sheet, field, number)
	case plugincfg.Number:
		number, _ := value.(float64)
		a.sheetNumber(c, sheet, field, number)
	case plugincfg.Text:
		text, _ := value.(string)
		// The host's answer to the last submit is in this frame's values.
		//
		// P13-hk（`docs/liquid-glass.md` §6.186）：文本字段在**回车那一帧**把框里
		// 的原文交上去，而宿主会把值收一遍（`plugincfg.coerce`：去首尾空白，全空白
		// 记成空值）。框里此前一直画着交上去的原文，于是「框写 ` /tmp/x `、插件存
		// `/tmp/x`」要等到失焦才对齐——同一个字段在**同一帧**有两个说法。
		//
		// 成功确认后退役这次提交的草稿，即使宿主原样接受，也交回宿主值。
		// 否则后续外部刷新永远被旧草稿遮住。失败留给重试，提交后新打的
		// 文字也保留：只能退役与 sent 相同的那份草稿。
		if sent, ok := a.pluginTextCommits[key]; ok {
			delete(a.pluginTextCommits, key)
			if !sheet.Failed && a.pluginTextDrafts[key] == sent {
				delete(a.pluginTextDrafts, key)
			}
		}
		edit := text
		if draft, ok := a.pluginTextDrafts[key]; ok {
			edit = draft
		}
		// The reference stacks a free-text field: it takes the row's full
		// width, not a trailing slot (`.plugin-config-text { width: 100% }`,
		// `plugin-config.css:110-113`; `STACKED_KINDS` in
		// `plugins/controls.tsx:291-296`).
		a.stackedRow(c, field.Label, field.Help, func() {
			input := a.sheetTextField(c, field.Placeholder, &edit).Label(field.Label)
			if input.Changed() {
				if a.pluginTextDrafts == nil {
					a.pluginTextDrafts = map[string]string{}
				}
				a.pluginTextDrafts[key] = edit
			}
			if input.Submitted() {
				// P13-hk（§6.186）：回车交一份**和宿主一样**的文本不是一次改动。
				// 此前它照样派发：宿主再写一遍同一个值，写成功就是白写一遍，写
				// 被拒就在一块没改过的面上亮出「保存失败」。现在只有真的不一样
				// 才交上去。
				if edit != text {
					sheet.Change(field.Key, edit)
					if a.pluginTextCommits == nil {
						a.pluginTextCommits = map[string]string{}
					}
					a.pluginTextCommits[key] = edit
				} else {
					// The user confirmed the already stored value. It must not
					// mask subsequent external updates while focus stays here.
					delete(a.pluginTextDrafts, key)
					delete(a.pluginTextCommits, key)
				}
			}
			// A refused write must leave the focused edit available for retry.
			// On blur, read the host's confirmed (possibly normalized) value.
			if !input.Focused() {
				delete(a.pluginTextDrafts, key)
			}
		})
	case plugincfg.Action:
		a.sheetAction(c, sheet, field)
	}
}

// sheetRadio is a stacked row: the label and help above, the choices under
// them, as the old build's radio group drew.
func (a *App) sheetRadio(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current string) {
	field.Options = uniquePluginOptions(field.Options)
	labels := make([]string, len(field.Options))
	keys := make([]string, len(field.Options))
	index := 0
	for i, option := range field.Options {
		labels[i] = pluginOptionLabel(option, a.copy().ConfigNone)
		keys[i] = option.Value
		if option.Value == current {
			index = i
		}
	}
	chosen := index
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		control.SegmentedFill(c, a.tokens(c), labels, chosen, func(i int) {
			if i >= 0 && i < len(field.Options) {
				chosen = i
				changed = true
			}
		}, keys)
	})
	if changed {
		sheet.Change(field.Key, field.Options[chosen].Value)
	}
}

// sheetCheckboxes is the same stacked shape, with a check box per option.
func (a *App) sheetCheckboxes(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current []string) {
	field.Options = uniquePluginOptions(field.Options)
	t := c.Theme()
	on := make([]bool, len(field.Options))
	for i, option := range field.Options {
		for _, value := range current {
			if value == option.Value {
				on[i] = true
			}
		}
	}
	pressed := -1
	a.stackedRow(c, field.Label, field.Help, func() {
		ui.Row(c).FillWidth().Gap(t.Space(3)).Wrap().Children(func() {
			for i, option := range field.Options {
				i, option := i, option
				checked := on[i]
				label := pluginOptionLabel(option, a.copy().ConfigNone)
				box := ui.Checkbox(c.Key("plugin.option."+option.Value), &checked, "").Label(label).MaxWidthPercent(100)
				box.Children(func() { ui.Text(c, label).MinWidth(0).SingleLine() })
				if box.Changed() {
					on[i] = checked
					pressed = i
				}
			}
		})
	})
	if pressed >= 0 {
		next := make([]string, 0, len(field.Options))
		for i, option := range field.Options {
			if on[i] {
				next = append(next, option.Value)
			}
		}
		sheet.Change(field.Key, next)
	}
}

// sheetSlider is the old build's capacity row: the label and help above, the
// track and its value under them, the value quantized to the schema's step
// and trailed by the schema's unit word.
func (a *App) sheetSlider(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	t := c.Theme()
	tokens := a.tokens(c)
	// Position and readout describe the same value, including values loaded
	// from an older configuration that are between the current step marks.
	v := boundedValue(field, value)
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		// The reference's track-and-readout row: a 10u gap between the track
		// and its value (`.plugin-config-slider { gap: 10u }`,
		// `plugin-config.css:165-172`).
		ui.Row(c).FillWidth().Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			e := ui.Slider(c, &v, field.Min, field.Max).Grow(1)
			if e.Changed() {
				changed = true
			}
			// The value is the reference's `.plugin-config-slider__value`
			// (`plugin-config.css:174-192`): the caption step, muted ink and
			// tabular figures, right-aligned in a 48u column so a dragged
			// number does not shift its neighbours.
			ui.Row(c).MinWidth(t.Space(12)).Justify(ui.End).Children(func() {
				ui.Text(c, boundedText(field, v)).FontSize(tokens.Caption).
					TextColor(t.TextMuted).FontFeatures("tnum").SingleLine()
			})
		})
	})
	if next := boundedValue(field, v); changed && next != boundedValue(field, value) {
		sheet.Change(field.Key, next)
	}
}

// sheetNumber is a trailing row: the label and help at the start, the
// reference's number control at the end.
func (a *App) sheetNumber(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	v := value
	changed := false
	a.row(c, field.Label, field.Help, func() {
		var slotChanged bool
		_, slotChanged = a.sheetNumberSlot(c, sheet.Schema.Plugin, field, &v)
		changed = slotChanged
	})
	if changed {
		// P13-hk（§6.186）：数字框交给宿主的数与滑块交给宿主的是同一条账—
		// 落在这个字段自己的网格上、收进它声明过的界。此前只 `clampNumber`，
		// `Step: 1` 的字段可以交上去一个 3.7，宿主再按网格收成 4：这一帧的
		// 派发值（3.7）与插件实际存下的数（4）不是同一个。
		sheet.Change(field.Key, boundedValue(field, v))
	}
}

// sheetNumberSlot draws the reference's number control (R59): the unit rides
// *inside* the field slot beside the input, and the input right-aligns against
// it (`plugin-config.css:120-164`). It is deliberately **not** the framework's
// `ui.NumberInput`: that control draws its own bordered pane and a pair of
// `−`/`+` steppers the reference never had, so inside the reference's 28u slot
// it read as a box within a box. A parseable keystroke applies at once and the
// caller clamps the reported value to the schema's bounds; while the box keeps
// the keyboard, its in-progress text lives in `pluginNumberDrafts` so a
// half-typed or emptied value is not overwritten by the stored one mid-edit.
//
// The draft is keyed by `plugin` + the field key, exactly as the text field's
// `pluginTextDrafts` is (`sheetField`): one settings App draws every plugin's
// sheet in turn (`shell/pluginconfig.go`), and a field key such as `size` is
// not unique across plugins. Keying the number draft by the field key alone
// let plugin A's half-typed `1e` survive the switch and paint inside plugin B's
// box in place of B's own stored value.
//
// It returns the slot for a caller that wants to bind it, and whether the
// frame's typing changed the value.
func (a *App) sheetNumberSlot(c *ui.Context, plugin string, field plugincfg.Field, value *float64) (ui.Element, bool) {
	t := c.Theme()
	tokens := a.tokens(c)
	if a.pluginNumberDrafts == nil {
		a.pluginNumberDrafts = map[string]string{}
	}
	key := plugin + "\x00" + field.Key
	draft, ok := a.pluginNumberDrafts[key]
	if previous, observed := a.pluginNumberObserved[key]; observed && previous != *value && ok {
		// Only a draft matching the previously confirmed value can follow a
		// later host update. Partial and refused edits remain the user's.
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(draft), 64); err == nil && parsed == previous {
			delete(a.pluginNumberDrafts, key)
			ok = false
		}
	}
	if a.pluginNumberObserved == nil {
		a.pluginNumberObserved = map[string]float64{}
	}
	a.pluginNumberObserved[key] = *value
	if !ok {
		draft = initialNumberText(field, *value)
	} else if parsed, err := strconv.ParseFloat(strings.TrimSpace(draft), 64); err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
		// A refreshed schema can move the bounds while the input keeps focus.
		// Rebase only a complete number; partial input remains the user's.
		if bounded := boundedValue(field, parsed); bounded != parsed {
			draft = initialNumberText(field, bounded)
			a.pluginNumberDrafts[key] = draft
		}
	}
	changed := false
	// The number box is the field slot plus the reference's 4u gap between
	// the input and its unit (`.plugin-config-number { gap: 4u }`,
	// `plugin-config.css:153-157`).
	slot := a.sheetFieldSlot(c).Gap(t.Space(1))
	slot.Children(func() {
		// The input is the reference's own `.plugin-config-number__input`: 52u
		// wide, right-aligned, tabular figures, no box of its own (the slot is
		// the box).
		text := draft
		in := ui.TextInputBase(c, &text).Width(t.Space(13)).
			TextAlign(ui.End).FontFeatures("tnum").Label(field.Label)
		if in.Changed() {
			a.pluginNumberDrafts[key] = text
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
				// P13-gu：框里写的必须是插件**真正拿到**的那个数。越界的输入
				// 此前被 `clampNumber` 悄悄夹到边界再派发，而框里还留着原样的
				// `999`——用户以为存了 999，插件收到 100，直到失焦才跳回。
				// 现在夹界与步长量化都把结果写回草稿，框与值同帧对齐；
				// 界内的输入一个字节不动（半输入 `1e` 仍然留在框里，§6.162）。
				clamped := boundedValue(field, parsed)
				changed = clamped != boundedValue(field, *value)
				*value = clamped
				if clamped != parsed {
					a.pluginNumberDrafts[key] = initialNumberText(field, clamped)
				}
			}
		} else if !in.Focused() {
			// The keyboard is elsewhere: the field reads the stored value
			// again, so a refused write never strands a half-typed one.
			delete(a.pluginNumberDrafts, key)
		}
		if field.Unit != "" {
			// `.plugin-config-number__unit` is the caption step in the muted
			// register (`plugin-config.css:159-164`), not the control's body
			// step.
			ui.Text(c, field.Unit).FontSize(tokens.Caption).
				TextColor(t.TextMuted).SingleLine()
		}
	})
	return slot, changed
}

// sheetFieldSlot draws the overlay's field box (R59, `plugin-config.css:97-118`):
// the reference's 28u control — a text or number control's value lives inside
// it; the control itself is the caller's child, so the slot never draws a
// second surface under it.
//
// P8-b 重定基线：面 `--glass-field` + `--glass-field-shadow` 凹槽 → 合约 §6.1 的
// 低光泽一档。几何（28u 高、`RadiusSM` 圆角、`ControlEdge` 发丝、8u 侧 padding）
// 全部保留，只把面收敛到 `Control`/`KeycapFace` 一档：一个字段槽不需要额外的
// 凹痕叠层，因为它坐在已经比卡更亮的面里。
func (a *App) sheetFieldSlot(c *ui.Context) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	slot := ui.Row(c).MinHeight(t.Space(7)).AlignItems(ui.Center).
		Padding(0, t.Space(2)).Radius(tokens.RadiusSM)
	// P12-c（§6.2 第 2-4 行）：弹层的字段槽与设置页的 `fieldFace` 是同一个
	// 材质——同一个应用里字段只有一种面。`LitFace` 的顶受光边 + 底对侧暗线
	// 也是同一对数值，不另起一套。
	//
	// P13-cc（§6.46）：那两行 `Border`/`Background` 曾经在这里又写了一遍（与
	// `fieldFace` 逐字节相同），所以「字段只有一种面」靠的是两处人肉同步。现在
	// 面、外圈与受光边都交给 `fieldFace`：这里只剩几何。
	return a.fieldFace(c, slot, true)
}

// sheetTextField draws the overlay's own text control (R59,
// `plugin-config.css:97-118`). It is deliberately not the framework's
// `ui.TextInput`: the reference's field is a 28u slot cut into the glass — a
// `--glass-field` fill, a `--glass-control-edge` hairline, the `--radius-sm`
// corner, the body step and 8u of side padding, with the field's hairline and
// inset shadow telling the user the value lives inside it. The input itself is
// the framework's look-free base, so its own padding, surface and border do
// not fight the slot.
func (a *App) sheetTextField(c *ui.Context, placeholder string, value *string) ui.Element {
	slot := a.sheetFieldSlot(c).FillWidth()
	var input ui.Element
	slot.Children(func() {
		input = ui.TextInputBase(c, value).Placeholder(placeholder).Grow(1)
	})
	// Submitted and Label belong to the editor; input events do not bubble
	// through the decorative slot.
	return input
}

// sheetActionButton draws one of the overlay's action buttons (R38,
// `plugin-config.css:223-260`). It is deliberately **not** the framework's
// filled `Button`: the reference's control is a quiet hairline pill that
// spends no accent fill — a 28u height, 10u of side padding, a
// `--glass-control-edge` hairline, a `999px` corner (half the 28u height, so
// the shape is a pill), the `--glass-control` plane and the `--text-caption`
// step in `--text-primary`. The danger variant is the one the destructive
// command wears: `color-mix(in srgb, var(--text-warning) 45%, transparent)` on
// its hairline, `--text-warning` on its ink, and a transparent plane, so it
// reads as the destructive action without becoming the overlay's primary one.
// Hover lifts either face to `--glass-control-hover`.
func (a *App) sheetActionButton(c *ui.Context, label string, danger bool, run func()) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(t.Space(7)).
		Padding(0, t.Space(2.5)).
		// P13-gv（`docs/liquid-glass.md` §6.169）：弹层里的胶囊此前一帧跳到悬停面，
		// 而同族的 `extensionActionFace` / `barButton` / `uninstallActionButton`
		// 都在 120ms 里走过去（§5 第一行）。面与边都只动颜色，所以是 `Colors`。
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep}).
		// P13-eo（`docs/liquid-glass.md` §6.110）：这枚胶囊的四角此前是 `RadiusSM`(8)
		// ——**圆角矩形**，而它上面那段注释、参考的 `plugin-config.css:241`
		// (`border-radius: 999px`) 与守它的那条 pin 都写着「胶囊」。
		//
		// 胶囊的半径只能是**半高**，而半高跟着 `--ui-scale` 走：把半径钉成字面 8，
		// 换一档界面大小就从胶囊漂成圆角矩形（盒在 `large` 档更高，8 不变）。
		// 参考的 `999px` 就是「永远半高」的写法，本仓的等价物是 `Space(3.5)`
		// （= `Space(7)` 的一半，随档缩放），与扩展页的 `actionButton`
		// （`t.Space(3.75)` 对 `t.Space(7.5)` 高）是同一条算术。
		// 半径不进布局：盒高、10u 侧 padding 与标签的缩进一个字节不动。
		Radius(t.Space(3.5))
	ink := t.Text
	edge := tokens.ControlEdge
	plane := tokens.Control
	if danger {
		ink = t.Warning
		edge = t.Warning.Alpha(0.45)
		plane = ui.Transparent
	}
	// P13-q（`docs/liquid-glass.md` §5）：弹层里的胶囊也有第三档。按下时面沉到
	// `--glass-control-press` 并按 `.extensions-action-button` 的几何收 0.97；
	// `danger` 臂静息的面是透明的，按下时才有一块能被压下去的面（白线仍只在
	// 有面时画，§1 禁令 3）。
	pressed := button.Pressed()
	if pressed {
		plane = tokens.ControlPress
	} else if button.Hovered() {
		plane = tokens.ControlHover
	}
	button.Border(1, edge).Background(plane)
	// P13-p（`docs/liquid-glass.md` §6.2b-4 第 9-20 行）：弹层里的一切是 E1（§2 判据 1），
	// 而这枚胶囊是本页唯一没戴那一对线的——同族的 `extensionActionFace`（9-1）、
	// `barButton`（9-5）、卸载对话框的动作胶囊（9-6）都戴了。受光边只在**有面**的
	// 时候画：`danger` 臂静息的面是透明的（`ui.Transparent`），在一块透明的面上
	// 画一条 0.09 的白线就是 §1 禁令 3 的「类玻璃边」；指针把面点亮之后它才有一块
	// 能站起来的玻璃（悬停档与中性臂同一个面）。外圈的发丝一个字节不动。
	if plane.A > 0 {
		tokens.LitFace(button)
	}
	if pressed {
		button = button.Scale(0.97, 0.97)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).TextColor(ink)
	})
	if button.Clicked() {
		run()
	}
	return button
}

// sheetAction is the two-step button: the first press arms it (the button
// becomes the confirm question, with the way back beside it), the second
// runs the command. The status line is the sheet's own failure caption.
func (a *App) sheetAction(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	t := c.Theme()
	armed := sheet.Armed == field.Key
	// The row's own label is a hit target, as it is for every other kind: it
	// arms the button, so the whole row answers the first press.
	clicked := a.row(c, field.Label, field.Help, func() {
		// The reference's cluster: inline-flex, flex-end, a 6u gap
		// (`.plugin-config-action`, `plugin-config.css:218-224`).
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Wrap().Children(func() {
			if armed {
				// The armed button is the old build's danger button: the
				// same quiet hairline pill, its own warning face.
				a.sheetActionButton(c, field.Confirm, true, func() { sheet.Run(field.Key) })
				a.sheetActionButton(c, field.Cancel, false, func() { sheet.Unarm(field.Key) })
				return
			}
			if sheet.ActionFailed == field.Key && field.Failed != "" {
				// The refused run's line sits beside the button, not in the
				// sheet's write-failure slot: the old build's
				// `.plugin-config-action__status`, the field's own wording.
				ui.Text(c, field.Failed).FontSize(a.tokens(c).Caption).TextColor(t.Warning)
			}
			a.sheetActionButton(c, field.Label, true, func() { sheet.Arm(field.Key) })
		})
	})
	if clicked && !armed {
		sheet.Arm(field.Key)
	}
}

// optionsOf converts a schema's choices to the settings pages' option shape,
// so one dropdown helper serves both.
func uniquePluginOptions(options []plugincfg.Option) []plugincfg.Option {
	seen := make(map[string]bool, len(options))
	unique := make([]plugincfg.Option, 0, len(options))
	for _, option := range options {
		if !seen[option.Value] {
			seen[option.Value] = true
			unique = append(unique, option)
		}
	}
	return unique
}

func optionsOf(options []plugincfg.Option, empty ...string) []i18n.Option {
	out := make([]i18n.Option, 0, len(options))
	for _, option := range options {
		out = append(out, i18n.Option{ID: option.Value, Label: pluginOptionLabel(option, empty...)})
	}
	return out
}

// Schemas may omit a display label; the stored value still names the choice.
func pluginOptionLabel(option plugincfg.Option, empty ...string) string {
	if option.Label != "" {
		return option.Label
	}
	if option.Value != "" || len(empty) == 0 {
		return option.Value
	}
	return empty[0]
}

// boundedText is a bounded value with its unit: "300 items", "30 days".
//
// P13-gu（`docs/liquid-glass.md` §6.164）：读数此前是 `int(math.Round(v))`，两条账
// 都错，而它们说的是同一件事——**读数必须是会被保存的那个数**。
//
//	① `Step < 1` 的字段把分数抹成整数：第三方 schema 可以声明 0.1 的步长
//	   （`plugincfg.Schema` 不限制 Step 的粒度），0.4 读成「0 items」。
//	② 读数取的是拖拽中的**未量化**值，而 `sheetSlider` 派发的是
//	   `quantize(v, Step)`：Step 10 拖到 12.7，框里写「13 items」、插件存 10。
//
// 现在先量化（`quantize`，Step 0 时原样），再按六位小数收口（与 `clampStep`
// 同一条，避免 `0.1 × 4` 打印成 `0.4000000000000001`），最后用最短写法。
// 整数值的读数逐字节不变（`300` → "300"）。
//
// P13-hk（§6.186）：读数还要先**收进 schema 自己的界**。同一条账的第三笔：一个
// 手改过的配置文件（或旧版本写下的值）把 `max_items` 放到界外时，轨道按框架自己的
// 区间停在头上，而读数照样印着界外的那个数——一条横线说「到这里了」，旁边说
// 「9999 items」。收进界之后，读数就是拖一下会存下的那个数（也是 `coerce` 的
// `clampStep` 会保留的那个）。
func boundedText(field plugincfg.Field, value float64) string {
	value = roundedNumberTextValue(boundedValue(field, value))
	text := formatFieldNumber(value)
	if math.Abs(value) >= 1e21 {
		// Keep an extreme finite readout from taking the slider's width.
		text = strconv.FormatFloat(value, 'g', -1, 64)
	}
	if field.Unit != "" {
		text += " " + field.Unit
	}
	return text
}

// initialNumberText is what a number box with no draft of its own reads: the
// stored value, held inside the field's declared bounds and on its step grid.
// A hand-edited settings file can hold 70000 in a field whose maximum is 65535;
// the box used to print that raw, so the first paint disagreed with every write
// the field can make.
func initialNumberText(field plugincfg.Field, value float64) string {
	value = boundedValue(field, value)
	if field.Step > 0 {
		// Match the host's six-decimal grid spelling without altering writes.
		value = roundedNumberTextValue(value)
	}
	return formatFieldNumber(value)
}

// Scaling a finite large value by 1e6 can overflow. At that magnitude it has
// no fractional digits to round, so preserve its finite decimal spelling.
func roundedNumberTextValue(value float64) float64 {
	if math.Abs(value) > math.MaxFloat64/1e6 {
		return value
	}
	return math.Round(value*1e6) / 1e6
}

// boundedValue is the number a bounded control hands the host: the value on the
// field's own step grid, held inside the field's declared bounds. The readout
// draws this same number (§6.164: the readout is what gets saved), so the two
// cannot drift.
//
// P13-hk（§6.186）：两条账。
//
//  1. 网格锚在 `Min` 上（见 `quantize`），与 `plugincfg.coerce` 的 `clampStep`
//     同一条；`Min` 落在网格上时逐字节不变。
//  2. 量化之后还要收进**声明过的**界：网格四舍五入能把 96 取到 100，而字段的
//     `Max` 是 99——`coerce` 会夹回 99，于是框里的读数（100）和插件真正存下的
//     数（99）就差着一格。声明过的界 = `Max > Min`：零宽的 `Min`/`Max` 是
//     「没写界」的零值（`plugincfg.Field` 的默认），不是「必须等于 0」。
func boundedValue(field plugincfg.Field, value float64) float64 {
	value = quantize(field, value)
	if field.Max > field.Min {
		value = clampNumber(field, value)
	}
	return value
}

// quantize rounds a dragged value to the schema's step, so a slider that
// steps by ten lands on round numbers.
//
// P13-hk（§6.186）：网格**锚在该字段自己的 `Min` 上**，不是锚在 0 上。
// `plugincfg.coerce` 的 `clampStep` 用的是 `round((n-lo)/step)*step + lo`，同一条
// 规则；而这里此前是 `round(v/step)*step`。`Min` 落在步长网格上时两者逐字节相同
// （本仓内置的 `max_items`、`history_days` 都是这种），但 `Field.Min` / `Step`
// 是自由浮点：`Min: 1, Step: 10` 的字段在 1.5 处会量化成 **0**——一个比下界还小、
// 宿主立刻夹回 1 的数，而这一帧的读数、滑块位置和派发值都在说 0。
func quantize(field plugincfg.Field, value float64) float64 {
	if field.Step <= 0 {
		return value
	}
	return math.Round((value-field.Min)/field.Step)*field.Step + field.Min
}

// formatFieldNumber is the number a number field edits: the shortest decimal
// spelling of the value, so "30" shows as "30" and not "30.000000".
func formatFieldNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// clampNumber holds a typed value inside the schema's bounds, as the
// framework's number input clamped a stepped one.
func clampNumber(field plugincfg.Field, value float64) float64 {
	if value < field.Min {
		return field.Min
	}
	if value > field.Max {
		return field.Max
	}
	return value
}

func stringsOf(value any) []string {
	list, _ := value.([]string)
	return list
}

// language is the stored interface language, for the copy the sheet owns.
func (a *App) language() string { return a.settings().Language }
