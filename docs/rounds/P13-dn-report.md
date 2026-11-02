# P13-dn 报告 · 复制按钮的确认语把整行推开 10px

表面：**设置页 → Integrations → Add a local tool → 脚本模式的内容动作行**
（`toolContentActions`，`internal/settingsui/settingsui.go:5407`）。
合约新增：`docs/liquid-glass.md` §6.83。规格：`docs/rounds/P13-dn-spec.md`。

## 1. 先量（动手前的像素事实）

720 帧、深/浅各一帧、真实 `ui.Tester` 画出来的盒子：

| 量 | 值 |
|---|---|
| 静息语 `Copy script` 墨宽 | **62** |
| 确认语 `Script copied` 墨宽 | **72** |
| 胶囊高度 / 圆角 / 侧内边 | 30u / 15u / 10u |
| 动作行内沿（标签列） | x=206，胶囊左沿 206/207 |
| 按下前 `Export script` 胶囊 x | **305.0** |
| 按下后 `Export script` 胶囊 x | **315.0** |

两套外观逐像素相同（文案是同一份 i18n）。也就是说：**这一行的布局是一次点击的函数**，
而点击只该改变一颗按钮说的话。

同族还有两对（程序模式）：`Copy command` / `Command copied`、
`Copy execution plan` / `Execution plan copied`；中文那一对是 6/5 字（`复制脚本` / `脚本已复制`），
方向相反 —— 所以地板必须**量**，不能按「确认语一定更长」写死。

## 2. 改了（一处，`settingsui.go`）

```go
floor := a.contentActionFloor(c, label, done)   // 先量两句里更宽的那句
if copied { label = done }
button := a.actionButton(c, label).MinWidth(floor)
```

`contentActionFloor` = `ceil(max(MeasureText(label), MeasureText(done))) + 2×Space(2.5) + 2`。
`MeasureText` 用的是画这行字的同一支画笔、同一档字号（`tokens.Body`）与字重（600），
所以量出来的就是胶囊里那一串墨的实际推进宽度。

**为什么 +2 而不是 +1**：+1 的地板实测仍让导出胶囊移动了**正好 1px**（`MeasureText` 报 71.x，
落字 72）。这条是跑出来的，不是猜的（见 §5 M2）。

## 3. 改后（同一把尺子重量的值）

| 量 | 改前 | 改后 |
|---|---|---|
| 按下前导出胶囊 x | 305.0 | **315.0** |
| 按下后导出胶囊 x | 315.0 | **315.0** |
| 静息语墨盒 x | 217.0 | **222.0**（在 94 宽胶囊里居中） |
| 确认语墨盒 x | 217.0 | 217.0 |

静息态那一格变宽了 10px：这是本轮**故意**的代价 —— 一颗按钮的盒子不再由「这一刻说的是哪句」
决定，而由「它可能说的最长那句」决定。墨、边、高度、圆角一个字节没动（V 账里没有任何一条
被重定基线）。

## 4. 台账（V / B / S）

- **V 0**：没有任何 pin 钉过这颗胶囊的盒子（旧测试只钉「这段文案在不在 / 按下去调没调
  `CopyText`」），所以一条旧断言都没有改。
- **B 0**：`TestLocalToolFormCopiesItsContent`、`…CopiesTheScript`、`…ExportsTheScript`
  等 8 条既有 B 类测试**零改动全绿**。
- **S 0**：结构没变，仍是 `contentAction` + `actionButton`，只多了一个量宽的地板。
- 新增：1 条 pin（`TestTheCopyButtonsDoNotReflowTheRow`，深/浅各跑一遍）+
  1 条 opt-in dump（`TestDumpP13DNContentRowPNGs`）。

## 5. mutation（实跑，逐条给了原始报错串）

**M1 · 地板只量静息语**（`if w > widest && label == labels[0]`）：

```
--- FAIL: TestTheCopyButtonsDoNotReflowTheRow (0.66s)
    contentjitter_test.go:70: dark=false: label {X:217 Y:721.5 W:62 H:17},
      confirmation {X:217 Y:721.5 W:72 H:17}, export 305.0 → 315.0
    contentjitter_test.go:73: dark=false: the confirmation pushed the export button
      from x=305.0 to x=315.0 — a press is not a layout change
    ... dark=true 同上
```

**M2 · 地板去掉 2px 顶格**（`+2` → `+1`）：

```
--- FAIL: TestTheCopyButtonsDoNotReflowTheRow (0.33s)
    dark=false: label {X:221.5 ...}, confirmation {X:217 Y:721.5 W:72 H:17},
      export 314.0 → 315.0
    dark=false: the confirmation pushed the export button from x=314.0 to x=315.0
```

两条都还原：`md5sum -c` → `internal/settingsui/settingsui.go: 成功`
（`67d478f4bd5dc0fde39fee072bc93bfe`），`touch` 后复跑 `ok 0.600s`（不吃测试缓存）。

