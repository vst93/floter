# P13-dt 报告 · 启动器夹具最后三条单位算术（pin `20 → 18`、空页带 `60 → 54`、过滤条带 `60..80 → 54..72`）

方向一句话：**让夹具说 app 的话** —— 三条 pin 此前量的是框架库存主题（`Spacing 4.0`）的帧，
其中过滤条带那条量的框子比真带子大 8px，于是它「带子里没有行」的断言有 8px 永远量不到。
规格 `docs/rounds/P13-dt-spec.md`，合约新增 **§6.89**。

## 1. 先量（实测，不是推算）

`TestP13DTProbe`（临时探针，出完数即删）在 `renderShipped` 帧上打的：

```
designUnit=0.9000  5spaces=18.0000  15spaces=54.0000  filters=4.5000
themedFrame=439  stockFrame=488
pin box      x=632.60 y=81.00 w=18.00 h=18.00 (right=650.60)
held pin     x=633.32 w=16.56 h=16.56 ratio=0.9200 tenths=166
empty        x=334.00 y=232.10 w=52.00 h=18.00 centre=241.10
chip         y=54.50 h=17.00   band top 54.00  bottom 72.00
```

| 量 | 库存尺（此前钉的） | 出货尺（实测） |
|---|---|---|
| pin 静息盒 | `20`（写成 `19.5 < W < 20.5` 容差带） | **18.00** |
| pin 按下盒 | 只断言「比静息小」 | **16.56** = 18 × 0.92（`transform: scale(0.92)`） |
| 空页句子中线 | 带顶 `60` + 帧 `488`，比较带 `±6` | 带 `54 .. 428.2`，中线 **241.10** |
| 过滤条带 | `60 .. 80` | **54 .. 72** |

**这一轮的真正发现**：过滤条带那条 pin 不是「假红」，是**假绿** —— 它量的框子（`60..80`）
比真带子（`54..72`）往下多 8px，`72..80` 里真有行也不会红。判据（进合约 §6.89，永久）：
**pin 量的框必须由 app 自己的几何原语算出来**（`fieldRowUnits` / `breathUnits` /
`filtersHeight` / `designUnit()`），不许写像素字面量，否则框子漂了 pin 只量空气。

## 2. 落地（三处，全在 `_test.go`，产品代码零改动）

1. `internal/launcher/powerfaces_test.go` `TestTheRowPinSinksLikeTheSharedFieldControl`
   `render` → `renderShipped(t, a, false)`；`19.5..20.5` 容差带 → `tenths(pin.W) ==
   tenths(5*4*designUnit())`；按下 → `tenths(held.W) == tenths(rest*0.92)`。
2. `internal/launcher/emptystate_test.go` `TestTheEmptyPageSaysSoUnderTheField`
   `render` → `renderShipped`；`bandTop = 15 × 4 × designUnit()`（54）、
   `height = themedFrameHeight`（439）；中线与带顶都走 `tenths()`，`±6` 删掉。
3. `internal/launcher/filterband_test.go` `chipBandScene`
   `renderIn` → `renderShipped(t, a, dark)`；`float32(ui.LightTheme().Spacing)` 删掉，
   改 `top = 15 × 4 × u`、`bottom = top + 5 × 4 × u`（`filtersHeight` = `Space(5)` = 18）；
   行位置比较走 `tenths()`；返回值 `float32` → `float64`（不再借浮点截断比较）。

## 3. 台账

**V 3 / B 0 / S 0** —— 三处全是视觉数值 pin，其中两处顺带**删容差带**（断言强度只升不降），
一处是**假绿改真绿**。产品代码零改动（`git status` 里只有 `internal/launcher/*_test.go` 与文档）。
测试函数 2489 → **2490**（新增 `TestDumpP13DTPinPNGs`，opt-in）。空页 dump 的取景从库存尺帧
488 换到出货尺帧 439，路径从 `/tmp/floter-P11-ag-empty-*` 迁到 `/tmp/floter-P13-dt-empty-*`
（旧路径退役，P11-ag 那一轮的证据仍在本仓提交历史里）。

## 4. 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | **空** |
| `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` | 三平台**净** |
| `go test -count=1 ./...` | **全绿** |
| `go test -count=1 -v ./... \| grep -c -- "--- PASS"` | **3325**（基线 3325，本轮新增的是 opt-in dump，PASS 不变） |
| `--- FAIL` / `--- SKIP` | **0** / **157**（+1：本轮那条 opt-in dump） |
| 测试函数 | **2490**（基线 2489，+1） |
| `GOOS=linux/windows/darwin go build ./...` | 三平台**过** |

## 5. 变异（2 条实跑变红，`md5sum -c` 逐字节核对后 `touch` 复跑）

- **M1** `designUnit()` 返回 `spacing`（0.9 → 3.6）——
  `powerfaces_test.go:236: the pin's resting box is 18.00 wide (18.0), want the reference's 20u at the shipped step, 72.00 (72.0)`；
  `emptystate_test.go:73: the sentence is centred at y=241.10 (241.1), want 305.90 (305.9) (the band under the field, 216.00..395.80)`；
  `filterband_test.go:91: the row "clip 035" is drawn at y=72.0, inside the chips band (216.0..288.0)`。
- **M2** `designUnit()` 返回 `1.0`（等价于退回库存尺）——
  `... want the reference's 20u at the shipped step, 20.00 (20.0)`；
  `... want 243.50 (243.5) (the band under the field, 60.00..427.00)`；
  `inside the chips band (60.0..80.0)`。**旧值 20 / 60 / 80 原样出现**，这就是「转换量的是出货尺」
  的直接证据，也说明三处都真的读这把尺。

