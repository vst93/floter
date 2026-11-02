# P13-dl 报告：权限边界那句散文被切在「不」字上

**一轮一句话**：工具编辑器 Advanced 里那句「这些选项不会创建安全沙箱」是这段披露里唯一诚实的
半句，它此前是一行**被截断的副标题**；本轮让它按散文排（同列、整宽、换行），**墨一个字节没换**。

**这轮为什么让它更轻/更有空间感**：那句披露不再穿「一行设置」的架子 —— 44px 地板、一条发丝线、
一行什么都不说的空 emphasis 标签、以及嵌在 `below` 列里多出来的 14px 缩进，一起没了；
它退到卡片里「说明」那一层，与它解释的块同列。整块从 74px 走到 71px，却多画了 17px 正文。

## 先量（改前，`toolFormEdit` 720×2200，Integrations → Edit… → Advanced）

| 面 | 盒子 | 墨（浅 / 深） | 判读 |
|---|---|---|---|
| `Integration ID` 名字 | `x=201.8 y=247.6 w=65 h=15` | 122 / 115 | caption 档 |
| `local.abc` 值 | `x=636.8 y=246.1 w=53 h=18` | 202 / 198 | emphasis 档，行尾、可选中 |
| 权限边界句 | `x=215.8 y=951.3 w=448 h=17` | 101 / 94 | 墨止于 375 列，尾列 **2px** = 省略号 |

## 改后（同一夹具、同一读法）

| 面 | 盒子 | 墨 | 判读 |
|---|---|---|---|
| 权限边界句 | `x=201.8 y=931.3 w=487 h=34` | 101 / 94（不变） | 两行读完，尾列 **5px** = 真字形 |
| id 名字 / 值 | 逐像素不变 | 122/115、202/198 | 本来就对，**故意不改** |

块内高度账：`Permissions & audit` 标题底 → 第一个开关标签顶 **74 → 71**。

## 图（深/浅 + 前/后，共 4 张）

```
/tmp/floter-P13-dl-boundary-light-before.png   /tmp/floter-P13-dl-boundary-light-after.png
/tmp/floter-P13-dl-boundary-dark-before.png    /tmp/floter-P13-dl-boundary-dark-after.png
```

裁切 = 那句话上下各留 60/70px，含块标题与头两个开关。

**这张图上看到的一处变化**：浅色 `before` 里那句话只有**一条**墨带（y 951–968，墨止于 x 591），
带尾是两个低点（2px）；`after` 里同一位置变成**两条**墨带（931–948、948–965），第一条跑到
x 688（整宽），第二条只到 x ~490 —— 也就是 `These controls do not create a sandbox.` 终于在
第二行上被画出来了，而它上面那行 19px 的空白 emphasis 行不见了（块标题到句子的距离从 38 收到 18）。

## 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                     → 全绿（--- FAIL = 0）
--- PASS                                   → 3318（基线 3315）
--- SKIP                                   → 152（基线 151，+1 = 本轮 opt-in dump）
测试函数                                   → 2478（基线 2474，+3 pin +1 dump）
GOOS=linux/windows/darwin go build ./...   → BUILD-OK
```

## V / B / S 台账

- **V（0 条）**：本轮没有重定任何视觉数值。那句话的墨改前改后同为 **101 / 94**，
  `t.FontSize` / 500 / `TextMuted` 与 `sectionHint` 逐字相同，没有新 token、没有新字号。
- **B（0 条改动，全绿）**：`cardRow` 的地板/分隔线/行距、`sectionGroup` 的 `Space(2)`、
  键盘模型、设置写回、`a.toolPermissions` 的表单状态与提交路径一个字节没动。
  既有 3315 条全绿，说明这块的布尔断言（`TestLocalToolAdvancedDisclosure` 等）没有任何一条
  依赖那句话的盒子 —— 它此前根本没人量。
- **S（0 条）**：没有测试需要改断言。三条 pin 是本轮**新增**的（此前这块零像素账）。

## mutation（2 条，实跑变红并还原）

- **M1**：把 `a.cardProse(c, copy.PermissionBoundary)` 还原成
  `a.row(c, "", copy.PermissionBoundary, func() {})`。原始报错串：

  ```
  readonlytools_test.go:98: light: the note starts at x=215.8, the disclosure's name at x=201.8 (Δ14)
  readonlytools_test.go:104: light: the note box is 17 tall — one line, so the sentence is cut
  readonlytools_test.go:107: light: the note's last ink column is 2px tall — that is the ellipsis, not a letter
  readonlytools_test.go:138: light: the disclosure's name to the first switch is 74, want 71 (P13-dl 重定基线：旧值 74)
  ```

  （深浅各 3 条 + 间距各 1 条，共 8 条红。）
- **M2**：给 `cardProse` 的 `ui.Text` 加回 `.SingleLine()`（盒子不缩进、只重新截断）。
  原始报错串：

  ```
  readonlytools_test.go:104: light: the note box is 17 tall — one line, so the sentence is cut
  readonlytools_test.go:107: light: the note's last ink column is 2px tall — that is the ellipsis, not a letter
  reloadtools_test.go:119: light: note box {201.8 931.3 487 17} ink 101 [128 128 135] last glyph 2px
  ```

  （深浅各 2 条 + 间距 pin，共 5 条红。）
- 还原：`md5sum internal/settingsui/rows.go` → `1459440a42745fc27b9ef4dcf960debf`（变异体
  `3fe1e5141e51ed3970d5fa73db6fdb70`），`settingsui.go` 从 `/tmp/settingsui.go.p13dl` 全量还原
  后 `git diff --stat` 只剩本轮的 6 行；随后全量复跑门槛（上面的数字即还原后的）。

## preflight（逐条过）

- **A 反 AI 套路**：本轮没有新增任何颜色/渐变/字重/装饰 —— 唯一的变化是**一段文字不再被截断**、
  以及一行空标签消失。无斜体（P-1）、无 emoji、无假数据（夹具用的是 `local.abc` / `Ripgrep`
  这类真实形状）、文案逐字未改。**过**。
- **B 设计读取一致性**：读取 = 自绘 Go 桌面工具（mygo canvas）、面向写本地集成的人、
  「近黑底 + 一条受光边」的轻便玻璃语言、层级靠**位置与列**而非多画一条线（§2）。
  本轮把一句披露从「与设置行平级」退到「设置行之下的说明」，正是这条读取。**过**。
- **C 动效抽查**：这句话没有动效（换行与否只在展开瞬间由布局决定，`a.toolAdvanced` 的切换
  是纯布局替换，`ReduceMotion` 与本轮无关）；没有新过渡、没有宽高动画。**过**。
- **D2 中文页**：中文文案逐字未改（「本地命令和脚本以当前系统用户身份运行，这些选项不会创建
  安全沙箱。」），中文走系统字体栈、可换行（这一轮恰恰是让中文句子也能整段读完）。
  **中文页的 D2 大字要求仍不过**：`tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」——
  这是**第 37 份证据**，属「需拍板」，本轮不擅自改字阶。
