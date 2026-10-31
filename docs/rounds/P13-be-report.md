# P13-be 报告 · 插件窗口正文的「一句话」：陈述档第一次比 chrome 重

规格 `docs/rounds/P13-be-spec.md`，合约新增 **§6.24**（5 行：4 `已落地` + 1 `故意不做`）。

## 这轮为什么让它更轻、更有空间感

层级只由**差**说话。此前捕获输出页正文里的四条状态行与它头顶的 bar 标题**逐级相同**
（浅 128–131 对 131、深 130–133 对 130），整个窗口读成「一条 bar + 一条同色的灰线」——
页面上只有一个 z 平面。这一轮把正文分成**三档**：结论（陈述档，emphasis + strong）、
进行（一档之下，emphasis + secondary）、故障（陈述档 + 语义警告色相）。窗口第一次有了
「chrome 最轻 → 进行次之 → 结论最重」这条阶梯，而列表里的注留在 chrome 那一档。

## 先量（实测 720×460 静息，窗口自己的地面 `tokens.Opaque` 浅 246 / 深 20）

| 位置 | 改前墨 | 浅 | 深 | 对地面 |
|---|---|---|---|---|
| bar 标题（`detached.title`） | `Caption` + `t.TextMuted` | 131 | 130 | 115 / 110 |
| 正文 running | 同上 | 131 | 130 | 115 / 110 |
| 正文 empty | 同上 | 130 | 131 | 116 / 111 |
| 正文 idle | 同上 | 128 | 133 | 118 / 113 |
| 正文 failed | `Caption` + `t.Warning` | 69 | 164 | 177 / 144 |
| 列表 status 行 | `Caption` + `t.TextMuted` | 131 | 130 | 115 / 110 |

候选墨（同一地面、同一 harness 实测）：`TextStrong` 30 / 234、`TextSecondary` 109 / 152、
`TextMuted` 128 / 133。

## 落地后的读数（同 harness 实测）

| 位置 | 改后墨 | 浅 | 深 | 对地面（权重） |
|---|---|---|---|---|
| bar 标题 | 不动 | 128 | 133 | 118 / 113 |
| 正文 running（进行档） | `Emphasis` + 500 + `TextSecondary` | 107 | 154 | 139 / 134 |
| 正文 empty / idle（陈述档） | `Emphasis` + 580 + `TextStrong` | **27** | **237** | **219 / 217** |
| 正文 failed（故障档） | `Emphasis` + 580 + `t.Warning` | 67 | 166 | 179 / 146 |
| 列表 status 行（不动） | `Caption` + `TextMuted` | 131 | 130 | 115 / 110 |

**三档成阶梯**：陈述 219 / 217 ≫ 进行 139 / 134 ≫ chrome 118 / 113，相邻两档的
下界钉在 **40 级**（进行对陈述实测 80 / 83，进行对 chrome 实测 21 / 21 —— 后者只要求
「高于 chrome」，不要求跨档，因为 chrome 本来就是最轻的一件）。

**一行几何变化（如实记录）**：字阶 `Caption`(11) → `Emphasis`(13)，行盒 **15 → 18px**。
窗口高度**不变**（body 是 `Fill()` 的 scroll，不是高度链上的固定盒），三平台 build 与
全量测试全绿。这不属于「高度链」红线，但它是**行盒**的变化，写在这里而不是藏在数字里。

## 落地项

| # | 位置 | 现状 | 落成 |
|---|---|---|---|
| 1 | `shell/detach.go` `detachedStatus`（一个 `failed bool` 管四臂） | 四臂同一寄存器 | 拆成 `detachedStateLine(c, text, kind, …)`，`kind` 是三个命名常量 |
| 2 | 陈述臂（idle / empty） | `Caption` + `TextMuted` | `Emphasis` + 580 + `TextStrong`（§4.7 的陈述档，与 `settingsui.emptyState` 同一条） |
| 3 | 进行臂（running） | 与陈述同档 | `Emphasis` + 500 + `TextSecondary` |
| 4 | 故障臂（failed） | `Caption` + `t.Warning` | `Emphasis` + 580 + `t.Warning`（色相不动，只抬字阶） |
| 5 | 列表 status 行 | 与陈述同墨 | **故意不动**：`detachedListNote`，说的是**列表**不是这一页 |

## 偷了什么（qiaomu-design，逐条）

1. **`references/design-systems/raycast/DESIGN.md` §3 层级表**：那张表里每一档是一个
   **命名角色**（Body 16/500、Caption 14/500、Small 12/600），不是「正文减两像素」。
   → 落成 `detachedStateKind` 的三个**命名常量**（`detachedStatement` / `detachedInFlight` /
   `detachedFault`），而不是三处内联的 `FontSize`/`FontWeight` 组合。这样下一轮改一档
   是改一个地方。
