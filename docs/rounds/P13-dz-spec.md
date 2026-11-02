# P13-dz 规格：会话行的 56u 地板 —— 先量，处方被证伪

## 意图

`docs/liquid-glass.md` §6.86 的 **73-6** 是那张表里最后一条 `未开始` 的**产品**行：
「会话行 49 对参考的 56 地板，少 7px，补 `min-height = Space(14)`」。这一轮按它给的处方做，
并按「先量再改」先确认处方能不能落地。

**结论：处方落不了地。** 注入之后像素**一个都不动**（实测 49 → 49），说明这一行的盒
不由行元素拥有。本轮因此是**证伪轮**：产品零改动，把「为什么不行 + 谁才是主人」写进合约，
免得下一轮照着同一个处方再撞一次。

## 为什么现在做

- 它是 §6.86 里唯一还开着的产品行（其余是取景债）。
- 处方是 P13-dd 写的（`docs/rounds/P13-dd-report.md`「仍待」第 1 条），
  而 P13-dd 的结论本身是「上一轮量的是生产上不存在的取景」——同一格已经被证伪过一次，
  这次要证伪的是**修法**，不是量法。

## 落地项（逐条）

| # | 位置 | 现状 | 处方 | 实测 |
|---|---|---|---|---|
| 1 | `internal/settingsui/rows.go:1488` `cardRow` | `floor := max(t.Space(11), rowFloorPx)` = 44，行内容 49 撑过地板 | 行级 `MinHeight(Space(14))` = 50.4 | **无变化**：卡片盒仍 `160..208` = **49** |
| 2 | `internal/settingsui/settingsui.go` 会话行 `rowValue(...)` | `rowFace{}` | `rowFace{Floor: t.Space(14)}` | 同上，**49** |

注入的形状（已还原，`git status` 空）：给 `rowFace` 加一个 `Floor float32`，`cardRow` 里
`if face.Floor > floor { floor = face.Floor }`，会话行传 `t.Space(14)`。
编译过、`gofmt` 空、`TestTheSessionRowsHeightAgainstTheReferencesFloor` **仍然绿**（它问的是 49）
——绿本身就是证据：这一行的盒不是行元素给的。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**全部零改动**（产品 0 行）。

## 门槛

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux|windows|darwin go build ./...
```

基线（P13-dy 实测）：**3326 PASS / 0 FAIL / 159 SKIP / 2493 测试函数**。

## mutation 计划

本轮的产品「变异」就是**上面那条注入本身**（注入 → 量到 49 → 还原 → `git status` 空）。
另加两条文档/断言侧的核对：

- M1：把 `sessionrowbox_test.go` 的 49 改成 50 → 必须红（红串给出真值 49），证明这条 pin
  真的在量这一行。
- M2：把 `sessionrowbox_test.go` 的 padding 8 改成 9 → 必须红（标题起点）。

两条都实跑，红串抄进报告。
