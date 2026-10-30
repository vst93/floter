# P12-a 报告 · 视觉合约升代：轻便液态玻璃 + 三档 elevation（token 层）

> 轮号 **P12-a**｜开工 HEAD `85a7a532`（P11-cr）｜提交见文末
> 规格 `docs/rounds/P12-a-spec.md`｜新合约 `docs/liquid-glass.md`

## 设计读取（qiaomu-design 第 0 步）

> **读取为：** 桌面启动器 / 设置 / 终端的**应用外壳**，面向每天用几百次的键盘用户，
> 用**轻便液态玻璃**（半透明面 + 一条受光边 + 柔和外投影）语言，倾向 **Apple 的
> elevation 阶梯 + 中性墨阶**（不是 macOS 26 的厚玻璃，也不是 Web 那套 backdrop-filter）。

三个拨盘（中文讲明白）：

| 拨盘 | 值 | 人话 |
|---|---|---|
| 视觉冒险度 | **4** | 功能页；不换脸，只把层级做出来。方向已由用户给死。 |
| 动效强度 | **3** | 高频操作（每次启动器唤醒）不加动效；只保留已有的 90/120/160ms 与按压。 |
| 信息密度 | **6** | 行高、文字列、键帽全部不动——这一支只改墨与面。 |

**功能契约**：这页存在的理由是「敲一个字就找到东西并运行它」；3 秒内必须看到输入框
在等光标、列表第一行是可运行的；必须能完成的动作是输入、上下选、回车跑、Esc 退。
本轮**一个字都没碰**这些（零行为改动）。

## 这轮为什么让它更轻 / 更有空间感（一句话）

**因为它把「所有东西同一层」的算术改掉了：面从 0.075/0.05 抬到 0.10/0.07（能一眼分家），
受光边从画不出来的 0.04 抬到 0.09（顶边真的亮起来），浮层的两层影加厚
（0.55/0.40 → 0.62/0.44，浅 0.18/0.10 → 0.22/0.12）——层级从此由「面差 + 一条线 + 两层影」
说，不再由 1px 描边说。**

## 偷了什么（craft-loop.md 工艺密度菜单，逐条落到数值）

按 `craft-loop.md` 的「参考 DNA 注入 + 工艺密度 ≥ 5」执行。方向已定（用户给的），
所以走「打磨模式」，DNA 供体从本仓自己的参考 `main` 的 `base.css` 与 Apple 的
elevation 语汇里取，逐条写清「取了第 N 项 → 落成哪个具体数值」：

| 菜单项 | 落成的具体数值 |
|---|---|
| **#5 多层染色阴影**（取 2 层，不是 1 层） | `ShadowAmbient` 深 0.62 / 浅 0.22（宽投影）+ `ShadowContact` 深 0.44 / 浅 0.12（紧贴边）；`theme.ElevThrow` 已按 `--elev-shadow-scale` 随透明度缩放（P11-an） |
| **#12 材质细节**（内边框高光） | `LensRim` 0.04 → **0.09**（唯一受光边，1x 屏上真能看见）；对侧 `LensEdge` 0.04 → **0.06**（顶亮一分、底暗一分，控件才站起来） |
| **#6 编辑细节**（hairline / 大写字距标签） | 不动：发丝线两档（0.09 / 0.055）与行态三档（hover/选中/按下）保持，因为它们是**行为契约**的一部分（B 类 pin 守着） |
| **#3 品牌 focus 环** | 不动色相：焦点仍靠 `EdgeHighlightActive` 的明度差（深 0.18 → 0.24，浅 0.96 不动），符合 §5「accent 只回答哪个被选中」 |
| **#1 氛围层**（透明度 < 6%） | 卡的 sheen 保持 `SheenTop` 0.04 / `SheenBottom` 0.01——**面不许反光**，轻便玻璃的反光是「一条线」不是「一片渐变」 |

**反向也记一笔（我故意没偷的）**：#7 展示排版动作、#8 流体字号、#9 自定义滚动条、
#10 收尾彩蛋。理由：这是一个每天开几十次的桌面外壳，任何一处「展示动作」都会在
第 100 次使用时变成噪音；而且这一支的红线是「只改墨与面」。

