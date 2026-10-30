# P13-z 规格：toast 的 ✕ 戴上 E1 的一对面（并把 P13-y 留下的假证修掉）

## 意图

`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用队列的「状态反馈」那一格。
P13-y 的「仍待」第 1 条点名了本轮对象：`internal/shell/notices.go` toast 里的 ✕
（`extensions.css:383-386` 的 `.app-toast__dismiss`）——**同一条 bar 上的键，同一族平涂**。
它坐在 **E2** 的 toast 面里（§2 判据 1：浮层内的一切都是面），所以它必须是 E1 的一对面，
不是一块 `ControlHover` 平涂。

顺带把 P13-y 的「仍待」第 2 条结清：那条 pin 的「冠线更接近 `LensRim` 而不是
`EdgeHighlight`」那一半，**打到 `theme.Tokens.LitFace` 函数体里不会变红**。原因本轮查明。

## 证据（实测，非旧报告抄写）

- `internal/shell/notices.go:486-489`（当前 HEAD `fec8b8b9`）：hover 臂只有
  `button.Background(tokens.ControlHover)`，`LitFace` 一次都没调用。
- 同文件 `:452` 的动作 chip 已经有 `tokens.LitFace(action)`（P13-j），**同一个 toast 里
  一块面戴线、一块不戴**——不一致就长在同一个父元素下。
- **假证的根因（实测）**：把 `theme.go:1142` 的 `t.LensRim` 换成 `t.EdgeHighlight` 之后，
  三个键的冠线读数是 221 → **233**，而 `barLineOver` 给的两条候选线是
  `LensRim` **223** / `EdgeHighlight` **252**（浅色）。即：键的冠线只拿到 token 的
  **约 1/3** 强度（探针列落在键的圆角里，1px 线被 AA 稀释），而 pin 比的是**绝对距离**：
  `|233−223| = 10` < `|233−252| = 19`，于是「冠线是 E2 的」这一半**永远绿**。
  这是 P13-y 记录的 M4 未红，不是环境问题。

## 落地项

1. `internal/shell/notices.go` 的 `dismissInk` 臂：`button.Background(tokens.ControlHover)`
   → `button = tokens.LitFace(button.Background(tokens.ControlHover))`。
   理由写进注释（E2 里的 E1 面；和 `barButton` / 动作 chip 同一对线）。
2. `internal/shell/pluginwindowbar_test.go` `barAssertE1` 的最后一段：绝对距离 →
   **比例判据**（冠线的抬升 ÷ 每条候选线的抬升，取更接近 1 的那条）。AA 稀释是同一乘数，
   比例把它约掉；同时保留 `barChannelDelta` 的「这两条线分得开」守卫，并新增
   「两条候选线的抬升都必须 > 0」。
3. 新增 pin `TestTheToastDismissKeyIsAnE1Face`（深/浅各一）：悬停 toast 的 ✕，
   读它自己那一列（`barTryRead`），断言冠亮、底暗、且冠线是 `LensRim` 档。
4. 新增 opt-in 截图 dump（`FLOTER_DUMP_P13Z`）：`/tmp/floter-P13-z-toast-<dark|light>.png`。

## 行为契约零改动红线

只改墨：不动 toast 的高度链、关闭语义（`c.CloseToast`）、`Label`（`DismissNotice`）、
命中盒、i18n 文案。`internal/shell` 之外的包零改动。

## 门槛与基线（本轮实测，HEAD `fec8b8b9`）

`gofmt` 空 | `go vet ./...` 净 | `GOOS=windows/darwin go vet ./...` 净 |
`go test -count=1 ./...` 全绿 | PASS **3003** / FAIL 0 / SKIP 82 / 函数 **2216** |
三平台 `go build` 过。

## mutation 计划

- **M1**：`notices.go` 撤掉 `LitFace`，回到平涂 → 新 pin 的「底暗」与比例判据变红。
- **M2**：`theme.go` 的 `LitFace` 冠线 `LensRim` → `EdgeHighlight`（E2 档）→ **必须**变红
  （这正是 P13-y 那条没红的变异，本轮是它的验收）。
- **M3**：`notices.go` 只在 hover 臂加 `LitFace` 但不给面（不 `Background`）→ 比例判据/面差红。
- **M4**：`theme.go` 的 `LitFace` 对侧 `LensEdge` → `LensRim`（上下都亮）→ 「底暗」红。
