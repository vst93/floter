# P11-ac 规格：列表的分组头落在它自己的文字列上

## 意图（一句话）

启动器列表与输出面板的**分组头**（section heading）都要落在**本表面结果行标题的那一列**；
现在两个表面各给了一个不同的错答案，而两处注释与合约 §4.4 都写着「与行文字列对齐」。

## 为什么现在做（证据）

选题顺序（任务书）：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目
（§6 全表 `已落地`，只剩 `⌘K` 动作面，属拍板项）→ 上一轮「仍待」第 1 条（同上）
→ **界面质感与交互**。用户 2026-10-28 的原话是「还是先优化当前的界面、质感、交互逻辑」，
本轮就取「质感」里最可量的一条：**对齐**。

实测（`ui.Tester` 的 `Find()` 返回的元素矩形，`InputWindowWidth` 窗口，单位 u = 4px）：

| 表面 | 元素 | 实测 x |
|---|---|---|
| 启动器列表 | 分组头 `Recently launched` | **62.0** |
| 启动器列表 | 它下面第一行标题 `Editor` | **54.0** |
| 输出面板 | 分组头 `Group A` | **20.0** |
| 输出面板 | 它下面第一行标题 `Alpha` | **52.0** |

两处注释各自声称对齐，逐字对不上：

- `internal/launcher/launcher.go:1988-1990`：「The inset is the row block's own gutter (12u)
  plus the row's 4u padding plus the icon column's 28u and the 10u gap, so the label sits on
  the same column a row's title does.」——实测 62，标题 54，**差 8u（右偏）**。算式本身也
  对不上代码：`Padding(1.5, 3, 1, 3)` 的左 padding 是 **12u**（不是 4u），行块的 gutter
  是 **8u**（不是 12u）。
- `internal/launcher/output.go:529`：「with the 6u/12u/4u inset the rows use」——行根本没有
  12u 的左 inset（行是 8u），而且**没有** icon 列 + gap 的 36u 偏移，所以分组头挂在
  **20u**，比它自己的标题（52u）左了 **32u**。

成因：分组头的左缩进是**两个手写数**（行的左 padding + 文字的 `Margin-left`），从来没有
从「行的标题列」这个和里推出来。P8-a 把行块缩进 4u→8u、行内 padding 12u→8u 之后，这两个
手写数一个都没跟着动，于是同一件事在两个表面上漂成 62 与 20。

**为什么这是「质感」而不是「一致性收尾」**：分组头是每一屏列表都看得见的一行字，它比
它管辖的标题右挂 8u（启动器）/ 左挂 32u（输出面板），读起来就是「这一行没对齐」；Raycast
的 `.launcher-section-title` 只有一份。

## 逐条落地项

### 1. 启动器列表的分组头（`internal/launcher/launcher.go:1992-1996`）

- 现状：`Padding(Space(1.5), Space(3), Space(1), Space(3))`（左 12u）+ 文字
  `Margin(0, 0, 0, Space(10.5))`（左 42u）→ 8（列表 gutter）+ 12 + 42 = **62u**。
- 应改成：行自己的横向 padding（`Space(2)` = 8u，与 `rowBody` 的 `Padding(0, t.Space(2))`
  同一档）+ 图标列 28u（`Space(7)`，与 `iconPlate` 同一档）+ 行自己的 gap 10u
  （`Space(2.5)`，与 `rowBody` 的 `Gap` 同一档）= 46u 的 `Margin-left`。
  于是 8 + 8 + 38 = **54u** = `TestTheFieldAndTheRowsShareTheReferenceTextColumn` 钉的
  文本列（合约 §3）。
- 纵向一字不动（`1.5u / 1u` 的上下 padding 即 `headingPadding` 的 6u/4u band）。

### 2. 输出面板的分组头（`internal/launcher/output.go:532-536`）

- 现状：`Padding(Space(1.5), Space(3), Space(1), Space(3))`（左 12u）→ 8（列表 padding）
  + 12 = **20u**。
- 应改成：行自己的横向 padding 8u（`outputRow` 的 `Padding(t.Space(1.5), t.Space(2))`）
  + 图标列 28u + 该行自己的 gap 8u（`Gap(t.Space(2))`）= 36u 的 `Margin-left`。
  于是 8 + 8 + 36 = **52u** = 输出面板自己的标题列。
- **不**把输出面板的 gap 改成启动器的 10u：那是另一件事（两表面的行几何各自成篇），
  本轮的规则是「分组头 == 本表面行标题的列」，不是「两个表面共用一个 gap」。

### 3. 新增 pin（`internal/launcher/headingcolumn_test.go`）

`TestTheGroupHeadingSitsOnTheRowsTextColumn`：两个表面各测一次，断言
「分组头的 `Find().X` == 它下面第一行标题的 `Find().X`」，并且（启动器侧）也等于
`Recently launched` 上面那一行内置行标题的 x。**断言的是相等，不是某个字面数**——
这样以后再挪列，两处会一起红。

## 行为契约零改动红线

- **高度链**：`headingPadding`（6u/4u）与 `WindowHeight` 的 band 一个数不动；只改横向。
- **键盘模型**：分组头仍然不可选、不可编号、Enter 无动作（`rowSelectable` 未动）。
- **会话管理 / 设置写回 / i18n 文案 / 命令接线**：零改动。
- **视觉**：字阶（body 步）、weight（500 / 620）、墨（`TextSecondary` / `TextMuted`）零改动。

## 门槛命令 + PASS 基线（本轮实测）

基线（HEAD `ef95721`，改动前实跑）：`go test -count=1 ./...` 全绿；
`--- PASS` **2350**；测试函数 **1911**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l`）。

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # ≥ 2351（新增 1 条 pin）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

- **M1**：启动器侧把 `Margin-left` 从 46u 改回 42u（即只改 padding 不改 margin）→
  `TestTheGroupHeadingSitsOnTheRowsTextColumn` 报「heading x=58, title x=54」。
- **M2**：输出面板侧把 `Margin-left` 从 36u 改成 0（等于回到「无偏移」）→ 同一条 pin
  报「heading x=16, title x=52」。
- **M3**：把启动器侧的左 padding 从 `Space(2)` 改回 `Space(3)`（12u）→ 报「heading x=58」。
