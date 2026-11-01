# P13-br · 设置页分组的内 gutter：一页只剩一档，材质组只留 padding

> 规格 `docs/rounds/P13-br-spec.md`｜合约 `docs/liquid-glass.md` §6.36（新）+ §6.35 的 35-3 收口
> 产品改动 1 处（`internal/settingsui/rows.go` 的组框签名 + 材质组的组 gap）

## 一、选题与意图

`docs/liquid-glass.md` §6 里**唯一**的 `未开始` 行是 §6.35 的 35-3，它由 P13-bq 亲手写下：
「分组之间：`sectionGroup` 8u / `sectionMaterial` 10u / `pageColumn` 22u；8 与 10 只差 2 个
物理像素；**下一格：先出图看这两个角色读不读得出来（General 页两张相邻分组卡的间距，
深/浅两套），再决定改不改**」。这一轮就是那个「再决定」，不是新开表面。

**这轮为什么让它更轻、更有空间感**（一句话）：参考实现给材质组写了两个例外
（`gap: 10u` 与 `padding: 10u 0`），量下来小的那个只值 **1.8px**、大的那个值 **9px**——
留大的、删小的之后，页面上的间距角色从「有一个角色两个说法」变成**四个角色四个值**，
而材质组「更松」的读法一点没丢。

## 二、先量：出图之后的读数（改之前）

General 页，`ui.Tester` 720×2000，深/浅两套逐项比对（**两套完全同值**）：

| 组 | 组盒 `y` | 组盒高 | 组头离自己盒顶 | 组头 → 第一张卡（像素扫描） |
|---|---|---|---|---|
| Appearance | 130 | 297 | 0 | **6.7** |
| Window | 447 | 240 | 0 | **6.7** |
| Menu bar | 707 | 170 | 0 | **6.9** |
| Terminal | 897 | 355 | 0 | **6.7** |
| **Material** | 1272 | **385** | **9** | **8.9** |

四个组盒间距（上一组盒底 → 下一组盒顶）：**20 / 20 / 20 / 20**（`pageColumn` 的 22u），
材质组的 padding 在**盒内**，没有把页面节奏撑成第二个值。

单位换算（这轮顺手纠了 P13-bq 的一处换算）：`theme.Space(n) = n × 3.6`（shipped `small`
步长下 1u = 0.9px），所以 **8u = 7.2px、10u = 9.0px**。P13-bq 报告里那句「9u = 36px」是把
u 读成了 4px；9u 实际是 **8.1px**，36px 是**行高**（8.1 × 2 + 18 的字），不是 padding。

**结论**：这 1.8px（7.2 → 9.0，+25%）夹在 20px 的组间距里，没有可读性；而材质组真正的身份是
那 9px 的 padding——它把组头上方的空白从 20px 变成 **29px**。

## 三、落地

| # | 位置 | 现状 → 落成 |
|---|---|---|
| 1 | `internal/settingsui/rows.go` `sectionGroupSpaced(c, title, gap, padding, fn)` | → **`sectionGroupPadded(c, title, padding, fn)`**。gap 从签名里**消失**：`sectionGroup` 与 `sectionMaterial` 都走共享的 `Space(2)`。这不是「约定」，是编译器守卫——上一轮那第二个刻度就是从多出来的那个参数进来的 |
| 2 | `sectionMaterial` 的 `gap` | **重定基线：旧值 `Space(2.5)`（10u / 9.0px）→ 新值共享的 `Space(2)`（8u / 7.2px）**。`padding = Space(2.5)` **保留**；`.settings-cards` 的卡与卡之间 10u **保留**（那是另一个角色，全页同值） |
| 3 | `internal/settingsui/sectionrhythm_test.go`（新，2 条 pin + 1 条 opt-in dump） | ①五组的「组头 → 卡」像素间距必须同值（spread ≤ 1.5px；容差是渲染器的亚像素吸附：7.2 会画成 6.7 或 7.7）；②材质组组头仍内缩 9px、其余四组为 0，且四个组盒间距同值（20px） |
| 4 | `internal/settingsui/materialspacing_test.go`（V 处置） | 两条旧 pin 的断言钉到新值，注释写明 `P13-br 重定基线：旧值 10u → 新值 8u`；padding 10u / 卡与卡 10u 两个半边**零改动** |

改后实测：Material 组头 → 卡 **8.9 → 6.9**（回到 6.7–6.9 那一条带里），组盒高 **385 → 380**
（3 条组级 gutter 各短 1.8px），组头内缩仍是 **9**，组盒间距仍是 **20 / 20 / 20 / 20**。

## 四、V / B / S 台账

