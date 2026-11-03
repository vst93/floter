# P13-fk 报告：控制槽 `Shrink(0)` —— 它从不吃标签列，但也没人守着

**意图**：`.pending-bugs.md` 挂了十几轮的「`cardRow` 控制槽 `Shrink(0)`」（P13-fj 的
「下一格（首选）」）。这一轮把它量到底：**它不会吃标签列，也不会把控件顶出卡框**，
但「控件是固有宽度、标签列是弹性」这句话此前没有任何东西守着。落地是
**零像素 + 一条 pin**（三条断言）。

## 1. 先量（探针跑完即删，数字抄在这里）

真设置页（720×900，五页，深浅两套**逐值相同**；行内容盒 **515.0**，行内容右沿 **691.2**）：

| 页面 | 最宽控制槽 | 剩下给标签列 | 槽右沿 − 行内容右沿 |
|---|---|---|---|
| General | **220.0**（字体族 `ui.Select`） | **260.0** | **0.0** |
| General | 200.0（数字档） | 280.0 | 0.0 |
| Shortcuts | 81.0 | 399.0 | 0.0 |
| Integrations | 88.0 | 400.0 | 0.0 |
| About | 67.8 | 412.2 | 0.0 |

三句话：

1. **最宽的一枚控件占掉内容盒的 220/515 = 42.7%**，标签列还剩 **260u（= 72px）**，
   是那一行标签墨（`Font family` 66 列）的 **3.9 倍**。所以「标签列被吃光」不可达。
2. **`over`（槽右沿 − 行内容右沿）在每一行、每一页、两套外观上都是 0.0**：槽永远贴着
   内容右沿站，不越出去，也就不需要 `Shrink(1)` 去救。
3. **控件不随窗宽变**：字体族那枚 picker 的墨跨在 720 是 **229** 列（x=461..690）、
   在 1600 还是 **229** 列（x=1341..1570）；变的是标签列——**260 → 1140**，
   正好是窗口长出来的 **880**。这就是 `Shrink(0)` + `Grow(1)` 那一对的分工，第一次被量出来。

顺带量到的两条，写下来给下一轮：

- **picker 的墨离内容右沿固定差 1.2px**（两个宽度都是 1.2）：那是它自己 1px 面的内缩 +
  抗锯齿，不是没对齐。所以右沿断言用 ±2。
- **`Width(220)` 是字面 px，不是 `u`**：`chooseSelect` 的注释写着「设置页保留框架的 `--u`
  度量、跟着缩放」，但这一句 `Width(220)` 不会随界面尺寸走。**本轮不动**（改它会改
  非默认缩放下 picker 的宽度 = 需先量），记进 §6.132 的 132-5。

## 2. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/controlslot_test.go` | `TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack`：深浅两套，四条断言——① picker 墨跨 720 == 1600（±1）；② 右沿贴内容右沿（±2，两个宽度都验）；③ 1600 时控件右移 **880**（±2）；④ 标签列 260u、标签墨不被切（720 与 1600 同宽） |
| 2 | `docs/liquid-glass.md` | 新增 **§6.132**（132-1…132-5 台账） |

产品代码**零像素改动**：`git diff --stat` 里只有 `internal/settingsui/controlslot_test.go`
与 `docs/`（spec / report / `liquid-glass.md`）。

## 3. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改；产品零改动。

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿（0 FAIL）
--- PASS                                    # 3411（基线 3410，+1）
--- SKIP                                    # 192（基线 192，±0）
测试函数（静态计数）                          # 2581（基线 2580，+1）
GOOS=linux/windows/darwin go build ./...     # 全过
```

### mutation（实跑，原始报错抄在下面）

**M1** — `rows.go:1620` 控制槽 `Shrink(0)` → `Grow(1)`：

```
--- FAIL: TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack (0.34s)
    controlslot_test.go:113: dark=false: the control starts 450 further right at 1600, want the 880px the window grew by (the label column, not the control, takes the slack)
    controlslot_test.go:129: dark=false width=1600: the picker's last painted column is x=1110, 461.2 off the row's content edge 1571.2: the control column is not right-aligned
    controlslot_test.go:135: dark=false: the picker paints 249 columns at 720 but 219 at 1600: the control is absorbing the window's extra width instead of the label column
    ... dark=true 同样三条，共 6 处
