# P13-m 报告：设置页三块「`Control` 平涂 + 一圈画框」的面戴上 E1 的一对面

- 分支 `mygo-rewrite`，起点 `0b3b7d67`（P13-l）。
- 规格 `docs/rounds/P13-m-spec.md`。
- 合约行：`docs/liquid-glass.md` §6.2b-3 新增 9-15 / 9-16 / 9-17。

## 这轮为什么让它更轻 / 更有空间感（一句话）

三块面停在「一圈画框圈住一块平涂」——它们与脚下的卡只差一个 alpha，读起来是**卡上贴的
纸片**；补上顶受光边 + 底对侧暗线之后，同一个画框里第一次出现「上亮下暗」的算术，块才从
卡面里**站起来**（浅色顶 119 → 125、底 119 → 115；深色顶 141 → 146、底 141 → 137），
而两侧一个像素没动——受光边是内线，不是第二圈画框。

## 选题依据（为什么是这三块）

`docs/liquid-glass.md` §6 在 P13-l 之后**没有 `未开始` 行**（唯一非「已落地」的是 9-11 的
`故意不做`，需拍板），按纪律转「细节打磨通用队列」的**层级一致性**格。

P13-i（十处 `Lens`）与 P13-l（两处漏网）的排查命令都是**单行**匹配：

```
grep -n "tokens.Control)" internal/settingsui/*.go | grep "Border(1"
```

`Border(...)` 与 `Background(tokens.Control)` 分开写的三处因此两轮都没进过视野：

- `config.go` `configMultiselectBox`（66u 多选字段箱）——**它自己的注释**（`config.go:815-817`）
  写着「与 `fieldFace`/`sheetFieldSlot` 同面……同一个应用里字段只有一种材质」，而那两个
  字段自 P12-c 第 2-4 行起就是 `tokens.LitFace(...)`：**注释在说一件代码没做的事**。
- `settingsui.go` `commandPlate`（扩展页命令列表的每一条命令块）——与 §6.2 第 2-3 行的分组卡
  同档（E1 面），卡有那一对线它没有。
- `textinput.go` `aliasField`——面板字段族里**唯一**没有那一对线的能打字的槽。它原来的注释
  说「它是 inline command-chip，不是 settings slot」，那说的是**边框**的选择
  （`Hairline` 而不是 `ControlEdge`），不是「不要受光边」的理由。

## 落地项

| # | 文件:行 | 改动 |
|---|---|---|
| L1 | `internal/settingsui/config.go:834-848` | `box = tokens.LitFace(box)`（在 `Clip()` 之后）。`MinHeight(66u)` / `RadiusSM` / `Border(1, ControlEdge)` / `Padding` / `Clip` 零改动 |
| L2 | `internal/settingsui/settingsui.go:3076-3081` | `plate = tokens.LitFace(plate)`（在 `Border(1, Hairline)` 之后、`Opacity(0.62)` 之前）。外圈 `Hairline` 不动 |
| L3 | `internal/settingsui/textinput.go:146-155` | `return tokens.LitFace(e.Background(tokens.Control).Border(1, tokens.Hairline))`。外圈 `Hairline` 不动 |
| L4 | `docs/liquid-glass.md` §6.2b-3 | 新增 9-15 / 9-16 / 9-17，并写清「这一族是**多行写法**漏在单行 grep 框外」 |
| L5 | `internal/settingsui/multilineface_test.go`（新） | 3 条像素 pin（每条深+浅）+ 1 条 opt-in dump（六行 before/after 对照 + 三块真组件） |

## 实测像素（同一次 dump，同一块地面 `#808080`）

浅色（面 119）：

| 面 | 顶内行（before → after） | 侧列 | 底内行（before → after） |
|---|---|---|---|
| 多选箱 | 119 → **125** | 119 = 面 | 119 → **115** |
| 命令块 | 119 → **125** | 119 = 面 | 119 → **115** |
| 别名框 | 119 → **125** | 119 = 面 | 119 → **115** |

