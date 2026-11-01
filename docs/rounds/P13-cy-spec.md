# P13-cy · meta 行的收尾词，比它收尾的三个事实大一级

## 意图（一句话）

集成卡 meta 行最后那个状态字与它前面三个事实**同一个字阶**：这条行的高度由它自己的
caption 决定，而不是由收尾那个词决定——标题行白长的 5px 还回去。

## 为什么现在做（证据）

P13-cx「仍待」第 1 条点名 `integrationMeta` 与 `packageChip` 是「块被画成卡体列的兄弟」
那一族的下两个候选，要求**先渲一帧量 x 再决定**。量完的结论分两半：

**（a）列是对的，不改**（Integrations 页 720×1700，深浅同坐标）：

| 事实 | 实测 |
|---|---|
| 卡面横向 / 1px 框线 | `192..700` / `191`,`701` |
| 卡上行标签 `Health` | `x=200` |
| 前导 `packageChip` | `200..230`（`Space(7.5)`=30px），左沿 = 行标签列 |
| 标题 / 描述 / meta 三行 | 同一条文字列 `x=242`（= 方格右沿 230 + `Space(3)`），与参考的 30px icon + 12px gap 一致 |
| meta 行右侧 | 停在卡内容右端 `693`，不顶卡框 |

**（b）行里面有一处字阶越界**：

| 节点 | 改前实测 |
|---|---|
| 三个事实（`1.2.3 · dev.floter.probed` / `· Extension package` / 运行来源） | `y=392.5`，`h=15` |
| 收尾状态字 `· Running` | `y=390`，**`h=20`** |
| 行内 `Tool unavailable` 暖胶囊 | `y=364`，**`h=19`** |

参考的 `.extension-row__meta` 声明 `font-size: var(--text-caption)`
（`main:src/styles/extensions.css:960-970`），`.extension-status` 只声明
`padding` / `line-height` / `border-radius`（`:1007-1015`）——字号继承 caption。
本仓 `stateMark` / `runtimeUnavailableBadge` 写的是 `t.FontSize-1`（12），而事实写
`tokens.Caption`（11）：同一行、同一 baseline 带里两种字号，行盒归大的那个管，于是整条
meta 行 20 高，标题行跟着长高 5px，卡里每一行往下挪 5px。

## 逐条落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `settingsui.go:2162` `stateMark` 破损暖胶囊的文字 | `FontSize(t.FontSize - 1)` | `FontSize(tokens.Caption)` |
| 2 | `settingsui.go:2178` `stateMark` 开着臂 | `FontSize(t.FontSize-1)` | `FontSize(tokens.Caption)` |
| 3 | `settingsui.go:2183` `stateMark` 关着臂 | `FontSize(t.FontSize-1)` | `FontSize(tokens.Caption)` |
| 4 | `settingsui.go:2207` `runtimeUnavailableBadge` 的文字 | `FontSize(t.FontSize - 1)` | `FontSize(tokens.Caption)` |

不动：字重 600 / 墨（`TextStrong`、`TextSecondary`、`Warning`）/ 那枚 `·` /
`Margin(0,0,0,Space(0.5))` / 胶囊的 `Padding(Space(0.5), Space(1.5))` 与 `ControlEdge`
内环 / 暖面 `RaisedWarm` / 行的 `gap` 与 `AlignItems(Center)`。

## 行为契约零改动红线（逐条）

- 高度链：本轮**只**改 meta 行内部的行盒（20→15）；标题行的 66u 地板、卡几何、
  行 padding 一律不动。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：零触碰（没有文本增删，
  只有字号）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3298 → 3300
测试函数                                               # 2450 → 2453
SKIP                                                   # 142 → 143（本轮新增 opt-in 出图）
GOOS=linux/windows/darwin go build ./...               # 过
```

## mutation 计划（≥2 条）

- **M1**：`stateMark` 两条透明臂回 `t.FontSize-1` → 预期
  `TestTheMetaLinesClosingWordIsOneRegister` 在深浅两套上各红两条
  （`the closing state word is 20 tall, the facts it closes are 15` /
  `the state word starts at y=390, the facts at y=392.5`）。
- **M2**：`runtimeUnavailableBadge` 回 `t.FontSize-1` → 预期同一条 pin 红两条
  （`the unavailable badge's label is 19 tall, the meta facts 15`）。
- **M3**：破损胶囊的文字回 `t.FontSize-1` → 预期
  `TestTheBrokenChipsLabelIsACaption` 红（深浅各一条）。
