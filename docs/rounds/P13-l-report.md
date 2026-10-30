# P13-l 报告：设置页最后两块「平涂 + 画框」的控件面戴上 E1 的一对面

- 分支 `mygo-rewrite`，起点 `72a4af9c`（P13-k）。
- 规格 `docs/rounds/P13-l-spec.md`。
- 合约行：`docs/liquid-glass.md` §6.2b-2 新增 9-13 / 9-14 / 9-14b。

## 这轮为什么让它更轻 / 更有空间感（一句话）

`shortcutRecorder` 与非危险臂的 `noticeBand` 停在「`Control` 平涂 + 一圈 `ControlEdge`
画框」——它们比坐着的 E1 分组卡还平，读起来是卡上贴的一张纸片；补上顶受光边 + 底对侧
暗线后，两块面从「同一 z 平面上的框」变成「贴着卡站起来的一层」。

## 选题依据

`docs/liquid-glass.md` §6 在 P13-k 之后**没有 `未开始` 行**（唯一非「已落地」的是 9-11 的
`故意不做`，需拍板），按纪律转「细节打磨通用队列」。队列里「层级 / 状态反馈」这一格在
`settingsui` 上还有**具体的行**可指：

- `grep -n "tokens.Control)" internal/settingsui/*.go | grep "Border(1"` 一共 11 处；
  逐处判过之后只有两处是「面」且**没有** E1 的一对面（其余是凹槽 `Sunken`、30u 方块
  （§6.4 4-2 `故意不做`）、4px 药丸（§6.2 2-9 `故意不做`）、带警告环的带子（§6.4 4-4
  同一条）、以及已经有 `LitFace` 的 `fieldFace` / `sheetFieldSlot`）。
- 两处都不在 `glasscensus_test.go` 的账上——它们**从来不是** `Lens` 的调用点，所以
  「透镜被故意退掉」那本账照不到它们。这是 P13-i 那一族**漏在框外**的两处。

## 落地项

| # | 文件:行 | 改动 |
|---|---|---|
| L1 | `internal/settingsui/settingsui.go:3723` | `shortcutRecorder` 的 `button` 建好后 `tokens.LitFace(button)`；三个 rung（静息 / hover / press）共用这一对线，只换面。`Border` / `Radius` / `MinWidth` / `MinHeight` / `FocusRing` 零改动。 |
| L2 | `internal/settingsui/settingsui.go:5904-5911` | `noticeBand` 的 `danger` 臂保持警告环；新增 `else { tokens.LitFace(band) }`。 |
| L3 | `docs/liquid-glass.md` §6.2b-2 | 新增 9-13（录制钮）/ 9-14（非危险带）/ 9-14b（`danger` 臂 `故意不做` + 理由）。 |
| L4 | `internal/settingsui/bandface_test.go`（新） | 3 条像素 pin + 1 条 opt-in dump。 |

## 实测像素（同一次 dump，同一块地面 `#808080`）

浅色档（面 119）：

| 面 | 顶内行 | 侧列 | 底内行 |
|---|---|---|---|
| 录制钮 · 旧（`Control` 平涂） | 125 | **116（≠ 面）** | 105 |
| 录制钮 · 新（`LitFace`） | **131** | **119（= 面）** | **112** |
| 告警带 · 旧（平涂） | 119（= 面，无内线） | 119 | 119 |
| 告警带 · 新（`LitFace`） | **131** | 119 | **112** |

深色档（面 141）：

| 面 | 顶内行 | 侧列 | 底内行 |
|---|---|---|---|
| 录制钮 · 旧 | 160 | **145（≠ 面）** | 142（环压住了暗线） |
| 录制钮 · 新 | **151** | **141（= 面）** | **133** |
| 告警带 · 旧 | 141（= 面） | 141 | 141 |
| 告警带 · 新 | **151** | 141 | **133** |

注意浅色档「录制钮 · 旧」的顶内行 **125 < 新 131**：旧的四层透镜那条环（`ControlEdge`
0.06 黑）压在 `LensRim` 上，把受光边压暗了——这正是 P13-h / P13-i 抓到的同一个根因，
只是这一处不是 `Lens` 调用点，账本照不到。

## 「偷了什么」（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 12 项「材质细节：内边框高光」** → 落成 `LensRim` 0.09 白 1px 顶内线 +
  `LensEdge` 0.06 黑 1px 底内线（`theme.Tokens.LitFace`）。实测浅色 119 → 131（+12）/
  112（−7）、深色 141 → 151（+10）/ 133（−8）。**不是**渐变白边——是一条实色 1px 线。
