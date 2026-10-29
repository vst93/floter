# P11-cd 规格 · 什么都没问的空页不报「没有匹配项」

轮号 P11-cd｜分支 `mygo-rewrite`｜前 HEAD `e0f78d57`（P11-cc）｜基线 PASS **2752** / 0 FAIL / 55 SKIP

## 1. 意图与证据

用户反馈 F1–F4 已收完，合约 §6 全 `已落地`，P11-cc「仍待」第 1、2 条都是拍板项 →
按用户定的优先级（界面质感与交互 → 基础功能 → …）自己找，落在一处**用户会直接读到的错话**：

`internal/launcher/launcher.go:847` 的 `emptyMessage(copy)` 只有一个 `default: return copy.NoResults`。
它同时接住两种页面：

| 状态 | 该说什么 | 今天说什么 |
|---|---|---|
| 有 needle、真的没命中 | 「没有匹配项」 | 正确 |
| 空查询 + 一条最近启动都没有（全新安装 / 最近记录被清空） | 什么都不用说（字段的占位句已经在说「输入命令或应用名称」） | **「没有匹配项」** |

实测（本轮，`720` 宽，`testApp()` 无最近启动）：帧上的字是
`["No results" "Search" "Type to search" "Settings"]` —— 用户还没输入任何字符，卡片就否认了一次没发生的搜索。
P11-bk 的注释写着「无 recents 时页就是空的」，但那一轮删的是内置指令行，`len(results)==0` 仍旧落进这条默认分支。
P11-bh 已经为「没有拖入文件」的页面立过同一条规则：**没有 needle 的页面不借 `NoResults`**。

## 2. 逐条落地项

| 位置 | 现状 | 改成 |
|---|---|---|
| `internal/launcher/launcher.go:886-888`（`emptyMessage` 的 `default`） | 无条件 `return copy.NoResults` | `strings.TrimSpace(a.Query) == ""` 时返回 `""`，其余照旧返回 `copy.NoResults` |
| `internal/launcher/launcher.go:1321-1326`（空态列） | 无条件 `ui.Text(c, a.emptyMessage(copy))`（空串也会建一个空 text 元素） | 句非空才建；空页只剩字段自己的占位句 |

不动：高度链（`statusBand` / `Geometry` / `rowUnits`）、键盘模型、会话、设置写回、i18n 文案（**没有新增也没有改字**，
只是让「没有 needle 的页」不再借用 `NoResults` 这一句）、命令接线、所有墨与面。

## 3. 门槛（本轮实测，不用旧数）

```
gofmt -l cmd internal                      → 空
go vet ./... / GOOS=windows / GOOS=darwin  → 净
go test -count=1 ./...                     → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS" → 2754（基线 2752）
GOOS=linux|windows|darwin go build ./...   → 过
```

## 4. pin 台账（V/B/S）

- **新 pin**：`internal/launcher/emptynoresults_test.go` 的 `TestAnEmptyQueryNeverClaimsNoResults`
  （两臂：空查询 + 无最近启动 → `emptyMessage == ""` 且帧上没有 `NoResults`、字段占位句还在；
  有 needle 且没命中 → 仍然 `NoResults`）。
- **S（结构变了）**：`internal/launcher/filters_test.go:326` 的「ordinary page」臂原本在**空查询**上
  断言 `NoResults`；空查询不再是有 needle 的空结果，夹具补一个真的没命中的 needle（`a.Query = "zzzz"`）。
- **S（产品规则变了）**：`internal/launcher/textheight_test.go` 的空页臂 `must` 去掉 `18`
  ——那一页不再画句子（同一条规则的后果）；`18` 仍由同测试的「有 needle 的空结果」与 `match` 臂守着。

## 5. mutation 计划（每条说明让哪个 pin 变红）

| # | 注入 | 预期 |
|---|---|---|
| M1 | 删掉 `emptyMessage` 的 `TrimSpace` 守卫（回到无条件 `NoResults`） | `TestAnEmptyQueryNeverClaimsNoResults` 两行红（第一臂） |
| M2 | 守卫之后的 `return copy.NoResults` 改成 `return ""`（真的没命中也不说话） | 同一 pin 的第二臂红 |