- **V 2**：`TestTheMaterialSectionKeepsItsOwnSpacing`（4 条 gutter 里 3 条：组头→卡 / 卡→脚注 /
  脚注→脚注）、`TestTheLiveMaterialSectionUsesItsOwnSpacing`（footnote gutter）。断言钉到新值并
  写明旧值 → 新值，其余半边不动。
- **B 0**：没有一条行为/度量/接线 pin 变红。
- **S 0**：没有结构断言需要改。

## 五、门槛（全部实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（EXIT=0，0 FAIL）
--- PASS 计数                → 3209（基线 3207，+2）
--- SKIP 计数                → 116（基线 115，+1 = 本轮 opt-in 截图 dump）
测试函数数                   → 2365（HEAD 2362，+3）
GOOS=linux|windows|darwin go build ./... → 三平台全过
```

## 六、变异（3 条实跑变红 + `md5sum -c` 逐字节还原）

还原基线：`rows.go` `e2f9a6a0b08ce19fd0c5bfeb8a3bf514`、
`sectionrhythm_test.go` `6b2e0c36586ae6b1f406e649d38e5c56`（`md5sum -c` 三行全「成功」）。

| # | 注入 | 实测失败串（原样） |
|---|---|---|
| M1 | 把材质组的 `gap: 10u` 放回去（`gap := Space(2); if padding > 0 { gap = Space(2.5) }`） | `[light] the groups' internal gap is not one number: map[appearance:6.70001220703125 material:8.89990234375 menubar:6.9000244140625 terminal:7.70001220703125 window:7.70001220703125] — spread 2.2px, want ≤ 1.5px (8u is 7.2px; the renderer snaps it to 6.7 or 7.7)` |
| M2 | 材质组 `padding` 归零（`sectionGroupPadded(c, title, 0, …)`） | `[light] the material group's heading sits 0.0px inside its box, want 9.0px (\`padding: 10u 0\`, settings.css:767-772) — the section's only remaining identity` |
| M3 | 四组都戴 padding（`sectionGroup` 传 `Space(2.5)`） | `[light] appearance's heading sits 9.0px inside its box, want 0: only the material group carries padding (P13-br)` |

**变异卫生**：M1 第一次注入写成了「先 `ui.Column(c)` 再 `ui.Column(c)` 覆盖」，多出来的那个
Column 会被 mygo 挂到当前父节点上，布局凭空高 20px（组盒 `y` 1272 → 1292）——那次红是脏的，
已重写成「先算 gap 再建一个 Column」，重跑后读数干净（8.9 vs 6.7，spread 2.2px）。
M3 有个副产物值得记：它**不**让间隙那条 pin 变红（padding 不改变组内 gutter），说明两条 pin
各自守的是不同的半边。

## 七、出图

`/tmp/floter-P13-br-general-{dark,light}.png`（改后）、`/tmp/floter-P13-br-before-{dark,light}.png`
（M1 注入时同一帧，即改前的布局），720×2000，opt-in（`FLOTER_PNG=1`）。

**这张图上看到的一处变化**：Material 组里「Liquid glass effect」那张卡的**顶边**离组头
从 8.9px 变成 6.9px —— 和上面四组一模一样了；而 Material 标题离上一组盒底仍是 **29px**
（20 的组间距 + 9 的 padding），所以它读起来还是「单独一口气」，只是那口气不再有第二个来源。

逐像素 diff（同尺寸、同帧参数）：dark **44796** 个像素不同，light **45075** 个，
差异盒 `x[187..703] y[473..1646]` —— 从 Terminal 组的中段开始往下整体上移，最多 5px
（材质组 3 条组级 gutter 各短 1.8px）。**这就是「零散改 1.8px」的真实代价：它会把下面所有
东西挪 5px**，所以这种改动只能一次改到位，不能靠肉眼一轮一轮微调。

## 八、用 `qiaomu-design`：偷了什么

按「已有方向，只要打磨」那条车道（`craft-loop.md` + `preflight.md`），**不走** Phase 2 的
四方向预览（自动车道没人点选，且用户已把方向给死：轻便液态玻璃）。

| 从参考的哪一条取的 | 落成哪个具体数值 |
|---|---|
| **`craft-loop.md` 工艺密度菜单第 12 项 · 材质细节**（「材料之间必须有可量的接缝」） | 接缝不再靠描述，靠**像素扫描**：组头文字底 → 卡面顶边的物理像素（6.7 / 6.7 / 6.9 / 6.7 / **8.9**）。扫描列取组宽的 55%（避开每行的标签与控件），底色取组盒上方 3px 处 |
| **`craft-loop.md` 第 6 项 · 编辑细节**（精度类细节：hairline、字距） | 把 1.8px 放进**四个角色**的对照里判：组内 **8u** / 卡与卡 **10u** / 组与组 **22u** / 组的内缩 **10u**。判据是「同一角色有没有两个值」，不是「这个数字顺不顺眼」 |
| **`craft-loop.md` 第 1 项 · 氛围层**（「加东西必须能被量出来，不能被感觉到」） | 本轮删掉的正是**量不出来**的那 1.8px；保留的 9px padding 是**量得出来**的（组头上方 20px → 29px）。删除与保留用的是同一条尺子 |
| **`apple-fluid-interfaces.md` §0 核心判断**（「如果做不到这四点，宁可删掉动效」） | 把它从动效搬到间距：**读不出来的差别不留在界面上**。这一条是本轮动手的授权——不是「我觉得太松」，是「它读不出来」 |

