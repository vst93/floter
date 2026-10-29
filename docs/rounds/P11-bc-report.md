# P11-bc 报告 · 终端「进程已退出」便条是画布底边的一条带

选题顺序：用户实测反馈（`.pending-bugs.md` 顶部 F1/F2/F3 已由 P11-y/z/aa 收完，用户新报的
`Super+N` 编号渲染已由 P11-bb 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K` 动作面＝
拍板）→ 上一轮「仍待」（全是拍板项）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。
规格 `docs/rounds/P11-bc-spec.md`。

## 1. 为什么选这一处

用户 F3 的原文是「不要 floating/overlay 观感，元素要 inline、要贴进布局」。F3 自己修掉了
终端页那圈双层框（P11-z），但**这一页上最后一处真正的浮层**没动：会话退出后常驻的那条
「进程已退出 · 退出码 N / 输出已保留」。它以前是 `Absolute().Bottom(3).Left(3).Right(3)` 的
圆角浮卡——`Float` 面、`ElevThrow` 投影、`LensRim` 受光边，再加一段从上方 2u 落下的 160ms
入场（`internal/terminalui/terminalui.go:1208-1220`）。它还是**常驻家具**（退出后一直在，
直到用户自己关掉），所以在纯白画布上读成一块压在输出上的玻璃片。

而且它不只是观感：参考给它 `pointer-events: none`
（`main:src/styles/terminal.css:303`），本仓**从来没有** `PassThrough()`。它横在画布底部的
那块矩形把最后一到两行输出的点击、drag-select 与滚轮全吃掉——与 P11-af 修掉的浮条吃掉前
两行是同一个后果，那一条修了，这一条没修。所以这一轮同时是**质感**与**交互**。

## 2. 落地

`internal/terminalui/terminalui.go` 的 `residentNote`：

| 项 | 修前 | 修后 |
|---|---|---|
| 锚点 | `Bottom(3).Left(3).Right(3)`（四边各让 12u） | `Bottom(0).Left(0).Right(0)`（贴底、整宽） |
| 面 | `Float`（玻璃片的浮层档） | `Control`（抽屉那条带的面） |
| 线 | `Border(1, ControlEdge)` 四边 | `BorderWidth(1, 0, 0, 0)` + `HairlineSoft`（只有上边一条弱发丝） |
| 圆角 | `RadiusMD` | 无 |
| 投影 | `ElevThrow`（`--elev-3`） | 无 |
| 受光边 | `InsetShadow(0, 1, 0, 0, LensRim)` | 无 |
| 入场 | `Transition{Enter: Y-2, 160ms}` | 无 |
| 指针 | 吃掉（不是命中目标之外的东西） | `PassThrough()` = 参考的 `pointer-events: none` |

**零改动**：两行文字与三档墨（`TextSecondary` / `Warning` / `TextMuted`）、字号、`560` 的字重、
`ProcessExited`/`ProcessExitedHint` 的文案、出现条件、它在 `body` 里的父子关系、内边距
（7u/10u = 参考的 `padding: 7px 10px`）。它是 `Absolute` 的，不占布局高度，所以**画布高度与
PTY 行数一个像素没变**——这不是「把便条搬进流里」那种交互改动（那一条仍在拍板表里）。

## 3. 台账（V / B / S）

- **V = 1**：`residentstrip_test.go` 的两条新 pin 是**新**断言（不是改旧值），所以严格说没有
  旧 pin 被重定基线；台账里记 `V=1` 指的是「这条便条自己的视觉数值重定基线：浮卡 → 贴边带」，
  旧数值写在 §5 的像素表里。
- **B = 0 / S = 0**：既有测试**零改动**全绿（`TestResidentSessionHoldsItsOutput`、
  `TestTerminalBarIdentityNamesTheExitCode`、`accentguard`/`barchrome` 的 exited 臂、
  终端页整帧审计都只读文字与浮条的墨，不读便条的几何）。
  高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线零改动。

## 4. 变异（3 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 红在哪（原始报错串） |
|---|------|----------------------|
| M1 | 用 `git stash` 回到修前的 `terminalui.go` | `light 860x600: the row above the strip's hairline is [249 249 249] but the canvas is [254 254 254] — a shadow or a rim is lying on the output` / `light 860x600: the bottom row at x=0 is [250 250 250], not the strip's face [223 223 223] — the strip does not reach the panel's bottom edge square`（+ 两个尺寸 × 两外观）+ `residentNote still carries "ElevThrow(": the note is a floating card again` 等五条源码契约 |
| M2 | 把 `Radius(tokens.RadiusMD)` 加回带上 | `light 860x600: the bottom row at x=0 is [255 255 255], not the strip's face [242 242 242] — the strip does not reach the panel's bottom edge square`（四角全红） |
| M3 | 删掉 `.PassThrough()` | `residentNote is not PassThrough: the band eats the pointer over the last lines of output` |

