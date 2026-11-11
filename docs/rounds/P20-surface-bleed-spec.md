# P20 · 面板画到卡片边缘，去掉四周内边

用户反馈：「设置页面四周有和背景不同的颜色内边，这太不符合桌面应用逻辑了…包括搜索页，
搜索框顶部也有一条灰线，不知道是不是故意的，但是不好看，需要一体性更强。」

## 现状（改前）

`internal/shell/shell.go` 的 `View` 给承载表面的那一列加了内边距，再把卡片自己的
`LinearGradient` 面画在它周围：

| 表面 | 内边距 | 结果 |
| --- | --- | --- |
| 设置 | 左右 `Space(2.5)` = 9px，上 `Space(1)` = 3.6px，下 `Space(2.5)` | 正文的 `Sunken` 面之外还有一圈更亮的窗口面 |
| 搜索 | 上 `Space(1)`、下 `Space(0.5)` | 搜索框上方一条 3.6px 的亮带（用户读作「灰线」） |
| 终端 | 0 | 不受影响 |

卡片*就是*窗口（`base.css` 的 `.collapsed-card { width: 100% }`），所以这圈内边距只可能
是应用自己画的一圈边框——和早先 Windows 上报的「窗口周边外圈还有一圈额外的底色」同一
形状，只是这次每个平台都有。

## 落成

| # | 位置 | 改法 |
| --- | --- | --- |
| 20-1 | `internal/shell/shell.go` `View` 的内边距列 | 不再加任何 gutter / inset（`padX`/`padTop`/`padBottom` 全归零并写清理由） |
| 20-2 | `internal/settingsui/settingsui.go` `View` 的根列 | 自己付页头的 4u：`Padding(t.Space(launcher.TopInsetUnits), 0, 0, 0)`。`Background` 画的是 border box，所以 `Sunken` 仍到卡片边缘，只有内容下移 |
| 20-3 | `internal/launcher/launcher.go` 的 `panel` | 自己付面板的 4u：`Padding(t.Space(TopInsetUnits), 0, 0, 0)`；三个绝对定位带（字段行、字段底线、chips 行）各自在 `Top` 上补同一个 `TopInsetUnits`，因为绝对子元素落在父的 border box 里，看不见 padding。2u 尾已在 `actionBarUnits` 里收过，不重复 |

## 判据

`internal/shell/panelring_test.go` 的 `TestNoSurfaceWearsARingOfTheCardsFace`：
浅/深 × 搜索页 + 五个设置页，读卡片描边内 3px 处的顶/左/右三边（底边不读：设置正文的
滚动淡出与横向滚动条都在那里，两者都是设计有意画的带），与启动器面板的 `Sunken` 像素
比，超过 6 级即报。改前逐条红（浅色差 21 级、深色差 14 级），改后全绿。

重定基线（V）：`internal/launcher/fieldsurface_test.go` 的 `floor` 从
`int(fieldRowUnits*spacing)-1` 改成 `int((TopInsetUnits+fieldRowUnits)*spacing)-1`——
字段带连同它的底线一起下移 4u，带子高度与窗口高度链一个数没动。

## 验证

`internal/settingsui`、`internal/launcher`、`internal/shell` 三包全量失败集与改前**逐条
相同**（35 / 4 / 14 条既有失败，其中 `TestTheSearchCardFillsTheWindowLessTheInset` 等
在改前就红）；`internal/ci`、`internal/theme`、`internal/glassmap` 全绿；终端包失败集
与改前相同（17 条既有失败）；`gofmt -l`、`go vet`、darwin/windows/linux 三平台
`go build ./...` 通过。

## 未做（说明）

- 未动终端页：它本来就没有内边距，画布自己到边。
- 未动插件窗口（`detach.go` / `pinned.go`）：它们是不透明窗口，根就是 `tokens.Opaque`，
  没有内边距带。
- 未动设置正文的滚动淡出与横向滚动条：那是「还有内容」的提示，不是内边框。

---

# P20 追加 · 命令结果页的顶部空白 + 底部动作条的 `⌘K`

## 用户反馈