**DNA 注入说明**：本轮没有引入新的供体站点。`craft-loop.md` §一 的 DNA 注入是为「新方向」
准备的，本仓的 DNA 早已钉死在 `docs/raycast-design.md` + `docs/liquid-glass.md`（近黑底 /
发丝线 / elevation 阶梯），这轮偷的是**方法**（可量接缝、角色对照、读不出就删），不是外观。

### preflight 逐条

| 门 | 结论 |
|---|---|
| 第 0 步 · 偏好账本 | 已读 `user-preferences.md` 生效条目。适用：**P-1（禁斜体）** 本轮零排版改动；**P-5（可读性优先）** 本轮不动任何墨/对比度；**P-14（加载/成功状态不占版面）** 未新增常驻文案；**P-15（顶部 bar 融入场景）** 不适用（本轮不碰 bar）。封面 / 预览页 / K3 分支**不适用** |
| A · 反 AI 套路 | **过**：无渐变、无纯黑、无外发光、无左竖条（P-38）、无三等分卡片、无「革命性/无缝/赋能」类文案（本轮零文案）、无斜体 |
| A2 · 进攻三步 | DNA 注入：**不适用并写明理由**（见上）；工艺密度：落地 **4 项**（第 1/6/12 项 + apple §0），本轮是 audit→polish 的收窄轮，凑 5 项就是加噪；两轮制：**第一轮已出图并四视角评审**（见下），第二轮只修 1 处 |
| A2 四视角评审（各挑一处） | 艺术总监：材质组「更松」的气质原先由两个数撑着，现在只剩一个，**读起来没变松也没变紧**——这正是要的；工程师：`sectionGroupSpaced` 的 gap 参数是这族漂移的入口，已从签名里删掉；甲方：3 秒内仍能分出五个分组（组盒间距 20px 未变）；读者：材质组脚注与卡之间的距离短了 1.8px，**逐行通读无差别**（8u 与 10u 都是同一档呼吸） |
| B · 设计读取一致性 | **过**：一句设计读取 =「设置页，面向已经在用 floter 的人，用轻便液态玻璃语言，落在 elevation 阶梯与一条间距刻度上」；产出（一档 8u + 保留 padding）与它一致；**记忆点** = 「一页只有一档组内 gutter」 |
| C · 动效抽查 | **不适用**：本轮零动效改动（`ColorStep` 120ms、按下面档 `Scale(0.986)` 一个字节没动） |
| D2 · 中文页 | **中文行高 ≥1.5：过**（未动）；**中西文空格：过**（零文案）；**字重 400/500/600：过**（`sectionHeading` 的 620 是既有值，本轮未动）；**「正文 ≥14px」：不过 —— 既有挂账**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2 的张力仍在 `.pending-bugs.md` 的「需用户拍板」里，抬它会动高度链（P13-bd 已实证），本轮**继续挂账，不擅自改** |

## 九、仍待（P13-br）

1. **§6 差距表已经没有 `未开始` 行**（35-3 是最后一条）。选题回通用队列。下一个真候选：
   **Terminal 组的组头 → 首行是 22px，而 Window / Menu bar 是 16px**（本轮顺手量到的）——
   差 6px，来源是首行控件类型不同（`choose` vs `checkbox`）。**先量再改**，别先动。
2. **需拍板（沿用）**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾（P13-bm 仍待 1）；终端弧 S1b / S1c / S2 尾；§6.32 的 32-4。
3. **P13-bo 仍待 1 原样挂着**：把「列表从属注 / 这一页结论」的分界写成一条共享判据，
   要同时改 `shell/detach.go` 与 `launcher`，跨表面，**另开一轮**。
4. **抢救 stash 复核（本轮做了）**：`stash@{0}` = `33e03d1f6a`（2026-11-01 11:58）的补丁是
   **空补丁**（`git stash show -p stash@{0} | md5sum` = `d41d8cd9…`，`git apply --check` 报
   「输入中没有合法补丁」）——那次抢救没有带来任何未提交工作，**无需接续**，也不会静默丢弃。
