# P13-bm 报告 · 设置行第二行的截断：量下来记号本来就在

## 这轮为什么让它更轻、更有空间感

一句话：**这一轮没有改墨，它改的是一条错的判据——「这行会被切成半截」是假的，所以没人需要
去补那道边。** 上一轮（P13-bl）用「`SingleLine()` 没有 `Ellipsis` ⇒ 硬切、两端不留记号」
解释了它的 `故意不做`，同一句话也躺在 `rows.go` 第二行的注释旁边。这条判断是**错的**，
而错的判据比缺的功能更贵：它会让人去补一个零像素的 `Ellipsis("…")`（假修复），或者让人不敢
把一行交给 `SingleLine()`。量到底之后，这一族只剩**一个真问题**（路径丢掉的是叶子目录名），
它被显式抬成「需拍板」，而不是继续以「漏了 Ellipsis」的形状挂在队列里。

## 起点：一条看起来像缺陷的代码

```go
// internal/settingsui/rows.go:1178（改前）
e := ui.Text(c, sublabel.Text).FontSize(size).FontWeight(500).
    TextColor(t.TextMuted).SingleLine()
```

同一段注释（`:1160-1164`）写的是「single-line so a long value **ellipsizes** in the text
column instead of growing the row」，而名字那一行（`:1136`）明写着
`.SingleLine().Ellipsis("\u2026")`。第二行没有 `Ellipsis`——看起来就是「一条路径被切在半个
字形上、两端不留记号」。

## 量下来：记号是框架给的

- `mygo/ui/element.go:1322`：`SingleLine()` 只设 `single = true; maxLines = 1`。
- `mygo/internal/text/layout.go:81`：`Params.Ellipsis` 的文档是
  **`Ellipsis ("…" when empty)`**。
- `:1194-1206`：`MaxLines` 用尽时**无条件**补上记号；`Params.Ellipsis` 非空只是换一个记号。

所以 `Ellipsis(s)` 是「记号长什么样」，不是「有没有记号」。第二行不写也有 `…`；
`rows.go:1136` 那个 `Ellipsis("\u2026")` 是把默认值又说了一遍（无害，保留）。

## 实测（`ui.Tester`，会话页 600u 高，第二行 = 64 个 `M`）

夹具用 `M` 的重复串而不是路径：`M` 是每个等宽字体都有、且**通高**的字形，所以「切在半个
`M` 上」与「以低处的省略号收尾」是 100% 与 ~20% 的墨高差别；换成真路径，最后一个字形可能
正好是个逗号，这条尺子就什么也量不出来。

| 窗口宽 | 文字列 | 墨的自然宽 | 墨行 | 尾端 | 结论 |
|---|---|---|---|---|---|
| 2000u | 放得下 | **511u** | 191..198 | 通高（x 末列高 191..198） | 尺子的零点 |
| 860u | 235..767（533u） | 511u | 191..198（h=8） | 通高，x=745 | 放得下，**没有**记号 |
| 560u | 235..467（233u） | 511u | 191..198（h=8） | 低处 197..198，x=459 | 被切，**有** `…` |
| 460u | 235..367（133u） | 511u | 191..198（h=8） | 低处 197..198，x=363 | 被切，**有** `…` |

深/浅两套（`tt.SetDark(true/false)`）读数**同值**：同一组 x/y，一个像素不差。

真实长路径（`/home/vst/workspace/floter/internal/settingsui/some/deeper/directory/than/the/column`）：

| 窗口宽 | 墨右端 | 文字列右端 | 尾部 Play 左端 | 结论 |
|---|---|---|---|---|
| 860u | 661 | 767 | 778 | 放得下，离控件 117u |
| 560u | 462 | 467 | 478 | 被切，`…` 占最后 5u，离控件 16u |
| 460u | 364 | 367 | 378 | 被切，`…` 占最后 3u，离控件 14u |

两档都 `tt.HasText(dir)` 为真：**切的是画，不是数据**（整串仍在元素上）。

## 落地

1. **产品代码不动**（唯一改动是注释）。补 `Ellipsis("…")` 会是零像素的假修复——默认已经是它。
2. `internal/settingsui/rowlinecut_test.go`（新，2 条 pin × 深浅两套）：
   - `TestTheRowSecondLineMarksItsCut`：先在同一条串的 2000u 档现量**自然宽**（511u），
     于是「这一行被切了吗」是量出来的，不是猜的；然后 `被切 ⇒ 尾端必须是低处记号`、
     `放得下 ⇒ 不许有记号`（反向臂：不能把放得下的字形换成省略号）。
   - `TestALongDirectoryLineStaysOutOfTheRowControls`：墨的右端必须留在自己的文字列内、
     且在尾部 Play 控件左侧；整串仍在元素上。
3. `internal/settingsui/rows.go:1160-1175` 注释补 P13-bm 一段：`maxLines` 由 `SingleLine()`
   钉、记号由 `internal/text/layout.go` 给；并写明**别**换成 `Clip()`（变异实测：会折成 3 行）。
