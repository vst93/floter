# P13-du 报告 · 夹具的「画家取整」一族（`28 → 26`、`8/711 → 7/712`、`int(2.5u) → 90`）

方向一句话：**让夹具按 painter 的取整去读格子** —— 三处 pin 此前用 `int()` 截断或库存尺字面量取像素列/行，
其中两处因此变成**假绿**（右沟少一列的红永远排不到；角弧判据读的是块**外面**那一行）。
规格 `docs/rounds/P13-du-spec.md`，合约新增 **§6.90**。

## 1. 先量（`TestZZProbeP13DU*`，临时探针，出完数即删）

### 1.1 便条板（`noteplate_test.go` 的 `readPlate`）

| 尺 | 板盒 | 宽 | 面差 door/note | 冠线 | 底对侧 | 字形差 drop |
|---|---|---|---|---|---|---|
| 库存 `render`（4.0） | `16..43` / 28×28 | **28** | 18 / 18 | 5 / 0 | 14 / 14 | 122 / 93 = **29** |
| 出货（3.6） | `14..39` / 25 高 | **26** | 18 / 18 | 6 / 0 | 14 / 14 | 101 / 87 = **14** |

深色出货：面差 24/24、冠线 19/0、底对侧 8/8、字形 101/78（drop 23）。
`25.2` 的盒子落 **26** 列：`round(16u)=14`、`round(44u)−1=39`。

**关键**：①②③ 在出货尺下**原样通过**；④ 的 `drop < 15` 阈值是**库存尺的产物**（库存实测 29），
出货尺浅色只有 14 —— 阈值本身就是个魔法数。实测两种墨的身份**恒等**：
门板 `101 == channelGap(TextMuted over face)`、便条 `87 == channelGap(TextTertiary over face)`
（深色 101/78 同样恒等），所以 ④ 从「差 ≥ 15」改成**身份 pin**，断言更强且没有魔法数。

### 1.2 行块（`raycast_test.go` `rowBlockIsTheInsetRoundedPill`，`renderShipped`）

```
row.X=48.60  row.Y=80.00  row.H=19.00   left=7  right=712   8u=7.20  mirrored=712.80  54u=48.60
```

| 量 | 旧 pin | 出货尺真值 |
|---|---|---|
| 左沟 | `int(LightTheme().Space(2))` = **8** | `round(8u)` = **7** |
| 右沟 | `InputWindowWidth − 8 − 1` = **711** | `round(720 − 8u) − 1` = **712** |
| 文字列 | `54u`，带 `±1.5` 容差带 | `48.60`（`tenths()` 整相等） |
| 角行 | `int(80 − 19/2)` = **70** | `round(70.5)` = **71** |

**两处假绿**：右沟的 −1 只在整宽时成立（出货尺下块涂到 712，但左沟先红，这条红永远轮不到）；
角行读的是块**外面**一行，方形角永远也读不到 —— 它只在换成出货尺后才暴露，因为库存尺行顶恰好是整数。

### 1.3 便条带差（`clipboardoff_test.go`）

出货尺实测：`note=104 plain=113`（差 **9**），而 `int(2.5 × 3.6)` = **8**（`float64` 的 `8.999…` 被截断）；
库存尺 `2.5 × 4.0 = 10` 恰好整，所以旧写法在库存尺下看不出病。

## 2. 落地（三处，全在 `_test.go`；产品代码零改动）

1. `clipboardoff_test.go`：`render` → `renderShipped`（`a.Spacing` 4.0 → 3.6）；
   期望差 `int(...)` → `tenths((rowUnits−launcherStatusUnits)×a.Spacing)`，实测差也走 `tenths()`。
2. `raycast_test.go`：`renderIn` → `renderShipped`；左沟 `round(8u)`、右沟 `round(720−8u)−1`、
   块顶 `math.Round`、文字列 `tenths()` 整数相等（删 `±1.5`）。
3. `noteplate_test.go`：新增 `listOutputShipped`（`c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight`），
   探针换到出货尺；`readPlate` 的 `28` → 左 `round(16u)`、右 `round(44u)−1`；
   ④ 改成墨的**身份** pin。新增 1 条 opt-in dump（`FLOTER_PNG=1`）。

## 3. 台账

**V 4 / B 0 / S 0** —— 4 处视觉数值：两处**假绿改真绿**、一处阈值换身份（更强的断言）、一处删容差带。
产品代码零改动（`git status` 只有三个 `_test.go` 与文档）。测试函数 2492 → **2493**（+1 opt-in dump）。
PASS **3325** 不变。

## 4. 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | **空** |
| `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` | 三平台**净** |
| `go test -count=1 ./...` | **全绿** |
| `go test -count=1 -v ./... \| grep -c -- "--- PASS"` | **3325**（基线 3325） |
| `--- FAIL` / `--- SKIP` | **0** / **158**（+1：本轮 opt-in dump） |
| 测试函数 | **2493**（基线 2492，+1） |
| `GOOS=linux/windows/darwin go build ./...` | 三平台**过** |

## 5. 变异（4 条实跑变红，`md5sum -c` 逐字节还原 + `touch` 复跑）