## 落地

### 1. 新合约（进仓库）

- `docs/liquid-glass.md`（新，单一来源）：§0 自绘的物理边界（面差 / 受光边 / 阴影 /
  半透明四件 + **四条硬禁令**）、§1 材质三档 + **光泽上限旧值→新值表**、§2 elevation
  三档（E0/E1/E2 + 三条判据）、§3 受光边与阴影数值、§4 圆角与间距刻度（重申不改）、
  §5 状态与动效、§6 **逐表面差距表 35 行**（选题队列）。
- `docs/raycast-design.md` 顶部降级为「已由 `liquid-glass.md` 升代，本文只保留行为契约
  与已落地条目」，并点名被取代的两条（低光泽上限 / 分层手段）；§1 token 表同步四行新值。

### 2. token 层（`internal/theme/theme.go`）

八个数，两套外观各一行，逐条带 `P12-a 重定基线：旧值 → 新值` 注释：

| token | 旧（dark / light） | 新（dark / light） |
|---|---|---|
| `control` | 0.075 / 0.05 | **0.10 / 0.07** |
| `controlHover` | 0.105 / 0.075 | **0.14 / 0.105** |
| `lensRim` | 0.04 / 0.04 | **0.09 / 0.09** |
| `lensEdge` | 0.04 / 0.04 | **0.06 / 0.06** |
| `edgeHighlight` | 0.15 / 0.92 | **0.20 / 0.92** |
| `edgeHighlightActive` | 0.18 / 0.96 | **0.24 / 0.96** |
| `shadowAmbient` | 0.55 / 0.18 | **0.62 / 0.22** |
| `shadowContact` | 0.40 / 0.10 | **0.44 / 0.12** |

**不动的**：`lensSheen`（0.04）、`sheenTop`/`sheenBottom`（0.04/0.01）、
`sheenTopActive`/`sheenBottomActive`（0.05/0.012）、`controlPress`、行态三档、
发丝线两档、accent 全家族、半径梯、高度链、字号梯。

### 3. 新 pin（2 条，+4 个 PASS）

- `internal/theme/theme_test.go` `TestTheThreeElevationRungsAreVisibleAndOrdered`
  （两套外观各一臂）：① 面 ≥ 0.07 地板 ② 浮 > 面 ③ 抬起/静息/按下 ④ **受光边严格
  亮于面**、≤ 0.09、不透明不是它、且必须中性（三通道相等）。
- `internal/ci/liquidglass_contract_test.go` `TestTheLiquidGlassContractSupersedesTheRaycastOne`：
  ① raycast 顶部必须点名升代 + 两条被取代的规则；② 差距表每一行的**状态只能是四个词**
  （`未开始` / `已落地` / `需拍板` / `故意不做`）。实测读到 35 行、35 行都带状态。

## V / B / S 台账

| pin | 类 | 处置 |
|---|---|---|
| `theme_test.go` `TestTheSurfaceLadderHasAStepToSee` | **V** | 钉到新值（`Control` 0.07/0.10、`ControlHover` 0.105/0.14）；「面是面不是发丝」地板 0.045 → 0.06 随新面档走 |
| `launcher/raycast_test.go` `TestTheCardWearsTheContractsRadiusEdgeAndCast` | **V** | 「整个透镜家族 ≤ 0.04」拆成三件：面 ≤ 0.04、受光边 ≤ 0.09、**受光边严格亮于面**；`LensEdge` 单独 ≤ 0.06 且必须无彩色 |
| `shell/namedrows_test.go` `TestTheSearchCardFillsTheWindowLessTheInset` | **V** | 边上那一列从「alpha ≤ 8」改成读**影的形**：三通道必须为 0（填充必带色相）且 alpha < 0.2 |
| 其余 2801 条 | **B** | **零改动全绿**（改产品不改测试） |

判不准的一律按 B 先跑：三处红全部是「断言钉着旧设计的数值」，不是产品回归。

## 门槛（实测）

