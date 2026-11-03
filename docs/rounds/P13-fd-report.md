# P13-fd 报告：空态那句话的居中带，比同一块面板的列表少算了一条底沿

轮号 P13-fd（HEAD 前 P13-fc）。方向：通用队列 · **空态满态**（`liquid-glass.md` §6.124 的
`124-5`：启动器无结果态的密度）。表面 = **启动器卡片 · 安静页**（结果区 / 插件模式空态）。
合约新增 **§6.125**。规格 `docs/rounds/P13-fd-spec.md`。

## 结论一句话

安静页那句话此前按「上 `listEdge` / 下 `Space(3)`」居中，而**同一块面板里列表**的下内边距是
`Space(0.5)+statusBand(c)` —— 同一个底沿两个数。量出来两条：那句话离卡底比离它回答的东西远
**10.8px**；按住 ⌘ 让停靠带出现时，**列表一行不动，这句话却往下跳 13.5px**。修法是让两页读
同一个表达式（`Space(0.5)+a.statusBand(c)`），那句话于是真的居中（19.6 / 19.6），带子来了也不动。

## 量的事实（真窗口 720×charged，en/zh × 深/浅四臂**逐值相同**）

| 臂 | charged | 那句话的盒 | 带顶→盒顶 | 盒底→卡底 |
|---|---|---|---|---|
| 打字无结果 `zzzz` | 113 | `Y=69.1 H=18` | **15.1** | **25.9** |
| 同上 + 按住 ⌘（停靠带在场） | 140 | `Y=82.6 H=18` | **28.6** | **39.4**（到带顶 12.4） |
| 剪贴板空态（chips 在场） | 131 | `Y=87.1 H=18` | **15.1** | **25.9** |

改后：

| 臂 | 那句话的盒 | 带顶→盒顶 | 盒底→卡底（/带顶） |
|---|---|---|---|
| 打字无结果 | `Y=**73.6**`（+4.5） | **19.6** | **19.6** |
| 同上 + 按住 ⌘ | `Y=**73.6**`（−9.0，**带子出现不再移动**） | **19.6** | **21.4**（到带顶 21.4） |
| 剪贴板空态 | `Y=**91.6**`（+4.5） | **19.6** | **19.6** |

`Padding` 的四值形式是 (top, right, bottom, left)（mygo `ui/element.go:720` `edges`），
所以「下 `Space(3)`」= 10.8，而列表是 `Space(0.5)`=1.8 + 带高 27。

## 落地

`internal/launcher/launcher.go`（安静页那一列，只此一处）：

```go
ui.Column(c).FillWidth().Grow(1).Center().
    Padding(listEdge, t.Space(3), t.Space(0.5)+a.statusBand(c), t.Space(3)).
```

上内边距（`listEdge`）、左右内边距、`.Center()`、`quietStatement` 的墨与字阶
（`Emphasis` + `TextStrong`，§4.7 / §6.47）、高度链（`Geometry.Height` / `RowCount`）、
键盘模型、i18n 文案：**一字节不动**。

## 台账

**V 1 / B 0 / S 0**。

- **V**：`TestTheEmptyPageSaysSoUnderTheField`（`emptystate_test.go`）把「带底 = 卡底 −
  `3*4*designUnit()`」写死在断言里，改成 `TailUnits*4*designUnit()`，注释写
  `P13-fd 重定基线：Space(3) → TailUnits`。断言强度不变（仍是十分位相等）。
- **B 0**：其余全绿（高度链、字段带、chips 带、键盘/停靠带族一条没红）。
- **S 0**。

新增 pin：`internal/launcher/emptydensity_test.go`

1. `TestTheEmptySentencesBandEndsAtThePanelsOwnTail`：三臂 × 深/浅 × en/zh，上/下空气各
   ∈ **[18, 21]px** 且两侧相差 **≤ 1.5px**（绝对像素判据，不是复述实现公式）。
2. `TestTheEmptySentenceDoesNotMoveWhenTheDockedBandArrives`：带子在场与不在场，那句话的 `Y`
   逐十分位相等；并断言 charged 真的变大（夹具自己在场，否则这条会「错着也绿」）。
3. `TestDumpP13FDEmptyDensityPNGs`（opt-in `FLOTER_P13FD_DIR`）：三臂 × 深/浅六帧。

## 门槛（全部实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` + `GOOS=windows go vet ./...` + `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**3396 PASS / 0 FAIL / 187 SKIP**
  （基线 = HEAD `f6b0827f` 开工时 **3394 / 0 / 186**；+2 PASS = 两条新 pin，+1 SKIP = 新增 opt-in 出图）
- `go test -count=1 -v ./... | grep -c -- "--- PASS"`：**3396**（只涨不跌）
- 测试函数：2560 → **2563**（只增不减）
- `GOOS=linux/windows/darwin go build ./...`：三平台全过

