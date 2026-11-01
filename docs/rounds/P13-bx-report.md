# P13-bx 报告：终端空态的两条出路行，按下去此前与悬停帧逐像素相同

轮号 `P13-bx`｜分支 `mygo-rewrite`｜基线 HEAD `66435145`（P13-bw）
规格 `docs/rounds/P13-bx-spec.md`｜合约 `docs/liquid-glass.md` §6.41 第 **41-1 / 41-2** 行

## 0. 一句话

终端空态的两条「出路」行（`↩ Start a session` / `⌘N New command`）是这一页**唯一能点**
的东西，可它按住不放的那一帧与悬停帧**逐像素相同**——一条能被按下去的行，按下去却不回答。
本轮补第三档：按下画**面**（`t.SurfacePressed`，与设置页那张卡的行同一个 token），并
**保留**悬停的抬墨；几何一个数没动。

**这轮为什么让它更轻、更有空间感**：静息与悬停都是「字」，只有按下那一刻才第一次出现
**一块面**——于是这一行在指针下走完「字 → 字更亮 → 一块板落下来」三步，而不是停在
「字 → 字更亮」这一步不动。行本身仍然没有描边、没有投影（§2 判据 2：长在底上的东西不
压影子），所以它增加的是**触摸感**，不是又一层画框。

## 1. 先量（改前，实测像素）

探针：行内**空隙**那一点（标签盒左 4px，即键帽右缘与首字之间），`860×600` 空态页。
改前 `emptyLine` 只有 `if line.Hovered() { color = tokens.TextStrong }` 一条臂（`Pressed()` 零处）：

| 外观 | 页面 | 静息 | 悬停 | 按下（改前） |
|---|---|---|---|---|
| 浅 | `252` | `252` | `252` | **`252`**（与悬停逐像素相同） |
| 深 | `21` | `21` | `21` | **`21`**（与悬停逐像素相同） |

复现命令（改前会红，报错串见 §5 的 M1）：

```
go test -count=1 -run TestTheEmptyStateWaysOutSinkWhenPressed -v ./internal/terminalui/
```

## 2. 落地

`internal/terminalui/terminalui.go` 的 `emptyLine`：

- 墨/面写成 `switch` 三臂：`Pressed` → `TextStrong` + `Background(t.SurfacePressed)`；
  `Hovered` → `TextStrong`；静息 → `TextSecondary`。
- 按下**保留**悬停的抬墨：指针还压在上面，字不该退回去（M2 就是这条的反例）。
- **不缩放**：这一行是**行**不是**键**，`Scale` 会把标签与键帽一起搬走
  （M3 实测标签盒 `{385 287.5 123 22}` → `{386.35 287.83 119.31 21.34}`）。
  面借 `settingsui.cardRow` 的第三档（`rows.go:1405-1416`），那一条也不缩放。
- 注释补「抬墨是悬停的说法、按下是面的说法」与不缩放的理由。
- `Padding` / `Gap` / `RadiusSM` / `Focusable` / 键帽 `LitFace` / `run()` 接线：
  **一个字节没动**。

改后（同一探针）：

| 外观 | 页面 | 静息 | 悬停 | 按下（新） | 标签墨（静息 → 悬停 → 按下） |
|---|---|---|---|---|---|
| 浅 | 252 | 252 | 252 | **221** | 145 → 225 → **225** |
| 深 | 21 | 21 | 21 | **60** | 133 → 216 → **216** |

## 3. 出图（深/浅 × 静息/悬停/按下，六张）

`/tmp/floter-P13-bx-emptylines-{light,dark}-{rest,hover,press}.png`（opt-in，`FLOTER_PNG=1`）。

**这张图上看到的一处变化**：悬停帧与按下帧的差异盒是 `(348,286)-(511,311)`——
正好是那一行的盒子（行高 26 = 键帽 20 + 上下各 2 的 padding），浅色 **4205** 点、深色 **4208** 点；
行外的像素一个都没变（同一张图上标题 `Terminal` 与下面那行 `Back to search` 逐值相同）。
键帽是不透明的，所以那块面**从键帽右侧才开始可见**——面在行盒里、被键帽压住一段，
这是「面属于行、键帽仍然是一块戴受光边的板」的正确读法。

## 4. 门槛（实测）

| 项 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 三套净 |
| `go test -count=1 ./...` | 全绿（FAIL 0） |
| `--- PASS` 计数 | 3217 → **3220**（+3：一条 pin + 深/浅两个子测） |
| `--- SKIP` | 121 → **122**（+1 = opt-in dump） |
| 测试函数 | 2376 → **2378**（+2） |
| 三平台 `go build ./...` | 过 |

**V 0 / B 0 / S 0**：旧 pin 一条没红，测试零改动（本轮只新增文件）。新增的 pin 是
`internal/terminalui/emptylinesink_test.go`。

## 5. 变异（3 条实跑全红，`md5sum -c` 逐字节还原）

基线 `internal/terminalui/terminalui.go` = `ce95f7aa820e263e060175d16cdc259f`。

- **M1**（按下臂不画面 = 本轮之前的行为）：
  `the held frame's gap reads 252 and the hovered frame's 252 — the press painted no face of its own (want |press − hover| ≥ 6)`（浅），深色同串 `21 / 21`。
- **M2**（按下把悬停的抬墨退回）：
  `the label carries 145 levels of contrast at rest and 145 while held — the press dropped the hover rung instead of keeping it (want press ≥ rest + 10)`，
  以及 `the label carries 225 levels under the pointer and 145 while held — the press dimmed the ink it had just lifted`。