还原：`cp` 备份 + `md5sum -c /tmp/dt-md5-before.txt` 四文件全「成功」+ `touch` 后三测复跑 `ok`。

## 6. 出图（自己复核过像素）

`/tmp/floter-P13-dt-{pin,empty,chipband}-{light,dark}.png`，全部 **720×439**（出货尺帧）。

- **pin 行**：pin 盒 `x=632.60 y=81.00 18.00×18.00`，指针停在它上面（悬停态 `LitFace`）。
  图上看到的一处：浅色帧里 pin 格子的墨是 `(41,41,43)`、底子是 `(246,246,248)`；
  深色帧里墨 `(223,223,225)`、底子 `(15,15,16)` —— 同一枚 18px 格子在两套外观下都是
  「受光的面 + 一枚 glyph」，不是旧版那种带框小按钮。
- **空页**：句子在带子（`54..428.2`）的正中 `241.10`，帧底从 488 收到 439。
- **chips band**：`chips band 54.0..72.0`，行停在 72.0 之下（上一版量的是 `60..80`）。

## 7. 偷了什么（qiaomu-design，`已有方向，只要打磨` 车道）

读了 `SKILL.md`（路由表 → 打磨模式）、`references/user-preferences.md`（P-1 斜体禁令 / P-5 可读性优先
/ P-7 创意不吃功能契约）、`references/craft-loop.md`（工艺密度菜单 + 两轮制的取舍顺序）。
**没走 Phase 2 的四方向预览**（方向已由用户给定：轻便液态玻璃），也没出 HTML 预览页、没停下来等人点选。

逐条取用：

| 取自 | 落成哪个具体数值 |
|---|---|
| `craft-loop.md` 工艺密度菜单 **第 6 项「编辑细节」**（hairline / 大写字距标签这一族：让细节是被写下来的一条规矩，不是随手一画） | pin 的按下不是「比静息小」，而是**正好 0.92 倍**：18.00 → **16.56**（`tenths` 166）。这是把参考 CSS 的一条 `transform: scale(0.92)` 变成自家可达的断言。 |
| `craft-loop.md` 工艺密度菜单 **第 12 项「材质细节」** | 本轮**不适用**：这一轮产品像素改动为 **0**，材质一律没碰（材质账在 §1~§3，`Lens` 只剩 1 个调用点是上一轮结的）。取它的**判据**而不取它的笔：受光边的框子必须由 app 自己的几何算，不许写像素字面量。 |
| `craft-loop.md` 工艺密度菜单 **第 5 项「多层染色阴影」** | 同上不适用（零像素轮）。这一轮把它换成了同族的**夹具纪律**：一条 pin 量的框若与真框不同源，它量的就是空气 —— 过滤条带 `60..80` 对真值 `54..72` 的 8px 空档就是现成案例。 |
| `preflight.md` 的取舍顺序「两轮制 > DNA 注入 > 工艺密度凑满 5 项」 | 本轮一条表面（夹具尺）做到底：三处单位算术全交割，不留「下轮再凑」，并删掉两条容差带。 |

## 8. preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：本轮无新视觉语言，但删掉一处 AI 味的**假绿**（pin 在真框外面量空气）。
  没有渐变白边、没有 glass-on-glass、没有模糊贴图冒充玻璃 —— 零像素轮天然满足 §0 的四条硬禁令。
- **B 设计读取一致性**：三条 pin 现在读**同一把尺**（`designUnit()` = `Spacing/4` = 0.9，
  `internal/launcher/view_test.go:100`），帧一律 `renderShipped` + `themedFrameHeight`。
  走过检查：`internal/launcher` 里再没有 `float32(ui.LightTheme().Spacing)` 这种「库存尺混入出货帧」的乘法
  （`grep` 结果只剩 `frameHeight` 自己的注释）。
- **C 动效抽查**：按下这条状态反馈的**几何**被钉死（0.92 恒定），不是「看起来沉了一点」。
  弹簧/时长未动（`theme.ColorStep` 那条链本轮没碰）。
- **D2 中文页 ≥14px**：**仍未过** —— `tokens.Caption` 9.9px、正文 10.8px（第 **45** 份证据）。
  这是「需拍板」项：改字号会改高度链与键盘模型的可读性预算，不擅自改。

## 9. 仍待（P13-dt）

1. **下一格**：夹具账剩下的**「画家取整」那一族**（§6.87 的「取整（V）2 条」+ 本轮新发现的第三条）：
   `clipboardoff_test.go:130` 的 `int((rowUnits-launcherStatusUnits)*a.Spacing)` 截断
   （`2.5 × 3.6 = 9.0`，截断与四舍五入分家）；`noteplate_test.go:163` 的 `28u` 图标列
   （25.2 的盒子落 26 列）；以及 **`raycast_test.go:216-224` 的 `rowBlockIsTheInsetRoundedPill`** ——
   它一个函数里有三个库存尺读数（`gutter = int(ui.LightTheme().Space(2))` = 8、镜像 gutter，
   和 `float32(54) * float32(ui.LightTheme().Spacing/4)` = 54 **带 `±1.5` 容差带**），
   真值是 7.2 / 48.6 —— 三处都要按 painter 的 `round(x+w) − round(x)` 判据重读（§6.77）。
   **先量再改**。
2. `P13-dg-l-2` = 5 条前提破 S；`-l-3` = 2 条混排 B；`-l-4`（产品墨）= `outputsubtitle` 的 6 级。
3. **需拍板（沿用 + 第 45 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十七次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
