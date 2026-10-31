# P13-bl · 门槛先修红：差距表那一行把 `||` 写成了转义竖线

## 意图

`HEAD`（`c1e86789`，P13-bk）的 `go test ./...` 是**红的**：P13-bk 在
`docs/liquid-glass.md` §6.30 写的 `30-1` 行，把 Go 的 `||` 写成 `\|` 转义，而
`internal/ci/liquidglass_contract_test.go` 的差距表解析器按**裸 `|`** 切格
（`strings.Split(trimmed, "|")`，`liquidglass_contract_test.go:62`），那一行的
「状态」列于是读成 `a.clipboard \`，判为「不认识的状态」。这轮先把这个红修掉，
再把「空态那句话在最窄窗口里不被硬切」这个边界钉成 pin——一句话被切成半截
是「界面坏了」，而这一页的句子是**唯一**没有 `Ellipsis` 的单行句子。

## 证据（实测，不是推断）

```
$ go test -count=1 ./internal/ci/
--- FAIL: TestTheLiquidGlassContractSupersedesTheRaycastOne
    liquidglass_contract_test.go:94: a gap-table row carries an unrecognised status
    "a.clipboard \\"; the four words are 未开始 / 已落地 / 需拍板 / 故意不做
    (line: | 30-1 | `launcher.go:1569` 字段行的 `switch` | `case a.mode != nil \|\| ...`)
    liquidglass_contract_test.go:101: the gap table has 228 rows, 227 of them carry a status
```

同一份 `HEAD` 上 `go test -count=1 ./...` = **1 FAIL**（`internal/ci`）。

## 落地项

1. `docs/liquid-glass.md:1723`（`30-1` 行）— 现状：`\|` 转义竖线，解析器把它当格分隔。
   应改成：那一格不写竖线，用「或」把三个条件说出来（`此前是 a.mode != nil 或
   a.clipboard 或 a.browser`）。**只改文档措辞，不动语义**：这一行仍是 `已落地`。
2. `internal/launcher/quietclip_test.go`（新增）— 现状：空态那一句话
   （`launcher.go` `quietPage` 臂的 `ui.Text(c, message).FontSize(tk.Emphasis)`）
   没有 `Ellipsis`，一旦自然宽度超过带子就被 `SingleLine` 硬切。应改成：**钉住
   今天没被切**这个边界（520u 与 760u 两档，量墨的两端）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
本轮产品侧一行未改（`git diff -- internal/` 只有新增测试文件）。

## 门槛（本轮实测）

```
gofmt -l cmd internal   → 空
go vet ./...            → 净
GOOS=windows go vet ... → 净
GOOS=darwin  go vet ... → 净
go test -count=1 ./...  → 0 FAIL
PASS 3190（HEAD 3189 + 1 新增；HEAD 同时有 1 FAIL）
SKIP 112 · 测试函数 2343 → 2344
三平台 go build ./...   → 过
```

## mutation 计划

- **M1**：把 `30-1` 行改回 `\|` 转义 → `internal/ci` 的
  `TestTheLiquidGlassContractSupersedesTheRaycastOne` 必须红（原报错串见上）。
- **M2**：把空态那句话的字阶 `tk.Emphasis` 乘 3 → 句子比带子宽，`SingleLine`
  硬切，`TestTheQuietPagesSentenceIsNeverCut` 必须红（墨的端点顶到 x=15）。
