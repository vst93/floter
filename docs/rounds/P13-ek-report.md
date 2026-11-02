# P13-ek 报告 · 设置页的分组标题比它自己的卡贴得还紧

规格 `docs/rounds/P13-ek-spec.md`，合约新增 **`docs/liquid-glass.md` §6.106**，
并销掉差距表里**最后一条** `未开始`（105-3）。

## 一句话：这轮为什么更轻、更有空间感

分组标题此前是「又一张卡」（6.7px 贴着自己的卡，比卡与卡之间的 9px 还紧），现在它是
**一条自己的空气带**（14.4px）：一眼能看出「这一块从这里开始」，页面的 z 从「一叠同样厚的
东西」变成「几块各有名字的东西」。

## 读了什么（qiaomu-design）

- `SKILL.md` → 路径选「**已有方向，只要打磨**」（`craft-loop.md` + `preflight.md`），
  **不走** Phase 2 的四方向预览、不出 HTML、不等用户点选（用户 2026-10-30 已把方向钉死：
  轻便液态玻璃；这个自动车道没人能点选）。
- `references/user-preferences.md`（偏好账本）——本轮相关的三条：
  P-1（禁斜体）、P-5（可读性优先于气质）、P-14（常规/成功状态不常驻占版面）。
- `references/craft-loop.md`（工艺密度菜单 + 两轮制）、`references/preflight.md`（A/B/C/D2）。
  没有全库通读。

## 偷了什么（craft-loop §二 工艺密度菜单，逐条落到数值）

| 取了第几项 | 菜单原文 | 落成哪个具体数值 |
|---|---|---|
| **#6 编辑细节** | 「章节编号（01/02）、页边注、hairline、**大写字距标签**」 | 分组标题是这一页唯一的「编辑标记」，所以给它**自己的空气带**：标题墨 → 第一张卡 **24px → 32px**（`Padding` 下边 `Space(2)` = +7.2px），与浮层里同一格 §6.105 的 32px 逐值相同 |
| **#12 材质细节**（取它的**反面**） | 「内边框高光、glass 折射边」 | 这一格**不许**用受光边或阴影去「画出」分组（那会把 E1 面档变成 E2 浮档）：分组只由**面差 + 空气**表达，卡仍然是 `Control` 一张面、`LensRim` 一条顶边，一个 token 没动 |
| **两轮制**（craft-loop §三） | 「第一轮 → 渲染 → 评审 → 只修 3-5 处」 | 第一轮就量了墨：`Window` 组标题墨 451..461 → 标签 486..498 = 24px 对行间 21px；第二轮只改**那一格**，复量 32 对 21，且组注那一对仍是 7.2px（没有被顺手抬走） |

> 说明：这一轮是「一个刻度」的轮次，`craft-loop` 的 5 项签名动作里能诚实认领的是 #6；
> 其余（氛围层、滚动条、大字号排版）不属于设置页这种功能型表面，硬凑就是噪音。
> 没做的项就是没做，不写成「已覆盖」。

## preflight 逐条（A / B / C / D2）

| 条目 | 结论 |
|---|---|
| **A 反 AI 套路** | 过。这一轮没有新增颜色、渐变、图标、文案：只改一条内边距。全仓 `font-style: italic` 不适用（自绘，无 CSS）；无紫蓝渐变 / 纯黑 / 外发光（token 表里没有这些值）。 |
| **A4 功能契约** | 过。分组标题仍然是分组的可访问名（`sectionLabel`），标题 → 卡的关系仍然「标题在上、卡在下」；页面 3 秒内仍能读出五组设置。 |
| **B 设计读取一致性** | 过。声明的方向是「轻便液态玻璃 + 明确 elevation 阶梯」，这一轮改的是**阶梯之间的空气**，不是材质：E0/E1/E2 三档的 token 一个没动，`LensRim`/`LensEdge`/`Shadow*` 全部未触碰。一句话记忆点：**分组标题有自己的空气带**。 |
| **C 动效抽查** | 过（本轮无动效改动）。布局变化不带动画：`Padding` 是静态几何，不引入 `transition: all`、不动 `transform`；页面进入的既有 120ms 颜色步进（`theme.ColorStep`）未改。 |
| **D2 中文页** | **不过**，且是本仓第 60 份证据：`tokens.Caption` **9.9px** / 正文 **10.8px** 对 D2「正文 ≥ 14px」。这一条从 P13-ea 起就挂在 `.pending-bugs.md` 的「需拍板」里——整页字号是设计基线的选择（小字号是启动器类工具的行业惯例），**不是一轮能擅自改的**，需要用户拍板。其余三项（系统字体栈 / 中西文空格 / 全角标点）过。 |
| **E 三秒测试** | 过。眯眼看：五块分组各自的标题现在**离开**自己的卡，视线能按块切分；争夺注意力的元素仍是 0 个（标题不是强调，是标记）。 |

## 四视角偏执评审（各挑一处，共 4 处）

- **艺术总监**：设置页最平庸的地方是**五个分组的标题长得一模一样、又都贴着自己的卡**——
  这一轮修掉了「贴」，剩下的「长得一样」是对的（它们是同一角色的五次出现，不是五个卖点）。