2. **同文件 §4 Ghost Button**：「`gray text #6a6b6c` → hover 时文字变白」，指针/状态
   换的是**墨的明度**而不是透明度。→ 落成「进行档用 secondary 墨 + 500 权重、陈述档用
   strong 墨 + 580 权重」：同一步阶里用**墨 + 字重**分档，与 P8-d 在 `actionWithGlyph`
   上判掉 `Opacity(0.78)` 的算术同源（`rows.go:1530-1548`）。
3. **`references/craft-loop.md` 工艺密度菜单第 6 项「编辑细节」**（大写字距标签 /
   章节编号 / hairline）→ 落成**字重差 580 vs 500** 作为阶梯的第二维度：实测陈述与进行
   的墨差 80 / 83 级里有 8 / 4 级来自字重（同为 emphasis 步）。
4. **`references/preflight.md` D 节「空态/加载态/错误态/成功态齐全」**：本轮就是把
   **空态 / 加载态 / 错误态**三种从「同一种画法」拆成三档——这一条此前在像素上是缺的。

## preflight 逐条结论

- **A（反 AI 套路）**：过。没有渐变白边、没有玻璃叠玻璃、没有拿透明度表达层级
  （本轮删掉了一个「四臂一个墨」的平涂）；受光边只在面与浮层上，这一页没有任何新边。
- **B（设计读取一致性）**：过。方向是用户钉死的「轻便液态玻璃 + 空间感」，本轮只动墨，
  VARIANCE 未变；一句话记忆点 = 「窗口的正文比它的标题重」。
- **C（动效抽查）**：过（本轮零动效）。`ReduceMotion` 在 pin 里开着；无 `transition: all`、
  无纯 fade 入场、无 `scale(0)`；字阶变化不参与过渡（`Text` 没有 `ContentTransition`）。
- **D2（中文页）**：**不能全过**，如实记录：①中文行高与字重（400/500/600）过；②「正文
  ≥ 14px」**不过** —— 这一页的状态行是 `Emphasis`(13) / chrome 是 `Caption`(11)，
  与任务书已挂着的「`tokens.Caption` 11px 张力」是同一条，属**需拍板**（改字阶 = 改高度链，
  P13-bd 已实测撞红）。③中西文空格：这两句文案都是纯中文/纯英文，不涉及混排。

## 门槛（全跑，实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → 全绿，FAIL 0
- PASS **3142 → 3147**（+5）｜SKIP **105 → 106**（+1，opt-in 出图）｜测试函数 **2309 → 2315**（+6）
- `GOOS=linux|windows|darwin go build ./...` → 过
- **V 0 / B 0 / S 0**（新增的 5 条 pin 全是新断言，没有一条旧断言被改）

## 变异（4 条实跑，全部变红并还原）

- **M1** 陈述臂退回 `t.TextMuted` → `TestThePluginWindowBodyOutranksItsOwnChrome` 红：
  `dark=false empty: the body line does not outrank the bar title (by 0, want ≥ 60; body {128 128 135}, title {128 128 135})`
- **M2** 进行臂抬到陈述档 → `TestTheInFlightLineIsOneRungBelowTheStatement` 红：
  `dark=false: the in-flight line is not one rung below the statement (gap 0, want ≥ 40; running {27 27 29}, statement {27 27 29})`
- **M3** 故障臂换中性 strong 墨 → `TestTheFaultLineKeepsTheWarningHue` 红：
  `dark=false: the fault line lost the warning hue (r−b = -2, want ≥ 25; ink {27 27 29})`
- **M4** 列表 status 行跟着抬到陈述档 → `TestTheListsOwnNoteStaysUnderTheStatement` 红：
  `dark=false: the list's note rose to the statement register (gap 0, want ≥ 60; note {27 27 29}, statement {27 27 29})`

每条都 `cp` 还原后 `md5sum -c /tmp/p13be-detach.md5` → `internal/shell/detach.go: 成功`，
再 `touch internal/shell/detach.go` 复跑绿（避免测试缓存）。

## 出图（自己用眼睛复核）

`/tmp/floter-P13-be-pluginwindow-{light,dark}-{running,failed,empty,idle,liststatus}-{before,after}.png`
（`FLOTER_P13BE_DUMP=1 FLOTER_P13BE_TAG=<before|after>`）。

整帧逐像素对照（`PIL` 全图扫描，before vs after）：

| 外观 | 臂 | 变化像素 | bbox |
|---|---|---|---|
| light | running | 360 | (13,55)-(69,68) |
| light | failed | 1632 | (12,55)-(257,68) |
| light | empty | 1358 | (12,55)-(217,68) |
| light | idle | 965 | (12,55)-(157,69) |
| light | **liststatus** | **0（IDENTICAL）** | — |
| dark | 同上五行 | 同上 | 同上 |

**这张图上看到的一处变化**：`light-empty` 的那一行从「灰线」（131）变成近黑（27），
而 bar 标题在它上方仍是 128 —— 同一帧里第一次出现「上面淡、下面重」，
窗口的 z 方向正过来了。`liststatus` 两帧**一个像素都没动**，就是 §6.24 第 5 行的证据。
