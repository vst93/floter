# P13-bw 规格：捕获输出页页头两枚 28u 方键的**第三档**

## 意图（一句话）

`docs/liquid-glass.md` §6.40 第 **40-3** 行（差距表里唯一一条 `未开始`）：捕获输出页页头的
「钉为独立窗口」「在窗口中打开」两枚 28u 方键，**悬停才长面、按下去纹丝不动**——把指针的
第三档补上，让这一族（`hover=2 / press=0`）在启动器侧也收口。

为什么现在做：P13-bv 收掉了设置页最后一枚同形状的键（`savealert.go` 的 ✕），并把它在
`.pending-bugs.md`「仍待」第 2 条里点名为下一格候选 ①；§6.40 的 40-3 也正是差距表里
唯一 `未开始` 的行。**一个表面一轮**：这一轮只碰这两枚键，不碰页头、不碰条子、不碰正文。

## 现状（动手前实测，非旧报告抄写）

`internal/launcher/output.go` 的 `control` 闭包（`outputBody` 内，`grep -n "func (a \*App) outputBody"`
定位，行号以本轮为准）：

- 悬停：`if button.Hovered() { button = tokens.LitFace(button.Background(tokens.ControlHover)) }`
  —— 只有一条 `if`，**没有 `Pressed()` 分支**。
- 因此按下那一帧与悬停帧**逐像素相同**（本轮实测见报告「先量」一节）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/output.go` `outputBody` 的 `control` 闭包 | `if button.Hovered()` 单臂：悬停面 + 一对线，按下无档 | 面写成变量 `plane`，`switch` 三臂：`Pressed` → `tokens.ControlPress` + `Scale(0.92, 0.92)`；`Hovered` → `tokens.ControlHover`；静息 → `ui.Transparent`。一对线**提到臂外**：`if plane.A > 0 { tokens.LitFace(button) }`（`power.go` / `savealert.go` 的既有形状，`internal/ci/litarm_contract_test.go` 逃生口 ②） |
| 2 | 同处注释 | 只写到「悬停的小键是 E1」 | 补第三档的存在与取舍：按下档**保留**一对线（同 P13-q / P13-bv：小格的面是「一块能被碰的板」，压下去仍在地面之上） |
| 3 | `docs/liquid-glass.md` §6.40 的 40-3 行 | `未开始`（下一轮候选，先量） | `已落地`，并写进实测值 |
| 4 | `.pending-bugs.md` 顶部新增本轮小节 + 「仍待」 | — | 按约定写 |

## 行为契约零改动红线

- **高度链**：条子的 `Padding(edge, 3, 0, 3)`、`Gap(1)`、键盒 `28u`、状态行 `FontSize-1`
  —— 一个数值都不动。`Scale(0.92)` 是**变换**，不参与布局（P13-bv 已证：按下帧的盒子与悬停帧相同）。
- **键盘模型**：`Focusable()` 顺序、`Label()`、`Enter` 行为不动。
- **会话管理 / 设置写回 / i18n 文案 / 命令接线**：不动。`PinText` / `DetachOutput` 的调用点、
  参数、`copy.HistoryPin` / `copy.OutputDetach(Hint)` 一字不改。
- 视觉轮只改墨与面。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # ≥ 3216（P13-bv 基线）
GOOS=linux|windows|darwin go build ./...
```

基线（HEAD `0432fc1a` 实测）：**3216 PASS / 0 FAIL / 120 SKIP / 2374 测试函数**。

## mutation 计划（≥2 条，实跑）

1. **M1**：把 `case button.Pressed():` 臂改成与 `Hovered` 同面（即本轮之前的行为）→
   新 pin 的「按下画了另一个面」判据必须变红。
2. **M2**：删掉臂外的共享一对线 `if plane.A > 0 { tokens.LitFace(button) }` →
   新 pin 的受光边/对侧暗线判据必须变红（`internal/ci` 的
   `TestEveryPointerStateFaceNamesItsPair` 也可能同时报，记在报告里）。
3. **M3（可选）**：把 `Scale(0.92)` 换成 `Scale(1)` → 若 pin 有几何判据则红；本 pin 只断言
   「盒子没被重排」，所以 M3 预期**不变红**，报告里写清为什么（不假装它有用）。

每条变异后 `md5sum -c` 逐字节还原，并 `touch` 复跑一次防缓存。