「去掉进入终端的 super+k 快捷键保留一个 super+回车就行了。如图输入框下快捷执行后，
展示结果上方会有一块和输入框同等高度的空白，这个需要去掉，但不要影响其他地方。」

## 两条各自的根因

### ① 结果页顶部那块空白（第二轮改判）

第一轮的判断**错了**：当时以为这一页本来就不画输入框，于是把 `edge` 整条去掉、
`View` 里输出分支直接 `return`。用户随即指出真正的形状：

> 「之前应该是直接在搜索页下显示的，输入框应该保持，只是列表部分展示结果，
> 结果分纯文本和终端渲染。」

对照原版确认：输出是在**输入行下方**替换结果列表，输入行保留（`main:src/App.tsx`
`:3634-3642`，`pluginText` 画在 `.launcher-bottom` 里、`LauncherResults` 的那个位置）。

真正的根因是 `View` 里那个 `return`：它让整张**面板**都不再建字段行，于是

- 输入框整条消失（`tt.Find("Search")` 找不到）；
- 而它本该占的那条带仍被 `Padding` 保留 —— 那才是用户看到的那块空白。

| # | 位置 | 改法 |
| --- | --- | --- |
| 20-4 | `internal/launcher/launcher.go` `View` | 删掉输出分支的 `return`；输出改为与配置表单、空态并列的一支，**只替换列表带** |
| 20-5 | 同上，`outputBody` 的调用点 | 恢复 `edge`（`fieldRow + 呼吸`）：输出从字段带下方开始，与结果列表同一个上沿 |
| 20-6 | `internal/launcher/output.go` `outputBody` | 保留 `edge` 参数与 `Padding(edge, …)`，即结果列表自己的上沿 |

### ①b 「搜索栏不动」的另外两处破口（第三轮）

改完 20-4/20-5 后逐帧量三个状态（已输入 / 执行中 / 输出已到），搜索栏的三枚控件
（`Search` / `Clear` / `Settings`）的盒子仍然会变，两处：

| # | 位置 | 改前 | 改法 |
| --- | --- | --- | --- |
| 20-9 | `internal/launcher/triggerhint.go` `triggerHintText` | `a.output != nil` 时**吞掉**行内提示（「␣ v 进入」）。输出一到，提示消失，字段瞬间宽 **110px** | 输出不是「占用了字段的模式」，提示是关于**字段**的，字段没变；只有 `pluginModeActive()` 才吞 |
| 20-10 | `internal/launcher/launcher.go` `runMode` | 运行前 `a.Query = ""`（注释写「旧版的运行会清空字段」——**旧版并没有**：`runExternalCommand` 只设 `externalRun`，`query` 不动，`main:src/App.tsx:950-966`） | 不再清空：字段**就是**命令行，运行属于它，参数保持可读可重跑。运行**也不离开模式**（见 20-17） |

### ①c 执行中的反馈（用户提的「loading 动画」）

异步命令从 Enter 到回调之间原本什么都不发生。现在列表带里画一行
「◌ 正在执行…」：环是 `footerSpinner` 的那一枚（0.7 s 一圈、同一个墨），
词是新条目 `OutputRunning`。

| # | 位置 | 改法 |
| --- | --- | --- |
| 20-11 | `internal/launcher/launcher.go` `App.running` | 新字段：正在执行的**那条查询**（空串为无） |
| 20-12 | `internal/launcher/search.go` `runCommandWith` | 发起时 `a.running = a.Query`，回调里清空（失败也清——失败仍会显示输出页，原因在里面） |
| 20-13 | `internal/launcher/output.go` `runningBand` | 在**列表带**（`edge` 起、居中）画环 + 词；`View` 里它排在 `output` 之前，条件是 `a.running == a.Query` |
| 20-14 | `internal/launcher/launcher.go` 字段的 `Changed()` | 编辑字段即放弃这条提示（`a.running` 与 `a.Query` 不再相等），列表立刻回来 |
| 20-15 | `internal/i18n` | 新条目 `OutputRunning`（en `Running…` / zh `正在执行…`） |
| 20-16 | `internal/launcher/output.go` `leaveOutput` | Escape 不再清空字段（旧版 R31 明写「the user's own words are never thrown away by a dismissal」，`main:src/App.tsx:1151-1161`）；它只让列表带回来 |

