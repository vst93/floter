# P13-eh 报告：权限块的分组标题穿回「分组标题」那一档

方向一句话：**同一个角色在同一页里只许有一个拼法**——卡内子分组不许比它自己的卡片标题更响。

规格 `docs/rounds/P13-eh-spec.md`。合约新增 **§6.103**。表面 = **设置页 / 权限块**。

## 落地（1 行产品代码 + 2 条测试）

| # | 位置 | 旧值 → 新值 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:1351`（`permissionTier` 的 `label`） | `Caption + 650 + TextStrong` → **`Caption + 620 + TextSecondary`** |

同页另外五个「分组标题」早就是 `620 + TextSecondary`（`rows.go` 的 `rowTitle` 与
`settingsui.go` 的四处小节标题）；这一处是第六个、也是唯一一个 `650 + TextStrong` 的。
父级是它下面那张卡的标题（`Title 13.5 + 620 + TextStrong`），所以旧值把**子级**的字重
（650）推到**父级**（620）之上，墨又一样高 —— 层级逆序。收口之后父子关系由**字号 + 墨**
两级承担（13.5 > 9.9，Strong > Secondary），字重不再逆序。

## 台账

- **V 1**：`permissionTier` 的字重与墨（视觉数值，无既有 pin 钉它）。
- **S 0**：结构未变。
- **B 0**：行为/度量/接线零改动；`internal/settingsui` 全包测试**零修改**全绿。

## 门槛（本轮实测）

```
gofmt -l cmd internal                              空
go vet ./... / GOOS=windows / GOOS=darwin           净
go test -count=1 ./...                             全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  3336（基线 3334）
GOOS=linux|windows|darwin go build ./...           过
```

测试函数 2506 → **2508**（`TestThePermissionTierLabelWearsTheSectionHeadingInk` +
opt-in `TestDumpP13EHTierHeadings`）；SKIP 166 → **167**（新 dump 的 opt-in 那一格）。

## 变异（3 条实跑变红并还原）

`md5sum` 逐字节核对还原成功，`touch` 后复跑全绿。

- **M1**（改回 `650 / TextStrong`）→ 红三条：
  `light: the tier label's contrast is 220, want the section heading's 136 (±4)` /
  `light: the tier label's contrast is 220 against the old row-label ink's 220: it is still wearing the stronger ink` /
  `light: the tier label's ink mass is 47498, want the section heading's 20952 (±6%: same weight)`。
- **M2**（字重对、墨错：`620 / TextStrong`）→ 红：contrast `214` 对 `136`、
  still-stronger、mass `32924` 对 `20952`。
- **M3**（墨对、字重错：`500 / TextSecondary`）→ **只**红 mass：`24704` 对 `20952`，
  深浅两套各一条。这一条正是 §6.51 判据 2 的复证：**最暗像素量不出字重，只有墨量能。**

## 出图（自己复核过）

`/tmp/floter-P13-eh-tier-{light,dark}-{before,after}.png`（`FLOTER_PNG=1` 的
`TestDumpP13EHTierHeadings`，一帧里先画一行「分组标题参照」，再画待测的那一行）。

**这张图上看到的一处变化**：浅色档那一行的最暗像素 **27（近黑）→ 101（中灰）**，
地面 247；深色档 **237（近白）→ 165**，地面 14。也就是说这一行从「行标签的墨」
退到了「分组标题的墨」，而它上面那行参照（620 / TextSecondary）在两张图上**一个像素没动**。

## 用了 qiaomu-design（`craft-loop.md` + `preflight.md`，「已有方向，只要打磨」那条路）

**偷了什么**（逐条：菜单项 → 落成哪个具体数值）：

1. **工艺密度第 5 项「多层染色阴影 / 面档」的层级前提** → 「谁压谁」是**面与墨的阶梯**，
   不是单一旋钮：父级 `Title 13.5 / 620 / TextStrong`、子级 `Caption 9.9 / 620 / TextSecondary`，
   两级差值分别是 **3.6pt** 与 **一档墨**，字重**不参与**分家（它在这台机器的字体栈上
   连 10 单位的步进都分不出来，见下）。
2. **第 6 项「编辑细节：hairline / 大写字距标签」** → 分组标题是这一页的「小标题语法」，
   它的**拼法**就是那条 hairline 的等价物：`Caption + 620 + TextSecondary` 五个调用点
   必须逐字节同拼法，第六个不许自己发明。
3. **第 12 项「材质细节」的反向用法** → 材质只在需要分层的地方出现；这一处旧值用
   **更重的字面**去表达「这是个分组」，等于拿材质当结构用，收回到墨与字号。
4. **三·两轮制（渲染 → 评审 → 只修 3-5 处）** → 第一轮先把「这一角色到底有几个拼法」
   数清楚（六个），第二轮只改**一处**产品代码 + 补两条测试，没有顺手改字号/间距。

**preflight 逐条结论**：

- **A（反 AI 套路）**：通过 —— 本轮不产界面，没有渐变、glow、描边字；`liquid-glass.md` §0
  的四条硬禁令一条未触。
- **B（设计读取一致性）**：通过 —— 一个仓库一个调色板来源（`internal/theme`），
  本轮把第六个拼法收回前五个用的 token（`TextSecondary` / 620），没有引入字面色值。
- **C（动效抽查）**：不适用 —— 不动动效；新 pin 的 `SetDark` 不带 `Advance`（读的是静态像素）。
- **D2（中文页）**：**未过（沿用，第 57 份证据）** —— `tokens.Caption` 9.9px / 正文 10.8px
  对「正文 ≥ 14px」。本轮**没有**动字号（它是「需拍板」项），所以这条不会因为本轮而变。

## 一条如实记录的负面读数

`internal/settingsui/weightrung_test.go` 把本仓在用的十二个字重
（460/480/500/510/520/540/560/580/600/620/650/700）在同一串字上逐档量了一遍：
这台机器上它们塌成**三张字面**（460..540 / 560..620 / 650..700），行盒一个字节不变。
但**这台机器没有出货的 UI 字体族**（`fc-match Inter` 退到 Noto Sans CJK），所以这张表
量的是**回退族**的分辨率，不是出货族的。它只打印、不断言，正是因为这个数属于运行环境
而不是这一页的设计。**教训**：任何「字重差一档」的视觉账，都不能在没有参考字体的机器上
用像素差来证明。

## 仍待（P13-eh）

1. **下一格（首选）**：回通用队列。满态（§6.48/§6.49/§6.69/§6.70）与禁用/只读面
   （§6.38/§6.63/§6.84）都已落地，空态两条是「需拍板」。建议下一格取
   **「同一角色两个拼法」的下一处**——本轮量到 `pluginsheet.go` 的浮层分组标题是
   `Title + 620`（设置页是 `Caption + 620`），先量两者的墨与盒，再决定是不是同一个角色。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px 对 preflight D2「正文 ≥ 14px」（第 57 份证据）/
   便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十一次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定
   与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