```
gofmt -l cmd internal                        → 空
go vet ./...                                 → 净
GOOS=windows go vet ./...                    → 净
GOOS=darwin  go vet ./...                    → 净
go test -count=1 ./...                       → FAIL 0
go test -count=1 -v ./... | grep -c "--- PASS" → 2805（基线 2801，+4）
测试函数总数                                   → 2084（基线 2081，+3）
SKIP 计数                                      → 59（基线 58，+1 = 本轮新增的 opt-in 截图 dump）
GOOS=linux/windows/darwin go build ./...      → 三平台过
```

**SKIP 58 → 59**：唯一新增的 skip 是本轮的 opt-in 截图 dump
（`TestDumpP12AElevationFrames`，未设 `FLOTER_PNG` 时 skip），与 P11-cq 的 dump 同一条
规矩——`t.Skip` 只允许这一种形状，且它是本轮唯一允许增加的一条。

## 变异（4 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑绿）

| # | 注入 | 实测报错（原文抄） |
|---|---|---|
| **M1** | `lensRim` 改回 `0.04` | `theme_test.go:1252: the lit edge (LensRim 10) is not brighter than the face (LensSheen 10)`；`launcher/raycast_test.go:97: the lit edge (LensRim 10) is not brighter than the face (LensSheen 10): one lit line is the whole of this language's gloss` |
| **M2** | `control` 改回 `0.075`/`0.05` | `theme_test.go:426: Control = {0 0 0 13 [0 0 0]}, want {0 0 0 18 [0 0 0]}`（浅）、`Control = {255 255 255 19 …}, want {255 255 255 26 …}`（深）+ `TestTheThreeElevationRungsAreVisibleAndOrdered` 红 |
| **M3** | 删掉 raycast 顶部的升代声明 | `liquidglass_contract_test.go:38: the raycast contract's header does not say it was superseded ("低光泽上限" missing)` + 同一行的 `("分层靠面" missing)` |
| **M4** | 差距表某行状态写成 `大概做了` | `liquidglass_contract_test.go:94: a gap-table row carries an unrecognised status "`大概做了`"; the four words are 未开始 / 已落地 / 需拍板 / 故意不做` |

M1 与 M2 都**同时**红在两个包里（`theme` 与 `launcher`）：同一条材质不变量在两处各有
一份语言，删一处守不住另一处——这正是 P11-cl 那条「只数次数守不住跑的是哪一行」的同类教训。

## 图（自验，深/浅两套，720×580）

`/tmp/floter-P12-a-{launcher,sessions}-{dark,light}-{before,after}.png`
（`before` 是干净 HEAD 渲出来的，`after` 是本轮；两张同尺寸、同场景、同 fixture）。

**我在图上看到的一处变化**（逐点读像素，不是「看起来差不多」）：

- `sessions/dark (400,200)`：一行**面**（设置行块）`(55,55,57,151)` → **`(66,66,68,154)`**。
  同一帧的页底 `(360,60)` 两版都是 `(32,32,34,119)` 没动——所以这一处变化是**面往上
  抬了一档**，底没动：这正是「面与底分家」的直接读数（11 个亮度级）。
- `sessions/light (400,200)`：`(226,226,228,148)` → **`(220,220,220,150)`**（浅色是黑 alpha
  抬起，所以像素**变暗** 6 级，同一件事的另一面）。
- `launcher/dark (1,300)`（窗口外沿、卡投影落进来的那一列）：`(0,0,0,25)` → **`(0,0,0,28)`**；
  `(5,300)`：`(0,0,0,52)` → **`(0,0,0,57)`**。三通道仍是 0 —— 它变浓的是**影**，不是底
  （底色是 `#0E0E10`，必带色相）。`launcher/light (5,300)`：`(0,0,0,16)` → `(0,0,0,21)`。
- 整帧差异包围盒：四张都是 `x=0…719, y=0…579`（token 层是全局的，所以是整帧），
  `launcher/dark` 23876 px、`launcher/light` 22318 px、`sessions/dark` 208758 px、
  `sessions/light` 206377 px 不同。

## preflight（A / B / C / D2 逐条结论）

**A · 反 AI 套路**

- 无紫蓝渐变 / 无纯黑 / 无外发光 / 无大标题渐变字：**过**（accent 仍是参考的蓝，警告红只
  在语义处；本轮一个色相都没加）。
