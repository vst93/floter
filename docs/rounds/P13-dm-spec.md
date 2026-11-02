# P13-dm · 工具表单的「工具链那一行」：一句话被塞进了设置项的控制槽

**一句话意图**：本地工具编辑器里「这台机器有没有这个语言的工具链」原本是一句**句子**，
却被画成「Language」这一行的第二个副本标签 + 右对齐的控制槽；把它放回它解释的那个选择器下面，
整列宽、按句折行 —— 最长的那句判词此前**被省略号吃掉 11px**（尾巴两个字 + 句号）。

**为什么现在做**：P13-dl 的「仍待」第 1 条点了两块没量过的面：扩展页空态（**需拍板**，
不动）与工具编辑器 `scriptRuntimeLine` 的运行时行（不需要拍板）。本轮取后者。

## 证据（改之前实测，720×1600，设计主题，深浅同值）

`internal/settingsui/settingsui.go:5594-5601`（改前）：

```go
a.row(c, copy.LocalToolLanguage, "", func() {
    ui.Text(c, a.scriptRuntimeLine(*language)).FontSize(a.tokens(c).Caption).
        TextColor(c.Theme().TextMuted)
})
```

量到的三件事：

1. **列错位**：句子挂在 `row` 的控制槽里，右对齐贴卡片右沿。`ready` 判词
   `node v20.11.0` 的盒子 **x=628.8 / w=61.0 / y=406.6**（深）、x 同（浅）；
   它解释的那个选择器的提示在 **x=201.8 / w=256**。两者相距 **470px**，
   而卡片文字列的右沿在 457.8 —— 这句话落在文字列**之外**。
2. **被切**：`missing` 判词
   `Not found on PATH: node. Install it, then reopen this form to check again.`
   的自然墨宽 **340**（同尺子、无约束实测），而控制槽只给 **331**（x=358.8 / w=331.0）。
   墨到 687 列停、盒子右沿 689.8 —— 尾端是框架的省略号，`again.` 两个字母 + 句号没了。
3. **行高跟着句子走**：三种判词的盒子宽度 61 / 331 / 145，同一行的文字列被反手压窄；
   并且这一行自己占一条 44u 地板的设置行，标签把 "Language" 念了第二遍
   （`tt.Texts()` 里 "Language" 出现三次：行标签、选择器 a11y 标签、以及这一行的标签）。

## 逐条落地项

| # | 位置 | 现状（改前） | 应改成 | 落地 |
|---|------|--------------|--------|------|
| 1 | `settingsui.go:5590`（`choose` 调用） | 选择器一行，运行时行是它后面的一条独立 `row` | 运行时行成为选择器自己的第二行（`below` 列），与静态提示同一列 | `a.choose(..., below ...func())` 加变参；`scriptRows` 把句子作为 `below` 传进去，旧的 `a.row` 整块删除 |
| 2 | `settingsui.go:5712`（新增 `scriptRuntimeNote`） | 无（句子直接塞进 `row` 的 control） | 整列宽的一行散文：`FillWidth` + `Space(2.25)` 上呼吸，caption 档、`TextMuted` | 新增 helper，与 `cardProse`（§6.81）同一形状，只是字阶取 caption（机器事实那一档）而不是 body |
| 3 | 行标签 | "Language" 被念两遍 | 一遍（选择器自己的标签） | 旧的 `a.row(...)` 删除后自动达成 |

**不动**：`scriptRuntimeLine` 的文案与三种判词、探针的缓存/epoch 语义、`row`/`cardRow` 的任何
度量（44u 地板、发丝线、1px 行距）、选择器的宽度与形状。行为契约零改动。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 3315（基线）
GOOS=linux/windows/darwin go build ./...                # 过
```

## mutation 计划

- **M1**：`scriptRuntimeNote` 的行加 `.Justify(ui.End)`（把句子推回右沿）。
  预期：`TestTheToolchainLineSitsUnderItsPicker` 的 6 条 x 断言全红。
- **M2**：把 `scriptRows` 还原成旧的 `a.row(...)`（句子回到控制槽）。
  预期：`TestTheToolchainLineIsNotCutWhenTheVerdictIsLong` 红
  （`the verdict inks 329px in the card but 340px unconstrained`）+ 上面那 6 条 x 断言红。

两条都实跑、看到原始报错、`md5sum` 逐字节还原、`touch` 后复跑。

## 钉（pin）判据

- 句子的左沿 == 它选择器的提示左沿（一格文字原点），墨从盒子第一列起。
- 句子在选择器的提示之下、脚本编辑器之上（它是选择器的第二行，不是一条设置行）。
- 最长判词在卡片里的墨宽 == 同一句话无约束时的墨宽（一个像素都不许被省略号吃掉）。
