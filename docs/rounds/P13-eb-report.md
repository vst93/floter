# P13-eb 报告 · 助记提示：画的与量的读同一个寄存器

轮号 `P13-eb`｜表面 = **启动器字段行的助记提示**（`␣ clipboard history`）｜规格
`docs/rounds/P13-eb-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.97**

## 1. 选题与意图

`P13-ea`「仍待」第 2 条 → §6.86 差距表里 `未开始` 的取景族。按「一轮只做一件事」挑了
**`de-5` + `de-11`** 这一对（画的与量的同一个名字），因为它们是那张表里**唯一不碰高度链**
的两行：

- `de-2` / `de-3`（输出页行盒的计费）换阶梯会改**窗口高度**——高度链是红线，需拍板；
- `de-5` / `de-11` 只动字号与夹具取景。

## 2. 先量（动手前）

- 出厂档 `t.FontSize = round1(12 × 0.9)` = **10.8** → 偏移量给 **9.8**；阶梯的
  `tokens.Caption = round1(11 × 0.9)` = **9.9**。两个数差 0.1pt，但**名字不是同一个**：
  下一个改 `t.FontSize` 的人只会改到画的那一半。
- 更要紧的是**尺**：`TestTriggerHintFits` 的 `wide` 夹具没有 `c.SetTheme(...)`，
  `c.Theme()` 是框架 stock（`FontSize` **14**）→ 它量的是 `t.FontSize - 1` = **13**。
  生产画 9.9，**pin 在量一个比真货大 33% 的字**。
- 换尺后那条「被挤」的前提**直接证伪**：240 宽的窗里 `natural 114` 对 `room 135.6`（300 宽），
  红串原文 `the nudge was not squeezed in the test window: 114 <= 135.6`。

## 3. 落地（产品 2 处 + 夹具 3 处 + 出图 1 个）

| 文件 | 改动 |
|---|---|
| `internal/launcher/triggerhint.go:160` | `size := t.FontSize - 1` → `a.tokens(c).Caption`（9.8 → 9.9），注释写 de-5/de-11 同进同退 |
| `internal/launcher/launcher.go:1636` | `.FontSize(t.FontSize-1)` → `.FontSize(tokens.Caption)` |
| `internal/launcher/triggerhint_test.go` | `wide` / `narrow` 两个夹具都换出货尺（`c.SetTheme(...)`）；宽窗 300 → **240**（出货尺下 300 宽根本不挤）；窄窗 120 → **90**（出货尺下 120 宽的 `room` = 45.6，仍高过六格地板）；`box.W != room` → `box.W == floor(room)`（`room` 105.6，画出来 105，P13-du 的 painter 格子） |
| `internal/launcher/triggerhintdump_test.go`（新） | opt-in 出图，`FLOTER_PNG=1` |

## 4. 台账

**V 4 / B 0 / S 1**（V：两处字号断言、一处窗宽、一处盒宽取整；S：两个夹具换了尺）。

- 产品改动 **2 行**（不含注释）。
- 测试函数 **2495 → 2496**（新增出图那一个），PASS **3327 → 3327**，SKIP **160 → 161**。

## 5. 门槛（全部实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows go vet ./...      → 净
GOOS=darwin  go vet ./...      → 净
go test -count=1 ./...         → 全绿（0 FAIL）
--- PASS 3327 | --- FAIL 0 | --- SKIP 161
测试函数 2496
GOOS=linux|windows|darwin go build ./... → 三平台过
```

## 6. 变异（3 条实跑变红并还原）

md5 逐字节核对：`triggerhint.go` `8e3386b2…`、`triggerhint_test.go` `36a830d7…`、
`launcher.go` `b4fbe177…`（三条都 `touch` 后复跑全绿）。

- **M1** `size := a.tokens(c).Caption * 4` → 六格地板失效，红串：
  `the nudge is missing at 240px: ["Launcher results" "Clipboard history" …]`。
- **M2** 拿掉 `wide` 夹具的 `c.SetTheme(...)`（退回库存尺）→ 红串：
  `the nudge's box = 90, want the row's room 104 on the painter's pixel grid (104)`。
- **M3** `triggerHintMinCells` 6 → 3 → 红串：`a box under six cells was kept` 与
  `a nudge shorter than six cells was kept`。
- **未做的一条（如实记）**：`size := Caption - 5` **不红**——量算只决定「显示/隐藏」，
  4.9pt 时六格地板仍然过得去。所以规格里的 M1 落地时改成了 `Caption × 4`（报告即此处）。

## 7. 出图（深浅两套，逐像素复核）

`/tmp/floter-P13-eb-hint-{light,dark}-{before,after}.png`（240×317，出货尺）。

