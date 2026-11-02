# P13-em 报告：失败那一次，同一张卡上有两处告警

表面：**扩展页的配置工具栏（失败档）**。合约新增 `docs/liquid-glass.md` **§6.108**。
规格 `docs/rounds/P13-em-spec.md`。

## 意图

**告警只有一处。** 保存失败时，`configToolbar` 的状态词和它下面的错误带子都在用 `Warning`
喊同一件事；带子扛着句子的全部信息（图标 + keyline + 错误原文），词退回陈述档
（`TextTertiary`）——**词一个字不改**。

## 先量（改前实测，`-run TestTheFailedSaveKeepsOneAlarmOnTheCard` 的红）

```
[light] the failed save's word is painted nearest the warning step (0 levels off),
        want the tertiary step {142 142 148 255 [0 0 0]}
[dark]  the failed save's word is painted nearest the warning step (0 levels off),
        want the tertiary step {110 110 118 255 [0 0 0]}
```

`0 levels off` = 失败帧的词与 `Warning` 的合成值**零级差**：失败与「还没试过保存」在墨上
是同一个面。同一帧里带子那句也是 `Warning`（带子那条断言当时就是绿的），所以这张卡上有
两处同色告警，而能读的那一处（带子）在下面。

## 落地

`internal/settingsui/config.go` `configToolbar`，在 §6.107 的在飞分支之后：

```go
if a.configError[integration.ID] != "" {
    ink = tokens.TextTertiary
}
```

- **词不动**：失败后 `dirty` 仍为真，`ConfigUnsaved` 仍然为真；换一句要新增 i18n 文案
  （`.pending-bugs.md` 的「需拍板」）。
- **几何不动**：带子插在字段与动作行之间，状态词在字段**上面**，它的盒子逐字节不变
  （pin ② 直接断言 `failedBox == dirtyBox`）。

| 档 | 改前 | 改后 |
|---|---|---|
| 干净 | `TextTertiary` | 不动 |
| 脏（未试过） | `Warning` | 不动（它是唯一载体） |
| 在飞 | `TextTertiary` | 不动（§6.107） |
| **失败** | **`Warning`（与脏零级差）** | **`TextTertiary`** + 带子仍是 `Warning` |

## 顺带修掉 HEAD 上的一条红（S 类）

`internal/ci/liquidglass_contract_test.go` 把 §6 里**任何**表头含「状态」列的表当成差距表，
于是 P13-el 写进 §6.107 的那张四行表（干净 / 脏 / 在飞 / 失败）被读成四个非法状态词：

```
a gap-table row carries an unrecognised status "干净"; ...
a gap-table row carries an unrecognised status "在飞"; ...
```

**HEAD 上这条合约测试是红的**（P13-el 那一轮的门槛没抓到它：`go test ./...` 的汇总行被
`grep -v "^ok"` 之外的东西遮住了）。本轮把 §6.107 那张表的首列表头「状态」改成「档」，
合约恢复全绿；并把它写进 §6.108 当规矩：**§6 里讲「档」的表，首列不许叫「状态」**。

## 台账

**V 0 / B 0 / S 1**（S = `docs/liquid-glass.md` §6.107 的表头）。产品代码 8 行（含注释）。

## 门槛（实测）

```
gofmt -l cmd internal        空
go vet ./...                 净
GOOS=windows/darwin go vet   净
go test -count=1 ./...       全绿（含 internal/ci）
--- PASS                     3343（HEAD，含 1 FAIL）→ 3345（0 FAIL）
--- SKIP                     171 → 172（新增的就是那条 opt-in 出图）
测试函数                      2517 → 2519
三平台 go build ./...         过
```

注：HEAD（P13-el）实测是 **PASS 3343 / FAIL 1**——`internal/ci` 那条红，见上一节。
本轮修掉之后才是 3345 / 0 FAIL。

## mutation（3 条实跑，全部变红后逐字节还原）

- **M1 退回**（删掉新那一段，`git checkout` + `touch` 复跑）→
  `[light]/[dark] the failed save's word is painted nearest the warning step (0 levels off),
  want the tertiary step …` 两条红。
- **M2 连带子一起熄灯**（`noticeLine` 的句子改 `TextTertiary`）→
  `[light] the error band is painted nearest the tertiary step (0 levels off),
  want the warning ink {156 31 24 255}` 与深色档 `{220 157 146 255}` 两条红
  ——证明 pin 钉的是「告警搬到哪」，不是「关掉就绿」。
