# P13-gc · 内容页那一档的 0.09，也是「框的可见 alpha」上的一步

## 意图

`Spec.ContentAlpha()`（`internal/glassmap/glassmap.go:392`）把 `contentStep`（0.09）加在
**`TintAlpha`**（应用自己画的那笔填充）上再反解 `overFrame`。`TintAlpha` 在 macOS 上是
被 `MaterialVeil` 补偿过的，于是「内容页比它压着的框深 0.09」换算成**可见** recess 时被乘了
`(1 − veil)`：`frosted` 0.079 / `regular` 0.070 / `liquid` **0.059**。

这是 P13-gb 修掉的浮层 0.10 的**孪生账**（同一个文件、同一个 `overFrame`、同一条 §1 契约、
同一个平台分歧），只是它慢了一轮：本轮把它量完。为什么现在做：P13-gb 的「仍待」第 2 条
（`docs/rounds/P13-gb-report.md`），且它踩在总目标第二条（空间感：面差要一眼看出）与
「三平台一致性」那一格上。

## 先量（实测）

recess 的**可见**一步 = `Composite(面) − Composite(框)` = `contentStep · (1 − veil)`：

| 平台 / 档（47% 滑块） | veil | 旧可见 recess | 新可见 recess |
|---|---|---|---|
| linux / windows | 0 | 0.090 | 0.090 |
| darwin `frosted` | 0.12 | 0.079 (−12%) | 0.090 |
| darwin `regular` | 0.22 | 0.070 (−22%) | 0.090 |
| darwin `liquid` | 0.34 | **0.059 (−34%)** | 0.090 |

换算成 1x 屏上的**级数**（§1 判据：面与它压着的面差 ≥ 8 级；探针地面取中灰 `RGB(128,128,128)`，
与 P13-ga §6.146 同一块底色。级数 = `recess × |面自有的颜色 − 地面|`）：

| 外观 | 地面 | `liquid` 旧 | 新 | `regular` 旧 | `frosted` 旧 |
|---|---|---|---|---|---|
| 浅（面 `243,244,247`） | 128 | **6.8** | 10.4 | 8.1 | 9.1 |
| 深（面 `17,18,20`） | 128 | **6.6** | 10.0 | 7.8 | 8.8 |

即：**`liquid` 厚档在 macOS 上，内容页那一档在 1x 屏上读不出**（6.8 / 6.6 < 8），
而 `regular` 是 8.1 / 7.8——正好骑在门槛上。同一格的 `Control` 洗色不受影响
（它是应用自己画在面之上的一笔，见 §2.1 第 1 条），本轮顺带 pin 住它。

## 逐条落地

1. `internal/glassmap/glassmap.go` · `Spec.ContentAlpha()`：
   现状 `overFrame(s.TintAlpha, min(1, s.TintAlpha+contentStep))`
   → `overFrame(s.Composite(), min(1, s.Composite()+contentStep))`。注释写清旧值 → 新值，
   以及「`TerminalFrameAlpha` 不动」的理由（它的目标是**绝对** alpha `TintAlpha`，在填充空间里
   本来就对；`ContentAlpha` 的目标是一**步**，必须落在可见 alpha 上）。
2. `internal/glassmap/contentplatform_test.go`（新）：三平台 × 四档 × 六个滑块档的
   **字面量** `ContentAlpha` + 两条不变量（可见 recess 恒为 0.09；同 `Composite` 则同填填充）
   + 反面臂（`off` 停板与本轮无关，逐位不变）。
3. `internal/theme/sunkenveil_test.go`（新，`ui.NewTester` 量像素）：
   两套外观 × 四档 × 三平台 × 三个滑块档，钉 ① 面 vs 框 ≥ 8 级；② `Control` vs 面 ≥ 8 级；
   ③ 设置体的不透明合成（`FaceOver(地, Sunken)`，`settingsui.go:1303`）在 darwin 上与改前
   同字节；④ 浮层仍严格高于面（§2 判据 1）。外加 opt-in dump 出图。
4. `internal/theme/theme_test.go:921` 那条 `darwin.Sunken.A < linux.Sunken.A` 是**旧基线的 V**：
   面档改成可见 alpha 上的一步之后，两个平台的填充**相等**（同 §2.2 里浮层那一条的推理）。
   → 改成相等，注释写 `P13-gc 重定基线：旧值 darwin 38 < linux 43 → 新值两者相等`。
   同时把 P13-gb 在浮层 pin 注释里那句「`Sunken` 那一条不动」改成指向本轮。
5. `docs/liquid-glass.md`：新增 **§2.3**（内容页那一档的 0.09）与 **§6.148**（本轮实测表）。

## 行为契约零改动红线

只改 `Spec.ContentAlpha` 的算式与墨。高度链、键盘模型、会话管理、设置写回、i18n、
命令接线零改动；linux / windows 的渲染逐字节不变（`Material == 0` 时 `TintAlpha == Composite`）。

## 门槛命令 + PASS 基线（本轮开工实测）

- `gofmt -l cmd internal` 空；`go vet ./...` / `GOOS=windows|darwin go vet ./...` 净；
- `go test -count=1 ./...` 全绿；PASS 基线 **3456**（P13-gb 收尾实测），测试函数 **2624**；
- `GOOS=linux|windows|darwin go build ./...` 三条都过。

## mutation 计划

- **M1**：`ContentAlpha` 退回 `overFrame(s.TintAlpha, …)` → `contentplatform_test.go` 的
  darwin 三档（frosted/regular/liquid）与 `sunkenveil_test.go` 的 darwin `liquid` 格必须变红。
- **M2**：`contentStep` 0.09 → 0.06（参考实现的旧 band）→ 三平台每一格都红（含 linux，
  证明这套 pin 不是「只有 macOS 会红」）。
- **M3**（像素层）：把 `tokens.Sunken` 换回不补偿的 `TintAlpha`（即让面等于框）→
  8 级判据在每一格红。
