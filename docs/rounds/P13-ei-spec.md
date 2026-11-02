# P13-ei · 浮层的名字是整块面上最淡的一行

## 意图

`PluginSheet` 顶部那行插件名（`pluginsheet.go:83`）用的是**全浮层最淡的一条墨**
（`TextMuted`），比它下面每一条字段的 help 行还淡、比它自己带的四个分组标题淡 101 级；
于是用户打开一块配置浮层时，唯一说明「你在改哪个插件」的那一行读起来像脚注。
这一轮只把它穿成设置页页头那一档的**墨与字重**（字号不动，见下），让这块面有名字。

## 为什么现在做

- 上一轮（P13-eh）把「同一角色两个拼法」收口到设置页的分组标题上，并留了下一格：
  「先量两者的墨与盒，再决定是不是同一个角色」。量完的结论是**不是**同一角色
  （浮层的分组标题是设置页**卡标题**那一档：`Title + 620 + TextStrong` + 8u 间距，
  见 `pluginsheet.go:143-153` 的注释与 `rows.go:1240` `rowTitle` 的对齐关系）。
  真正同一角色、两个拼法的是**页的名字**：
  - 设置页页头 `settingsui.go:1572`：`tokens.Display` / 650 / `t.Text`（最响的一条墨）
  - 浮层页头 `pluginsheet.go:83`：`t.FontSize`(body 10.8) / 600 / `t.TextMuted`（最淡的一条墨）
- 实测（`pluginSheetFrame`，设计主题，720×600，浅色档，`scroll.Y = 0`）：

  | 行 | y 带 | 最暗像素 | 用的是 |
  |---|---|---|---|
  | 浮层的名字（`Probe plugin`） | 67..78 | **128** | `TextMuted` `#808087` |
  | 第一个分组标题（`First section`） | 100..109 | **27** | `TextStrong` |
  | 字段标签（`Toggle number 1`） | 135..143 | 27 | `TextStrong` |
  | 字段 help 行 | 144..156 | **57** | 二级墨 |

  也就是说：**名字（128）比它下面每一条 help（57）都淡，比分组标题（27）淡 101 级**，
  而 `pluginsheet.go:75-77` 的注释写着「the hierarchy a reader sees is
  name (body, muted) > section (title) > field (label) > help」——注释说的顺序和像素
  给的顺序是反的。名字排在最后。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/pluginsheet.go:83` | `FontSize(t.FontSize).FontWeight(600).TextColor(t.TextMuted)` | `FontSize(t.FontSize).FontWeight(650).TextColor(t.Text)`（只换墨与字重，字号/行盒不动） |
| `internal/settingsui/pluginsheet.go:75-77` 的注释 | 「name (body, muted) > section (title) …」 | 改写成实测的顺序与理由：名字**靠墨**站在这一页的最前面，字号仍比分组标题低一级（行盒不许动，见下） |

**为什么不动字号**（这是本轮的取舍，写进报告）：名字行是 `Height(t.Space(7))` = 25.2px，
名字的墨从 y=67 起，视口顶到墨正好 **11 行**；§6.72 的顶边高就是 11 行（3u = 10.8px），
余量 **0.2px**。名字一旦长到 `Title`(12.6) / `Display`(14.4)，墨顶会抬到 66 / 65 行内，
`TestThePluginSheetEdgesSitOnBlankAtBothEnds` 会红（那条 pin 的注释已经预告了这个失效模式）。
所以「名字站到阶梯顶端」这一条**不进本轮**，写进 `.pending-bugs.md` 的「需拍板」
（它要动行盒 7u 或顶留白 1u，属于几何，不是墨）。

## 行为契约零改动红线

高度链（`Space(7)` 行盒、`Gap(Space(3))`、scroller 的 `Padding(1,4,3,4)`）、
`PluginScroll.MaxY`、键盘模型、`Change`/`Arm`/`Run` 接线、i18n 文案：**一个字节不动**。
本轮只改一行墨 + 一行字重 + 注释。

## 门槛命令与 PASS 基线（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3336，只许涨
GOOS=linux/windows/darwin go build ./...               # 过
```

## 新 pin

`internal/settingsui/sheetnamerung_test.go`
`TestThePluginSheetNameIsNotTheQuietestLineOnItsOwnFace`（深浅两套）：

1. 名字的对比度必须**不比**它下面的分组标题淡超过 8 级（实测改前 128 对 27 = 101，
   改后同级）。
2. 名字必须**明显重于**它下面每一条字段的 help 行（≥ 40 级，实测改前 128 对 57 = 71，
   改后 ~27 对 57 = 30…… 这条要用**方向**判据而不是绝对差，见报告）。
3. 行盒与 `MaxY` 逐字节不动（墨只换色，几何零改动）。
4. opt-in 出图（`FLOTER_PNG=1`）。

## mutation 计划（≥2，逐条说明让哪个 pin 变红）

- **M1**：名字退回 `TextMuted` → 判据 1 红（101 > 8）。
- **M2**：名字换成 `TextSecondary`（升一档但不够）→ 判据 1 红（57 对 27 = 30 > 8）。
- **M3**：字号顺手换成 `tokens.Title` → 判据 3 红（行盒/MaxY 或顶边 pin 红）。