## 6. 门槛（本轮实测，全套）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
--- PASS 3321（基线 3320，+1）
--- SKIP 154（基线 153，+1 = opt-in dump）
测试函数 2483（基线 2481，+2）
GOOS=linux/windows/darwin go build ./...               → BUILD-OK
```

## 7. 图（8 张，自己复核）

```
/tmp/floter-P13-dn-contentrow-<dark|light>-<before|after>-<rest|copied>.png
```

这张图上看到的一处变化：`dark-after-rest.png`（3013 B）比 `dark-before-rest.png`（2984 B）
多出的一片像素全在 `Copy script` 胶囊的右侧 —— 静息态的胶囊从 82 宽变成 94 宽，
字在盒里居中；`copied` 那一帧两张图逐字节同尺寸，因为确认语本来就贴着地板。

## 8. 量过、但这一轮**不动**的地方（写清理由）

- **`paramempty` 的空槽**：槽 191..702、内沿 200..693、1px 虚线 + 8u 内边、句子墨宽 210。
  逐边算过：左 `191+1+8 = 200` ✓、右 `702-8-1 = 693` ✓、行盒 15（`Caption`）✓、
  盒高 33（`1+8+15+8+1`）✓ —— **没有可改的偏差**。P13-ch 抬过墨与字重，
  这一格已经是这一族里最完整的那个，判「已量、不改」，差距表里不留空行。

## 9. 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

按打磨模式只取本轮表面用得上的三项，逐条落成具体数值：

1. **菜单第 4 项「签名交互」**（一个足矣）：把「确认」做成**原地**的状态 ——
   落成 `contentActionFloor` = `ceil(max(MeasureText)) + 2×t.Space(2.5) + 2` = **94px** 地板，
   按下前后导出胶囊恒在 **x=315.0**。
2. **菜单第 6 项「编辑细节」**（一条线、一个字距上的克制）：确认语与静息语共用同一个
   居中（静息墨 x=**222**、确认墨 x=**217**，都在 94 宽的胶囊里），换字不换位。
3. **菜单第 12 项「材质细节」**：本轮**只扩盒子、不碰材质** —— 胶囊仍是 E1（`Control` 面 +
   `LensRim` 冠线，§3 的「受光边比面亮」不变量不该被一次状态反馈影响）。

## 10. preflight（`references/preflight.md`）逐节结论

- **A 反 AI 套路**：过。本仓是自绘近黑/近白底，无紫蓝渐变、无 `#000` 纯黑、无外发光、
  无渐变标题；中文页无斜体（P-1）；文案无「革命性/无缝/赋能」类空词，感叹号 0；
  无三等分卡片横排；无眉批编号。
- **A2 进攻三步**：DNA 注入一条（见 §9 与 §11）；工艺密度 3 项（打磨模式下只对**这一个表面**计，
  不是新页面）；两轮制：第一轮「地板 = 最宽那句」→ 截图/像素复核 → 第二轮只修 **1px 顶格**
  这一处（M2 就是它）。
- **A4 功能契约**：过。「复制脚本 → 我在哪一刻知道它成了」这个契约由 1.6s 的确认语承担，
  本轮没有改它说的是什么，只保证它说出来时不推走邻居。
- **A4.1 disabled / read-only 语义**：过（本轮既没有新增禁用面，也没有把确认态做成禁用态）。
- **B 设计读取一致性**：过。读取 = 「设置页 · 集成 · 本地工具表单，面向写脚本的用户，
  用安静的工具语言，倾向本仓 `liquid-glass.md` 这一支（E1 面 + 一条受光边）」；
  记忆点 = 「按下去，只有这一颗按钮变，别的都不动」。
- **C 动效抽查**：过。本轮没有新增动效；确认是一帧内换字（无 `width` 过渡、
  无 `transition: all`、无 `scale(0)`），故 `prefers-reduced-motion` 无新增负担；
  按压反馈沿用框架的 `ButtonBase` 现成态。
- **D2 中文页**：**不过（第 39 份证据）**。`tokens.Caption` 9.9px / 正文 10.8px 与
  「正文 ≥ 14px」的冲突照旧 —— 这是全仓字号阶梯的决定，需要用户拍板（沿用「需拍板」表）。
  本轮唯一相关事实：中文正好是**短的那一句**（`复制脚本` 6 / `脚本已复制` 5 字），
  所以地板按英文那一对定，中文胶囊里会多 1 字宽的余量，不换行、不截断。

## 11. 仍待（P13-dn）

1. **下一格（细节打磨 · 状态反馈的另一半：只读面）**：`rows.go` 的 `keyValue` / `dataRow`
   在**只读**（不可编辑）语义下的墨档还没人量过 —— 现在它们与可编辑行用同一档
   `TextStrong`；「这行只是给我看的」没有任何墨上的记号。先量再改。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口
   （§6.75/§6.76/§6.77），本轮一字未动，仍在队里。
3. **需拍板（沿用 + 第 39 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十一次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
