# P13-bh 规格 · 全仓最后两处斜体，都压在中文上

轮号 **P13-bh**｜表面：**设置页 · 扩展卡的「最近一次输出」凹槽**｜合约新增 **§6.27**

## 意图（一句话）

`internal/` 与 `cmd/` 里 `.Italic()` 只剩 **2 个调用点**，两处都在 `settingsui.go` 的
`runOutput` 凹槽里，而且两处压的都是**中文**：

| 行 | 文案（zh） | 墨 | 斜体 |
|---|---|---|---|
| `settingsui.go:6899` | `（无输出）` | `t.TextMuted` | ✅ |
| `settingsui.go:6926` | `……输出已截断（上限 64KB）` | `TextTertiary` | ✅ |

中文没有斜体字形。苹方/思源/微软雅黑都只有正体，`font-style: italic` 落到**引擎伪合成**的
斜切（pango `pangoStyleItalic`、DirectWrite `fontStyleItalic`、CoreText
`syntheticItalic`）——11px 的笔画被重采样一次，横向拉伸约 15°，结果就是「糊」。
这是用户偏好账本 **P-1（硬禁令：界面排版禁用斜体，中文尤其）** 在全仓的最后一处违反，
也是 `chinese-typography.md` §三「中文禁斜体（无斜体字形设计，强制倾斜=笔画变形）」的
最后一处违反。

这轮把它清零，并用**墨**（而不是斜切）承担「这不是真输出」这件事。

## 为什么现在做（证据）

- 队列：`docs/liquid-glass.md` §6 全表 `已落地` / `故意不做`，没有 `未开始` 行 →
  按纪律回通用队列。这一条同时落在**细节打磨**（第 1 轴）与**空间感/糊**
  （第 3 轴：「平、糊、旧」的「糊」字正是伪斜体的产物）。
- census（本轮实测）：`grep -rn "\.Italic()" cmd internal --include=*.go | grep -v _test.go`
  → **2 条**，就是上表两行。除此之外全仓没有第三处。
- 参考 CSS 为什么写斜体（`main:src/styles/extensions.css:1259-1267` 与 `:1268-1280`）：
  它用 `font-style: italic` 说「这一行不是程序的输出，是一句旁白」。在中文里这个载体不存在
  ——载体失效，语义就得换一个能读的载体。
- 实测（见下节「先量」）：斜体帧的**伪斜切位移**与**反锯齿边缘占比**都能量出来。

## 先量（本轮开工实测，720×2000 真扩展页，zh）

| 臂 | 文案 | 正体 / 斜体的字形位移（顶 25% 行 → 底 25% 行的墨重心 Δx） | 中间灰（反锯齿）占非地面像素 |
|---|---|---|---|
| 空输出 `（无输出）` | `t.TextMuted` | 待测（下文「实测」填） | 待测 |
| 截断尾 `……输出已截断（上限 64KB）` | `TextTertiary` | 同上 | 同上 |

> 判定：正体 CJK 是方块字，顶行与底行的墨重心**同一列**（Δx ≈ 0，±0.5px 内）；
> 伪斜体按 15° 斜切，11px 行盒上顶/底相差 **≈ 1.5–2px**，且重采样让中间灰的占比上升。

## 逐条落地项（行号本轮实测）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:6899` 空输出占位 | `TextColor(t.TextMuted).Italic()` | 去掉 `.Italic()`；墨仍 `t.TextMuted`（它是这块凹槽唯一的字，不许再淡） |
| 2 | `settingsui.go:6926` 截断尾 | `TextColor(tokens.TextTertiary).Italic()` | 去掉 `.Italic()`；墨仍 `TextTertiary` |
| 3 | 「不是真输出」的载体 | 伪斜体（中文里不存在） | 文案自带的载体：占位被**全角圆括号**包住（`（无输出）`）、尾行以**省略号**开头（`……输出已截断`），两者都不像程序输出；再靠墨档分家（占位 `TextMuted` > 尾行 `TextTertiary`） |

**不动的**：字阶（两处都是 `tokens.Caption`）、行盒（`MaxHeight(45u)` / `LineHeight`）、
凹槽的面与两道滚动边（§6.17）、`runOutputFont`（真输出仍是等宽）、i18n 文案**一个字节不改**
（文案里已经有括号与省略号，正好是这轮要用的载体）。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线——全部不动。
只删两个方法调用，`ui.Element` 的布局与行盒不变（`Italic` 不改行盒，只改字形）。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线：PASS **3157** / FAIL **0** / SKIP **108** / 测试函数 **2323**（HEAD `b33023ad` 实测）。

## mutation 计划

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | 空输出那行把 `.Italic()` 加回去 | `TestNoSurfaceInTheAppSlantsItsChinese`（census + 像素位移两条） |
| M2 | 截断尾把 `.Italic()` 加回去 | 同上（第二个调用点） |
| M3 | 占位墨退回 `TextTertiary`（与尾行同一档） | `TestTheEmptyCaptureStaysAheadOfTheTruncationAside`（两臂墨档差） |
| M4 | 在 census 里加一处新 `.Italic()`（`internal/shell` 里任取一处中文 Text） | `TestNoSurfaceInTheAppSlantsItsChinese` 的 census 臂 |