深色（面 141）：三块都是顶 141 → **146**、底 141 → **137**、侧列 141 = 面。

真组件（同一套外观、同一块地面）：多选箱 `380×59`、命令块 `420×121`、别名框 `240×25`，
三块的四条读数与上表逐位相同——pin 量的不是探针，是产品画出来的东西。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿，FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2941（基线 2932，+9）
GOOS=linux/windows/darwin go build ./...               → 过
```

- **PASS 2932 → 2941（+9）**：3 条 pin × 2 套外观 = 6 个子测试 + 3 个顶层测试。
- **FAIL 0**；**SKIP 70 → 71**（+1：新的 opt-in dump）。
- **测试函数 2161 → 2165**（+4，一条都没删）。

## V / B / S 台账

- **V 0**：没有任何旧设计数值被重定基线——这三处此前根本没有「新值」可言（它们是漏网的，
  不是被钉住的）。
- **B 0**：行为 / 度量 / 接线的测试**零改动**，全绿。
- **S 0**：结构没变（`LitFace` 只加两条 1px 内线，不占布局，不改外圈线色）。
- 断言总数只涨：新增 3 条 pin，没有一条既有断言被放宽、加容差或跳过。

## mutation（5 条实跑，全部真红，逐字节还原）

`md5sum -c /tmp/p13m.md5` 在每次还原后核对，三个文件全部「成功」；`git status --porcelain`
只剩本轮的三个生产文件 + 新增测试 + 新增文档。

| # | 注入 | 实测报错（原始串，截一行） |
|---|---|---|
| M1 | 注释掉 `configMultiselectBox` 的 `LitFace` | `[light] the multiselect box: the rows inside its top frame are {119 119 119}, not brighter than its face {119 119 119} — the face lost its lit edge` |
| M2 | 注释掉 `commandPlate` 的 `LitFace` | `[light] the command plate: the rows inside its bottom frame are {119 119 119}, not darker than its face {119 119 119} — E1 is a pair` |
| M3 | `aliasField` 的返回值去掉 `LitFace` | `[light] the alias field: the rows inside its top frame are {119 119 119}, not brighter than its face {119 119 119}` |
| M4 | 给多选箱补一笔 `Shadow(0, 4, 8, 0, ShadowContact)`（E1 不许投影） | `[light] the multiselect box: its left columns at mid height are {116 116 116} but its face is {119 119 119} — a ring is drawn around the block` |
| M5 | 命令块的 `LitFace` 换回 `tokens.Lens`（四层环） | `[light] the command plate: its left columns at mid height are {116 116 116} but its face is {120 120 120} — a ring is drawn around the block` |

**M4 的观察（写下来，因为它说明判据的边界）**：投影没有撞「块外仍是地」那条判据，而是撞
「两侧列 == 面」与「底暗」——因为 `paintedBox` 会把影算进块的范围，于是影落在**读数带内**
而不是带外。结论仍是红的，但覆盖它的是另外两条判据，报告如实记下。

## 出图（自己复核）

`/tmp/floter-P13-m-faces-light.png` / `/tmp/floter-P13-m-faces-dark.png`：六行
「同一个画框 before/after」+ 三块真组件，全部画在同一块 `#808080` 地面上。

**这张图上看到的一处变化**（读的是这张 PNG 的像素，不是凭印象）：浅色那张的
`multiselect · LitFace` 行，框内第一行是 125 而它自己的面是 119——同一个画框里第一次有
一条比面亮的线；而这一行的左右两列仍是 119，与 `flat` 行逐位相同。也就是说变化**只发生在
框内上下一像素**，画框与外圈没有任何一个像素被改写。

## 用了 `qiaomu-design`（偷了什么）