```

**M2** — `rows.go:1616` 标签列 `Grow(1)` → `Grow(0)`：

```
--- FAIL: TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack (0.19s)
    controlslot_test.go:102: dark=false: the control right of "Font family" paints nothing
```

两条都 `git checkout` 还原，`md5sum -c` 逐字节核对（`rows.go: 成功`、`settingsui.go: 成功`、
`controlslot_test.go: 成功`），`touch internal/settingsui/controlslot_test.go` 后复跑全绿。

**另记两条没红的变异**（也是这一轮的信息量，不是失败）：
`chooseSelect` 的 `Width(220)` → `260`（picker 变宽但**墨跨不变**：那是内容不是盒子），
以及控制槽 `Gap(Space(2))` → `Gap(0)`（标签墨离列尾还有 194u，这一档根本够不着）。
两条都证明「这一族的问题不在这些格子上」，于是没有为它们写断言——写了就是空断言。

## 4. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP11COSettingsFrames` →
`/tmp/floter-P11-co-{general,integrations}-{light,dark}-after.png`（复用既有的设置页 dump：
本轮零像素改动，图是**证据**不是 before/after）。
**这张图上看到的一处变化**：General 页右列那排开关/胶囊的右沿落在同一条竖线上（691），
而最宽的那枚 picker 的右沿也在同一条线上——宽度差全被左边的标签列吃掉，右侧不留一条锯齿。

## 5. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **2** 项 布局密度（栅格 / 一列一基线） | 控制槽右沿 = 行内容右沿 = **691.2**，五页全等；picker 的墨固定离它 **1.2px**（自己的面内缩），于是「右列」是一条线而不是一条带 |
| 第 **11** 项 响应式细节（谁让位） | 只有**一个**东西是弹性的：标签列（260 → 1140，正好 880）；控件是固有宽度（229 列两个宽度逐列相同） |
| 第 **9** 项 边界与截断 | 最宽控件吃掉 42.7% 后仍留 3.9 倍标签宽度；断言按「墨跨」量而不是按盒子量，盒子会骗人 |
| 第 **6** 项 编辑细节（编号 / 数字表） | 五页 × 最宽槽 × 剩余列写成一张**编号表**（§6.132 的 132-1…132-5），每行一个实测数 |

只取 4 项：这一轮是量尺轮，凑第 5 项就得往产品里加东西，而 §0 的减法禁令优先。

## 6. preflight 逐条（A / B / C / D2）

- **A 反套路**：不产生像素。无渐变、无发光、无纯黑、无斜体（`noitalic_test.go` 仍绿）、
  无三等分卡片、无「赋能/革命性」文案（本轮零文案）。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，用轻便液态玻璃的
  语言」；本轮的判据只有一条——**一列一基线，只有标签列让位**。
- **C 动效抽查**：不动效；没有 `transition: all` 可违反。
- **D2 中文页**：不新增一个汉字（i18n 红线）。既有缺口仍是 `tokens.Caption` 9.9px /
  正文 10.8px 对「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨改动，两条不受影响。

## 7. 仍待

1. **下一格（首选）**：`chooseSelect` 的 `Width(220)` 是字面 px，而它的注释说设置页
   「跟着 `--u` 缩放」——**非默认界面尺寸下 picker 不缩放**。这一条要先量（在最大界面尺寸下
   标签列还剩多少、picker 会不会压到标签），所以本轮只记不改。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2。
3. **需拍板（沿用）**：启动器两行结果行 36px 盒装 27 行墨 / `ui/tooltip.go:133` 禁用元素不弹
   提示 / §6.111 / 113-5 / 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
4. **多选箱宽度**（§6.18 尾巴的同族另一半）：它在**插件配置浮层**里，属另一个表面，
   按「一个表面一轮」留给它自己那一轮。
