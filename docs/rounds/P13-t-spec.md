# P13-t 规格 · 锁定的表单：一道淡，不是三道

## 意图

**锁定（`locked`）的扩展配置表单，今天同时存在两个淡档：字段 / 开关 / 下拉是有效
0.3（框架的 0.5 × 产品的 0.6），列表框是 0.6（只有产品那一档），框里的勾选框是
0.3（列表框 0.6 × 自己的 `Disabled` 0.5）。** 同一张表单上，「不可用」有三种深度，
而它们都表示同一件事。P13-s 已经把这一族在**胶囊**上收成「一道淡 + 灯熄了」，
这一轮把**槽与开关**（字段、开关、下拉、列表框）收进同一条规则：**一道淡 = 框架
的 0.5，材质照原样，只是整体退到一半**。

## 为什么现在做

- 通用队列「状态反馈」的另一半（禁用 / 只读面）。P13-s 做的是**按下**与**禁用胶囊**，
  报告「仍待」第 1/2 条把 accent 与警告环推给拍板，但**槽类控件从没量过**。
- 实测证据（`internal/settingsui/zz_measure_scratch_test.go` 当轮临时探针，
  卡面 / 地面基准：浅色 247、深色 14~15）：

| 控件 | 外观 | 静息面 | 锁定面（改前） | 锁定墨（改前） |
|---|---|---|---|---|
| 配置文本字段 `extensionField` | 浅 | 230（−17） | **242（−5）** | 177（对比 65） |
| | 深 | 39（+24） | **22（+7）** | 87（对比 65） |
| 布尔开关 `switchControl` | 浅 | 79 | **195（−52）** | — |
| | 深 | 187 | **67（+53）** | — |
| 列表框 `configMultiselectBox` | 浅 | 230（−17） | **236（−11）** | 选项文字 160（−76） |
| | 深 | 39（+24） | **29（+15）** | 选项文字 26（−3） |

  两个读法：①字段的 E1 一对面在 0.3 下**双双归零**（顶内线 = 面 = 242，底内线 =
  面），那不是「一块不能碰的槽」，是一块几乎不存在的槽——与 P13-s 量到胶囊的 4 级
  台阶是同一个算术；②同一个表单里列表框停在 0.6、字段停在 0.3，**深度不一致**，
  用户读到的是「有的禁用有的没禁用」。

## 设计读取

读取为：**桌面应用里一张「正在运行中所以只读」的扩展配置表单**，面向会改扩展配置的
用户，用「轻便液态玻璃」语言（E1 面 / 凹槽 / 一道淡）。本轮只改**状态反馈的墨**，
不动布局、形状、文案。

- `VARIANCE` 3：不加任何新形状、新 token。
- `MOTION` 0：不动效（锁定的过渡本来就是 120ms 颜色过渡，不新增）。
- `DENSITY` 6：表单本来就密，本轮只补「一块槽该有的算术」。

## 落地项（行号开工时核对）

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/settingsui/config.go:507`（boolean / `switchControl`） | `.Disabled(true).Opacity(0.6).Cursor(...)` | `.Disabled(true).Cursor(...)` |
| 2 | `config.go:533`（select / `configSelect`） | 同上 | 同上 |
| 3 | `config.go:604`（multiselect 列表框） | `.Opacity(0.6).Cursor(...)`（**没有** `Disabled`） | `.Disabled(true).Cursor(...)`——一个 flag 落在控件根上，整棵子树 0.5，滚轮仍可滚（`ui/input.go:515-536` 的 `scrollBy` 不查 `flagDisabled`） |
| 4 | `config.go:627`（number 字段） | `.Disabled(true).Opacity(0.6)` | `.Disabled(true)` |
| 5 | `config.go:680`（text / password / path 字段） | 同上 | 同上 |
| 6 | `config.go:877-880`（`configOptionRow` 的勾选框） | `locked` 时自己再 `Disabled(true)` | 删掉——列表框已经是 `Disabled`，`IsDisabled` 走祖先链（`ui/element.go:1349-1356`），再挂一次就是 0.5 × 0.5 = 0.25 |

**为什么不新增 token**：P13-s 已经定过「这一档框架已经给了」。0.5 落到字段上是
238（−9）、落到开关轨道上是 163、落到列表框上是 238（−9），E1 那一对面仍在
（顶内线 +1、底内线 −7 级）——只是不再被 0.3 抹平。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字节不动**：

- `locked` 的判定（`a.configLocked`）不动；`e.Changed() && !locked` 的守卫不动。
- 列表框的**滚动**必须仍然可用（`Disabled` 只挡点击/焦点/hover，不挡滚轮）。
- 勾选框的点击仍被 `!locked` 挡住；`Disabled` 只是让 hover 不再点亮。
- 没有新增/删除任何 UI 文案。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 2973 PASS / 0 FAIL / 76 SKIP
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2973
GOOS=linux/windows/darwin go build ./...               → 过
测试函数数                                              → 2188
```

## mutation 计划（实跑结果见报告）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `lockedControl` 加回 `.Opacity(0.6)`（一道淡变两道） | `TestTheLockedFieldFadesOnce` / `TestTheLockedSwitchFadesOnce` / `TestTheLockedListBoxFadesOnce`（面掉回 242 而不是 238） |
| M2 | `configMultiselectBox` 根上的 flag 退回 `.Opacity(0.6)` | `TestTheLockedListBoxFadesOnce`（箱面 236、选项文字 160） |
| M3 | `configOptionRow` 给整行再挂一次 `Disabled(true)` | `TestTheLockedListBoxFadesOnce`（选项文字 0.25 → 205，与箱面不同档） |

**已证伪的一条**：`lockedControl` 改成 `Opacity(0.5)`（不挂 `Disabled`）时像素完全相同，
像素 pin 钉不到「用哪条机制淡」——这条只有语义差别，靠注释守。
