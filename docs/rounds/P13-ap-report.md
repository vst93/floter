# P13-ap 报告：空态的出口按它的种类取间距

方向：**细节打磨 / 空态满态**（`docs/liquid-glass.md` §6 无 `未开始` 行 → 通用队列）。
规格：`docs/rounds/P13-ap-spec.md`。合约条目：§4.3 第四行 + 新增 §6.13。

## 一、这轮为什么让它更有空间感

空态是「一句话 + 一个出路」，而出口的重量来自它**不属于**那句话。`emptyState` 对两种完全
不同的出口给了同一个 2u：会话页那个是一枚 30u 高的 `Control` 板（有 `ControlEdge` 一对、
受光边、按下会缩 0.97），自定义快捷键那个是一个 `+` 加一行强调色文字。实测（理由那一行的
**墨底** → 板的**顶边**）**14px**，而那句理由离它上面的句子本来就有 **12px**：相差 2px 的
两对，读起来是同一件事——三行字加一枚板 = 四行字。改成板 **23px**（深）/ **22px**（浅），
文字那一档留在 **10px**，于是这个区域的三对第一次成阶梯：**8 / 12 / 23**（同一块 / 同一段的
下一行 / 一个新的东西）。中间那一档也量过：8u 只有 **19 / 18px**，仍在行距里。

## 二、落地

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/settingsui/empty.go` | `emptyState` 加 `plate bool` 参数；动作臂 `lead := t.Space(0.5)`，`plate` 为真时 `t.Space(3)`（12u）→ 盒子 8u+12u=20u；注释写清 P13-ap 的旧值→新值 |
| 2 | `internal/settingsui/settingsui.go:1527` | 会话页空态传 `true`（出口是 `pageButton` 板） |
| 3 | `internal/settingsui/custom.go:50` | 自定义快捷键空态传 `false`（出口是 `actionWithGlyph` 文字） |
| 4 | `settingsui.go:1737` / `empty_test.go:33` / `integrationsempty_test.go:91` / `customempty_test.go:133` | 无动作的调用点补 `false`（盒子零变化） |
| 5 | `internal/settingsui/emptyaction_test.go`（新） | 5 条账 × 深浅两套 + opt-in 出图 |
| 6 | `docs/liquid-glass.md` §4.3 + §6.13 | 第四行填上数字；新增 16-1…16-8 |

**没有动的东西**：`MinHeight(floor)`、`Padding(6u)`、`Center()`、`head` 的 4u、理由的 8u、
`pageButton` / `actionWithGlyph` 自己的面与墨、i18n 文案、键盘模型、会话管理、设置写回。

## 三、墨上实测（`emptyaction_test.go` 的同一把尺）

720×720，`emptyLeft/Right/Top` = 188/680/120。

| 量 | 旧 | 新 |
|---|---|---|
| 标记 → 句子 | 8px | 8px（不动） |
| 句子 → 理由 | 12px | 12px（不动） |
| 理由墨底 → 板顶边（深 / 浅） | **14 / 13px** | **23 / 22px**（8u 只有 19 / 18px，被否） |
| 标签盒子 → 理由盒子 | 15.4px | 24.4px |
| 文字出口 → 理由盒子（另一档） | 11px | 11px（不动） |

## 四、出图与整帧 diff

`FLOTER_P13AP_DUMP=1 go test -run TestDumpEmptyActionInk ./internal/settingsui/` 写出
`/tmp/floter-P13-ap-sessions-empty-{dark,light}.png`；改前/改后各一份
（`/tmp/floter-P13-ap-empty-{dark,light}-{before,after}.png`）。PIL 逐像素比：

```
dark  size (720, 720) diff px 2660 bbox (399, 225, 492, 263)
light size (720, 720) diff px 2680 bbox (399, 225, 492, 263)
```

**这张图上看到的一处变化**：diff 的 bbox 正好是那枚 93×38 的板——标记、句子、理由是**零位移**
（区域向下长，不是重新居中），整帧只有板连同它自己的受光边、键线和字整体下移 9px；深色帧上
板的上方因此多出一条干净的 20px 空带，板的顶边与那句理由不再是一条「行距」。

## 五、V / B / S 台账

- **V 0**：没有钉旧数值的断言失败——`empty_test.go` 的 160u 地板账走的是无动作调用
  （`nil`），盒子零变化。
- **B 0**：`go test -count=1 ./...` 全绿，行为/接线测试零改动。
- **S 0**：没有结构变更；`emptyState` 只是多了一个 `bool`。
- 签名变更波及 5 个调用点（2 生产 + 3 测试），都是**编译期**强制的地方，没有一处靠宽容差。

## 六、门槛（实测）

```
gofmt -l cmd internal                                # 空
go vet ./...                                         # 净
GOOS=windows go vet ./...                            # 净
GOOS=darwin  go vet ./...                            # 净
go test -count=1 ./...                               # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 3054 → 3056
go test -count=1 -v ./... | grep -c -- "--- SKIP"    # 96 → 97（+1 opt-in 出图）
测试函数（同一把尺：只数 cmd/ + internal/）          # 2260 → 2263
GOOS=linux|windows|darwin go build ./...             # 三个都过
```

## 七、变异（4 条实跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 板那一档 `t.Space(3)` → `t.Space(0.5)` | **红**：`dark: the way out sits 14px…` / `light: 13px…` |
| M2 | 默认 lead 也改成 `t.Space(3)`（两档合一） | **红**：`the text way out sits 21px under the why, want the reference's 2u (≈10px)` |
| M3 | 板那一档 → `t.Space(10)`（40u） | **红**：`dark: the way out drifted 48px off its paragraph` |
| M4 | 板那一档 → `0` | **红**：`dark: the way out sits 12px…` / `light: 11px…` |
| M5 | 板那一档 → `t.Space(2)`（8u，被否的那一档） | **红**：`dark: the way out sits 19px…` / `light: 18px…`、`label box sits 21px…` |

