# P13-al 报告：按下这一档落在 shell 的四个键上（静息 246/20 → 悬停 220/53 → 按下 238/32）

规格：`docs/rounds/P13-al-spec.md`。合约：`docs/liquid-glass.md` **§6.7**（新增 7-5..7-8）+ §5「按下」。

## 这轮为什么让它更有空间感

空间感是**东西知道自己在哪一层、并对手做出反应**。插件窗口的 bar 与 toast 的键是应用里
最常被点的四个面，它们此前只有两档：静息（透明，只剩字形）与悬停（E1 的一个面，P13-y/P13-z 的
成果）。指针**按下去**的那一帧与悬停帧**逐像素相同**——面不沉、盒子不收，读起来是「点了没
反应」，于是这三个窗口里最像实物的东西反而最像一张贴纸。补上第三档之后，同一个键有了
**三个 z 深度**：静息=无面、悬停=抬起一格（受光边 + 对侧暗线）、按下=沉回静息方向一格并且
整块往里收 8%。这正是 §5 那一行写死的读数，也是 §2 阶梯在**交互维度**上的那一半——层不是
画上去的，是按出来的。

## 落地

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/shell/detach.go` | `barButton` 闭包在 `!enabled` 与 `Hovered` 之间插 `else if button.Pressed()` 臂：`LitFace(Background(ControlPress))` + `Scale(0.92)`；新增 `pressScaleChip`/`pressScaleKey` 两个族常量 |
| 2 | `internal/shell/pinned.go` | 便条 ✕ 同一条臂（`ControlPress` + 0.92，`LitFace` 在臂内重画） |
| 3 | `internal/shell/notices.go` | toast ✕：按下臂在前，`LitFace(Background(ControlPress).Scale(0.92))`，墨保持 `TextStrong`；toast 动作胶囊：`Background(ControlPress)` + `Scale(0.97)` + `LitFace` |
| 4 | `internal/shell/pressrung_test.go`（新，+3 测试函数） | 两条像素 pin（面 / 几何，深+浅 × 三个键）+ 一条 opt-in 出图 |

**改动后实测**（同一帧、同一列，取键的 padding 列 = 材料而不是字形）：

| 键 | 静息 | 悬停 | 按下 |
|---|---|---|---|
| output-rerun（浅） | 246/246/247 | 220/220/221 | **238/238/239** |
| output-close（浅） | 246/246/247 | 220/220/221 | **238/238/239** |
| pinned-close（浅） | 246/246/247 | 220/220/221 | **238/238/239** |
| output-rerun（深） | 20/20/22 | 53/53/55 | **32/32/34** |
| output-close（深） | 20/20/22 | 53/53/55 | **32/32/34** |
| pinned-close（深） | 20/20/22 | 53/53/55 | **32/32/34** |

按下的面**严格落在静息与悬停之间**（浅 238 在 246 与 220 之间、深 32 在 20 与 53 之间）——
这就是「第三档是往静息的回退」的算术，不是第三种颜色。几何：画出来的键宽 45 DIP（重跑键）
/ 32 DIP（两个 ✕），按下时不收就是 45/32 原样（M2 实测），收 0.92 后窄两格以上。

## 这一轮在帧上看到的一处变化

同一帧、同一坐标 `(键盒左缘 + 6, 键盒中线)`：**浅色 220 → 238、深色 53 → 32**。
两个方向相反——浅色的梯子是「越按越浅」（黑洗从 0.14 退到 0.032），深色是「越按越暗」
（白洗从 0.105 退到 0.05）——这正是同一条「往静息退」的梯子在两套外观上的两种算术。
出图 12 帧：`/tmp/floter-P13-al-<output-rerun|output-close|pinned-close>-<light|dark>-<hover|press>.png`。

## 偷了什么（qiaomu-design / craft-loop.md 工艺密度菜单）

- **第 12 项「材质细节」**：落成**按下档的那一对面**——`ControlPress` 的面 + 臂内重画的
  `LitFace`（顶 `LensRim`、底 `LensEdge`），与静息/悬停共用同一对 token，只换面的 rung。
  具体数值：浅色面 220 → 238（升 18 级）、深色 53 → 32（降 21 级）。
- **第 4 项「签名交互」**（单页签名交互 ≤1 的约束下，这一族的选择是 hover/active 的**揭示**）：
  落成几何收档两档：`pressScaleChip = 0.97` / `pressScaleKey = 0.92`，与 P13-q 设置页的
  胶囊/小格是同一对数，不另起第三个数。
- **第 6 项「编辑细节」**：落成 `internal/ci` 臂级 census 的**既有约束**（按下臂也要有那一对线），
  于是四个新臂都各自带 `LitFace`——不是靠注释约定。
- **第 5 项「多层染色阴影」故意不用**：`liquid-glass.md` §2 判据 2 —— 键是 E1，长在 bar 上，
  压不出影子；给它影就是把「贴着的块」画成「浮起来的纸」。
- **第 1/2/3/7/8/9/10/11 项不适用**：自绘 Go 界面，无 `::before` 氛围层、无 `::selection`、
  无浏览器 focus 环、无流体字号、无滚动条造型、无图形资产位、无页脚。

## preflight 逐条结论

- **第 0 步 用户偏好账本**：P-1（斜体）无斜体；P-5（可读性优先）按下只动面与几何，字墨在
  按下时保持 `TextStrong`，对比度不降；P-38（禁左侧竖线）零命中；P-7/A4（功能契约）——
  键的 `Label` / `Clicked` / `Disabled` 一个字节没动，**过**。
- **A 反 AI 套路**：无紫蓝渐变 / 无纯黑 / 无外发光 / 无渐变字 / 无斜体 / 无三等分卡片 /
  无左侧竖线 / 文案零改动、感叹号 0 → **过**。
- **A2 进攻三步核验**：密度第 12 + 第 4 项（有具体数值，见上）；两轮制 = 量（M1 注入后
  220↔220 / 53↔53 证明两帧相同）→ 改 → 再量（238 / 32 落在两端之间）→ 出 12 帧复核；
  四视角各挑一处见下 → **过**。
- **A4.1 Carbon 决策门（disabled / read-only 不混淆）**：这一轮只碰**能用的键**；禁用的重跑键
  仍走 `Disabled(true)` 那一臂（P13-y 定的 0.5 单道淡），按下臂写在 `!enabled` **之后**，
  禁用的键按下去不会亮面 → **过**。
- **B 设计读取一致性**：深浅两帧同一套 token（`ControlPress` / `LensRim` / `LensEdge`），
  没有为某一套外观写死像素；记忆点一句话——「同一个键有三个深度，按下去会沉一格」→ **过**。
- **C 动效抽查**：本轮零动效改动（这四个键本来就没有 `Transition`；按下就是当帧切面，
  `Scale` 是渲染变换，没有 `transition: all`、没有时长魔数）。§5 的 120ms 颜色过渡是
  `Control` 面族的规则，这几枚键从未设过，本轮没有把「补一档面」偷偷变成「补一条动效」→
  **过（不适用项以「未改动」记）**。
- **D2 中文页追加项**：i18n 零改动；中文仍是 `pressrung_test.go` 之外的既有标签
  （本轮的键只有 ✕ 图标与英文词条），中文断行/全角标点未触碰 → **过**。
- **E 三秒测试**：按下的一瞬间，指针下的键亮了又沉、并且小了一圈——「它收到了」→ **过**。

### 四视角评审（各挑一处）

- **艺术总监**：最弱的一处是「悬停之后没有下文」——一层抬起来的面按下去毫无变化，等于把
  交互的最高潮停在半句。
- **工程师**：`Scale` 是渲染变换，布局盒不动，所以第二枚 pin 量的是**画出来的键宽**而不是
  `Find` 的盒子；顺带把「渲染器改成挪盒子」这条路也堵死了。
- **甲方**：这三个窗口里的键现在按下去会动，我不再需要怀疑自己刚才那一下有没有点中。
- **读者**：静息只有字形 → 悬停有面 → 按下有面且更小，三层读法互不重叠。

## 门槛（实测）

- `gofmt -l cmd internal` 空；`go vet ./...`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...` 净。
- `go test -count=1 ./...` 全绿、FAIL 0；PASS **3045 → 3047**；SKIP 92 → **93**（+1 opt-in 出图）；
  测试函数 2249 → **2252**。