- **M3**（按下加 `Scale(0.97)`）：
  `the label's box moved {385 287.5 123 22} → {386.35 287.83 119.31 21.34} between the hovered and the held frame — a row is a row, not a key: the press rung changes the face, never the geometry`。

每条还原后 `md5sum -c` 成功、`touch` 后复跑一次仍绿（避免测试缓存复用）。

## 6. 用了 qiaomu-design skill（`craft-loop.md` + `preflight.md`，打磨车道）

读：`SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性优先 /
P-14 不常驻系统自言自语 / P-15 层级融合 / P-17 自发现问题要沉淀）→
`craft-loop.md`、`preflight.md`。**没有**走 Phase 2 的四方向预览（用户已把方向钉死为
「轻便液态玻璃 + 空间感」，自动车道无人点选，按任务书不许停下来等）。

**偷了什么**（逐条落到具体数值）：

1. **`craft-loop.md` 工艺密度菜单第 4 项「签名交互」**（示例：「hover 揭示 / 数字滚动，一个足矣」）
   → 落成：这一行唯一的签名动作是**按下帧长出一块 `SurfacePressed` 面**（浅 `252 → 221`、
   深 `21 → 60`），全轮只加这一个动作，不叠第二个。
2. **第 12 项「材质细节」**（示例：「内边框高光、glass 折射边」）
   → 落成：不新造色号，面直接读 `theme.Tokens.SurfacePressed`（`theme.go:487`
   `= p.controlPress`），与设置页 `cardRow` 的按下臂**同一个 token**——材质细节靠
   **复用同一条算术**表达，而不是加一道新的边。
3. **第 6 项「编辑细节」**（示例：「hairline、大写字距标签」）
   → 落成：**不加**。这一行本来就有键帽的 `LitFace` 一对线（§6.3 3-8），再给行补
   hairline 就是 §3 禁止的「画框分层」，所以这一项判「故意不做」，理由写进 §6.41 的 41-3。
4. **DNA 供体**：58 站库是 web 站，本仓是 Go 自绘桌面应用，供体只能取**本仓自己的**
   设计系统——`settingsui.cardRow`（`rows.go:1405-1416`）的「行按下 = 面、不缩放」。
   偷到的三个具体值：`t.SurfacePressed`（面）、`TextStrong`（抬墨一档）、`无 Scale`（几何）。

**两轮制**：第一轮出图（rest/hover/press 六张）→ 四视角评审 → 第二轮只改了注释里的一处
措辞（把「不画面」改成「静息与悬停不画面」），没有推倒重做。

四视角（各挑一处，如实写）：

- **艺术总监**：那块面从键帽右侧才开始可见，读起来像「标签下的一条高亮」而不是「整行一块板」。
  **不修**：键帽是不透明的 E1 板，要让它也变成面就得给行加 `Padding` 并搬动整条网格
  （会破 `drawerband` / `overlap` 那几条几何 pin），而「面在行盒里、被键帽压住一段」正是
  §2 的层级读法。
- **工程师**：行左端 4px 的 padding 永远被键帽盖住，视觉上不存在——记在这里，不改。
- **甲方**：三秒内能看出「按住这一行有反应」吗？能：浅色 31 级、深色 39 级，一帧之内。
- **读者**：抬墨 + 面同时出现会不会糊？不会：墨画在面之上，实测按下帧的墨级与悬停帧**同值**
  （225 / 216），面只是把地面换了一档。

**preflight 逐条**：

- **A（反 AI 套路）**：通过。无渐变字、无紫蓝光晕、无外发光、无三等分卡片、无 `font-style: italic`
  （P-1；本仓另有 `noitalic_test.go` 守着）；文案是既有 i18n 串，一个词没改（无空词、无感叹号）。
- **B（设计读取一致性）**：通过。读取＝「终端空态，面向第一次打开终端页的人，用轻便液态玻璃的
  语言，倾向本仓 `liquid-glass.md` 的 elevation 阶梯」。记忆点一句话：**按住出路时那块板第一次落下来**。
  明暗主题各自独立（深/浅各跑一遍 pin），动效气质与视觉一致（`theme.ColorStep` 120ms 颜色过渡，
  面是颜色不是几何）。
- **C（动效抽查）**：通过（本仓自绘，无 CSS）。按压有反馈（本轮补的就是它）；无 `transition: all`、
  无 `scale(0)` 入场；面是颜色档，不跑 layout（只动颜色，`Transition{Colors:true}` 是行自己的
  `theme.ColorStep`）；按下**不**加 `Scale`，所以没有「只动 transform」之外的越界；
  reduced-motion 由框架的 `Preferences.ReduceMotion` 统一兜底（本轮的帧在 dump 里就是这么出的）。
- **D2（中文页）**：**部分不适用 + 一条需拍板（沿用）**。本轮改的是英文空态行的**面**，
  没有新增任何中文字；中文串走既有 i18n（`internal/i18n`），中文行高/全角标点/中西文空格都在
  既有 pin 里。**「正文 ≥ 14px」这条仍与 `tokens.Caption`（11px）冲突**——这是
  `.pending-bugs.md` 里挂了七轮的第 4 份证据，需用户拍板，本轮没有擅自改字阶（改字阶＝改高度链）。

## 7. 没做 / 仍待

- 终端**外观抽屉**的三条「整行就是开关的名字」的行（`drawerRow` 的 `Clicked()` 臂）同样是
  零指针反馈，但它们**没有** `Padding`/`Radius`（面画不下），补面要动整条网格的
  `GapX/GapY` 或给行加 padding+负 margin——**会碰 `drawerband`/`drawermetrics` 的几何 pin**，
  写进 `.pending-bugs.md` 的下一格候选，不在本轮顺手做。
