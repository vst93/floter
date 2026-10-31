# P13-ak 报告：审批记录是「规格表」——名字退一档，事实占 emphasis（202/101 → 122/202）

规格：`docs/rounds/P13-ak-spec.md`。合约：`docs/liquid-glass.md` **§6.10**（新）。

## 这轮为什么让它更有空间感

空间感不是加阴影，是**一眼看出这一行谁说了算**。审批记录的四行此前把最响的墨给了
「清单摘要」这种**名字**，把最轻的墨给了 `sha256-aaaa` 这种**事实**（实测 light 202 / 101、
dark 198 / 94 —— 反差是反的）。对调之后：小字是「这是什么」，大字是「它是什么」，
块从「又一段设置列表」读成**一张规格表**；同一块面上有了两级重量，而不是四条平铺的字。

## 先量再改（动手前的数）

英文帧 760×1400，`inkSpan`（文字盒内 luma 跨度 = 面到墨的距离），三行 × 深浅两套：

| 帧 | `Approved` / `Manifest at approval` / `Manifest on disk now` | 它们的值 |
|---|---|---|
| light | **202 / 202 / 202** | 101 / 101 / 101 |
| dark | **198 / 198 / 198** | 94 / 94 / 94 |

## 落地

| 文件 | 改动 |
|---|---|
| `internal/settingsui/rows.go` | 新增 `a.factRow(c, name, value)`：`cardRow` 框架逐字沿用（44px 地板 / 分隔线 / 1px 行距 / 文字列 / 空控制槽），只换墨与字阶——名字 `tokens.Caption` / 500 / `TextSecondary`，值 `t.FontSize+1` / 580 / `TextStrong` |
| `internal/settingsui/settingsui.go` | `approvalDetail` 的四行（含 `record == nil` 的 `未记录` 臂）改调 `factRow`；块标题行不动 |
| `internal/settingsui/factrow_test.go`（新，+2 测试函数） | `inkSpan` + `TestTheApprovalRecordPutsTheFactInTheEmphasisRung`（值比名字重 ≥1 档 / 名字 ≤150 级 / 值在名字下方 / 同一列 × 深浅两套）+ `TestDumpP13AKApprovalRecord`（opt-in 出图） |
| `docs/liquid-glass.md` | **§6.10** 新增 13-1..13-4，全 `已落地` |

**改动后实测**（同一窗口、同一算法）：

| 帧 | 名字 | 值 |
|---|---|---|
| light | 122 | **202** |
| dark | 115 | **198** |

差值：light 122→202（**+80**）、dark 115→198（**+83**），方向与改前相反。

## 这一轮在图上看到的一处变化

`/tmp/floter-P13-ak-approval-{light,dark}-{before,after}.png`，改前帧 = M1 注入时的 dump。
逐像素差（阈值 >6 级）：bbox `(187,483)-(744,815)`，light **24 728 px**、dark **24 163 px**
——变化**只落在审批记录这一块**，且四个值的行都变亮了、四个名字的行都变淡了；
块外一个像素没动（bbox 不外溢），说明只换了墨，没有挪盒子。

## 偷了什么（qiaomu-design / craft-loop.md）

- **工艺密度菜单第 6 项「编辑细节」**：落成**两级字阶的键值对**——键 `tokens.Caption`（浅 122 / 深 115 级），
  值 `t.FontSize+1` 580（浅 202 / 深 198 级），两行间距沿用行自己的 `rowLineGap()` = 1px，
  **没有**另造一个新刻度。
- **第 5 项「多层染色阴影」故意不用**：`liquid-glass.md` §2 判据 2 —— E1 长在底上，压不出影子；
  记录行更不是一块面。
- **第 7/8/9/10/11 项不适用**：自绘控件，无流体字号、无滚动条、无图形资产、无彩蛋位；
  第 1/2/3 项（氛围层 / `::selection` / focus 环）属于 web 页面，本仓是 Go 自绘。
- **未注入 58 站 DESIGN.md 供体**：本轮不是风格注入，是**行内墨级校正**；本仓的单一来源是
  `docs/liquid-glass.md` 的 token 表，从外部品牌偷数值会直接与 §1/§3 的已定基线冲突。
  这条按 craft-loop「DNA 注入 > 密度凑满」的取舍顺序如实降级，不冒充。

## preflight 逐条结论

- **A 反 AI 套路**：无紫蓝渐变 / 无纯黑 / 无外发光 / 无渐变字 / 无斜体 / 无三等分卡片 /
  无左侧竖线（P-38）/ 文案零改动、感叹号 0。→ **过**。
