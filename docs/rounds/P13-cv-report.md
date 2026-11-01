# P13-cv 报告：探针列表的分隔线贴着自己的上家，还替列表收了尾

**这轮为什么让它更有空间感**：分隔线是列表里唯一的「标点」。它贴在文字上（3px）时，
读起来是上一行的下划线，列表就成了「一堆各自带下划线的行」；搬到两行正中间（8/8）之后，
它才真的把两行分开，而列表结尾是空气——一个块从此有「里面」和「外面」两种状态，
而不是被一条条线一路切到底。

## 1. 选题与先量（P13-cu 留的「下一格」）

| 项 | 值 |
|---|---|
| 探针行 | `internal/settingsui/healthfaces.go:146`（改前）`row.InsetShadow(0, -1, 0, 0, tokens.Hairline)` |
| 实测（浅色，Integrations 页，720×1100） | `Checks` 439..454、`up` 462..477、线 **480**、`extras` 489..504、线 **507**、`Failures` 516 |
| 实测（深色） | 同上，逐像素同坐标 |
| 线到上家文字 / 到下家文字 | **3px / 8px**（不是居中，贴着自己那一行） |
| 线条数 | 2 条探针 → **2 条线**：最后一条下面 8px 就是下一组标题（**孤立线**） |
| 线的合成值 | 浅 229 → 208（Δ −63 三通道和 = 7 级）；深 39 → 176（Δ +57 = 19 级） |

**主线的猜测（「首行也顶一道，与上面卡片的底边重复成双线」）不成立**：`InsetShadow` 的 y
偏移在本仓框架里是「往下推」，负偏移把未覆盖的那一条留在盒子的**底边**（`mygo/ui/shadow_test.go`
的 `InsetShadow(0, 6, 0, 0)` 案例正是「正偏移只剩顶部」）。所以首行上方是干净的，问题在另一端：
**每一行都在自己的底边收一道，列表因此被最后一条线收了尾。**

## 2. 改了什么

1. `healthfaces.go`：删掉探针行自己的 `InsetShadow`——行只管内容（11px mark、monospace id、
   trailing duration、4px 竖 padding）。
2. `settingsui.go` `healthDetail` 的 checks 组：逐条画之后，**只要后面还有探针**就补一个
   `ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).Shrink(0)`。这是同页
   `diagnosisCheckRow` 已有的写法（它也是「最后一行不画」）。

## 3. 改后实测

| 项 | 改前 | 改后 |
|---|---|---|
| 2 条探针的分隔线 | 2 条（含收尾） | **1 条** |
| 线到上一行文字 | 3px | **8px** |
| 线到下一行文字 | 8px | **8px** |
| 最后一条探针下面 | 8px 后一条线 | **没有线**（空气 12px 后是下一组标题） |
| 线的墨 | 208 / 176 | **208 / 176**（不变） |
| 3 条探针 | 3 条线 | **2 条线**（`485`、`517`，各 1px） |
| 块内高度 | — | 每两行之间 +5px（4 空气 + 1 线 + 4 空气），下方内容整体下移 5px |

差分（浅色 20680 px / 深色 20678 px，bbox `(191,480)-(701,719)`）：两条整行 509px 的旧线消失、
一条 509px 的新线出现，其余是 5px 位移带来的文字/边框重排。逐行直方图见本轮提交说明。

## 4. 交付的 pin（`internal/settingsui/probelistrule_test.go`）

- `TestTheProbeListSeparatesItsRowsAndDoesNotCloseItself`：N 条探针 → **N−1** 条 1px 线；
  每条线上下空气**相等**；最后一条探针与下一个标题之间**不许有任何线**。2 条与 3 条各跑一遍。
- `TestTheProbeRuleIsTheHairlineNotATextRule`：线的墨就是 `Hairline`（浅 −63、深 +57，
  三通道和、1 级 = 3），深浅各一遍——文字墨会跑出 −605 / +594。
- 辅助：`p13cv_dump_test.go`（opt-in 出图，`FLOTER_P13CV_DUMP=1`）。

## 5. 门槛（实测）

```
gofmt -l cmd internal                   空
go vet ./...                            净
GOOS=windows go vet ./...               净
GOOS=darwin  go vet ./...               净
go test -count=1 ./...                  全绿
--- PASS 3294（基线 3292）  --- FAIL 0   --- SKIP 140（基线 139，新增 opt-in dump）
测试函数 2446（基线 2443；新增 2 pin + 1 dump，命令：grep -rn "^func Test" --include=*_test.go .）
GOOS=linux/windows/darwin go build ./... 过
```

V/B/S 台账：**V 0 / B 0 / S 0**（无既有断言改动；三条新 pin 全是新增；`TestProbeLinesAreTheReferenceCompactRows`
`TestTheHealthGroupsAreLabelledNotRuledRows` `TestStoredHealthReportShowsItsProbes` 零改动通过）。

## 6. 变异（3 条，全部实跑并逐字节还原）

