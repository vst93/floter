# P13-db 规格 · 插件配置弹层：那句「这条没写进去」挂在字段那一列

轮号 **P13-db**｜表面：**弹层**（`internal/settingsui/pluginsheet.go` 的 `PluginSheet`）｜
选题：通用队列 `空间感` /「状态反馈」的另一半（`.pending-bugs.md` P13-da「仍待」3 之中唯一
不需要拍板的那一格）｜上一轮 HEAD `f2048493`（P13-da）。

## 1. 意图（一句话）

**同一块浮层上的两条「写被插件拒了」，一条坐在字段那一行里，一条挂在浮层的边框列上——
这一轮把后者收进字段列。** 它让弹层更像一块面：一条句子属于它讲的那批东西，不属于外框。

为什么现在做：`docs/liquid-glass.md` §6 没有 `未开始` 行，选题走通用队列；弹层（toast
之外的那块配置浮层）是这一支里唯一还没被「对齐一列」这条规则量过的面（P13-cw / P13-cx
在健康卡上做的是同一件事）。

## 2. 证据（实测，行号为动笔前读到的真实位置）

`internal/settingsui/pluginsheet.go` `PluginSheet`：

- 写失败线（`if sheet.Failed { … Settings.SaveFailed … }`）此前是
  `FontSize(t.FontSize - 1)`，**没有任何横向缩进**，于是它的墨左沿落在 scroller 自己的
  4u padding 上——和浮层**标题**同一条列，而不是和它下面那些**字段**同一条列。
- 动作自己的失败线（`sheetAction` 里 `field.Failed`）在字段那一**行**里，右侧贴着胶囊。
- 分组标题（`sheetSection`）本来就带着 `Padding(0, t.Space(3.5))`——它的注释写着
  「the group title starts on the column the row labels under it start on」。

量法（可复现）：`renderPluginSheet`（720×900，深浅两套，**装了设计主题**
`c.SetTheme(a.tokens(c).Theme)`），用 `tt.Find` 取盒、再用 `inkLeft` 扫每盒中线的
第一列墨（盒的右端取样地面，同 `inkRows` 的取法）。

| 节点 | 改前墨左沿 | 改后墨左沿 |
|---|---|---|
| 浮层标题 `Clipboard history` | `14.0`（浅）/ `14.0`（深） | 不动 `14.0` |
| 写失败线 `Could not save that change.` | **`14.0` / `14.0`** | **`27.0` / `27.0`** |
| 字段标签 / 分组标题的列 | `30.0` / `29.0` | 不动 |

改前那条失败的句子离浮层的列 **0.0px**、离字段的列 **16.0px（浅）/ 15.0px（深）**；
改后离字段列 **3.0 / 2.0px**（一个字形 left bearing）、离浮层列 13.0px。

## 3. 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `PluginSheet` 的 `sheet.Failed` 臂 | `FontSize(t.FontSize-1)`，无缩进 | `FontSize(tokens.Caption)` + `Padding(0, t.Space(3.5))`（分组标题同一个缩进） |
| 2 | 同上的注释 | 只写「caption step in the warning colour」 | 补一条 P13-db 记录：为什么它挂字段列、标题为什么留在浮层列 |

**量过、不改（写进报告与合约）**：

- `t.FontSize + 2`（分组标题，12.8）对阶梯的 `tokens.Title`（12.6）：在本仓的取景里是
  同一个行盒（`h=18`），改前/改后**逐像素零差分**。真改会让
  `sheetcutedge_test.go` 的 `TestThePluginSheetEdgesAddNoGeometry` 变红——**因为那条
  pin 的取景没有装设计主题**（`pluginSheetFrame` 只拿了 `tokens.Theme.Background`），
  它量的是框架库存的 `t.FontSize`（14），不是本仓的 10.8。取景先修，字号后改。
- 浮层标题的字号（`t.FontSize` = body）：它是这块面的名字，挂在浮层的列上，与设置页
  页头行同为 4u（`settingsui.go` 的页头行）。**故意不做**。

## 4. 行为契约零改动红线

只动一个句子的横向内边距与字号来源。高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 /
命令接线一个字不改；`PluginScroll` 的滚动范围不动（该臂只在 `Failed` 时存在，且不改变
列高：正文仍是单行 caption）。

## 5. 门槛与 PASS 基线（本轮开工前实测）

```
gofmt -l cmd internal                                  → 空
go vet ./... / GOOS=windows / GOOS=darwin              → 净 / 净 / 净
go test -count=1 ./...                                  → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       → 3304
FAIL 0 / SKIP 143 / 顶层测试函数 2450
GOOS=linux/windows/darwin go build ./...                → 过 / 过 / 过
```

## 6. mutation 计划（≥2 条，实跑）

1. **M1**：把这一轮的三处改动退回 HEAD（`git checkout -- internal/settingsui/pluginsheet.go`）
   → 新 pin 的「nearer to the fields' column」臂必须红，报出 `14.0 vs 30.0`。
2. **M2**：给浮层标题行加 `Padding(0, t.Space(3.5))`（让**名字**也跟进字段列）
   → 新 pin 的「标题留在浮层列（`Space(4)` = 14.4）」臂必须红。

两条都要 `md5sum -c` 逐字节还原 + `touch` 复跑。