- **A2 进攻三步**：密度第 6 项（见上，有具体数值）；两轮制 = 量（202/101）→ 改 → 再量（122/202）
  → 出图复核（bbox 不外溢）；四视角各挑一处，见下。→ **过**。
- **A4.1 disabled / read-only 不混淆**：这一轮正是修这一条——禁用是「一个不能用的控件」（淡一道，
  P13-s/t/u 已结案），只读是「一个已经确定的答案」（不淡，把重量交给答案）。→ **过**。
- **B 设计读取一致性**：深浅两帧同一套 token，没有混主题；记忆点一句话——「这张块是规格表：
  小字说这是什么，大字说它是什么」。→ **过**。
- **C 动效抽查**：本轮零动效改动（无 `transition: all`、无时长、无 hover 动画；行 hover 面由
  `cardRow` 既有逻辑负责，一个字节没动）。→ **过（不适用项以「未改动」记）**。
- **D2 中文页**：i18n 零改动，中文标签仍走系统字体栈、字重 500/580、全角标点原样。**一处不过**：
  键用的 `tokens.Caption` 为 11px，低于 D2 的「正文 ≥14px」下限——这是全仓既有的 caption 步
  （会话 meta 行、进度读数都用它），改它等于改整条字阶，**本轮不动**，写进「仍待」等拍板。

### 四视角评审（各挑一处）

- **艺术总监**：改前这一块最平庸的是四条同重量的行——它看起来像「设置列表的续页」；
  现在它有一级「值」的重量，与上面的开关行天然分家。
- **工程师**：`mustFind` 找不到「第二个」同文本的框（fixture 两侧摘要相同会撞车），所以本轮
  测试自带 `approvalFixture`（两个摘要不同）；同时发现 `auditFixture` 的两侧摘要相同，
  不适合做「按文本定位」的账，已在测试注释里写明原因。
- **甲方**：三秒内能答出「我什么时候批准的、批准的是哪一份摘要」——答案现在是块里最亮的字。
- **读者**：眯眼看，块内最亮的是 4 行**值**，不再是 4 行抽象的名字。

## 门槛（实测）

- `gofmt -l cmd internal` 空；`go vet ./...`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...` 净。
- `go test -count=1 ./...` 全绿；PASS **3044 → 3045**（HEAD 亲测 3044/91，任务书里的 3043/90 早了一版）；
  SKIP 91 → **92**（+1 = 本轮 opt-in 出图）；测试函数 2247 → **2249**；FAIL 0。
- `GOOS=linux/windows/darwin go build ./...` 全过。
- 台账：**V 1 / B 0 / S 0**（V = 本轮新增的像素 pin；没有旧 pin 被改写）。

## 变异（实跑）

- **M1** `factRow` 退回旧墨（名字 emphasis strong / 值 muted）→ **红**：
  `light: "Approved" resolves 202 levels and its value "2023-11-15 06:13" only 101 — the fact must out-ink the name of the fact by a whole rung (§6.10)`
  与 `light: the record's key "Approved" resolves 202 levels ...`（dark 同）。
- **M2** 两行顺序对调（值在上）→ **红**：
  `light: "2023-11-15 06:13" sits at y=480.69998, not below its name at y=502.69998 — the record reads name-then-value, the same order it is written in`。
- 两条均 `md5sum -c /tmp/p13ak.md5` **逐字节还原成功**，`touch` 后复跑绿。

## 仍待（P13-ak）

1. **需拍板**：`tokens.Caption` = 11px 与 preflight D2 的「正文 ≥14px」冲突（不只记录行，
   会话 meta 行与进度读数也在用）。要动就是动整条字阶，属视觉基线变更，不擅自改。
2. 同族未改：`settingsui.go:5278` 的 `PermissionBoundary` 与 `:5762` 的 `tool.Path` 也是空控制槽的只读行，
   但它们的**值就在标签里**（墨已经是 strong），不需要对调；`updateRow` 的只读臂（`当前版本: vX`）
   的值也内联在标签里——这两处已在 §6.10 记作 `故意不做（值已在 emphasis 档）`。
3. 空记录臂（`未记录`）现在读作「名字小字 + `未记录` 大字」；如果下一轮要给它一点空态语气，
   属「空态满态」队列，不是本轮。
4. 两条抢救 stash（`afc38b24` / `114b7d15`）本轮再次复核：内容已在 HEAD（P13-af 的
   `metaSeparator`、`.pending-bugs.md` 的 P13-ad 小节），**无需再试合**，按纪律没有静默 drop。
