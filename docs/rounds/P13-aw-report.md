# P13-aw 报告：插件配置浮层的两端——也是「滚不动」的那一处

## 一句话

`pluginsheet.go` 的 `plugin.config` scroller 在 app 里**根本没有视口**（`MaxY` 恒为 0），
schema 比窗口高时末尾几个字段被窗口下沿切掉、滚不到；补上视口之后，它又成了「滚动收口」族里
最后一块还在两端平刀切的面。这一轮把两件事一起收掉：给 scroller 一个可滚的视口，
给它的两端各 12px 的软边。

## 设计读取（qiaomu-design · 打磨模式）

> **读取为：桌面启动器里的通用插件配置浮层，面向已经选好插件、只想改两个开关的用户，
> 用「轻便液态玻璃」的语言，倾向 macOS 26 之前的克制材质家族（面差 + 一层受光边 + 浮层影）。**

三拨盘（中文）：**视觉冒险度 3**（功能型表单，只有边与面在动）、**动效强度 1**（这一轮不动效）、
**信息密度 6**（一张表单，不增不减）。

功能契约：进来 → 认出是哪个插件 → 改一个字段 → 关掉。3 秒必得信息 = 标题那条（插件名）。
必须能完成的动作 = 改字段 / 触发动作 / 关掉。**这一轮唯一的契约改动是把「末尾字段够不到」
变成「够得到」**，语义与交互一个没改。

## 这轮为什么让它更轻 / 更有空间感

改前内容在窗口最后一行像素上被 `.Clip()` 一刀切断（实测两端最外 4 行的墨展开 **100%**，
浅 228/228、深 240/240）——那不是「轻」，那是被剪刀剪断的纸；改后 40% / 39%。
更重要的是：被剪断的那一截此前**滚不到**，用户以为表单就到这里为止。

## 落地项

| # | 位置 | 现状 → 落成 |
|---|---|---|
| 1 | `internal/launcher/config.go` `configBody` | `ui.Box(c).FillWidth()` → `.Fill()`：内容高度的父盒不给 scroller 可用高度 |
| 2 | `internal/settingsui/pluginsheet.go` | 包一层 `ui.Column(c).Fill()`，`Scroll` 由 `.FillWidth()` 改 `.Fill()`（与 §6.16 的 `body` 同一写法） |
| 3 | 同处 | 顶/底各一块 `Absolute().Top(0)/Bottom(0)` + `PassThrough()`、`Height(t.Space(3))` 的 `glass.ScrollEdge` |
| 4 | 新增 `internal/settingsui/sheetedge.go` | `sheetEdgeFace` = `theme.FaceOver(t.Background, tokens.Sunken)`（不透明）：薄罩直接交给 `ScrollEdge` 会被 `Alpha` 乘成 0.14 |
| 5 | `internal/settingsui/config.go` | 顺手修 `configMultiselectBox` 的过时注释：写着 2u，代码与 P13-av 实测都是 3u |

**门**：几何自门控（§6.16 那条），不用 `ScrollState.Y`。顶边压在 1u 顶留白 + 标题行的行盒上身；
底边的 3u(12px) 底留白与边高逐像素相等。实测：停开头时最外 12 行 **0**、滚到底时最后 12 行 **0**。

## 实测数字

- **改前**：`MaxY` **0**；两端最外 4 行 **228/228（100%）**、**240/240（100%）**。
- **改后**：`MaxY` **500**（内容 1100 − 视口 600）；19 档滚动位置最坏值
  顶 **91/228（40%）**、**95/240（40%）**；底 **88/228（39%）**、**95/240（40%）**。
- **盒子零改动**：标题盒 `(20,63) 90×22` 与改前逐像素相同。
- 图：`/tmp/floter-P13-aw-pluginsheet-{light,dark}-{before,after}.png`。
  **在 `light-after` 上看到的一处变化**：带子最上一行不再是半个字头，而是一条从面板面
  （246）往内容面（229）压下去的 12 行斜坡——边在，但读不成一条线。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单，逐条落成数值）

1. **第 12 项「材质细节：内边框高光」→ 受光边不是加一条白线，而是「边淡向它脚下那张面」**：
   `ScrollEdge` 的 `Background` 取 `FaceOver(t.Background, Sunken)`（浅 `246,246,248` / 深
   `15,16,18`），不是 `Sunken` 的 0.17 薄罩本身——落在「0.85 会被乘成 0.14」这条实测上。
2. **第 6 项「编辑细节：hairline」→ 边高 3u = 12px**：小于一个行盒（14px × 1.45 ≈ 20px），
   最多软化一行的边缘；4u 正好等于一个行盒，会整行吞掉。
3. **第 5 项「多层染色阴影」→ 这一面**故意不用**：浮层里的一切是「面」档（§1 禁令 2：
   glass 不许叠 glass），这套表单坐的是面板，不是纸，两层影会把它读成第二张纸。
   所以这轮**只取面与边**，影留给真正的浮层（设置页卸载对话框那类）。