## mutation（2 条实跑 + `md5sum -c` 逐字节还原 + `touch` 复跑）

1. **M1** 底内边距退回 `t.Space(3)`：**两条 pin 同时红**。
   - `emptydensity_test.go:150: miss/light/: the sentence sits 15.1px below the band's top edge, not the 19.6px of a centred statement (box {X:334 Y:69.1 W:52 H:18}, band top 54.0)`
   - `emptydensity_test.go:190: light/: the sentence moves 13.5px when the docked band arrives (y=69.10 without it, 82.60 with it): the quiet column must reserve the band the way the list does (§6.125)`
2. **M2** 只加带子、tail 仍留 `Space(3)`（`Space(3)+statusBand`）：**pin 1 红、pin 2 绿**
   （`miss/light/: the sentence sits 15.1px below … 24.1px above the card's bottom inset …`）
   —— 证明「tail」与「带子」是两笔账，pin 2 只守带子那一笔。

两次都用 `cp` 备份 + `md5sum -c` 逐字节核对（`internal/launcher/launcher.go: 成功`）后 `touch` 复跑。

## 出图（自己用眼睛复核）

`/tmp/p13fd/{before,after}/floter-P13-fd-<arm>-<light|dark>.png`（`before` = 把那一行退回
`Space(3)` 渲的同一帧，六帧同尺）。

| 帧 | diff px（浅 / 深） | bbox |
|---|---|---|
| `miss` | 415 / 416 | `(335,73)-(386,86)` |
| `miss+band` | 542 / 542 | `(335,77)-(386,95)` |
| `clipboard-empty` | 812 / 814 | `(310,91)-(410,107)` |

**这张图上看到的一处变化**：`miss+band` 那一帧里，停靠带（`Ctrl` + `↩` + 「Open a terminal
session」）在改前改后都停在 `y=119`，而它上方那句话改前贴着 `y=82.6`、改后回到 `y=73.6`
——**带子没动，字退回去了**，那句话与带子之间第一次有了和它上方一样多的空气。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 12 项（材质细节：接缝 / 内边框高光）** → 落成**底沿 1.8px**：那句陈述的居中带底沿改成
  面板自己的 `TailUnits`（2u = **1.8px**），与列表下内边距**同一个数**。不是「居中好看一点」，
  是「同一块面板的接缝只许有一个值」。
- **第 6 项（编辑细节：hairline 作为基准）** → 落成**以 `y=50.4` 的发丝线为量空气的基准**：
  字段块的下沿那条 1px `HairlineSoft`（`launcher.go` 的 `Top(fieldRow-1)`）是页面上唯一一条
  真线，所有「上方空气」都从它量，不从文本框的盒量（盒比墨高 8px）。
- 第 5 项（多层染色阴影）**不适用**：空态按 §4.7 无底板、无阴影，本表面没有可加的层。
- 两轮制：第一轮先渲三臂六帧 → 四视角里「工程师」（同一面板两个底沿）与「读者」（带子一来
  字就跳）各挑出一处 → 第二轮只改这一行。

## preflight 逐条（`preflight.md`）

- **A 反 AI 套路**：本轮零新增视觉语汇（不动色、不动字、不加底板/发光/渐变），A 节无一项被触发；
  斜体/紫蓝渐变/纯黑底在本仓由 `raycast-design.md` §1 的 token 层直接排除。
- **B 设计读取一致性**：读取仍是「近黑/近白底 + 三级面 + 发丝线 + 紧凑行」的轻便液态玻璃；
  本轮只把一句话的**底沿**对齐到同一块面板的 tail，不改变读取，记忆点仍是「一句话 + 空气」。
- **C 动效抽查**：本轮不动动效；带子出现时那句话**不再有位移**——这正好是 C 节「高频/键盘
  触发操作没有动画」的同一条精神（按住 ⌘ 是高频键盘态）。
- **D2 中文页**：zh 臂与 en 臂**逐值相同**（`没有匹配项` 盒 `X=330 W=60`、`暂无复制记录` 盒
  `X=324 W=72`，`Y` 与 en 完全一致）；字号未改。**未过项**：正文 10.8px / caption 9.9px 对
  D2「正文 ≥ 14px」仍是**沿用中的需拍板项**（第 72 份证据），本轮不擅自改字阶。

## 仍待

1. **下一格**：通用队列的**边界情况**这一族剩下的最后一条——安静页在**最小窗宽**下那句话
   （`SingleLine()`）会不会被裁；以及 §6.125 的 `125-5`（全新安装那一页收 113px 却一个字都不画）
   要动高度链，**需拍板**。
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条（字号阶梯对 preflight D2、
   启动器结果行盒高、§6.111/§6.113/§6.114 的尾巴、终端弧 S1b/S1c/S2 尾）。
