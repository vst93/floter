# P13-ey 规格 · 设置面板的「说明句」一族：一页两个寄存器，还有一句比标题左 2px

**意图一句话**：Integrations 页上同一角色的一句解释（分组标题下、列表上），此前按它
落在哪一组穿两种字号，其中一句还比自己的标题往左探出 2px —— 本轮把这一族收成
**一条竖线 + 一个寄存器**。

为什么现在做：`P13-ex`「仍待」第 2 条 = 通用队列的**文案密度**（「设置页各分组的提示句
与读数句是否同一档密度」）。先量再改，量的结果如下。

## 证据（实测，760×1400，深浅逐值相同；`internal/settingsui/hintregister_test.go` 的探针）

| 句 | 位置 | X | 标题 X | 行盒 | 墨重（浅/深） |
|---|---|---|---|---|---|
| `BasePluginsHint`（`settings-section__hint` 支） | `rows.go:295` | **188.8** | 190.8 | 17（2 行，盒高 34） | 104 / 95 |
| `DetectedHint`（`.extensions-section-hint` 支） | 同上 | 190.8 | 190.8 | **15**（1 行，盒高 15） | 104 / 95 |

三处病，同一族：

1. **两个寄存器**：`extensionHint(c, hint, caption bool)` 的 `caption` 支把 Detected 那句
   画成 caption 档（9.9），旁边 Base plugins 那句是正文档（10.8）—— 行盒 15 vs 17。
   同一页、同一条滚动、同一角色。
2. **2px 外凸**：`caption` 支缩进 `Space(1)`=4，非 caption 支 `Space(0.5)`=2，而分组标题
   （`extensionSection`，`rows.go:275` 附近）自己用 `Space(1)`=4。于是 Base plugins 那句的
   首个字形停在 188.8，它解释的标题在 190.8。
3. **参数只有一个用途**：`caption` 布尔值去掉缩进后只剩「把这句话画小一档」这一个作用，
   而这一档正是 preflight D2 的 9.9px 争议档（需拍板项）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `rows.go:295` `extensionHint` | 两条支：caption 档 / 正文档，2u 与 4u 两种缩进 | 一条：正文档（`tokens.Body`）、缩进 `t.Space(1)`（= 标题自己的） |
| 2 | `rows.go:295` 签名 | `extensionHint(c, hint string, caption bool)` | `extensionHint(c, hint string)` |
| 3 | `settingsui.go:1917` / `:6170` | 传 `false` / `true` | 去掉第三个实参 |

**墨不动**：两句都留在 `tokens.TextTertiary`。这不是漏改 —— 分组标题在面板里是
`--text-muted`（`extensionSection`），说明句比标题低一档才成立；设置卡片页的标题是
`--text-strong`，所以那边的 `sectionHint` 用 `--text-muted`。**规则是「说明句 = 标题墨
低一档」，不是「全仓同一个色号」。**

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线 **零改动**：本轮只改
一句解释的**字号档与左缩进**，文案串、分组顺序、列表内容、点击目标都没动。
Detected 那句在 760 宽下仍是 1 行（改后盒高 17，未回绕），所以面板高度只长 2px。

## 门槛命令与 PASS 基线（实测，改前）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3374
grep -c -- "--- SKIP"                               # 181
GOOS=linux/windows/darwin go build ./...            # 过
```

## mutation 计划（≥2 条，实跑）

- **M1** 缩进 `t.Space(1)` → `t.Space(0.5)`：`TestThePanelsExplanationsStartOnTheirTitlesOwnLine`
  应变红（两句都外凸 2px，报 `-2.0px off its title's line`）。
- **M2** 字号 `tokens.Body` → `tokens.Caption`：`TestThePanelsExplanationsShareOneLineBox`
  与 `TestTheExplanationsOnBothPagesWearTheSameStep` 应变红（15 vs 17）。
- **M3** 等价改写 `tokens.Body` → `a.tokens(c).Body`：应全绿（证明 pin 钉的是**读出来的**
  行盒与位置，不是某个写法）。
