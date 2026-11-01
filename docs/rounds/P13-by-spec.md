# P13-by 规格：正文与标题说的不是同一页（一条走不到的臂 + 一句「还没做完」的文案）

轮号 `P13-by`｜分支 `mygo-rewrite`｜基线 HEAD `6de76d1b`（P13-bx）

## 0. 意图

设置页的**正文**有一个 `default` 臂，画「此页面将在后续迭代中到来。」；而同一帧的**页头**
用的是 `a.page(a.Page)`，它的 `default` 臂返回第一页。三处读同一件事，两处说常规，一处说
这一页不存在。删掉这条走不到的分支，让三处读同一个夹取，并让那句「还没做完」的话从
`i18n.Copy` 里退场。

**为什么现在做**：§6 差距表已无 `未开始` 行，选题回通用队列的「文案密度」。这一格量下来
不是墨、不是面，是**同一帧里两处文案互相矛盾**——比任何 1 像素的差都更容易被用户读到，
只是它藏在一条永远不执行的分支里。

## 1. 证据（行号本轮实测）

- `internal/settingsui/settingsui.go`（改前）`body()` 的 `switch a.Page`：
  五个实页 + `default: ui.Column(...).Padding(t.Space(4)).Center()` →
  `ui.Text(c, copy.PagePlaceholder)`。
- `internal/settingsui/settingsui.go:1062` `func (a *App) page(i int) page`：`default: return
  page{c.PageGeneral, c.PageGeneralHint}` —— 标题的夹取是**第一页**。
- `internal/settingsui/settingsui.go:79` `func PageName(page int) string`：
  `if page < 0 || page >= pageCount { return pageNames[PageGeneral] }` —— 存进
  `last_settings_page` 的 id 也是第一页。
- `internal/i18n/copy.go:291`（EN）/ `:1137`（ZH）：`PagePlaceholder` 的两条文案。
- `internal/i18n/i18n.go:312-314`：字段声明。**全仓只有 `settingsui.go` 那一条臂读它**
  （`grep -rn PagePlaceholder internal/` 只有 4 个命中：声明 1、两条文案、那一处读）。

为什么这一页永远走不到：`pageCount` = 5，五个常量全被 `body()` 的 `switch` 覆盖，
`a.Page` 只由 `selectPage(PageName(page))` 写入，而 `PageName` 与 `PageByName` 都把不认识的
id 夹回第一页。

## 2. 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `settingsui.go` `body()` | `switch a.Page`，`default` 画 `PagePlaceholder` | `switch bodyPage(a.Page)`，无 `default`；`bodyPage` 越界即 `PageGeneral` |
| `settingsui.go`（新函数，紧邻 `page()`） | 无 | `func bodyPage(page int) int`：`page < 0 \|\| page >= pageCount` → `PageGeneral`，否则恒等 |
| `i18n/i18n.go` | `PagePlaceholder string` + 注释 | 删字段与注释 |
| `i18n/copy.go` | EN / ZH 两条 | 两条都删 |

## 3. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、命令接线、`pageCount` / `pageNames` / `PageName` /
`PageByName` / `a.page()`：**一个字节不动**。本轮只改「正文画哪一页」的兜底与一条死文案。
五个实页的渲染代码、它们的 `switch` 顺序、`ui.Column` 的层级全部不动。

## 4. 门槛命令与 PASS 基线（本轮实测的改前值）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3220
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 122
测试函数数                                              # 2378
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

## 5. mutation 计划（≥2 条）

- **M1**：删掉 `bodyPage` 的夹取（`return page` 直接返回）→ `pageNames[-1]` 越界，
  `TestAnUnknownPageDrawsTheSamePageItsTitleNames` 必须红（panic `index out of range [-1]`）。
- **M2**：把「此页面将在后续迭代中到来。」塞回一条活着的文案（`SaveFailed`）→
  `TestNoPageSaysItIsNotBuiltYet` 必须红。

两条都实跑、看到红、`md5sum -c` 逐字节还原、`touch` 后复跑一次。