### ①d 回车不退回搜索页（第四轮）

上一轮修掉了「字段被清空」，但 `runMode` 还留着 `leaveCommand()`：回车后**模式**
关掉，scope 标记消失，参数被交回普通搜索——`-m` 变成一次搜 `-m` 的查询（「No
results」），想重跑得重新找到命令行。这正是用户说的「自动切回到最初的搜索状态」。

旧版从来没有这么做：`runExternalCommand` 只写 `externalRun`，`pluginMode` 与 `query`
都不动（`main:src/App.tsx:950-966`），注释里也写着「the command is re-run by editing
the arguments and pressing Enter on a fresh (empty) list」
（`main:src/App.tsx:1484-1489`）——运行是模式**内部**的一件事，不是模式的结束。

| # | 位置 | 改前 | 改法 |
| --- | --- | --- | --- |
| 20-17 | `internal/launcher/launcher.go` `runMode` | `a.leaveCommand()` 后 `runCommandWith` | 不再 `leaveCommand()`：只跑，模式与文本都留着。Escape 仍是唯一的出口（逐层：先输出、再模式、最后查询），且每层都保留文本 |

### ①e 结果页按自己画出的高度开窗（第五轮）

用户的报告：「现在显示的内容并不算很多，但还是触发了滚动条」。三个数在同一页上
互相不一致，一处欠、一处多：

| # | 位置 | 病 | 改法 |
| --- | --- | --- | --- |
| 20-18 | `internal/launcher/height.go` `Geometry` | 结果页正文**上方**那条带（命令行标题 + 结局句）画出来了，却从未被计入窗口高度——每个输出都矮这一条 | 新字段 `OutputHeader`（**像素**，不是 spacing 单位：它是一行字，行盒不是任何单位的整数倍），由 `outputHeaderHeight` 从 `MeasureText` 量出，`View` 传进 `Geometry`，`Height` 与其它带一起相加 |
| 20-19 | `internal/launcher/output.go` `pluginTextBlock` | 文本形态按 40u **整档**计价（旧版 `pluginViewRows × ROW_HEIGHT_TWO_LINE`），3 行文字被算成 2 档 = 80u，实际画 3 × 14.8 = 44.5 + 呼吸 —— 差 25px，于是开窗即滚、末行被切 | 按**真实行盒**计价：`max(3 行, 实际行数) × SubtitleLine`，上限仍是旧版那 10 档（`pluginTextMaxUnits`，超过就自己滚）。下限也从「参考的 72u」换成「本 build 画的 3 行」，一行回答下面不再留 25px 空白 |
| 20-20 | `internal/launcher/height.go` `RowInfo.Block` | — | 新增：一个自己量过自己的块（像素），与 `Band`（spacing 单位档）并列，文本形态走它 |
| 20-21 | `internal/launcher/output.go` `closeOutput` | 离开结果页后 `outputHeader` 不清零，搜索列表会被算上一条它不画的带 | 随页面一起清零 |

### ①f 集成页丢失窗口玻璃（第六轮）

用户报告：「添加工具中 本地工具页就有问题」。量出来的现象是**整窗不透明**：同一帧
里其他设置页的卡面是 alpha **207**，本地工具页是 **255**，而且连卡片圆角外那几
个本该透明 `{0,0,0,0}` 的像素也变成 `{14,14,16,255}` —— 是整张窗口被重画了一层，
不是某个控件画了黑底。

| # | 位置 | 病 | 改法 |
| --- | --- | --- | --- |
| 20-22 | `internal/settingsui/rows.go` `segmentedControlKeyed` | 用 `c.SetTheme(&pinned)` / `c.SetTheme(saved)` 切回「集成卡的字面 px」度量。`Context.SetTheme` **不是**作用域切换：它同时写根节点背景（`ui/context.go:349-355` 的 `c.root.bg = t.Background`），而 shell 正是把根刷成 `Transparent` 让玻璃透出桌面（`internal/shell/shell.go` 的 `c.Root().Background(ui.Transparent)`）。于是「恢复」这一步把整个窗口涂成了主题的不透明底色 | 改用 `ui.Themed(c, t).Background(ui.Transparent)` 包裹：作用域正确，根不动。度量经 `cardTheme` / `cardSpace` 传入，与原来一致 |

