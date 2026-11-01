# P13-cx 规格：同一张卡上的第二块——Freshness 也挂在卡框上

选题：通用队列 `空间感`（P13-cw「仍待」第 1 条）。上一轮把健康块从卡框收回卡的行列，
并留了一句「按本轮这一族推断，Freshness 块很可能同样挂在卡框上，**先渲一帧量出 x 再决定**」。
本轮做的就是那句「先量再改」。

## 意图

一句话：**把 Freshness 块从卡框上收回卡的行列里，纵向一个像素不动。**

为什么现在做：P13-cw 修的是「块被画成卡体列的兄弟、于是内容比它自己的标题行左 8px」这个
**族**。同一个函数 `integrationRow` 里，`freshnessSection` 是**同一个写法**留下的最后一个
实例。用户说的「所有东西挤在同一个 z 平面上、靠 1px 描边分家」，在这一块上的具体形式是：
块的左端贴卡的内框、右端（`Result` 的值）贴卡的外框，块与卡之间没有空气，读起来是卡的
一部分而不是新鲜度这一点事实。

## 实测（Integrations 页，720×1700，深浅两套逐像素同坐标）

| 事实 | 改前 | 改后 |
|---|---|---|
| 卡面横向 / 1px 框线 | `192..700` / `191`,`701` | 不变 |
| 卡上行标签 `Health` | `x=200` | 不变（`200..617`） |
| 块标签 `Freshness` | **`x=192`** | **`x=200`** |
| 元数据标签 `Last probe` / `Commands` | **`x=192`** | **`x=200`** |
| 元数据网格第二格（`Result` 的值） | `452.5..701`（顶卡框） | `452.5..693`（卡内容右端） |
| 纵向：`Freshness` / `Last probe` / `Commands` / `Health` 的 y | 394 / 409 / 449.5 / 478 | **完全一致** |

`Freshness` 与 `Last probe` 的行盒是**紧贴**的（394+15 = 409）：块内四行本来就是零间隔的
兄弟，所以本轮的包装列必须 `Gap(0)`，否则纵向会凭空长出 8px。

## 逐条落地项

位置按本轮实测行号（动手前已核：`settingsui.go:5813` 起）。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `freshnessSection` 的四个直接子元素（块标签、元数据网格、命令计数行、健康来源注脚） | 直接画进卡体列，块左端 = 卡面第一像素（192），元数据右端 = 卡框（701） | 包进 `ui.Column(c).FillWidth().Gap(0).Padding(0, a.rowInset(t))`：块进入卡的行列（200 / 右端 693），**纵向零改动** |

**不动**：块内四行的字阶 / 字重 / 墨（`TextSecondary` / `TextTertiary` / `TextStrong` / `T.Muted`）、
`metadataCell` 的两列网格（`GapX(Space(3))`）、计数字阶与那枚 `·`、健康来源注脚的 `Wrap()`、
组间纵向节奏、i18n 文案、交互与命令接线。

## 行为契约零改动红线

只改列：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部零改动。
本轮没有交互面，也没有新文案。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

PASS 基线 **3296** / FAIL 0 / SKIP 141 / 测试函数 **2449**（P13-cw 收尾实测值）。

## mutation 计划（≥2 条）

1. **M1**：包装列 `Padding(0, a.rowInset(t))` → `Padding(0, 0)`（恢复旧列）→ 列 pin 与注脚 pin 必红。
2. **M2**（过冲臂）：`Padding(0, a.rowInset(t)*2)` → 列 pin 红（不是越宽越好）。
3. **M3**：`Gap(0)` → `Gap(t.Space(2))` → 纵向 pin 红（块内四行凭空长 8px）。

## 视觉产出

`ui.Tester` 导出 `/tmp/floter-P13-cx-freshness-{light,dark}-{before,after}.png`，
报告写「这张图上看到的一处变化」+ 差分像素数 / bbox。