- **E 三秒测试**：这块的三秒读法是「先看到开关是什么，再看到它不保证什么」；本轮让第二句
  终于念得完，记忆点 = 那句「不创建沙箱」。**过**。
- **A2.1 执行者路由**：默认由当前 pi 直接执行，未调用 K3（P-46）。**过**。

## 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落数）

这一轮改的是一段散文的排版，菜单里与「面 / 材质 / 氛围」有关的项不适用；取到的是这三项：

1. **第 6 项「编辑细节（hairline / 大写字距标签 / 页边注）」** → 落成**去掉一条 hairline**：
   那句话原来自己带一条发丝线（`cardRow` 的规则），现在它与块标题之间没有线，只有
   `Space(2.25)` = **9px** 的呼吸；块标题到句子 **38 → 18px**，句子到首个开关 **19 → 19px**
   （不变，仍由开关键自己的规则起头）。
2. **第 5 项「多层染色阴影」的替代物：同一平面上的列对齐**（本仓 §2 的等价机制）→ 落成
   **x=201.8**：句子的左沿与它解释的「Permissions & audit」标题**同列**（旧值 215.8 多缩进 14px）。
3. **第 10 项「收尾彩蛋 / 只在特定时刻出现的细节」** → 落成**整句**：这句披露的后半段
   `These controls do not create a sandbox.` 只在这一块展开后出现，而它此前从未被画出过；
   它现在占第二条墨带（宽 ~290px 对首行的 487px），是所有状态下这一块唯一「读得完」的断言。

不适用并写明理由：第 1 项氛围层（桌面自绘面板，纹理上限 3%，本轮不碰面）、第 2 项
`::selection`（mygo 自绘）、第 3 项品牌 focus 环（不在本块）、第 4 项签名交互（这块的交互
就是展开/收起，未动）、第 7/8 项展示排版与流体字号（一段说明不需要展示级排版）、
第 9 项滚动条、第 11 项图形资产、第 12 项材质细节（字没有面）。

**两轮制**：第一轮 = 换成 `cardProse` 并出前后图 → 看图时发现第二行明显短于第一行
（整盒有墨的列 411 列、盒宽 487），一度怀疑宽度不够想改列宽或手动断行；复核后确认断点落在
`These` 之前、句号在末列、尾列 5px 是真字形 —— 这是英文句子在 487px 上的自然断点，
**因此不改宽度、不手动断行**，把它写进 pin（尾列 ≥4px + 盒高 ≥30）而不是去调宽度。

## 仍待（P13-dl）

1. **下一格（细节打磨 · 只读面清零后的空态/满态）**：扩展页空态「只有一行结论没有出路」
   （`.pending-bugs.md` 的老条目）与工具表单的**运行时行**（`scriptRuntimeLine`，
   `settingsui.go:5590` 一带：一句「这台机器有没有它的工具链」）此前都没量过盒子；先量再改。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口
   （§6.75/§6.76/§6.77），仍未动，在队里。
3. **需拍板（沿用 + 第 37 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第三十九次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