只有画了分段控件的页面受影响 —— 也就是集成页的那两排导航 —— 所以「其他设置页看着
是对的」。

### ①g 集成页两层菜单（第七轮）

用户报告：「这里的两层菜单样式和交互太丑了」。集成页有**两排**导航：页面自己的三
个 tab（我的工具 / 添加工具 / 导入与导出）和「添加工具」里的三种方式（已检测 / 本地
工具 / 安装包）。两排此前是**同一个** `integrationTabs` 画了两遍，一模一样的胶囊，
一上一下 —— 第二排读起来是第一排的重复，而不是它的下一级；页面在任何内容之前先给出
三排等重的板。

| # | 位置 | 改法 |
| --- | --- | --- |
| 20-23 | `internal/settingsui/integrationnav.go` `integrationTabs` | 加 `primary bool`：父级是**tab bar**（无板、无 track 底色、`TextMuted`/`TextStrong` 墨、选中项底边一条 2px accent、整条 track 底边一条 `HairlineSoft` 基线），子级保留胶囊（28u、`RowSelected` 面 + `LitFace`） |
| 20-24 | 同文件 `integrationToolbar` | 子级包在 `Padding(Space(0.5))` 的盒子里，紧贴父级；此前它与父级之间走的是页面自己的 `Gap(Space(4))`，19.8px 的空隙让它读成「页面的第一段」 |

父级下划线用 `segment.InsetShadow(0, -2, 0, 0, t.Accent)`，不是绝对定位的子盒：绝对
定位落在 segment 的 **padding box**，而这一行是 `Center()` 的，规则会横穿文字而不是
落在下面（第一版实测就是这样）。

窗口高度链一个数没动：`Geometry.Height` 的 `chrome` 仍按输入行计，多出来的高度落在
正文/列表的 `Grow(1)` 里，实测按 charged 高度渲染时最后一行墨紧贴卡片下沿
（`deadBelow=0`），所以「不影响其他地方」。

### ② 底部动作条的 `⌘K`

「在终端中运行」那一行（`isBar`）的尾部画了两枚键帽：它自己的 `⌘↩`，以及
`footerActionsBadge()` 的 `⌘K` / `Ctrl+K`。后者是 Raycast 的「动作面」图例，而那个面
**在本 build 里从未建起来**，chord 也从未绑定（`docs/raycast-design.md` §4.6 自己写着
「这个 chord 在本 build 里没有绑定」）。于是它是一枚指不到任何东西的键帽，在
「进终端」这一行上还被读成第二条进终端的路。

| # | 位置 | 改法 |
| --- | --- | --- |
| 20-7 | `internal/launcher/launcher.go` `row` 的 `isBar` 分支 | 删掉那枚图例键帽；这一行只留它真正的 chord `⌘↩` |
| 20-8 | 同处 `footerActionsBadge` | 函数随图例一起删除（零调用点） |

## 判据

- `internal/launcher/outputair_test.go` 的
  `TestTheOutputTakesTheListsPlaceUnderTheField`：① 输入框仍在（`Search` 有盒）；
  ② 输出的页头落在字段带**下方**，不压在字段上；③ 它与字段带之间只隔一格呼吸。
  对第一轮的代码报「the output page draws no search field」，对第二轮的代码全绿。
- `internal/launcher/outputsteady_test.go` 的
  `TestABackgroundRunMovesOnlyTheListBand`：逐帧比搜索栏三枚控件的盒子（`Search` /
  `Clear` / `Settings`），三个状态（已输入 / 执行中 / 输出已到）**逐字节相同**；
  同时钉住执行中列表带里有 `OutputRunning`、输出到达后 `InOutputView()` 为真、
  编辑字段后输出让位。20-9 / 20-10 未修时它报
  `"Search" moved 54.00/22.86/455.65/19.08 -> 54.00/22.86/565.20/19.08`。
