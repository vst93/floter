# P13-cj 规格：选中行的副标题与标题同墨，而同一张卡的另一张列表只提一档

## 意图

**同一张卡里的两张列表，选中行的副标题应该落在同一个档位。** 捕获输出页的选中行
把副标题的墨一路提到标题的墨（`t.Text`），于是两行只差字重一条腿；启动器结果列表
在同一状态下只提一档（`TextMuted` → `TextSecondary`）。一张卡里同一种状态两套读法。

为什么现在做：这是 P13-ci「仍待」第 1 条，也是 `docs/liquid-glass.md` §4
「层级靠面差 + 字重，不靠画框」在**状态**这一面上的最后一块：P12-b / P13-cg 已经把
两张列表的面、灯、游标收成一条规则，剩下副标题的墨还是两套。

## 证据（实测，先量再改）

fixture：捕获输出页两张带副标题的行（第 0 行被选中）、启动器结果列表 6 个应用
（第 0 行被选中，副标题取 `.desktop` 的 `Comment`）。量法：每个字带读**离行面最远的
像素**（§6.51 判据 2——深色外观下最暗的是面，不是字）。

| 面 | 标题 墨距 | 副标题 墨距 | 差 |
|---|---|---|---|
| 捕获输出 · 选中（改前） | 214 | **214** | **0** |
| 捕获输出 · 选中（改后） | 214 | **131** | 83 |
| 启动器 · 选中 | 211 | **131** | 80 |
| 捕获输出 · 静息 | 229 | 140 | 89 |
| 启动器 · 静息 | 226 | 140 | 86 |

深色同形：改前 **201 / 201（差 0）**，改后 **201 / 111（差 90）**，启动器选中
**194 / 111（差 83）**。

## 逐条落地

1. `internal/launcher/output.go`（`outputRow` 的副标题分支）
   - 现状：`if chosen { subtitle.TextColor(t.Text) }` —— 与标题同墨。
   - 改成：`if chosen { subtitle.TextColor(tokens.TextSecondary) }`。
   - **不提字重**：这一页的行高是内容撑的（没有 `MinHeight` 地板），
     `FontWeight(600)` 会换一个字体面，把选中行从 **54px 压到 53px**（实测），
     游标一移整张列表抖 1px。启动器那边的选中行副标题是 600，但它的行有 40u 地板。
     这条取舍写进 `docs/liquid-glass.md` §6.53 判据 2。
2. `internal/launcher/outputsubtitle_test.go`（新 pin，四条臂）
   - ① 选中行标题与副标题的墨距差 ≥ 60（改前 0，改后 83 / 90）。
   - ② 选中行副标题的墨距与启动器选中行**逐级相同**（±4；改后 131 / 111 两处相等）。
   - ③ 静息行两处副标题的墨距相同（±4；两处 140 / 138）——选中改的只是「提一档」。
   - ④ 选中行与静息行**一样高**（改后 54 / 54；加字重的那一版 53 / 54，红）。
3. `docs/liquid-glass.md` 新增 §6.53（判据 + 数值 + 反面候选）。

## 行为契约零改动红线

只改**一个颜色的取值**：没有动高度链（行高仍由内容撑、`outputRows()` 的
`Height: 2` 不动）、键盘模型、会话管理、设置写回、i18n 文案、命令接线。
`outputText()` / `runOutputRow()` / 点击分支一个字节不动。

## 门槛命令与 PASS 基线（实测当前值）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
GOOS=windows go vet ./...                # 净
GOOS=darwin  go vet ./...                # 净
go test -count=1 ./...                   # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3254（P13-ci）
```

## mutation 计划（≥2 条）

- **M1**：把 `tokens.TextSecondary` 改回 `t.Text` → ① 与 ② 双红
  （`the chosen row's subtitle is 0 levels from its title's ink`）。
- **M2**：给副标题补 `FontWeight(600)` → ④ 红
  （`the chosen row is 53px tall and the plain row 54px`）。
- **M3**：把副标题的静息墨改成 `TextSecondary`（即整族提一档）→ ③ 红
  （静息行两处副标题不再同级）。

## 台账预估

V 0 / B 0 / S 0：没有旧 pin 断言过选中行副标题的墨（`grep` 过 `outputaccent_test.go` /
`outputstyle_test.go`），行为 pin 全部零改动。
