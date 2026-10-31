# P13-bm 规格 · 设置行第二行的截断：先量，再决定要不要改

## 意图（一句话）

**把「`rows.go` 第二行漏了 `Ellipsis`」这条看起来像缺陷的代码，量到底**——量下来它是
框架的默认行为，于是这一轮的正确交付不是补一个 `Ellipsis("…")`（零像素的假修复），
而是**把「被切的行必须留下记号」钉成 pin**，并把记录改对。

## 为什么现在做

`docs/rounds/P13-bl-report.md` 与 `internal/launcher/quietclip_test.go` 的开头都写着
「`SingleLine()` 没有 `Ellipsis` 就会被硬切，两端不留记号」。这条判断**是错的**：
`mygo/internal/text/layout.go:81` 的 `Params.Ellipsis` 文档是
`Ellipsis ("…" when empty)`，`:1194-1206` 在 `MaxLines` 用尽时**无条件**补上那个记号；
`Ellipsis(s)` 换的是记号的样子，不是「有没有记号」。同一份错误判断也写在
`internal/settingsui/rows.go:1160-1170` 的注释旁边——那里名字行显式写了
`.Ellipsis("\u2026")`，第二行没写，读起来像漏了。

一条错的「为什么」会一直传下去（P13-bl 的 `故意不做 Wrap()` 结论仍然成立，但理由是错的：
那句话真超宽时会显示 `…`，不是被切）。所以这一轮先归零这条判断，再钉边界。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/rows.go:1178` | `ui.Text(...).SingleLine()`，注释说「ellipsizes」但没有 `Ellipsis` | **不动产品代码**。量的结果是：框架默认给 `…`，`rows.go:1136` 的 `Ellipsis("\u2026")` 只是把默认值又说一遍。 |
| `internal/settingsui/rowlinecut_test.go`（新） | 不存在 | 两条 pin：①第二行被切时尾端必须是低处的记号（不是半个字形），且放得下时**不许**出现记号；②长路径的墨不许跑进尾部 Play 控件，整串仍在元素上（切的是画，不是数据）。 |
| `internal/settingsui/rows.go:1160-1175` 注释 | 说「single-line so a long value ellipsizes」但没说是谁给的记号 | 补一句：记号是框架默认（`mygo/internal/text/layout.go:81`），不是这一行自己写的。 |
| `docs/liquid-glass.md` §6.31 | 不存在 | 记这一轮的账（6 行：4 `已落地` / 1 `需拍板` / 1 `故意不做`）。 |

## 量出来的数（`ui.Tester`，会话页 600u 高，第二行 = 64 个 `M`）

| 窗口宽 | 文字列 | 墨的自然宽 | 尾端 | 结论 |
|---|---|---|---|---|
| 2000u | 放得下 | **511u** | 通高字形 | 自然宽 = 511u（尺子的零点） |
| 860u | 235..767（533u） | 511u | 通高字形（x=745） | 放得下，**没有**记号 |
| 560u | 235..467（233u） | 511u | 低处 2 行高（x=459） | 被切，**有** `…` |
| 460u | 235..367（133u） | 511u | 低处 2 行高（x=363） | 被切，**有** `…` |

深浅两套读数**逐像素同值**（同一组 x/y）。

长路径那条（`/home/vst/workspace/floter/internal/settingsui/some/deeper/...`）：

| 窗口宽 | 墨右端 | 文字列右端 | 尾部 Play 左端 | 余量 |
|---|---|---|---|---|
| 860u | 661 | 767 | 778 | 106u |
| 560u | 462 | 467 | 478 | 5u（`…` 就在这儿） |
| 460u | 364 | 367 | 378 | 3u |

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一个字节不动**。
产品代码唯一的改动是注释。

## 门槛命令 + PASS 基线（开工实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows/darwin go vet ./...  → 净
go test -count=1 ./...           → 0 FAIL，PASS 3190，SKIP 112，函数 2344
GOOS=linux/windows/darwin go build ./... → 过
```

本轮新增 2 个测试函数、0 个 SKIP。

## mutation 计划

- **M1**：`rows.go:1178` 的 `.SingleLine()` → `.Clip()`（记号没了，只剩盒子的硬边）→
  期望 `TestTheRowSecondLineMarksItsCut` 在 560/460 两档红。
- **M2**：`.SingleLine()` → `.Wrap()`（第二行不再单行）→ 期望同一族里至少一条红
  （行被撑成多行，`Find` 的盒子高度变了，墨的两端不再是「一行的两端」）。
