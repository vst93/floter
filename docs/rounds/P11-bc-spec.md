# P11-bc 规格 · 终端「进程已退出」便条是画布底边的一条带，不是压在输出上的玻璃片

## 意图（一句话）

把终端页那条**常驻**的「进程已退出 · 退出码 N / 输出已保留」从 `Absolute` 圆角浮卡
（`Float` 面 + `ElevThrow` 投影 + `LensRim` 高光 + 160ms 入场动画）改成**贴画布底边的一条带**
（`Control` 面 + 上边一条 `HairlineSoft` 发丝，无圆角、无投影、无高光、无动画、整宽），
并补上参考的 `pointer-events: none`。

## 为什么现在做

- 选题顺序：用户实测反馈（`.pending-bugs.md` 顶部 F1/F2/F3 已由 P11-y/z/aa 收完，用户新报的
  `Super+N` 编号渲染已由 P11-bb 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K` 动作面
  ＝拍板）→ 上一轮「仍待」（全是拍板项）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。
- 用户 F3 原话：「终端视图外围一圈类玻璃/毛玻璃质感的边框，与整体想要的**轻盈**气质不符」，
  要求「**不要 floating/overlay 观感，元素要 inline、要贴进布局**」。F3 修的是面板那圈双层框
  （P11-z）；**终端页上最后一处真正的浮层**就是这条便条。
- 它不只是「不好看」，还是**交互缺陷**：`main:src/styles/terminal.css:303` 给它
  `pointer-events: none`，本仓从来没有 `PassThrough()`——于是它横在画布底部的那块矩形把最后
  一到两行输出的**点击、drag-select 与滚轮全吃掉**（与 P11-af 浮条吃掉前两行是同一个后果，
  那一条已修，这一条没修）。

## 现状（行号本轮实测，`internal/terminalui/terminalui.go`）

| 位置 | 现状 |
|---|---|
| `:1208` | `ui.Row(c).Absolute().Bottom(t.Space(3)).Left(t.Space(3)).Right(t.Space(3))` —— 四边各让 12u 的浮卡 |
| `:1209` | `Gap(2).AlignItems(Center).Padding(1.75, 2.5)` —— 7u/10u（参考的 `padding: 7px 10px`） |
| `:1210-1212` | `.Radius(tokens.RadiusMD).Border(1, tokens.ControlEdge).Background(tokens.Float)` |
| `:1213-1216` | `Transition{Enter: &Motion{Y: -2}, Duration: 160ms}` —— 从上方 2u 落下的入场 |
| `:1219` | `theme.ElevThrow(note, t, …)` —— `--elev-3` 投影 |
| `:1220` | `note.InsetShadow(0, 1, 0, 0, tokens.LensRim)` —— 顶部受光边 |
| 全函数 | **没有** `PassThrough()` |

## 逐条落地

1. **锚点**：`Absolute().Bottom(0).Left(0).Right(0)` —— 贴画布底边、整宽。
2. **面**：`Background(tokens.Control)`（抽屉自己那条带的面）。
3. **线**：`BorderWidth(1, 0, 0, 0).BorderColor(tokens.HairlineSoft)` —— 只有上边一条弱发丝，
   与抽屉（P11-av）同一套说法；圆角 0（`Radius` 整句删掉）。
4. **减法**：删 `ElevThrow`、`InsetShadow`、`Transition`、`Border`。
5. **内边距**：`Padding(t.Space(1.75), t.Space(2.5))` 保持不变（7u/10u，参考
   `.terminal-resident` 的 `padding: 7px 10px`）。
6. **交互**：加 `.PassThrough()`，对齐参考的 `pointer-events: none`。
7. **零改动**：两行文字、字号、`TextSecondary`/`Warning`/`TextMuted` 三档墨、`ProcessExited`
   与 `ProcessExitedHint` 的文案、出现条件（`session.Resident && session.Term != nil`）、
   它在 `body` 里的父子关系、画布高度、PTY 行数、抽屉几何、`DrawerScroll` 全不动。

### 行为契约零改动红线（逐条声明）

- **高度链**：不碰。便条仍是 `body` 里的 `Absolute` 孩子，不占布局高度 → 画布高度不变 →
  PTY 不被 resize。
- **键盘模型**：不碰（便条没有可聚焦元素；`PassThrough` 只影响指针命中）。
- **会话管理**：不碰（`Resident`/`ExitCode` 的读写全在别处）。
- **设置写回**：不碰。
- **i18n 文案**：不碰。
- **命令接线**：不碰。

## 门槛命令 + PASS 基线（本轮实测，不是旧报告的数字）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2429（P11-bb 实测）
GOOS=linux/windows/darwin go build ./...                # 过
```

## 新 pin（`internal/terminalui/residentstrip_test.go`）

1. `TestTheResidentNoteIsADockedStrip`（像素）：浅/深两外观，`860×600` 有常驻会话。
   - 底行（`y=h-1`）在 `x=2` 与 `x=w-3` 上是**同一个**颜色 = `Control` 合成在画布底色上；
     浮卡的 12u 让位会让这两点落在画布底色上（修前红）。
   - `x=0` 与 `x=w/2` 的底行同色 → 没有圆角。
   - 从底往上找那条发丝：它上面一行**逐字节等于**画布底色（修前的 `ElevThrow` 会在那里留
     一圈比画布更暗的投影）。
   - 发丝那一行比它下面一行暗（线在）。
2. `TestTheResidentNoteLetsThePointerThrough`（源码契约 + 行为）：
   - `funcBody(t, "residentNote")` 含 `.PassThrough()`；不含 `ElevThrow(`、`InsetShadow(`、
     `Radius(`、`Transition(`。
   - 行为：指针停在便条带内、画布上，`ClickAt` 后终端视图仍拿到焦点（修前被便条吃掉）。

## mutation 计划（≥2 条，每条说清让哪个 pin 变红）

- **M1**：`Bottom(0).Left(0).Right(0)` → `Bottom(t.Space(3)).Left(t.Space(3)).Right(t.Space(3))`
  （回到浮卡的锚点）→ `TestTheResidentNoteIsADockedStrip` 的「底行两点同色」红。
- **M2**：把 `theme.ElevThrow(note, …)` 加回去 → 同一条 pin 的「发丝上面一行逐字节等于画布底色」红。
- **M3**：删掉 `.PassThrough()` → `TestTheResidentNoteLetsThePointerThrough` 的源码契约红。

## 视觉证据

`/tmp/floter-P11-bc-resident-{light,dark}.png` + 同名 `-before.png`（用 `FLOTER_PNG=1` 跑 dump）。
报告里写「我在这张图上量到了什么」的像素值，不写形容词。