4. **第 3 项「品牌 focus 环」→ 迁移成「门」**：把「什么时候该有边」写成几何前提（两端落在留白上），
   而不是再加一个条件判断；这是 §6.16 的机制，本轮沿用并实测两端最外 12 行零墨。
5. **第 9 项「自定义滚动条」→ 反向落地**：这块面**不加**滚动条（§6.1 起「启动器列表不要滚动条」
   是用户拍过的），收口只靠边——所以「滚得动」必须在这一轮真的成立，否则用户连「还有更多」
   都看不出来。

## preflight 逐条结论

- **A 反 AI 套路**：无渐变高光边（禁令 3）、无 glass 叠 glass（禁令 2）、无伪 blur（禁令 1）；
  文案零新增（`i18n` 一行没动）。**过**。
- **B 设计读取一致性**：读取是「克制的功能型玻璃表单」，产出只有面与边在动，密度 6 未变；
  记忆点 = 带子两端那 12px 的退场。**过**。
- **C 动效抽查**：这一轮**不动效**（MOTION 1）：滚动本身走框架，边是静态材质，无常驻动画，
  因此 `prefers-reduced-motion` 一组不适用（无 transition 可关）。**不适用，已写明理由**。
- **D2 中文页**：无新增文案；行高与字阶沿用既有 `tokens`。**过**，但把「`tokens.Body` 实测
  10.8px / `tokens.Caption` 11px 与「正文 ≥ 14px」的张力」记进 `.pending-bugs.md` 需拍板——
  **这一条不过，且不许擅自改**（全局字阶是用户决定的事）。

## V / B / S 台账

- **V 0**：没有既有视觉 pin 需要重定基线（这块面此前没有任何边）。
- **B 0**：`TestThePluginSheetPaintsNoTextOverText`（30 个子测试）、`pluginconfig_test`、
  `pluginsheet_test` 全部零改动通过——`Fill()` 没有改变任何文字的盒子。
- **S 0**：没有删断言、没有加容差、没有 `t.Skip`（新增的一条 skip 是 opt-in 出图，需显式
  `FLOTER_P13AW_DUMP=1`）。

## 门槛（实测）

```
gofmt -l cmd internal                      # 空
go vet ./... ; GOOS=windows/darwin go vet  # 净
go test -count=1 ./...                     # 全绿
PASS 3091 → 3100 (+9) | FAIL 0 | SKIP 101 → 102 | 测试函数 2284 → 2288 (+4)
三平台 go build ./...                      # 过
```

## 变异（4 条实跑，4 红，`md5sum -c` 逐字节还原 `28aff152…`）

1. **M1** wrapper 退回 `.FillWidth()` →
   `the sheet does not overflow its window (MaxY = 0), so there is nothing to scroll …`
2. **M2** 两条边都不画 →
   `the sheet's first 4 rows read at 228 against the inside's 228 (100% of it) …`（底同）
3. **M3** `face := tokens.Sunken` →
   `… read at 205 against the inside's 228 (90% of it)` / 底 `199 … (87%)`
4. **M4** 底留白 3u → 1u →
   `blank ends: bottom 12 rows at the end 26` → `… the 3u bottom padding no longer equals the
   edge's height … (want ≤4)`

## 两轮制（craft-loop）

- **第一轮**：按上面五项落地，出 before/after 四张图。
- **四视角评审**（各一处）：
  - 艺术总监：斜坡落在**空白的页尾**上时是死的（滚到底那一帧看不到边）——这是对的，
    但它意味着「页尾留白」成了材质的一部分，注释里必须写死（已写进 §6.19 第 21-4 行）。
  - 工程师：`Top(0)` 的相对原点在这条链上比预期低了 4px（`layoutAbsolute` 从 padding box 起算的
    老坑，§6.18 同一条），实测靠逐行剖面确认边落在 56..68，而不是假设。
  - 甲方：三秒内还认得出这是「插件的配置」，标题仍在最上一行——只换了它退场的方式。
  - 读者：带子顶端那 12px 里**没有字**被压到读不清（两端最外 12 行零墨，实测）。
- **第二轮只修 3-5 处**：① wrapper 从 `FillWidth` 改 `Fill`（否则 pin 无法成立）；
  ② 门从 `ScrollState.Y` 改回几何自门控（实测两端零墨，条件判断是多余的一层）；
  ③ 边的底色从 `Sunken` 改为 `FaceOver`（0.14 那条坑）；④ 修过时注释（2u → 3u）。

## 仍待（下一轮候选）

1. **多选箱的宽度**（12 项时勾选框列离箱面右沿 8px）与 `cardRow` 控制槽 `Shrink(0)` 同族，
   **需拍板**。
2. **需拍板**：`tokens.Body` 10.8px / `tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」的张力
   （全局字阶）。
3. 终端弧 S1b / S1c / S2 尾，仍等拍板。
