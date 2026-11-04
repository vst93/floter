# P13-gb · 浮层那一档的 0.10，量的是「框的可见 alpha」而不是「应用自己画的那一笔填充」

## 意图

`Float` 这一档在 macOS 上比在 Linux / Windows 上矮：`FloatAlpha()` 把 `floatStep`（0.10）
加在 **`TintAlpha`**（应用自己画的那一笔填充）上，而 `TintAlpha` 在 macOS 上是**被
`MaterialVeil` 补偿过的**（`compensate` 把窗口材质的 alpha 从填充里扣掉）。于是同一条
契约（§1「浮 = 框 tint + 0.10」）在 macOS 上换算成**可见**抬升时被乘了 `(1 − veil)`：
材质越厚，浮层离框越近。同一张脸在三个平台上不是同一档——这正是通用队列里
「三平台一致性」那一格，且踩在总目标（空间感：面差要一眼看出）上。

为什么现在做：`ContentAlpha()`（同文件、同一个 spec）**已经**用 `overFrame` 把它的 0.09
表达成「可见 alpha 上的一步」；`FloatAlpha()` 却在填充空间里做加法。两个兄弟方法对同一条
契约有两种读法，而分歧只在**有材质的那个平台**上显现——没有任何 pin 跨平台量过它。

## 先量（实测，`go test ./internal/glassmap/ -run TestTheFloatRung -v` 之前手算 + 之后实测）

浮层可见抬升 = `1-(1-C)(1-F) − C = (1−C)·F`，`C = Spec.Composite()`（框的可见 alpha），
`F = FloatAlpha()`。默认 47% 滑块：

| 平台 / 档 | veil | `TintAlpha` | 旧 `FloatAlpha` | 旧可见抬升 | 新 `FloatAlpha` | 新可见抬升 |
|---|---|---|---|---|---|---|
| linux / windows（任意档） | 0 | 0.470 | 0.570 | **0.302** | 0.570 | **0.302** |
| darwin `frosted` | 0.12 | 0.398 | 0.498 | 0.264 (−13%) | 0.570 | 0.302 |
| darwin `regular` | 0.22 | 0.321 | 0.421 | 0.223 (−26%) | 0.570 | 0.302 |
| darwin `liquid` | 0.34 | 0.197 | 0.297 | 0.210 (−30%) | 0.570 | 0.302 |

滑块 10%（材质把框的可见 alpha 抬到 veil 那一格，契约 §1 已写明）：

| 平台 / 档 | `C` | 旧 `F` | 旧可见抬升 | 新 `F` | 新可见抬升 |
|---|---|---|---|---|---|
| linux `liquid` | 0.10 | 0.20 | 0.180 | 0.20 | 0.180 |
| darwin `regular` | 0.22 | 0.10 | 0.078 | 0.32 | 0.250 |
| darwin `liquid` | 0.34 | 0.10 | **0.066** | 0.44 | 0.290 |

两条量出来的事实：
1. **阶梯方向是反的**：macOS 上材质越厚（`frosted` → `liquid`），浮层离框越近
   （0.264 → 0.210），而「厚材质 = 更立体的玻璃」本该让层级更清楚。
2. **最薄的那一档并不是最矮的**：10% 滑块 + `liquid` 时抬升只剩 **0.066**，
   是 linux 同滑块（0.180）的 **37%**。

## 逐条落地

1. `internal/glassmap/glassmap.go` · `Spec.FloatAlpha`（现状：`min(1, s.TintAlpha+floatStep)`）
   → `min(1, s.Composite()+floatStep)`。**只在 veil > 0 时改变结果**，linux / windows
   逐位不变（`TintAlpha == Composite` 当 `Material == 0`）。注释写清旧值 → 新值。
2. `internal/glassmap/floatplatform_test.go`（新）：三平台 × 四档（含 `off`）× 六个滑块档，
   逐格钉**字面量**填充值 + 两条不变量（浮 > 框可见 alpha；同 `C` 则同 `F`）。
3. `internal/theme/theme_test.go:924` 那条 `darwin.Float.A < linux.Float.A` 是**旧基线的 V**
   → 改成相等，注释写 `P13-gb 重定基线：旧值 darwin 0.42 < linux 0.57 → 新值两者相等`。
   `Sunken` 那一条（:921）**不动**：内容页是「可见结果相同、填充更薄」，浮层是
   「可见抬升相同、填充相同」——同一个原理的两个量，不是两套规则。
4. `internal/theme/floatlift_test.go`（新）：真 `Tokens`，两套外观 × 两档 × 两个滑块档，
   钉「darwin 与 linux 的 `Float.A` 相等」+ 可见抬升的字面量；opt-in dump 出图。
5. `docs/liquid-glass.md` §1 那一行（`float` = 框 tint + 0.10）与 §2.1 补一句：
   `floatStep` 是**可见 alpha** 上的一步，`MaterialVeil` 之后要重新补偿（§6.147）。

## 行为契约零改动红线

只改 `Spec.FloatAlpha` 的算式与墨；高度链、键盘模型、会话管理、设置写回、i18n、命令接线零改动。

## 门槛命令 + PASS 基线（本轮开工实测）

- `gofmt -l cmd internal` 空；`go vet ./...` 净；`GOOS=windows/darwin go vet ./...` 净；
- `go test -count=1 ./...` 全绿；PASS 计数基线 **3453**（P13-ga 收尾实测 3453；本轮收尾实测 3456）；
  测试函数 **2620** → **2624**（`git grep -h "^func Test" HEAD -- internal cmd | wc -l`）；
- 三平台 `go build ./...` 过。

## mutation 计划

- **M1**：`FloatAlpha` 改回 `s.TintAlpha + floatStep`（旧公式）→ `floatplatform_test.go`
  的 darwin 臂（`frosted`/`regular`/`liquid` 三格 + 10% 两格）与 `theme` 的相等 pin 必须变红。
- **M2**：`floatStep` 0.1 → 0.05 → 三平台每一格与可见抬升 pin 全红（含 linux，证明不是
  「只有 macOS 的 pin」）。