按 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md` 的
「已有方向，只要打磨」路径走，没走 Phase 2 的四方向预览（用户已把方向给死）。

**从 `craft-loop.md` 的工艺密度菜单偷了什么（逐条落到数值）**：

1. **第 12 项「材质细节：内边框高光」** → 落到 `LensRim` = `rgba(255,255,255,0.09)`，
   只画在**内线**上（`InsetShadow(0,1,0,0,…)`），外圈 `ControlEdge` / `Hairline` 一个字节
   不改。这一轮三块面用的是同一条线、同一个值。
2. **第 5 项「多层染色阴影，拒绝单层灰」** → §2 判据 2 禁止 E1 投影，所以取它的**分层**
   思路而不是它的影：一层顶亮（+6 / +5 级）+ 一层底暗（−4 级），代替原来那一圈单色画框。
3. **第 6 项「编辑细节：hairline」** → 外圈线保持原样（`ControlEdge` 与 `Hairline` 两种），
   因为「受光边是内线、画框是外线」正是这一支的不变量；这一轮同时验证了两种画框下同一条
   内线都成立。

**`preflight.md` 逐条结论**：

- **A（反 AI 套路）**：过。没有渐变白边、没有模糊贴图冒充玻璃、没有 glass 叠 glass
  （三块都在 E1 档，内部元素仍是面）；没有新增阴影。
- **B（设计读取一致性）**：过。读取为「常驻浮层面板 → 设置页的扩展/命令管理 → 用轻便
  液态玻璃语言」；三块面与 §6.2 第 2-3/2-4 行**同一套 token**（`LitFace`），没有新数值。
- **C（动效抽查）**：过（本轮不动效）。`Opacity(0.62)` 的停用臂仍是整块状态，没有被材质
  改写；`Transition` 一条没动。
- **D2（中文页）**：过。本轮零文案改动，没有新增字符串、没有新增字体、没有斜体（P-1）。

## 仍待（P13-m，追加在 P13-l 之后）

1. **多行 grep 的盲区已清空（本轮的收获，结案）**：用
   `grep -n "Background(tokens.Control)" internal/settingsui/*.go` 全量看过 16 处，
   剩下的都是**故意不做**或**已落地**：`noticeLine` / `savealert`（警告环，9-14b 同一条）、
   `healthfaces.go` 权限药丸（2-9：4px 药丸）、`packageChip`（4-2：30u 方块）、
   `sessionChip`（6.5u，无边框）、两处录制态（accent 环是身份线，9-11 同一条）、
   `runParamsBox` / 扩展输出块（`Sunken` 凹槽，2-1）、`PalettePreview`（终端调色板的底色，
   不是本应用的面）。**下一轮不必再扫这一族。**
2. **§6.2b 9-11 仍是 `故意不做`**：`control.SegmentedIndex` 选中段的 accent 四边环，需拍板。
3. **P13-l 第 3 条（三处警告带的两种做法）原样有效**：`config.go` `noticeLine` 与
   `settingsui.go` `noticeBand(danger)` 有警告环，`savealert.go` `shortcutRejectedAlert`
   只有警告色句子 + `Control` 面。查过原版：`.settings-save-alert`（`settings.css:279-302`）
   本身就**没有** warning ring（只有 `--glass-control-edge` 边框 + warning 字色），
   `.extensions-notice--error` 才有。**所以这不是偏差，是两份不同的参考组件**——这一条可以
   从「下一轮候选」降级为「已核对，不做」。
4. **P13-k 第 1 条原样有效（需拍板）**：`glass.ScrollEdge` 回放不透明底色。
5. **P13-k 第 2 条原样有效（需拍板）**：`FloatAlpha()` 未补偿。
6. **P13-c 四条原样有效**：S1b（`CloseAll` 改 detach，等拍板）、S1c（Windows ConPTY）、
   客户端不告而别的检测、capture 通道迁移。
7. **P13-h 的观察原样有效（需拍板）**：`iconPlate` 暖面戴 `LitFace` 时深色档那条 0.09
   白线几乎读不出来，与 P12-c 的 2-6 同类。