4. `docs/liquid-glass.md` **§6.31**（6 行：4 `已落地` / 1 `故意不做` / 1 `需拍板`）。

## V / B / S 台账

- **V 0**（零视觉数值变更：产品侧只动注释，像素逐点不变——同一夹具深浅两套读数同值）
- **B 0**（`go test -count=1 ./...` 全绿，零测试改动）
- **S 0**（无结构变更）

## 门槛（全部实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 0 FAIL
PASS 3190 → 3192（+2）| SKIP 112（不变）| 测试函数 2344 → 2346（+2）
GOOS=linux/windows/darwin go build ./...  → 过
```

## mutation（2 条，实跑变红，逐字节还原）

- **M1**：`rows.go:1178` 的 `.SingleLine()` → `.Clip()`（记号没了，只剩盒子的硬边）：
  ```
  --- FAIL: TestTheRowSecondLineMarksItsCut
      dark=true 560u: box 235..467 (w=233), 192 painted columns, ink rows 191..228 (h=38),
      last column x=455 (natural 511), cut=true marked=false
      rowlinecut_test.go:162: the second line is cut (221 of its 511 natural width) with no
      ellipsis — its last ink reaches y=194 of a line that runs 191..228, so the last thing
      painted is the top half of a glyph
      dark=true 460u: ... ink rows 191..258 (h=68) ...
  ```
  深/浅各两档，共 4 条红。还原后 `md5sum -c` 成功（`0e9aea645287cc3637781f107c8e31cc`），
  `touch` 复跑绿。**顺带量到一件规格外的事**：`Clip()` 不只去掉记号，还让整行折成 3 行
  （墨 191..258），所以它是比「漏了 Ellipsis」严重得多的改动。
- **M2**：`.SingleLine()` → `.Wrap()`（第二行不再单行）：同一串报错逐字相同（`Wrap()` 在
  无空格的长串上等同于 `Clip()` 的横向截断），4 条红。还原后 `md5sum -c` 成功，`touch` 复跑绿。

## 用 skill 的账（`qiaomu-design`）

- 开工读了 `SKILL.md` 的路由表与 `references/user-preferences.md`。本轮适用的两条：
  **P-5**（可读性优先于气质——一行被切时，记号比气质重要）、**P-1**（禁斜体：本轮零排版改动）。
- `craft-loop.md` 的工艺密度菜单：取第 **7** 项「一族的度量要有一个零点」——落成
  `lineSpan()`：自然宽不是常量，是在 2000u 那一档**现量**的 511u，于是「被切/没被切」是
  量出来的，而不是写死的阈值（写死 511 会在换字体/换字号时静默失效）。
- `preflight.md`：**A**（反 AI 套路）本轮无新增视觉元素，过；**B**（设计读取一致性）第二行仍是
  「值」的语法（`TextMuted` + 500 + 单行 + 省略号），与名字行（`TextStrong` + 580）同一套
  层级，过；**C**（动效抽查）无动效改动，N/A；**D2**（中文页正文 ≥ 14px）**不过**——本轮的
  第二行是 `tokens.Caption`（11px，`rows.go:1176` 的 `size = tokens.Caption`），这是沿用六轮的
  「需拍板」项（`tokens.Caption` 与 D2 的张力），本轮**第六份证据**，没有新增判断。

## 仍待

1. **需拍板（新，本轮唯一的真问题）**：路径被切时留下的是**头**（`/home/vst/workspace/floter/inter…`），
   丢掉的是**叶子目录名**——那才是标识这个会话的那一段。默认省略号保头弃尾，而路径的信息在尾。
   要改成中段省略（`/home/vst/…/settingsui`）或尾优先，得动共享行原语 `rowTitle` 并定一条新的
   截断规则（还要想清楚它对「非路径的灰字」是否也成立），不是墨的问题，故不擅自改。
2. **需拍板（沿用，第七份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」的张力
   （本轮第二行就是它）。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；`fieldSettingsButton` 按下档的一对线；终端弧 S1b / S1c / S2 尾；安静页六滤片旁注
   「Ctrl+D to favorite」（要新 i18n）。
4. **记录更正（本轮做的）**：`P13-bl-report.md` 与 `internal/launcher/quietclip_test.go` 开头
   那句「没有 `Ellipsis` 就会被硬切」按本轮实测是错的（默认记号是 `…`）。P13-bl 的
   `故意不做 Wrap()` 结论仍成立（墨确实留在带子的内边距里），但理由应改读成「超宽时会显示
   `…`，不是被切」；本轮没有回改历史报告，只在 §6.31 与本节记下更正。
5. **抢救 stash（本轮复核）**：`stash@{0}`=`afc38b247c`（`metaSeparator`）已在 HEAD；
   `stash@{1}`=`114b7d1585` 只改 `.pending-bugs.md`。两条都无需再试合。
