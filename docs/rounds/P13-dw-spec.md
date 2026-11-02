# P13-dw 规格：同一张卡的两张列表，两半此前各跑一把尺子（混排收口）

## 意图

`P13-dv`「仍待」第 1 条 = `docs/liquid-glass.md` §6.86 表里的「**混排**（按 B 先跑），2 条」：
`TestTheTwoListsShareOneTextColumn` 与 `TestOutputGroupHeadingSharesTheLauncherSectionRegister`
各有一半节点跑在框架**库存主题**（`Spacing 4.0` / 字体 14）上、另一半跑在这一面的出货尺上，
于是两条 pin 量的是一个生产上不存在的**混合帧**——它们**绿**，但绿得没有意义。

本轮把两半都装上这一面的调色板与出货尺帧，再用同一把尺子重新读那两个数（文字列 / 标题行盒），
把「两条列表共有的一列」和「两个标题共有的字阶」钉在一个**app 真会画**的帧上。

为什么现在做：§6.86 的 `dg-l` 21 条里 V 类（单位算术 / 取整 / 阶梯）与 S 类（前提破）已被
P13-dq…dv 逐族交割，混排是 `dg-l` 剩下的**唯一一族夹具账**；之后这一面只剩 1 条产品墨（`dg-l-4`）。

## 先量（改前 vs 改后，同一台建机实测）

| 读数 | 库存尺（两条 pin 今天量的帧） | 出货尺（app 真会画的帧） |
|---|---|---|
| 帧 | 列表 `720×817`（`WindowHeight("small")+500`）/ 输出 `720×488` | **`720×439`**（`themedFrameHeight(maxResultSlots)`） |
| 列表行标题 X | `54.0000` | **`48.6000`** |
| 输出列表行标题 X | `54.0000` | **`48.6000`** |
| 列表小节标题 X / 行盒高 | `54.0000` / **`22.0000`** | **`48.6000`** / **`17.0000`** |
| 输出分组标题 X / 宽 / 行盒高 | `54.0000` / `33.0000` / **`22.0000`** | **`48.6000`** / **`25.0000`** / **`17.0000`** |
| 输出行盒宽 | `650.0` | **`656.9999`** |

两条 pin 在库存尺上**都是绿的**（`54.0 == 54.0`、`22 == 22`）——这正说明「绿」不等于「量的是这个设计」：
§6.86 里那条红串（`list title x=54.0, output panel's at 48.6` / `heading is 17 tall, launcher's 22`）
是**两半不同尺**时才会现形的 bug，而同一把库存尺下它永远不现形。

`48.6 = 54 × designUnit()`（`u = Spacing/4 = 0.9`），`17 = designMetrics()` 的 `SubtitleLine`；
两者都是 §6.76 判据里的出货量，不是新写的字面量。

## 落地项

1. **`internal/launcher/textcolumn_test.go:34`**（列表半）：裸 `ui.NewTester(a.View, InputWindowWidth,
   `int(WindowHeight("small"))+200`)` → **`renderShipped(t, a, false)`**。原注释「a taller window so the
   block is not clipped」是库存尺的帧高不足造成的；出货尺下 `themedFrameHeight(maxResultSlots)` 装得下。
2. **`textcolumn_test.go:44`**（输出半）：`runListOutput` → **`listOutputShipped(t, b, rows, false)`**。
3. **`textcolumn_test.go:60`**：新增一条**锚**——文字列必须等于参考算术 `54u = 16u + 28u + 10u`
   换算到出货步子的值（`tenths(54 × designUnit())`）。原 pin 只问「两张列表相等」，
   两张一起漂它也绿；锚把这一列钉在参考的推导上。
4. **`textcolumn_test.go:87/99`**（opt-in dump）：两半同换出货尺，否则出图与 pin 量的不是同一帧。
5. **`internal/launcher/outputheading_test.go:23`**（列表半）：裸 `ui.NewTester(… +500)` →
   **`renderShipped(t, a, false)`**。
6. **`outputheading_test.go:37`**（输出半）：`runListOutput` → **`listOutputShipped(t, b, stdout, false)`**。
7. **`outputheading_test.go`**：新增一条 pin——分组标题的行盒高 == `designMetrics()` 的 `SubtitleLine`（17），
   即「标题印在副标题那一档上」，而不是「两个数碰巧相等」。
8. **`internal/launcher/headingcolumn_test.go:94`**：`sameColumn` 的 `±0.5` **容差带**换成
   `tenths(a) == tenths(b)`（§6.77/§6.84 的同一条判据：问「同一列」要落到 painter 的十分位，
   不是「差得不多」）。这一条同时收紧 4 条同族 pin（`headingcolumn_test.go:43/57/74`、`textcolumn:47/56`）。

## 行为契约零改动红线

- 产品代码**零改动**（`internal/launcher` 只有 `_test.go`）；高度链、键盘模型、会话管理、
  设置写回、i18n 文案、命令接线全不碰。
- 断言强度只许**加强**：删容差带、加锚、加字阶 pin；**不许**加容差、`t.Skip`、放宽断言。

## 台账预判

V：`sameColumn` 容差带 → 十分位（4+2 条 pin 收紧）；混排两半的取景（读数的旧值 → 新值）。
B：0（若红，先按 B 判产品回归）。S：0。

## 门槛命令 + PASS 基线（P13-dv 实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿，PASS 基线 3325 / FAIL 0 / SKIP 159
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨
GOOS=linux/windows/darwin go build ./...            # 全过
```

## mutation 计划（实跑，红 → 还原 → md5 → touch 复跑）

- **M1**：`textcolumn_test.go` 的列表半退回裸 `ui.NewTester`（库存尺）。
  预期红：`the launcher list's title starts at x=54.0, the output panel's at 48.6`。
- **M2**：`outputheading_test.go` 的输出半退回 `runListOutput`。
  预期红：`the output group heading is 17 tall, the launcher's section title is 22`。
- **M3**：`sameColumn` 改回 `±0.5`，并在参考算术锚上注入 0.4px 偏移 → 容差带全绿、锚独红，
  证明这条锚比旧比较器强（若该变异不红，则说明锚写错了，重写）。