- **M1** `designUnit()` 返回 `spacing`（0.9 → 3.6）：
  `"Door2": the plate starts at x=14, want the 28u icon column's left edge (round(16u) = 58)`；
  `the row block starts at x=7, want the 8u gutter (x=29)` / `ends at x=712 … (x=690)` /
  `the title starts at x=48.6, want the reference's 54u (194.4)`。
- **M2** `renderShipped` 去掉 `c.SetTheme(...)`（退回库存尺）：
  `the row block starts at x=8, want the 8u gutter (x=7)` / `ends at x=711 … (x=712)` /
  `the title starts at x=54.0, want … (48.6)` —— **旧值 8 / 711 / 54.0 原样出现**。
- **M3** `listOutputShipped` 去掉 `c.SetTheme(...)`（便条板探针退回库存尺）：
  `"Door2": the plate starts at x=16, want … (round(16u) = 14)`。
- **M4** `readPlate` 右沿 `round(44u)−1` → `int(44u)−1`：
  `"Door2": the plate ends at x=39, want … (round(44u) − 1 = 38)`。

还原：`md5sum -c` 三文件全「成功」，`touch` 后三条 pin 复跑 `ok`。

**M2 的盲点照实登记**：便条带差那条 pin 唯读 app 自己量出来的 `a.Spacing`，所以在库存尺帧下
它**依然自洽**（`2.5×4.0=10`）——它钉的是「带子 = 2.5 个 app 的单位」，本来就不该由它判尺子；
判尺子的是 M2 里同时变红的那三条。

## 6. 出图（`FLOTER_PNG=1`，四张，全 720×439）

`/tmp/floter-P13-du-plates-{light,dark}.png`、`/tmp/floter-P13-du-rowblock-{light,dark}.png`。
用 `image.Decode` 逐像素复核 `plates-light`：门板在 `y=155` 那一行是 `14..39` 的连续暗块
（= 26 列，不是 28），其后 `49..50` / `54..57` / … 是标题字形的墨迹；`rowblock-light` 的帧高
从库存尺的 488 变成 **439**。**这张图上看到的一处变化**：板子的暗块比库存尺窄两列，
文字列因此落在 48.6 而不是 54 —— 与 pin 的数值一致。产品墨本轮零改动，所以深浅两套的**颜色**不变。

## 7. 用了 skill（`qiaomu-design`，打磨车道）

读了 `SKILL.md`（Router：已有方向 → 打磨模式）→ `references/craft-loop.md` → `references/preflight.md`（A/B/C/D2）。
本轮是**夹具账**（产品墨零改动），所以「偷」落在**量法**上，落地值逐条如下：

1. **`craft-loop.md` §二 菜单第 12 项「材质细节（内边框高光 / glass 折射边）」** → 落成便条板 ①冠线
   `door 6 / note 0`（浅）、`19 / 0`（深）与 ③底对侧 `14 / 14`、`8 / 8` 的**像素级**读数：
   「受光边只给可以被碰的面」这条材质分档现在有出货尺的数字。旧 pin 量的 5/0 与 10/10 是库存尺的帧。
2. **`craft-loop.md` §二 菜单第 6 项「编辑细节（hairline）」** → 落成板盒与行块沟的**发丝列**：
   `[round(x), round(x+w))` 是发丝唯一能落的那一列，pin 现在按它读（`14..39` / `7..712`），
   不再把 28 / 8 这些库存尺字面量当坐标。
3. **`craft-loop.md` §三 两轮制（Render → Critique → Refine）** → 本轮的第一轮就是**出图 + 逐像素复核**
   （`image.Decode` 扫 `y=155` 的连续块）， critique 抓到的是「板子窄两列、文字列左移 5.4px」，
   第二轮只修这三处 pin，没有顺手改别的表面。

`preflight.md` 逐条：

- **A（反 AI 套路）** 过：本轮零产品墨改动，没有新增渐变/纯黑/外发光/斜体/三等分卡片；
  `plates-{light,dark}` 的墨色与 P13-cp 完全一致。
- **B（设计读取一致性）** 过：三处 pin 现在都读**出货尺**（3.6 / 25.2 / 48.6 / 7.2），
  与 `docs/liquid-glass.md` §4 的刻度同源；旧读数是库存尺（4.0 / 28 / 54 / 8），已清零。
- **C（动效抽查）** 本轮不适用：没有动效改动；`renderShipped` 的 `Advance(500ms)` 仍保证读到过渡结束后的面。
- **D2（中文页 / 正文字号）未过**：`tokens.Caption` 9.9px、正文 10.8px 仍低于 preflight 的「正文 ≥ 14px」。
  这条从 P13-ar 起一直挂在「需拍板」（第 46 份证据）—— 字号是设计契约的一档，不是本轮夹具账能改的。

## 8. 仍待（P13-du）

1. **下一格（首选）**：`P13-dg-l-2` / `-l-3` / `-l-4`（`internal/launcher` 夹具账的剩余项：
   5 条前提破 S、2 条混排 B、产品墨 `outputsubtitle` 的 6 级）；先量再改。
2. 其后：`P13-dg-t` / `P13-dg-l` / `P13-dg-s` 的取景收口（terminalui + 另两个画面包），每包一格。
3. **需拍板（沿用 + 第 46 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十八次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
