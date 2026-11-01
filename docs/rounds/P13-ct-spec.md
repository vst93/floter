# P13-ct 规格 · 降级面的灯：一条关着的命令，冠线既不是亮的，也不是 62% 亮的

## 意图一句话

扩展页的**命令块**在开关关着时不再点灯（`LitFace` → `DimFace`），从此「这条命令是关的」
在两套外观上是同一个说法：灯熄、暗线留、面还在。

为什么现在做（选题来源）：任务书「下一格从『状态反馈』的另一半（**禁用 / 只读面**）里挑，
先量再改」；`docs/liquid-glass.md` §6 差距表已无 `未开始` 行，走通用队列。而
`.pending-bugs.md` 的「仍待」里禁用面一族只剩这条**不是 `Disabled` 的降级面**没有量过——
§6.15 18-1 只回答了「整张卡片 `Disabled`」那一半。

## 证据（先量，实测；本文件里的数字都是量的，不是抄的）

平铺 harness：`realPageGround` 的两种地面、`Root().Background(ground)`、420×220、
`ReduceMotion`、同一段 `commandPlate` 调用（`Command{ID:"build", Enabled: …}`）。
`litSum` 三通道和，**1 个亮度级 = 3**。

| 状态 | 外观 | 地面 | 面 | 冠线行 − 面 | 底对侧 − 面 |
|---|---|---|---|---|---|
| 开着的命令 | 浅 | 742 | 690 | **+18（6 级）** | 63（21 级） |
| 开着的命令 | 深 | 44 | 118 | **+57（19 级）** | 24（8 级） |
| 关着的命令（改前） | 浅 | 742 | 709 | **+9（3 级）** | 39（13 级） |
| 关着的命令（改前） | 深 | 44 | 89 | **+39（13 级）** | 12（4 级） |

（改前的冠线数字取自变异 M1 的实测输出：`718 over a 709 face (+3 levels)` /
`128 over a 89 face (+13 levels)`。）

**问题不是「太淡」，是「没有决定」**：`commandPlate` 先无条件 `tokens.LitFace(plate)`，
再对关着的命令 `plate.Opacity(0.62)`（参考的 `.extension-command-list__row--off`）。
两道一叠，冠线成了「62% 的灯」——浅色档 6 级 → 3 级（一条本来就只有 6 级的线上，
读出来是一道洗白的边），深色档 19 级 → 13 级（还明明白白亮着）。**同一句话在两套外观上
是两个说法**，而 §3 说受光边是「这块面可以被碰」的记号。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `commandPlate`（`if command.Enabled` 分叉） | `tokens.LitFace(plate)` 无条件；关着时 `Opacity(0.62)` | 开着 → `tokens.LitFace(plate)`；关着 → `tokens.DimFace(plate)` + `Opacity(0.62)`（**淡仍只有一道**） |
| 2 | 同上，注释 | P13-m 只说了「命令块是 E1，要那一对线」 | 补 P13-ct 一段：降级面为什么换 `DimFace`（§3 的记号语义 + §6.15 18-1 的同一半），以及「它不是 `Disabled`」的理由（开关与别名框仍然活着，参考那边是 `--off` 不是 `:disabled`） |
| 3 | `internal/settingsui/platecrown_test.go`（新增） | 无 pin：两条命令的冠线/底对侧一个像素都没钉 | ① 开着的命令冠线 ≥ 3 级 ② 关着的命令冠线 ≤ 2（`litSum` 单位，同 `assertDisabledFace` 的容差）③ 关着的命令底对侧留着（浅 ≥ 5 级 / 深 ≥ 2 级）④ 关着命令的面仍离地面 ≥ 6 级（浅在下、深在上）；深/浅两套各跑一遍 |
| 4 | 出图 | — | `TestDumpP13CTCommandPlate`（opt-in，`FLOTER_P13CT_DUMP=1`，`FLOTER_P13CT_TAG=before/after`）写整页 PNG |

## 行为契约零改动红线

- 高度链 / 行盒：`commandPlate` 的 `Padding`、`Gap`、`Radius`、`Border` 一个字节不动。
- 键盘模型、命中框、`Disabled` 位：**不加**。这条面不是禁用面——开关、别名框、点击都照旧。
- 会话管理、设置写回、i18n 文案、命令接线：不碰（本轮不改任何字符串）。
- `Opacity(0.62)` 保留：参考的 `.extension-command-list__row--off`，且
  `TestTheCommandListDrawsTheReferencePlates` 钉着「关着的板比开着的板淡」。
- 开着那条命令的面与一对线**逐像素不变**（差分 bbox 不含它）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3289，只许涨
GOOS=linux/windows/darwin go build ./...               # 过
```

基线（HEAD `9b8d6081` 实测）：PASS **3289** / FAIL **0** / SKIP **138** / 测试函数 **2437**。

## mutation 计划（每条会让哪个 pin 变红）

1. **M1 · 关着的板重新戴 `LitFace`**（回到改前的两行）→
   `TestTheOffCommandPlateTurnsItsCrownOffAndKeepsItsShade` 的「still wears a lit edge」两红（深浅各一）。
2. **M2 · 删掉那道 0.62** → `TestTheCommandListDrawsTheReferencePlates` 的
   「the off command's plate is not dimmed」红（证明「一道淡」还在被钉着）。
3. **M3 · 关着的板什么材质都不戴** → 本轮的「lost its counter-side」红（证明 `DimFace`
   收的是「灯熄、暗线留」，不是「两样都没有」）。