- **工程师**：`section` 的组列有三个孩子共用一个 `Gap`；抬 `Gap` 会连「卡 → 组注」一起抬，
  所以空气加在**标题下边**。已用 M2 证明「加在上面」会被 pin ④ 抓住。
- **甲方**：三秒看完记住的是「这是设置页，分五块」。修前记住的是「一大坨卡片」。
- **读者**：`Window` 组四行的行距仍是 21px（没被挤），标题与第一行之间 32px 不空旷；
  深色档下标题与卡顶边之间没有一条「悬空的线」（受光边仍只在卡自己的顶边）。

## 前后对比图（深 / 浅两套）

`/tmp/floter-P13-ek-sections-{before,after}-{light,dark}.png`（520×380，裁自 `y=120`）。

| | 像素差 | bbox（裁图坐标） | 第一处变化 |
|---|---|---|---|
| light | **44679** | `(187,36)-(519,378)` | 裁图 y=36 = 页面 **y=156**，正是**第一张卡的上边**；标题墨（138..147）一个像素不在框里 |
| dark | **47425** | `(187,36)-(519,378)` | 同上 |

图上看到的一处变化：**第一张卡整体下移 7px，而它上面的分组标题一行不动**——标题与卡之间
因此多出一条空气带（浅色档上这条带比卡与卡之间那条宽 5px，眼睛先读「组」，再读「行」）。

## 台账（V / B / S）

| 类 | 条数 | 明细 |
|---|---|---|
| **V**（视觉数值重定基线） | **2** | `materialspacing_test.go` 的 `heading to first card`：旧值 `Space(2)` → 新值 `Space(4)`；`sectionheading_test.go` 的 `action bottom`：旧参照「行的外框底」→ 新参照「内容盒的底」（行现在有一条 `Space(2)` 的下内边距） |
| **B**（行为/接线） | **0** | 全仓 `go test ./...` 零 B 类红：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线都没动 |
| **S**（结构） | **0** | 树结构没变（同一个 `sectionHeadingInline`，只多一条内边距） |

产品代码：**1 行**（`rows.go:180` 的 `Padding` 多两个参数）+ 注释一段。
测试：新增 pin 1 条 + opt-in 出图 1 条；测试函数 **2513 → 2515**。

## 门槛（全部实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows go vet ./...        → 净
GOOS=darwin  go vet ./...        → 净
go test -count=1 ./...           → 全绿，0 FAIL
--- PASS                         → 3343（基线 3342，+1 = 新 pin）
--- SKIP                         → 170（基线 169，+1 = opt-in 出图）
测试函数                          → 2515（基线 2513，+2）
GOOS=linux/windows/darwin build  → 全过
```

## mutation（3 条实跑，全部变红并逐字节还原）

`md5sum internal/settingsui/rows.go` = `48b753a4def6d38b74123191d1e05f01`，每条都
`cp` 还原 + `md5sum -c` 成功 + `touch` 后复跑一次（避免测试缓存复用）。

1. **M1 退回**（`Padding(0, Space(0.5))`）：
   ```
   [light] appearance's heading sits 6.7px above its first card, want 14.4px
   [light] the heading→card gap is 6.7px and the cards' own gap is 9.0px (a -2.3px difference)
   [light] heading 451..461, first label 486..498, ... (heading air 24, row air 21, card gaps [6.7 6.7 6.9 6.7 6.9])
   ```
2. **M2 加在标题上面**（`Padding(Space(2), Space(0.5), 0, Space(0.5))`）：
   ```
   [light] appearance's heading sits 7.5px above its first card, want 14.4px
   [light] the first group's heading moved to y=137.5, want 130
   [dark]  the first group's heading moved to y=137.5, want 130
   ```
3. **M3 抬半档**（下内边距 `Space(2)` → `Space(3)`）：
   ```
   [light] appearance's heading sits 17.7px above its first card, want 14.4px
   [light] window's heading sits 17.9px above its first card, want 14.4px
   [light] menubar's heading sits 18.3px above its first card, want 14.4px
   ```
   ——这条证明 pin 有**上边界**：不是「越大越松越好」，是一整档。

## 与既有合约的关系

- §6.105（浮层）与 §6.106（设置页）现在是**同一个数**：标题 → 它的第一张卡 = `Space(4)`。
- §4.3 的规矩原样成立：抬的只是「标题 / 卡」这一对；「卡 / 组注」这一对（同一块）仍是 7.2px。
- §2 的 elevation 阶梯一个 token 没动：分组靠**空气 + 面差**分层，没有借受光边或阴影
  （§1 禁令：受光边是「这块面可以被碰」的记号，不是分组的记号）。

## 仍待

1. **设置页的其余四个表面**（Sessions / Shortcuts / Integrations / About）用的是**同一个**
   `sectionHeadingInline`，本轮一次改完（同族同源）；`105-3` 因此一次销账，不留「下轮清空」。
   它们的读数已由新 pin 覆盖（`PageGeneral` 五组），其余四页的 `sectionGroup` 与 General 同一
   函数，不需要各改一次。
2. `tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2 的 14px —— **需拍板**（第 60 份证据）。
3. 终端弧 S1b / S1c / S2 尾 —— 仍在等拍板，不擅自改会话管理语义。
