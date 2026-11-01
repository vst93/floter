# P13-cd 规格：同一张卡的两个空态，一个是一句陈述，一个是「假装成输出的一行」

## 意图

启动器卡片里有两个页面可以「什么都没有」：结果区（查不到 / 插件模式没内容）与
捕获输出页（命令写了个空）。它们今天说的是**两种话**——一个是一句居中的陈述，
一个是 12px、左对齐、`Text` 墨、还能被选中的正文行，读起来与命令自己打印的一行
字**逐像素同档**。这一轮把这两个空态收成**一个函数**说的一句话。

为什么现在做：这是 P13-cc「仍待」第 1 条（`空态满态` 队列）。上一轮的任务书要求
「先量启动器结果区与扩展页命令列表的空态、长列表滚动收口是不是同一档空气」——
量完的答案是：结果区那一半是（§6.9 / §6.13 / §6.23 已落地），**捕获输出页那一半
不是**，而且它是这个族里唯一一个把空态画在**正文寄存器**上的页面。

## 证据（HEAD = dafdd455，行号已核）

720×488 帧，深浅两套，同一帧内量「墨盒」（|Δluma| ≥ 40 的像素包围盒）：

| 表面 | 盒子（`tt.Find`） | 墨盒 | 读数 |
|---|---|---|---|
| 结果区空态 `No results`（`launcher.go:1385-1395`） | `{X:334 Y:259 W:52 H:18}` | `{X:335 Y:263 W:51 H:10}` | 墨居中在 x=360.5（帧心 360）；`Emphasis` + `TextStrong`，无底板无标记 |
| 捕获输出页空态 `The command wrote nothing`（`output.go:673-677`） | `{X:12 Y:118 W:696 H:22}` | `{X:12 Y:123 W:188 H:16}` | 墨**贴在正文左内边距上**（x=12），落在正文第一行；`FontSize(t.FontSize)`=12 + `t.Text` + `Selectable()` |

同一帧里 `t.Text` 的另一个使用者是**页头那一行命令行**（`output.go:578`，墨盒同档
`darkest=25`）。也就是说：这句话戴着「这一页的名字」那一档墨，坐在输出正文的位置上，
和「命令真的打印了这句话」在界面上无法区分。

## 落地项

1. `internal/launcher/launcher.go`：把结果区空态里那两行（`ui.Text(...).FontSize(tk.Emphasis)
   .TextColor(tk.TextStrong).SingleLine()`）抽成 `App.quietStatement(c, message)`，
   结果区改成调它。**几何、`listEdge` 上边距、`Grow(1)`、`Center()` 一行不动**。
2. `internal/launcher/output.go`：`OutputView` 加 `Empty bool`（只有「这一页没有任何
   输出」那一支置真，见 `output.go:191-193`）。`view.Text` 仍然写成 `copy.OutputEmpty`
   ——`pluginTextRows(view.Text)`（高度链）、`PinText`（复制）与 `outputText()` 全部照旧。
   正文臂加一支：`view.Empty` 时画 `ui.Column(c).Fill().Center()` + `a.quietStatement`，
   **不建 `Scroll`**（一行字没有可滚的东西）。其余正文（真输出、`failureReason`）不变。
3. 为什么用标志而不是比较字符串：一条命令**真的打印** `The command wrote nothing` 时，
   它必须仍然是正文（可选中、可复制）。字符串比较会把那一帧也画成空态。
4. 新 pin `internal/launcher/outputempty_test.go`：
   - **A（像素，深浅两套）**：空态的墨盒**水平居中**（|墨盒心 − 帧心| ≤ 3）、
     墨盒左沿 > 帧宽 /3（今天 12，必须红）、墨盒**不在正文第一行**（墨盒顶 > 帧高 30%），
     且竖直落在「页头条子下沿 → 面板底内边距」这条带的中线上（±12px）。
   - **B（普查，AST）**：`quietStatement` 的调用点**恰好两处**，分别在 `launcher.go`
     与 `output.go` 的函数体里；本包其余非测试文件不许再出现
     `Emphasis` + `TextStrong` 这一对（空态只有一处决定）。
5. 合约：`docs/liquid-glass.md` 新增 **§6.47**（空态族收口：一句话、一处决定）。
   `docs/raycast-design.md` §4.7 保留为行为契约（陈述、无底板、无 accent、无图标）。

## 行为契约零改动红线

高度链（`outputRows`/`pluginTextRows`/`Geometry`）、键盘模型（Esc / Enter / 复制 /
钉住 / 滚轮）、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
`view.Text` 的**内容**一个字节不改，只改它被画成什么样。

## 门槛（PASS 基线用本轮实测）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3232（本轮基线）
三平台 build                    # 过
```

## mutation 计划

- **M1**：`output.go` 的 `view.Empty` 分支退回「照旧走 `Scroll` + 正文墨」（删掉那一支）。
  → pin A 三臂（左沿、居中、竖直）全红，深浅两套共 6 个子测试。
- **M2**：`quietStatement` 里 `FontSize(tk.Emphasis)` → `FontSize(tk.FontSize)`
  （把陈述降回正文寄存器）→ pin B 不红（它查的是路由），但 pin A 的墨盒高度/竖直中线
  会漂；若 A 不红，说明 A 的判据不够硬，就地补一条「墨盒高度 ≥ 12px」再重跑（不许放宽）。
- **M3**：把 `output.go` 那一支改回内联 `ui.Text(...Emphasis/TextStrong...)`（绕过
  `quietStatement`）→ pin B 红（调用点只剩 1 处）。