| 变异 | 注入 | 实测红 |
|---|---|---|
| M1 | `if i < len(report.Probes)-1` → `if i >= 0`（每行都补，恢复「收尾线」） | `2 probes draw 2 rules, want 1 (one *between* rows, none closing the list): [485 517]` |
| M2 | `Height(1)` → `Height(2)` | `2 probes draw 2 rules … [485 486]`（一条线长到 2px） |
| M3 | `tokens.Hairline` → `tokens.TextStrong` | `dark=false: the rule is -605 off the face … ; dark=true: … 594 …` |

还原：`md5sum -c /tmp/p13cv.md5`（`settingsui.go` 9afdc3a6…、`healthfaces.go` f3060cec…）逐字节通过，
`touch` 后复跑 `-run TestTheProbe` 绿。

## 7. 图（深浅两套，前后对照）

```
/tmp/floter-P13-cv-healthprobes-light-before.png   /tmp/floter-P13-cv-healthprobes-light-after.png
/tmp/floter-P13-cv-healthprobes-dark-before.png    /tmp/floter-P13-cv-healthprobes-dark-after.png
```

**我在图上看到的一处变化**：`before` 里 `up` 那行字的最后一行像素（y=476）下面只隔 3 行就是
一条横贯整块（509px）的线（y=480），紧贴着字，像这行字的下划线；`extras` 下面 8 行又是一条
（y=507），再往下就是 `Failures` 标题。`after` 里那条线搬到了 y=485——正好在两行之间，
上下各 8 行；而 y=507 那条**整条没有了**，`extras` 之后直接是 12px 空气和下一组标题。
线本身没变粗细、没变深浅（仍是一条 1px 的 208 灰）。

## 8. 没做的事（写进 `.pending-bugs.md`）

- 探针行「首行上方有没有线」保持现状（标签 → 列表没有线），与同页诊断列表一致；
  参考实现的 `border-top` 语义若真要照抄，会把线放到标签下面——那属于「需拍板」，不在本轮。
- `healthDetail` 里「Checks」标签与首条探针之间的 4px 没有动。

## 9. 偷了什么（qiaomu-design / craft-loop.md 工艺密度菜单，逐项落到数值）

本轮开工前读了 `SKILL.md` → `references/user-preferences.md`（生效 46 条里与本轮相关的：
P-38 禁左侧竖线装饰、P-14 不常驻系统自言自语、P-5 可读性优先）→ 本轮表面用的
`references/craft-loop.md`（工艺密度菜单）与 `references/preflight.md`（A/B/C/D2 门）。
走的是「已有方向，只要打磨」那条自动车道，**没有**出四方向 HTML 预览、没有等点选。

| 偷自 | 菜单项 | 落成的具体数值 |
|---|---|---|
| `craft-loop.md` 工艺密度 **第 6 项「编辑细节：hairline」** | hairline 是页面的标点 | 分隔线从「行底边」搬到「两行之间」：**3/8 → 8/8**（4px gap + 1px 线 + 4px gap） |
| 工艺密度 **第 5 项「多层染色阴影」的逆向使用** | 只留该留的那一层 | 删掉每行一枚 inset（2 条探针 → 1 枚），阴影只留在卡/浮层；探针行零阴影 |
| 工艺密度 **第 12 项「材质细节：内边框高光」的逆向使用** | 面内元素不加受光边 | 分隔线只有一道 `Hairline`（浅 **7 级**、深 **19 级**），不加 `LensRim`/`LensEdge`——按本仓合约 §3，受光边与阴影只给「面 / 浮」，列表分隔线属于面内 |
| `craft-loop.md` 第三节「两轮制」 | 看图再修 | 第一轮出图（before，`/tmp/floter-P13-cv-healthprobes-*-before.png`）→ 量到 3/8 与孤立线 → 第二轮只改标点（本轮 diff 仅 2 处：一行删、一处循环） |

## 10. preflight 逐条结论

- **A 反套路扫描**：无渐变/纯黑/外发光/斜体/三等分卡片/假数据（本轮不动文案，只动一条线的位置）；
  无左侧竖线装饰（P-38：本轮的线是**横**分隔线，且不改任何选中态）。**通过**。
- **A2 进攻三步**：DNA 注入见 §9（三条，全部落到数值）；工艺密度 4 条（≥5 项里本轮表面只装得下
  4 类动作，第 5 类「签名交互」在设置页属越界，故意不取）；两轮制见 §9 末行。**通过（密度 4/5，
  缺口理由已写）**。
- **B 设计读取一致性**：读取 = 「设置页集成卡的检查列表，面向配置扩展的人，用克制的近平面语言，
  倾向 light liquid glass 的面差 + 发丝线家族」；产出与之一致：只改线与空气，不加面、不加色。
  记忆点一句话：**这条线现在是两行之间的分界，不是上一行的下划线**。**通过**。
- **C 动效抽查**：本轮零动效改动（无 transition/时长/曲线），列表高度变化随帧重排，无动画。
  **通过（不适用项已说明）**。
- **D2 中文页**：本轮无新增/修改可见文案（i18n 零改动），中西文空格与全角标点不受影响。
  **通过**；但沿用一条**需拍板第 23 份证据**：`tokens.Caption` 11px < D2 的「正文 ≥ 14px」——
  这是全仓最普遍的元数据字阶，改它是一次全局字阶重定基线，必须用户点头。