- **M3 一刀切**（无条件 `ink = tokens.TextTertiary`）→ 对照组
  `[light] a dirty form's word is painted nearest the tertiary step (0 levels off),
  want the warning ink` 红。
- 每条都用 `cp` 备份 + `md5sum -c`（`成功`）/ `cmp` 逐字节核对，`touch` 后复跑。

## 出图

`/tmp/floter-P13-em-failure-{before,after}-{light,dark}-{dirty,failed}.png`（530×268，
裁自状态词左上方 10/8px）：

```
compare -metric AE  dirty   light 0      dark 0
compare -metric AE  failed  light 421    dark 418
bbox（failed，浅/深）       83×12+10+11（两张同）
```

**这张图上看到的一处变化**：失败帧里唯一变的是**状态词自己的那 83×12 个像素**——它从
`Warning` 的红降到 `TextTertiary` 的灰；框外（包括下面那条错误带子）一个像素没动，
`dirty` 帧两张图逐字节相同。

## 偷了什么（qiaomu-design）

开工前读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`
（走「已有方向，只要打磨」那条路径，**没有**出四方向预览页、没有等点选）。

- **`craft-loop.md` §二 工艺密度菜单第 4 项「状态四态」（静息 / 悬停 / 按下 / 禁用）→
  落成第五态的判据**：这一格不是控件而是**词**，它的「态」是**谁在扛告警**。落成一句可判的
  规则：`词是唯一载体 → Warning`；`另有载体 → TextTertiary`。四个档位（干净 / 脏 / 在飞 /
  失败）因此只有两档墨，且每一档都能指着「谁在说」。
- **第 6 项「编辑细节」→ 落成一个具体数值**：失败帧的词墨 = `TextTertiary`
  （浅 `142/142/148`、深 `110/110/118`），与 "Saved" / "Saving…" **同一档**——同一张卡上
  「陈述」只有一档灰，不新增第五个灰。
- **`preflight.md` A4.2「错误状态必须可分辨」→ 落地为**：pin ④ 要求带子那句仍是 `Warning`
  （不是「把红都去掉」），所以「失败」在整张卡上仍然可分辨，只是**只有一个地方**红。
- **`user-preferences.md` P-14（状态不得常驻占版面）→ 落地为**：词不新增行、不改字号、
  不改盒子；失败的信息量全部留在带子里，词只留一个灰字。

## preflight 逐节结论

- **A 反套路**：过。没有新增颜色 / 渐变 / 阴影 / 字号 / 图标 / 控件；改动是一格墨。
  无纯黑、无斜体、无三等分卡、无假数据、无空词（词一个字没改）。
- **B 设计读取一致性**：过。一句话记忆点：**「喊一次就够了，喊在能读的那一处」**。
  与 §6.107 同一条判据的正反面；无新增动效。
- **C 动效抽查**：过（不适用面）。本轮零动效；「脏 → 失败」是换墨不是动画，
  `prefers-reduced-motion` 无需新增兜底。
- **D2 中文页**：**不能过**，且不是本轮引入的——状态词与带子都在 `tokens.Caption` 9.9px /
  `Body` 10.8px 那一档，对「正文 ≥ 14px」（**第 62 份证据**）。改它要动字号阶梯，属于
  `.pending-bugs.md` 的**需拍板**项；本轮只换墨不动字号，没有扩大这条债。中英混排空格：
  本轮零新增文案；全角标点：不涉及。
- **E 三秒测试**：过。三秒内读到的顺序变成「哪里出错了（带子）→ 还没存住（灰字）」，
  而不是「两处一样红的字」。

## 仍待

1. **下一格**：`状态反馈` 这一族在本表面已经四档齐全（干净 / 脏 / 在飞 / 失败），转
   **空态满态**——但两条空态（扩展页只有结论没有出路、会话页说错对象）都要**新增文案或
   新控件**，属「需拍板」，不许擅自做。可做的是**满态**：先量「一个列表/一张卡填满
   视口时，最后一行与容器下沿的空气」在启动器 / 设置页 / 浮层三处是否同一个数。
2. **需拍板（沿用 + 第 62 份证据）**：`tokens.Caption` 9.9px 对 preflight D2「正文 ≥ 14px」/
   浮层页头字号上阶梯（`104-2`）/ 启动器两行结果行 36px 盒装 27 行墨 / 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十六次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定
   与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