- **第 6 项「编辑细节：hairline」** → 两条内线都是 1px、**零 blur、零 spread**
  （`InsetShadow(0, ±1, 0, 0)`）。变异 M5 把一笔 `Shadow(0, 2, 8, …)` 加上去，pin 立刻
  以「块外投影」判据变红——这就是这条工艺密度的守卫。
- **第 3 项「品牌 focus 环」——本轮的反向结论**：录制钮的 `FocusRing(true)` **一个字节
  没动**。它是**状态**线，不是材质线；把它一起换掉就会让「受光边」与「焦点」抢同一圈像素
  （§2「一个面只说一句话」）。

`craft-loop.md` 的其余项（氛围层 / 展示排版 / 流体字号 / 插画资产等）属于网页与营销面，
本仓是 Go 自绘的桌面 UI，不适用；按 skill 的「不套模板」原则**故意不取**。

## preflight 逐条结论（`references/preflight.md`）

**A · 反 AI 套路**

- 无渐变白边冒充高光：受光边是 1px 实色内线，不是 gradient —— **过**。
- glass 不叠 glass：两块面都是 E1（面板内元素），都没有外投影 —— **过**。
- 无纯黑 `#000000` / 无霓虹外发光 / 无大标题渐变字 / 无斜体 —— **过**。
- 文案无「革命性/颠覆/无缝/赋能」一类空词（本轮零新增用户可见文案）—— **过**。
- A2 两轮制：**不完全适用**。本轮按 skill 的「已有方向，只要打磨」车道（`craft-loop.md` +
  `preflight.md`），不走 Phase 2 四方向预览；第二轮以**变异实跑 + 出图复核**替代人工四视角
  评审（自动车道无人可评审）。这条按「不能过要写为什么」记在此处。

**B · 设计读取一致性**

> 读取为：桌面启动器的设置面板，面向调快捷键 / 关扩展的日常用户，用**轻便液态玻璃**语言，
> 倾向「一条受光边 + 一片不反光的面」这一支。

- 产出与读取一致：只加受光边与对侧暗线，没有加光泽、没有加影 —— **过**。
- 主题锁定：深/浅两套各自独立渲染，同一 pin 分别跑 —— **过**。
- 记忆点一句话：**「原本只剩一圈发丝的两块面，现在顶边亮一分、底边暗一分」** —— **过**。
- 动效气质：本轮零动效改动 —— **过（不适用）**。

**C · 动效抽查**

本轮**没有**动效改动（只加两条 `InsetShadow`，不参与过渡）。唯一相关的 `FocusRing` 未改。
C 节全部条目不适用；`prefers-reduced-motion` 由测试端的 `ui.Preferences{ReduceMotion:true}`
在探针里开启，渲染结果与常速一致（像素 pin 不读时间）。

**D2 · 中文页追加项**

- 用户可见中文文案：**零新增**（`noticeBand` 的句子来自 `copy.*`，本轮未改；i18n 红线未碰）。
- 中文行高 / 字号 / 字重：未变（`FontSize(t.FontSize)`、`LineHeight(1.45)` 原样）。
- 全角标点：本轮新增的中文只出现在代码注释与本文件，不进 UI —— **过**。

## V / B / S 台账

- **V 1**：`docs/liquid-glass.md` §6.2b-2 新增三行（视觉数值台账）。
- **B 0**：现有 2923 条 PASS 零改动全绿（`settingsui` 里此前**没有**这两处的任何 pin，
  所以没有任何旧断言需要改）。
