# P11-bh 规格 · 文件页的空态说自己那件事，不再借「没有匹配项」

## 意图

`files` 模式（拖入的文件）**还没有任何文件**时，空态显示的是搜索用的
`NoResults`（en "No results" / zh "没有匹配项"）。它读起来像「你查的东西不存在」，而这句
话出现时**什么都没查**。同一族的另外三张页各有自己的一句：剪贴板 `ClipboardEmpty` /
`ClipboardEmptyFilter` / `ClipboardEmptyFavorites`，计算器 `CalculatorEmpty` /
`CalculatorEmptyFavorites`，浏览器 `BrowserDisabled` / `BrowserNoProfile` /
`BrowserEmpty`。这是这一族里唯一借邻居句子的一张页。

为什么现在做：用户 2026-10-28 定的优先档是「界面质感与交互逻辑」；上一轮（P11-bg）的
「仍待」第 1 条全是老拍板项、第 2/3 条是设计决定与侦察结论，合约 §6 全表 `已落地`
（只剩 `⌘K`＝拍板）。这一条属于「一张页两件事共用一个答案」的可见文案缺陷，且不需要拍板。

## 现状（本轮实测行号）

- `internal/launcher/launcher.go:897-899`（`emptyMessage`）：
  ```go
  case a.files:
      return copy.NoResults
  ```
  三张邻居页在同一 switch 里各自返回自己的句子（`:857` 起）。
- `internal/i18n/i18n.go:102-105` 的 `Launcher` 只有 `FilesSection` / `FilesMore` /
  `FilesMoreHint`，没有空态那一句；`internal/i18n/copy.go:144`（en）与 `:991`（zh）是
  对应的两份文案。
- 渲染路径：`launcher.go:1318` 起把 `a.emptyMessage(copy)` 画成空态的唯一一句
  （`FontSize(tk.Emphasis)` + `tk.TextStrong`，§4.7 的安静页）。

## 落地项

1. **新增一句自己的话**（不改任何既有文案）：
   - `i18n.Launcher.FilesEmpty`，en `"No files dropped yet"`、zh `"还没有拖入文件"`。
   - 文档注释写明「这是打开着的页还没有被喂过东西，不是查询失败」，并点名 P11-bh。
2. **两个状态两句话**（`launcher.go` 的 `emptyMessage`）：
   - `len(a.Dropped) == 0` → `copy.FilesEmpty`；
   - 有拖入文件、针没匹配上（`filesItems()` 为空且 `Dropped` 非空）→ 仍是 `copy.NoResults`
     —— 那**是**一次失败的查询，句子不变。
3. **不动渲染**：墨、字阶、位置、居中、`listEdge` 都不碰（视觉轮只改墨与面，本轮连墨都不改）。

## 行为契约零改动红线

- 高度链（`height.go` 的 `Geometry`）、键盘模型、会话管理、设置写回、命令接线：零改动。
- i18n 的**既有**文案一个字节不改；只新增一个键，en/zh 各一句。
- `NoResults` 的其它调用点（普通搜索页）行为不变。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2443（基线 2442）
测试函数 grep -rhn '^func Test' | wc -l             # 1989（基线 1987）
GOOS=linux/windows/darwin go build ./...            # 过
```

## 变异计划（≥2 条，逐条说明会让哪个 pin 变红）

- **M1**：把 `case a.files` 分支改回 `return copy.NoResults` → 新 pin
  `TestTheFilesPageSaysWhyItIsEmpty` 的第一臂红（空页没有那句自己的话，且仍然借了
  `NoResults`）。
- **M2**：把 en 的 `FilesEmpty` 改成 `"No results"`（与 `NoResults` 同字）→ 同一 pin 的第一
  臂红。验证的是「两句真的不同」，不是「键存在」。
- **M3**（可选）：让 `FilesEmpty` 在有拖入文件时也返回（即 `if len(a.Dropped) == 0` 改成
  `if true`）→ 同一 pin 的第二臂红（失败查询不再说 `NoResults`）。
