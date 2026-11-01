# P13-dc 规格 · 插件配置浮层：先把取景装成设计，再谈字阶

## 意图（一句话）

`pluginSheetFrame` 这块**取景**从 P13-aw 起就没有 `c.SetTheme(a.tokens(c).Theme)`，
所以 `TestThePluginSheetEdges*` 三条 pin 一直在量**框架库存的**主题（字 14 / 间距 4），
而不是本仓的设计（字 10.8 / 间距 3.6）——本轮把取景装对、把三条 pin 重钉到真实读数，
再把浮层最后那个「body 的偏移量」字号（`t.FontSize+2`）换成阶梯上的 `tokens.Title`。

为什么现在做：P13-db 的「仍待」第 1 条就是这一格（`docs/rounds/P13-db-report.md` §2-1），
它挡着「弹层里的字号换到阶梯」这件事——不先修取景，换字号会让一条**量错对象**的 pin 变红，
从而把一次正确的改动判成回归。

## 证据（动手前实测，行号已核）

- `internal/settingsui/sheetcutedge_test.go:57-68`：
  ```go
  tt := ui.NewTester(func(c *ui.Context) {
      tokens := a.tokens(c)
      c.Root().Background(tokens.Theme.Background)
      ...                                  // ← 全程没有 c.SetTheme(tokens.Theme)
  }, 720, 600)
  ```
- 生产侧（`internal/settingsui/pluginsheet.go:62-63`）读的是 `t := c.Theme()`：
  间距、字号、行高全部来自 `c.Theme()`。取景不装主题 ⇒ 这里读到的 `t.FontSize` 是
  框架库存的 14、`t.Space(1)` 是库存的 4，而生产是 10.8 / 3.6。
- 同一条口径教训已经记过两次：P13-cm（`.pending-bugs.md` 顶部）与 P13-db §2-1。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/sheetcutedge_test.go` `pluginSheetFrame` | 只取 `tokens.Theme.Background` | 每帧 `c.SetTheme(tokens.Theme)`（先把 tokens 取出来，再装主题、再画底） |
| 2 | 同上三条 pin（`TestThePluginSheetFadesAtBothEdges` / `…SitOnBlankAtBothEnds` / `…EdgesAddNoGeometry`） | `MaxY` 500、标题盒 `(20,63) 90×22` 等库存字下的读数 | 重钉到装主题后的实测值（V：旧值 → 新值，注释写明「取景此前没装设计」） |
| 3 | `internal/settingsui/pluginsheet.go:153` | `FontSize(t.FontSize+2)`（= 12 + 2 = 14 at scale 1.0，**不随界面大小缩放**） | `FontSize(tokens.Title)`（阶梯上的 title 档，12.6 / 0.9 步） |
| 4 | `internal/ci/fontladder_contract_test.go`（新） | — | AST 账：`internal/settingsui/pluginsheet.go` 里不许再出现 `t.FontSize±N`；只许 `tokens.<档>` 与那一个**故意留在 body 档**的浮层标题 |
| 5 | `internal/settingsui/sheetcutedge_dump_test.go`（沿用） | 老图 | 复跑出深浅两帧前后对比图 |

**行为契约零改动红线**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线——本轮
只改**取景**（测试）与**墨/字阶**（`t.FontSize+2 → tokens.Title`，同一行盒，实测零像素差）。

## 门槛（PASS 基线用本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿，PASS 3307（基线）/ FAIL 0 / SKIP 144
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

新 pin 会让 PASS 涨 1（contract test 一条）+ opt-in dump 不变；SKIP 不变。

## mutation 计划（≥2，每条说明会让谁变红）

- **M1**：把 `pluginSheetFrame` 的 `c.SetTheme(tokens.Theme)` 删掉（退回 HEAD 的取景）→
  `TestThePluginSheetEdgesAddNoGeometry` 必须红（`MaxY` 与标题盒回到库存字的 500 / 90×22）。
- **M2**：把 `FontSize(tokens.Title)` 退回 `FontSize(t.FontSize+2)` →
  `TestThePluginSheetHeadingsUseTheLadder`（新 contract test）必须红。
- **M3**：给浮层标题也加一个 `t.FontSize-1`（模拟「顺手加一个偏移」）→ 同一条 contract 红，
  且报的是**标题行**不是分组行。

## 判定（V / B / S）

- **V**：三条既有 pin 的读数被重钉（原因：取景此前没装设计，量的是框架库存的字）。
- **B**：`go test ./...` 全绿，零行为 pin 改动。
- **S**：只有 `pluginSheetFrame` 这个**夹具**换了取景方式，产品结构不变。