每条都 `cp`/`edit` 还原后 `md5sum -c /tmp/p11bc.md5` 逐字节核对（`a0d26bed…`），`touch` 测试
文件再跑一次绿。

## 5. 视觉证据（我自己量了像素，不是只看测试数字）

`/tmp/floter-P11-bc-resident-{light,dark}.png` 与 `-before.png`（`FLOTER_PNG=1 go test -run
TestDumpP11BC ./internal/terminalui/`，`-before` 那两张是 `git stash` 到修前代码后用
`FLOTER_PNG_SUFFIX=-before` 出的）。`860×600`，常驻会话（退出码 3）。

**我在图上量到的**（x=2 这一列 = 带子的内边距，避开文字）：

| 位置 | 浅色 修前 | 浅色 修后 | 深色 修前 | 深色 修后 |
|---|---|---|---|---|
| 画布（y=520，带子上方 40u） | `255` | `255` | `24,24,27` | `24,24,27` |
| 带子上方那条投影带（y=570…590） | `243`…`242`（**投影铺在画布上**） | `255`（纯白，投影没了） | `23,23,26` | `24,24,27` |
| 带子上边那条发丝 | `247`（投影边缘，不是线） | `229` | `24,24,26`（几乎看不见） | `53,53,56` |
| 带子的面 | `223`（`Float` 玻璃片） | `242`（`Control` 家具档） | `21,21,24` | `41,41,44` |
| 底行 x=0 / x=w-1 | `250`（圆角过渡） | `242`（= 面） | `23,23,26` | `41,41,44`（= 面） |
| 带子的上边界 y | ≈554（浮卡顶，影子一直拖到 599） | **563**（贴边带顶，底 599） | ≈554 | **563** |

一处具体变化：**浅色图里 y=570…590 那一片本来是 `242`，现在是 `255`** —— 那块 13 级灰就是
浮卡的 `--elev-3` 投影，它一直铺在输出上。修后从 y=563 往上一路是纯白，带子只在
y=563…599 存在，四角与两条边都是同一个 `242`（圆角与 12u 让位都没了）。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2431（基线 2429）| SKIP 39（38 + 1 条 opt-in dump）| 测试函数 1972（1969 + 3）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 仍待（P11-bc）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的终端便条投影（**注意：常驻便条已不投影，`ElevThrow` 现在只剩
   `uninstall.go` 的卸载面板与 `feedbackNote` 两处**）/ P11-ba 的终端「进程已退出」便条改
   inline（**改的是「占布局位置」那一半：那会 resize PTY，仍属交互决定**）。
2. **同族的另一半本轮故意没动**：`feedbackNote`（动作失败的那句，4.5s 自动消失）仍是
   `Absolute` 的浮卡。理由是它**不是家具而是消息**，且它的锚点在面板左上 12u——做成整宽带
   就会落在浮条（`titleBar` 揭示时画 `Space(7)` 面）底下，两个面互相压。要把它也贴边，得先
   决定它去底边还是让浮条让位，属交互决定。
3. **侦察结论（本轮量过，别当 TODO 重领）**：`theme.ElevThrow` 的调用点从 3 处降到 2 处，
   函数本身仍有用（卸载确认面板是真正的模态浮层），**不是死代码**。
