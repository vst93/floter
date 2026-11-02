# P13-dh · 抽屉的顶缝不是丢了，是那把尺子在 50.8 上截了尾

**意图**：让「画布/抽屉交界那一像素受光边」这条 pin 真的量在**生产尺子**上
（`Spacing 3.6` / `FontSize 10.80`），并把它的取行方式改成和 painter 同一个取整。

**为什么现在做**：`docs/liquid-glass.md` §6.76 把 `P13-dh` 记成「设计间距下顶缝丢失，
y=strip.Y 是画布白、y+1 才是抽屉面，发丝线跑到窗口底边」。本轮开工先只量不改，
**结论是产品没坏、pin 量错了行**：

- 抽屉自己就是 `ui.Scroll` 带 `BorderWidth(1,0,0,0)` + `BorderColor(tokens.LensRim)`
  （`internal/terminalui/drawer.go:112-115`），它的盒子就是 pin 里那个 `strip`。
- 设计尺下这个盒子的顶是**小数**：实测 640×360 → `Y=50.80002`、720×360 → `109.20001`、
  900×600 → `384.40000`、1200×800 → `616.80000`。
- painter 画盒子前先 `snap` 到整数设备像素，用的是 `math.Round`
  （`/home/tar/go/pkg/mod/github.com/egoist/mygo@v0.7.3/ui/paint.go:163-170`）。
- pin 用的是 `int(strip.Y)`，**截断**。50.8 → 50（画布那一行），616.8 → 616（画布那一行），
  而发丝实际画在 51 / 617。于是 pin 读到「seam = 画布白、face = 受光边」，
  接着 `above := inkAt(top-1)` 又是同一个画布白 → 报出「发丝画到画布上了」。

实测四窗口 × 两外观，**每一帧的缝都在它该在的地方**（浅：缝 `[236 236 236]`、
面 `[230 230 230]`；深：缝 `[58 58 59]`、面 `[39 39 40]`；窗口底行 = 面）：

| 窗口 | `strip.Y` | 发丝行 | `int()` 截断 | 判 |
|---|---|---|---|---|
| 640×360 | 50.80002 | 51 | 50 = 画布 → 假红 | 截断错 |
| 720×360 | 109.20001 | 109 | 109 = 缝 | 碰巧对 |
| 900×600 | 384.40000 | 384 | 384 = 缝 | 碰巧对 |
| 1200×800 | 616.80000 | 617 | 616 = 画布 → 假红 | 截断错 |

## 落地项

1. **`internal/terminalui/view_test.go`**（`render` 在 `:22`）
   现状：`render` 不装主题，量的是库存尺（`4.0/14.00`）。
   改成：新增 `renderThemed`（同一个 `App.View`，但 `c.SetTheme(tokensOf(a, c).Theme)` +
   `ReduceMotion`），本轮只给缝这两条 pin 用；其余 pin 的库存尺由 `P13-dg-t` 整体收口，
   本轮的 `render` 一个字节不动。

2. **`internal/terminalui/drawerseam_test.go`**（`:40` 起的 `TestTheDrawerSeamIsOnItsTopEdge`，
   取行在 `:69`）
   现状：`render(...)` + `top := int(strip.Y)`。
   改成：`renderThemed(...)` + `top := drawerSeamRow(strip)`；新增
   `drawerSeamRow` 记录「painter 用 `math.Round`、不是截断」并附上面那张实测表。

3. **`internal/terminalui/litedge_test.go`**（`:131-132`、`:164-166` 的 `TestTheDockedDrawerWearsTheE1LitEdge`）
   现状：同一条缝，同样 `int(strip.Y)`；900×600 上 `384.4` 碰巧落在正确行，是**潜伏的同一条错**。
   改成：`drawerSeamRow(strip)`，一个函数两处共用。

4. **`internal/terminalui/drawerseam_test.go`** 的 `TestDumpP11AVDrawerSeamPNGs`（`:117`）
   现状：dump 用库存尺。
   改成：`renderThemed`，让证据图第一次是**用户看到的那把尺子**。

**不改**：`P13-dg-t`（终端页整包收口）的红一条不动 —— 本轮只做缝。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n / 命令接线：**零改动**。
本轮只动 `internal/terminalui/*_test.go` 与两份 `docs/`，`git diff --stat` 里没有生产文件。

## 门槛 + PASS 基线（实测）

- `gofmt -l cmd internal` 空；`go vet ./...` / `GOOS=windows` / `GOOS=darwin` 净
- `go test -count=1 ./...` 全绿
- 基线（P13-dg 实测）：**PASS 3311 / FAIL 0 / SKIP 148 / 2467 测试函数**

## 变异计划（≥2，逐条指明谁会红）

- **M1**：把 `drawerSeamRow` 改回 `int(r.Y)` → `TestTheDrawerSeamIsOnItsTopEdge`
  在 640×360 与 1200×800 × 深浅上必须红（「the row above the strip is the seam colour」）。
- **M2**：`drawer.go:114` 的 `BorderWidth(1, 0, 0, 0)` 改回 `BorderWidth(0, 0, 1, 0)`
  （P11-av 修掉的那条回退）→ 缝那一条必须红（「the strip's first row is its own face」）。
- **M3**：`BorderColor(tokens.LensRim)` → `tokens.HairlineSoft` →
  `TestTheDockedDrawerWearsTheE1LitEdge` 必须红（「nearer the old weak hairline」）。