- `GOOS=linux/windows/darwin go build ./...` 全过。
- 台账：**V 2 / B 0 / S 0**（V = 本轮新增的两条像素 pin；没有旧 pin 被改写）。
- `internal/ci` 的臂级 census（`TestEveryPointerStateFaceNamesItsPair`）**零改动**即通过：
  四个新臂里都写了 `LitFace`，不需要往 `pointerFaceAllow` 里加任何豁免。

## 变异（实跑）

- **M1** 三处按下臂的 `ControlPress` → `ControlHover` → **红**：
  `light/output-rerun: the key under the pointer is {220 220 221 254} whether or not the button is down (rest {246 246 247 254}): the press rung paints no face of its own — `--glass-control-press` is one step back toward the resting face (docs/liquid-glass.md §5)`（dark 同，三个键各一条）。
- **M2** `Scale(pressScaleKey, pressScaleKey)` → `Scale(1, 1)` → **红**：
  `light/output-rerun: the key paints 45 DIP wide hovered and 45 pressed — the press does not step the box back (0.92 is the family's number, P13-q's icon keys)`（dark 同，✕ 两个键是 32/32）。
- 两条均 `md5sum -c /tmp/p13al.md5` **逐字节还原成功**（`internal/shell/{detach,pinned,notices}.go` 三个文件全 `成功`），
  `touch` 后复跑两条 pin 全绿。

## 仍待（P13-al）

1. **同一族的第五个键没做**：`internal/terminalui/terminalui.go:1400` 的 `barButton`（终端浮条上的
   五个 28u 键：设置 / 钉住 / 新命令 / 外部打开 / 关闭）仍然只有两档。它不能直接抄
   `ControlPress`——那一条的墨来自**画布**（`canvasInks`，命名调色板时是调色板自己的前景），
   而 `barInks` 目前只有 `Ink/Muted/Tertiary/Wash/Face` 五档，没有「按下的洗色」这一档；
   要补的是「调色板自己的 press 洗色」（`inks.Wash` 的两倍？）——这是**材质决策**，
   需要一次单独的量（下一轮候选）。
2. **需拍板**：按下档要不要保留那一对线，全仓仍不一致——`launcher.rowControl/Pressed` 与
   `launcher.fieldSettingsButton/Pressed` 在 `pointerFaceAllow` 里记作「需拍板」（P13-aa 仍待 2），
   而设置页（P13-q）与这一轮都**保留**了。本轮按多数派做，没有动那两行豁免。
3. 两条抢救 stash（`afc38b24` / `114b7d15`）与 `git stash list` 里其余条目本轮未变，
   内容仍在 HEAD，按纪律没有静默 drop，也没有再试合。
