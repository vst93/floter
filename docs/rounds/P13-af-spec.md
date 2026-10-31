# P13-af 规格：分隔符属于它引入的那条事实

**意图**：集成行 meta 行的 `·` 不再是行里的一个独立子节点，而是**它引入的那条事实自己的
第一个字形**——行折行时，dot 跟着事实一起换行，不会留在上一行末尾。

**为什么现在做**：`docs/liquid-glass.md` §6 无 `未开始` 行，选题走通用队列的「文案密度 /
边界情况」。这一条是 P13-ae「仍待」第 2 条（集成行 meta 行到底允许折还是一行），也是
`stash@{0}`（`afc38b247c`，2026-10-31 10:06 抢救）里的未完成工作——本轮把它接上、补完、
落账。

## 证据（本轮实测，不是旧报告的数字）

- 参考把 dot 写成**下一条 fact 的 `::before`**：`main:src/styles/extensions.css:979-984`
  （`content: "·"; margin-right: 6px; opacity: .7`），状态字另有一条
  `:not(--recommended):not(--broken)` 的规则（`:986-995`），外加 `.extension-status`
  的 `margin-left: 2px`。**dot 永远只可能出现在它引入的东西前面。**
- 本仓此前把 dot 画成行 `Row(...).Wrap()` 的**独立 flex 子节点**（`settingsui.go` 旧
  `integrationMeta` 里的 `sep := "·"` 循环）。460u 宽下 meta 列是 187u，四条事实各占一行，
  于是三行**都以 `·` 结尾**、下一行以裸事实开头。用 tester 实测旧结构下 labels 里是
  `"·" "Extension package" "·" "Managed"` 四个独立节点（变异 M1 的报错串即此形态）。
- 参考 meta 行**允许折行**（`.extension-row__meta { flex-wrap: wrap }`，`extensions.css:
  960-972`），所以本轮不把这一行收成一行——收的是「dot 不许掉队」。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go` `integrationMeta` | `sep` 独立 `ui.Text` 子节点 | 循环调 `a.metaFact(c, tokens, fact, i > 0)`；dot 在事实自己的 `RichText` 节点里 |
| 2 | 新增 `a.metaFact` | — | `ui.RichText(dot span, fact span)`：dot 是 `tokens.TextTertiary.Alpha(0.7)`（参考的 `.7`），事实 `Weight: 500` / `TextTertiary`。一个文本节点 = 折行时不可分 |
| 3 | `stateMark` 签名 | `(c, integration, copy)` | 加 `lead bool`：行里已有事实时状态字也带 `·` |
| 4 | `stateMark` 的两条透明臂 | 裸 `Text(text)` | `RichText(· span, text span).Margin(0,0,0,t.Space(0.5))`——参考 `.extension-status{margin-left:2px}` 即 8u gap 之后再加 2u |
| 5 | `stateMark` 的 Broken 臂 | 胶囊，无 dot | **不加 dot**（参考的两个 `:not()`），只补 `Margin(0,0,0,t.Space(0.5))` |

## 行为契约零改动红线

只改 meta 行这一条**墨与节点结构**：高度链（行仍是 66u 标题行 + below 块）、键盘模型、
会话管理、设置写回、i18n 文案、命令接线**零改动**。折行行为本身保持不变（参考也折）。

## 门槛（动手前实测的基线）

PASS 3026 / FAIL 0 / SKIP 87 / 测试函数 2234；gofmt 空；三平台 vet 净；三平台 build 过。

## mutation 计划（每条说明会让哪个 pin 变红）

1. **M1** dot 退回独立 flex 子节点 → `TestAMetaSeparatorTravelsWithItsFact`（"not one node" +
   "bare `·`"）。
2. **M2** 状态字丢掉前导 dot → `TestTheMetaLineDotsTheStateWordUnlessItIsABoxedChip/running|off`。
3. **M3** 破损胶囊也补 dot → 同测试的 `bareDots` 计数（独立 dot 节点）。
4. **M4** 状态字去掉 `Margin(...,0.5)` → `TestTheStateWordSitsHalfAUnitPastTheRowGap`
   （实测 8.0u，期望 10.0u）。