- 逐像素 diff：light **445** 个像素不同、dark **446** 个，全部落在 **(56,20)-(155,31)**
  这个盒里——就是提示那一行字；最大通道差 **118 / 125**。
- **这张图上看到的一处变化**：`␣ clipboard history` 的描边在 after 里整体右移并重新光栅化
  （同一句话在 9.9 上比 9.8 宽约 1px，字重 520 的边缘从「灰糊」变成「一格一格」）。
  行盒、字段、右侧的 `Clear` / `Settings` 两枚键一个像素都没动。

## 8. 这轮为什么让它更轻 / 更有空间感（一句话）

提示是字段行里**唯一一处「自己发明字号」**的字：它读偏移量、不读阶梯，于是同一行里
标题/副标题/提示各说各的尺。现在它与 `tokens.Caption` 对齐——一个字段行只剩两个寄存器
（`Body` 与 `Caption`），层级靠**寄存器数变少**而不是靠微调偏移量说话。

## 9. 偷了什么（qiaomu-design 的工艺密度菜单，逐条落到数值）

- **第 6 项「编辑细节」（hairline / 大写字距标签 / 一致的字阶）** → 落成
  `tokens.Caption` 9.9 这一个**命名寄存器**替换两处 9.8 的偏移量，以及
  `floor(room)` 这一条「盒读 painter 的格子」。
- **第 12 项「材质细节」** → 本轮**不适用**：一个字的字号，面/受光边/投影一个字节没动
  （§6.13 起这一行就是「字」不是「面」）。不硬凑。
- 其余 10 项（氛围层、::selection、focus 环、签名交互、多层染色阴影、展示排版、流体字号、
  自定义滚动条、彩蛋、图形资产）在**一个 240px 宽字段行的一行提示**上没有落点。
- **参考 DNA 注入**：供体是**本仓自己的阶梯**（`base.css` 的 `--text-caption`，10 × 0.9 = 9.9），
  不是外部站点——这一格要的是「同一个名字」，外部 DNA 会引入第三套数。

## 10. preflight 逐条结论

**A. 反套路（AI Tells）**

- 无紫蓝渐变 / 纯黑 / 霓虹外发光 / 渐变文字 —— 过（只改一个字号）。
- 字体不是 Inter/Roboto/系统默认 —— **故意不适用**（自绘桌面应用，字体由 `tokens` 决定）。
- 全页无斜体 —— 过（`noitalic_test.go` 常驻）。
- Hero 居中 / 三等分卡片 / John Doe / 「革命性」式文案 —— 过（无营销页）。
- 无左侧竖线装饰（P-38）—— 过（这一行只有字）。

**B. 设计读取一致性**

- 读取：**启动器的字段行，面向每天用键盘的开发者，用轻便液态玻璃（Raycast 系）语言**。
  VARIANCE 4 / MOTION 2 / DENSITY 6 —— 只动一个字阶，三个拨盘都不动，产出与声明一致。
- 一页一主题 —— 过（light / dark 各出一张图，不混）。
- 记忆点一句话：**字段行只剩两个寄存器**。
- 动效气质一致 —— 过（无新增动效）。

**C. 动效抽查** —— 本轮**没有新增动效**（无 `transition: all`、无 `scale(0)`、没碰
width/height/padding 的动画）。**这条不是「跳过」，是「没有对象」。**

**D2. 中文页追加项（未过，需拍板）**

- 中文行高 ≥ 1.5 / 中西文空格 —— 过（本轮没动中文串）。
- **正文 ≥ 14px —— 仍未过**：`tokens.Caption` 9.9 / 正文 10.8（第 51 份证据，本轮无新增；
  本轮反而**又一次**把字号钉到 9.9，理由见 §2）。
- 暗色正文对比度 ≥ 7:1 —— 本轮没动墨，沿用 §6.x 的墨表结论。

**E. 三秒测试** —— 眯眼看这一帧：字段、一条结果行、一条提示。争夺注意力的元素 ≤ 1（字段里
打进去的字）。**不像 AI 做的**：没有装饰光斑、没有玻璃边，层级只由字阶 + 面差说话。

## 11. 仍待（P13-eb）

1. **下一格（首选）**：§6.86 剩下的取景族 `de-2` / `de-3`（输出页行盒计费）——**先拍板**：
   换阶梯会改窗口高度（高度链红线）。若拍板「画也换成阶梯」，则两行一起改，行盒 22 → 24。
2. `de-6`（结果行标题 `t.FontSize + 2`）与 `de-2` 同源，必须同进同退。
3. `73-5`（`internal/launcher` 四十多条 pin 还在量库存字 14）与 `P13-dg-t`
   （`internal/terminalui` 的夹具取景收口）：不同族，各占一轮。
4. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2「≥ 14px」。
5. 抢救 stash 核对（第五十五次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