四次还原后 `md5sum -c` 核对 `internal/settingsui/empty.go` =
`7334b89fca4d74d58ec90183e1be85ec`，`touch` 后再跑一次两条 pin 全绿。

## 八、偷了什么（qiaomu-design）

本轮读的是 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md`
（「已有方向、只要打磨」那条路径，不走 Phase 2 四方向预览）。

- **从 `craft-loop.md` 工艺密度菜单取了第 5 项「多层/成对的间距不是均匀的」邻近条目**——
  准确说取的是菜单第 6 项「编辑细节」里的节奏纪律与 `craft-loop` 第一节的「偷具体数值」：
  参考的 `margin-top: 2px` 是**给一行字**的具体值，本轮把它按**种类**分档，板那一档落成
  `Margin(3u)` = 12u（盒子 20u、墨上 20/18px），文字那一档原样 2u。**能写出来的具体数值**：
  8u / 12u / 20u 三档，墨上 8 / 12 / 23px（8u 实测 19 / 18px，被否）。
- **从 `craft-loop` 第三节「两轮制」取的动作**：第一轮先出图量（14px 对 12px），
  四视角里「工程师」指出「板上只有 2px 的空气 = 第四行」，「艺术总监」指出「出口读不出重量」，
  第二轮只修这一处（不顺手改标记/理由），再出图确认 diff bbox 只覆盖那枚板。
- **从 `preflight.md` 的 A4.1 取的一条**：「disabled / read-only / hidden 语义没有混淆」——
  这里对应的是**出口的种类**（板 vs 字）不能混成同一种语法，本轮把它显式化成 `plate bool`。
- **从 `preflight.md` D2 的一条**（中文行高/字号）：本轮的 11px 理由行**没有动**；
  它与 D2「正文 ≥ 14px」的张力是 P13-an/ak 起就记着的「需拍板」，见 §九。

## 九、preflight 逐条结论

- **A（反 AI 套路）**：过。无渐变、无发光、无斜体、无左侧竖线；本轮的改动只有一对间距，
  没有引入任何装饰。**A5（四方向预览）不适用**——任务书禁止在自动驾驶道上出 HTML 预览页。
- **B（设计读取一致性）**：过。读取 = 「功能型桌面设置页的空态，轻便液态玻璃，三档墨 +
  成对间距」；产出只动了间距，与 §4.3 / §6.13 写下的规则一致，深浅两套同值。
- **C（动效抽查）**：过（本轮零动效变更）。板原有的按下面（`Scale(0.97)` + `ControlPress`
  + 120ms `theme.ColorStep`）没有动；无 `transition: all`、无 `ease-in`。
- **D2（中文页）**：**部分不过，如实记录**：理由行仍是 `tokens.Caption`（11px），
  低于 D2 的「正文 ≥ 14px」。这不是本轮引入的，属 P13-ak/an/ao 都记过的「需拍板」；
  本轮没有把任何新文字放进这一档（只动了它下面的空白）。
- **E（三秒测试）**：过。眯眼看会话页空态：先看到标记+句子，再是理由，
  然后**一眼能看出**下面那枚板是可以按的（20px 的空带把它分出来了）。

## 十、仍待（P13-ap）

1. **需拍板（沿用 P13-an 仍待 1/2）**：`cardRow` 控制槽 `Shrink(0)`；`tokens.Caption` 11px
   与 preflight D2「正文 ≥ 14px」的张力。
2. **需拍板（终端弧）**：S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾（客户端不告而别）。
3. 下一轮候选：① 空态的**满态**那一半（`sessions` 有多条会话时，列表与页脚的关系；
   或剪贴板页满屏时的最后一行）；② `ContentAlpha()` 沿透明度滑块两端（任务书「先量再改的两个数」
   之一，**仍未量**）。
4. 抢救 stash 复核：`stash@{0}`（`afc38b247c`）的 `metaSeparator` 变体**已被 P13-af 的 `metaFact`
   取代**（HEAD 里 `metaSeparator` 出现 0 次，P13-af 提交信息写明已接上补完）；
   `stash@{1}`（`114b7d1585`）只改 `.pending-bugs.md` 的 P13-ad 段，文字已在文件里。
   两条都没有静默 drop，也没有再试合。