- `internal/launcher/actionlegend_test.go` 的
  `TestTheActionBarDrawsOnlyTheChordItAnswers`：动作条上不能出现 app 不回答的键帽
  （`K`）。改前报 `the footer still draws the 'K' of an unbound actions legend`，
  改后全绿。
- `internal/launcher/rerunafterrun_test.go` 的
  `TestARunKeepsTheCommandModeSoTheArgumentsCanBeEdited`：回车后①模式仍在、②字段仍是
  跑的那行参数、③scope 标记还在、④输出到达后模式与文本都没被带走、⑤改参数即让输出
  让位、⑥**再按回车能重跑改过的参数**。20-17 未修时五条同时报，其中第六条
  `the second Enter ran 1 times, want twice ([[ttt]])` 正是「得重新找命令行」。
- 同文件的 `TestEscapeLeavesTheModeAndKeepsTheArguments`：Escape 逐层退出（先输出、
  再模式），且两层都不改文本（R31）。
- `internal/launcher/outputfit_test.go` 的 `TestTheOutputPageOpensAtTheHeightItDraws`：
  七种内容（1/3/5/12 行文本、1/3/10 行列表）各自按**它自己要求的高度**开窗，然后
  `Output.MaxY` 必须为 0。20-18 未修时 text/3、text/5、text/12 三条同时报
  「the page opened …px scrolled」——列表形态本来就是对的，所以它同时钉住「别把
  已经对的臂改坏」。
- 同文件的 `TestALongOutputStillScrollsInsideItsOwnCeiling`：一百行输出仍然自己滚，
  上限仍是旧版那 10 档——修滚动条不能把上限一起修掉。
- 同文件的 `TestTheOutputStripIsChargedOnlyOnItsOwnPage`：执行中与离开结果页时
  `OutputHeaderHeight()` 都是 0，只有结果页开着时才 > 0。20-21 未修时报
  「the strip outlived the page」。
- `internal/shell/themealpha_test.go` 的 `TestEverySettingsPageKeepsTheWindowsGlass`：
  浅/深两档下，集成页 / 添加工具 / 本地工具 / 本地工具表单四种状态的卡面 alpha 必须
  等于 General 页的。20-22 未修时报
  「local tool form: page face at alpha 255, the General page at 207」。
- `internal/settingsui/menurank_test.go` 的两条：`TestTheTwoNavigationRanksDoNotLookAlike`
  读两排选中项各自的填充色，必须不同（20-23 未修时报「the two ranks paint the same
  face ({33 33 35 255})：the child tab and the parent tab are the same control」）；
  `TestTheChildRankRidesUnderTheParentRank` 钉住两排之间是一格呼吸而不是页面的段落
  间距（改前 19.8px）。

重定基线（V）：`rowtrailing_test.go` 与 `raycast_test.go` 里两条「键帽形状」的 pin
原本借 `⌘K` 的第二枚板作对照，现在改用动作条**真正**的 chord（`enterBadge` 的
`⌘↩`）作对照 —— 形状判据（与行的 Tab 板同高、20u）一个数没动。

## 验证

`internal/launcher`、`internal/settingsui`、`internal/shell` 三包全量失败集与改前
**逐条相同**（4 / 35 / 14 条既有失败）；`internal/ci`、`theme`、`i18n`、`settings`、
`shortcuts` 全绿；`gofmt -l`、`go vet`、darwin/windows/linux 三平台 `go build ./...`
通过。

## 未做（说明）

- 未改窗口高度链：输出页按「输入行 + 呼吸 + 面板上下内边距」计价，与它现在真的画出来的
  结构一致；正文/列表的 `Grow(1)` 吃掉余量，静止帧最后一行为墨贴在卡片下沿
  （实测 `deadBelow=0`）。
- 未给 `⌘K` 补一个绑定：动作面还没做，补绑定等于造一个没有面的入口。图例与它要打开
  的面一起回来。