- 字体不是 Inter/Roboto/默认：**过**（Go 自绘，字体来自主题，本轮未动）。
- 无 `font-style: italic`：**过**（自绘没有斜体这一档）。
- Hero 居中构图 / 三等分卡片 / 假数据 / 空词文案：**不适用**（不是落地页，是桌面外壳）。
- 无左侧竖线装饰（P-38）：**过**（P8-d 已删掉 `cardRow` 的 hover 前导规则条，本轮未复活）。
- **本轮新增的一条自查**：受光边必须是**中性**——pin 里已断言 `LensRim` 三通道相等；
  这条防的正是用户点名的「用渐变白边模拟高光」，色相是它的第一条泄密点。

**B · 设计读取一致性**

- 声明了「轻便液态玻璃 + elevation 阶梯」，交付的是**八个数**与**两条判据**（面 > 底、
  受光边 > 面），一致：**过**。
- 一页一个明暗主题：**过**（深/浅两套各自一套 token，不混）。
- 这一页的记忆点一句话：**「面真的浮起来了，但一条线亮在它顶边」**。
- 动效气质与视觉气质一致：**过**（本轮零动效改动；`§5` 明确「没有手势动量就没有 bounce」）。

**C · 动效抽查**

- 无 `transition: all` / 无 `scale(0)` 入场 / UI ≤ 300ms：**过**（本轮未动任何时长；
  既有的是 90ms 颜色、120ms 行进入、160ms 浮层入场、6px 的 `shell-enter`）。
- `prefers-reduced-motion` 兜底：**过**（`ui.Preferences` 已有；本轮没新增动画所以没新增
  兜底需求）。
- 只动 transform/opacity：**过**（本轮改的是颜色与阴影的 alpha）。
- **一条不能过的项，写清为什么**：`hover 动效包在 @media (hover: hover)` —— 这条在
  Web 上有意义（触屏没有 hover 态会「粘住」），在 Go 自绘里由框架的指针事件决定，
  本仓没有这个媒体查询；记为**不适用**，不静默跳过。

**D2 · 中文页追加项**

- 中文正文/UI 用系统字体栈：**过**（自绘主题的字体栈，本轮未动）。
- 行高 ≥ 1.5 / 正文 ≥ 14px / 字重只用 400/500/600：**过**（字号梯本轮未动；
  `docs/raycast-design.md` §3 已钉「字号档位不动」）。
- 中西文之间有空格、全角标点：**过**（本轮没有新增任何用户可见文案——`liquid-glass.md`
  与 `raycast-design.md` 都是文档，不是 UI 文案）。
- 暗色页正文 ≥ 7:1：**过**（P11-aa 的 `TestEveryInkStepClearsTheReadableContrast` 仍在，
  本轮未动墨色）。

## 仍待（P12-a）

1. **`docs/liquid-glass.md` §6 的差距表就是下一轮的队列**：第一个 `未开始` 是
   **§6.1 启动器**（7 行：卡片 / 选中块受光边 / 页脚 / 键帽 / 输出面板 / 空态 / 暖面行）。
   下一轮按「一个表面一轮」做启动器，做完把那 7 行标 `已落地`。
2. **E1 的受光边还没落到任何一个组件上**：本轮只把 `LensRim` 抬到 0.09，`theme.Tokens.Lens`
   已经在画它，但**哪些面该画、哪些面不该画**是逐表面的活（差距表 §6.1–§6.6）。
   其中三条我已在表里标 `故意不做` 并写了理由（4px 药丸 / 包 chip / 状态灯），
   剩下的是逐个表面的判断。
3. **`Float` 档的差值（+0.10）没有动**：实测默认 47% 时 `Float` 是 0.57、`Control` 是 0.10，
   差距足够；但如果用户把透明度拉到最低（10%），`Float` 是 0.20 而 `Control` 仍是 0.10——
   那时两者只差 0.10，可能需要按框的 alpha 缩放面档。**这是下一轮启动器切片要量的第一个数**
   （先量再决定改不改，别先改）。
4. **macOS 的 E0 没有投影**（由窗口服务器给）。§6.1 第 1 行标 `未开始` 但结论很可能是
   `故意不做`——要在一台真 macOS 上看一眼才能拍，本轮没有那台机器。