- **S 0**。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2932（基线 2923，+9）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      → 0
go test -count=1 -v ./... | grep -c -- "--- SKIP"      → 70（基线 69，+1 opt-in dump）
grep -rn "^func Test" internal/ cmd/ --include=*_test.go | wc -l → 2161（基线 2157，+4）
GOOS=linux/windows/darwin go build ./...               → 三平台过
```

`+9 PASS` = 3 条 pin × （1 父 + 2 外观子测试）；`+4 函数` = 3 条 pin + 1 条 dump。

## mutation（5 条全部实跑变红，逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 录制钮换回 `tokens.Lens(button)` | `--- FAIL: TestTheShortcutRecorderIsAFaceNotARing/light` / `/dark`；`[light] … its left columns at mid height are {116 116 116} but its face is {120 120 120} — a ring is drawn around the block` |
| M2 | 删掉录制钮的 `LitFace` | `--- FAIL: …/light` / `/dark`；`[light] … the rows inside its top frame are {119 119 119}, not brighter than its face {119 119 119} — the face lost its lit edge` |
| M3 | 删掉 `noticeBand` 非危险臂的 `LitFace` | `--- FAIL: TestTheNoticeBandIsAFaceNotARing/light` / `/dark`；`[dark] the notice band: the rows inside its top frame are {141 141 141}, not brighter than its face {141 141 141}` |
| M4 | 给 `danger` 臂也加 `LitFace` | `--- FAIL: TestTheDangerNoticeBandKeepsItsWarningRing/light` / `/dark`；`[light] the danger band's ring is {165 51 45} on top but {147 29 23} below: E1's lit pair is painted over the warning ring` |
| M5 | 给录制钮加 `Shadow(0, 2, 8, 0, rgba(0,0,0,.30))` | `--- FAIL: …/light` / `/dark`；`[light] the shortcut recorder casts a shadow beside itself ({120 120 120} against the ground {128 128 128}): E1 adds none` |

每条之后 `cp` 还原 + `md5sum -c` 逐字节核对（`settingsui.go` 与 `bandface_test.go` 两行都
「成功」），再 `touch` 复跑一次全绿。

### M4 把 pin 自己纠正了一次（写下来）

第一版 M4 的判据是「顶内行必须暖（`r−b ≥ 25`）」。实跑发现**加了 `LitFace` 之后那一行仍然暖**
（浅色 `156,31,24` → `165,51,45`，`r−b` 仍有 120）——因为 `LensRim` 在浅色档是 **0.09 白**，
压在警告环上只是把它洗淡 9%，并不会变中性。所以第一版判据抓不到这条变异。

改判据为**对称性**：一圈 1px 环在四条边上同色，而 E1 的一对面按定义上亮下暗，所以
「顶内行 == 底内行（Δ ≤ 4）」才是这条「不加 E1」的判据；暖度检查留着回答「这是哪一圈环」。
这条判据也不怕亚像素：若环落在半覆盖的行上，顶与底会以同样的比例混合，仍然相等。

## 出图（自己用眼睛复核过）

`/tmp/floter-P13-l-faces-light.png`、`/tmp/floter-P13-l-faces-dark.png`（各 4 行：
录制钮 Lens / 录制钮 LitFace / 告警带平涂 / 告警带 LitFace）。

**图上看到的一处变化**：浅色档第 1 行（录制钮 Lens）与第 3 行（告警带平涂）的**顶边是一条
暗线**（灰度剖面 `====-::....::====`），而第 2 / 4 行（LitFace）的顶边是**一条亮线**
（`===*####…####*====`）；深色档同理，只是 Lens 那一行的顶边也被环叠亮（160 > 151），
差别落到两侧列与底内行上。

## 仍待（P13-l，追加在 P13-k 之后）

1. **`control.SegmentedIndex`（§6.2b 9-11）仍是 `故意不做`**：accent 四边环与白冠线谁说话，
   **需拍板**。
2. **`t.Space()` 当 px 用的同源族：抽检了最像 px 的八处，没有第二处**。判据是「两位数
   就是可疑」（`Space(10)`=40px、`Space(26)`=104px 这种）；全仓 `Space(n)` 里 n ≥ 6 的
   调用点共 40 处，逐处看过上下文的那八处（`scripteditor.go:81 Space(18)`=72u、
   `rows.go:995 Space(11)`=44u 行高地板、`rows.go:654 Space(9)`=36u 开关轨、
   `settingsui.go:1837-1838 Space(16)`=64u 行地板、`settingsui.go:2471 Space(16.5)`=66u 卡高、
   `notices.go:352/363/371 Space(16)/Space(8)/Space(6)`=64/32/24u）**全部是 u 数，不是 px**。
   剩下三十二处是 `Space(7)`（=28u 图标板/图标格）一族，与 P13-h 刚量过的 28u 板同值。
   P13-g 的猜测到此结案，**不再作为下一轮候选**。
3. **新观察（下一轮候选）**：`savealert.go` 的 `shortcutRejectedAlert` 是**警告色的句子**
   装在 `Control` 面上，但**没有**警告环——而同一页的 `config.go` `noticeLine` 与
   `settingsui.go` `noticeBand(danger)` 都有。三处「警告带」现在两种做法，值得单独量一轮
   （属于「状态反馈」，但改它会动一个语义决定，先记下）。
4. **P13-k 第 1 条原样有效（需拍板）**：`glass.ScrollEdge` 回放不透明底色。
5. **P13-k 第 2 条原样有效（需拍板）**：`FloatAlpha()` 未补偿。
6. **P13-c 四条原样有效**：S1b / S1c / 客户端不告而别 / capture 通道迁移。
7. **P13-h 的观察原样有效（需拍板）**：`iconPlate` 暖面在深色档的受光边读不出来。
