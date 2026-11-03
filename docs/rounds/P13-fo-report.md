# P13-fo 报告：同一张卡上两枚 picker，宽度该来自同一条规则

**意图**：§6.133 把字体族 picker 从 `Width(220)` 改成 `MinWidth(220)`（220 是地板），但
Terminal appearance 卡上的另一枚 picker——`paletteRow`——还留着字面 `Width(220)`。同一个盒子
的宽度，一半来自内容、一半来自常数。出厂内容看不出（九个调色板名最宽 80px，都装得下），
所以这一轮落地的是**规则**。

选题来源：P13-fn 报告「仍待」第 1 条 = §6.133 的 **133-4**（`故意不做（表面归属）`）——
它说「那是终端外观卡自己的一轮」。这一轮就是那一轮。

## 1. 先量（720×900，四台阶 × 深浅逐值相同）

出厂最宽调色板名 `High contrast` 的墨对盒内净宽：

| 台阶 | u | 最宽调色板名墨 | 盒内净宽 | 盒子 | 空气 |
|---|---|---|---|---|---|
| tiny 0.8 | 3.2 | 58 | 184.8 | 218 | 69% |
| small 0.9（出厂） | 3.6 | 66 | 180.4 | 218 | 63% |
| default 1.0 | 4.0 | 73 | 176.0 | 216 | 59% |
| large 1.1 | 4.4 | 80 | 171.6 | 220 | 53% |

九个出厂名字都装得下——所以现状**不红**，它是**规则**上的不一致：
`settingsui.go:4737` 写 `Width(220)`，而同一张卡的 `chooseSelect`（`:4718`）写 `MinWidth(220)`。

一个 55 字的标签（`i18n.Settings.Palettes` 是 `paletteRow` 的参数，调用方可以传任意
`Option.Label`）在 `Width(220)` 上：

| 台阶 | 自然墨宽 | 实际画出 | 盒子 |
|---|---|---|---|
| small | 310 | **176** | 351 |
| default | 341 | **172** | 385 |
| large | 374 | **160** | 423 |

被 `SingleLine` 切掉一半以上，而右边的预览块不动、标签列也无处让。

## 2. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `paletteRow` | `Width(220)` → **`MinWidth(220)`**，注释写清与 `chooseSelect` 同源同规则（§6.133 / §6.134） |
| 2 | `internal/settingsui/palettefloor_test.go`（新） | `TestThePalettePickerIsAFloorNotACeiling`：A 九个出厂名 × 四台阶 × 深浅画出同一个盒子；B 长标签画满自然墨宽（±1px AA）且盒子长大 |
| 3 | 同上 | opt-in dump `TestDumpP13FOPaletteFloor`（`FLOTER_PNG=1`） |
| 4 | `docs/liquid-glass.md` | 新增 **§6.134**，并把 §6.133 的 **133-4** 从 `故意不做（表面归属）` 结账成 `已落地（P13-fo）` |

改后实测（长标签下盒面）：

| 台阶 | 改前 | 改后 |
|---|---|---|
| small | 351 | **503** |
| default | 385 | **499** |
| large | 423 | **495** |

深浅两套逐值相同（±2px AA）。出厂九个名字的盒子**一个像素没动**（218 / 218 / 216 / 220）。

## 3. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改。新增 2 个测试函数（1 pin + 1 opt-in dump）。

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿（0 FAIL）
--- PASS                                    # 3413（基线 3412，+1）
--- SKIP                                    # 194（基线 193，+1 = 本轮 opt-in dump）
测试函数（静态计数）                          # 2585（基线 2583，+2）
GOOS=linux/windows/darwin go build ./...     # 全过
```

### mutation（实跑，原始报错抄在下面）

**M1** — `MinWidth(220)` → `Width(220)`（回到本轮之前）：

```
--- FAIL: TestThePalettePickerIsAFloorNotACeiling (4.83s)
    palettefloor_test.go:56: dark=false step=small: the long palette label paints 176 of its 310px natural ink: it is being cut, not the box growing
    palettefloor_test.go:56: dark=false step=default: the long palette label paints 172 of its 341px natural ink: ...
    palettefloor_test.go:56: dark=false step=large: the long palette label paints 160 of its 374px natural ink: ...
    ... dark=true 同样三处，共 6 处红
```

**M2** — `MinWidth(220)` → `MinWidth(120)`（地板调低）：

```
--- FAIL: TestThePalettePickerIsAFloorNotACeiling (4.55s)
    palettefloor_test.go:60: dark=false step=small: the box stayed 351px (floor 503) for a 310px label: 220 is being used as a ceiling
    palettefloor_test.go:60: dark=false step=large: the box stayed 425px (floor 495) for a 374px label: ...
    ... dark=true 同样三处，共 6 处红
```

M2 顺带量到一件事：地板降到 120 之后，长标签的盒子**仍然**长到 503 / 499 / 495——但
**出厂九个名字**的盒子掉到 351 一线（A 段那一条会红）。「右列是一条线」靠的正是这个地板。

两条都 `cp` 还原，`md5sum -c` 逐字节核对（`settingsui.go: 成功`、`palettefloor_test.go: 成功`），
`touch internal/settingsui/palettefloor_test.go` 后复跑全绿。

## 4. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP13FOPaletteFloor` →
`/tmp/floter-P13-fo-palette-{small,large}-{light,dark}-{before,after}.png`（720×240，只画这一行）。
before 是同一份 dump 在 `Width(220)` 上重跑的，不是同一张图改名字。

**这张图上看到的一处变化**：长标签在 before 里被切在盒边（`…sessions` 之后什么都没有），
after 里整句画完、盒子向左长出去 152px（small）/ 72px（large）；右侧预览块（那条三行小终端）
**一个像素没挪**。

## 5. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **6** 项 编辑细节（hairline / 一列一基线） | 九个出厂调色板名 × 四台阶 × 深浅画出**同一个**盒子（218 / 218 / 216 / 220）；盒子由内容定，不由常数定 |
| 第 **8** 项 流体字号（`clamp(min, vw, max)` 的**容器**版） | 盒宽 = `max(220, 内容)`：`MinWidth(220)` 就是 `clamp` 的地板，内容超了就长 |
| 第 **9** 项 边界与截断（收口处的手艺） | 截断点从「盒边」推到「真的放不下」：55 字标签从 176/310 被切 → 画满（±1px） |
| 第 **12** 项 材质细节（盒与面分开量） | 盒 218 对盒内净宽 180.4：差的 37.6px 是两侧内边距 + 值到箭头的空隙 + 箭头。pin 量的是**面**，所以断言不写 220 |

只取 4 项：这一轮是**一块面**的宽度来源，凑第 5 项就得往产品里加东西，而「细节打磨」的减法优先。

## 6. preflight 逐条（A / B / C / D2）

- **A 反套路**：不新增渐变、发光、纯黑、斜体（`noitalic_test.go` 仍绿）、三等分卡片；
  零文案改动（i18n 红线）。唯一的新视觉行为是「长标签不再被省略号切」。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」。
  本轮判据一条——**同一张卡上的两枚 picker 用同一条宽度规则**。此前一枚问内容、一枚问常数；
  现在两枚都是「220 是地板」。
- **C 动效抽查**：不动效；没有 `transition: all` 可违反；picker 的开合仍是框架原样。
- **D2 中文页**：不新增一个汉字（i18n 红线）；`zh` 的调色板名（`跟随应用` / `高对比` / `纸张` /
  `墨黑` / …）比 en 短，量值与 en 的盒子逐值相同。既有缺口仍是 `tokens.Caption` 9.9px /
  正文 10.8px 对「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动，两条不受影响。

## 7. 仍待

1. **下一格（首选）**：`palettePreview` 自己的宽度来源——`c.Theme().Space(37)` 是**字面台阶**
   （tiny 118.4 / small 133.2 / default 148.0 / large 162.8），而它装的三行墨是固定的
   （`floter --version` 自然宽 56…81，随字号走）。出厂台阶上盒内还有 40px 空，但 tiny 台阶
   只剩 12.8px 的内边距之外的空间——**要先量**它会不会在更小的台阶上切字，再决定要不要给它
   一条地板。按「一个表面一轮」，这是**同一张卡的下半块**，下一轮就是它。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2。
3. **需拍板（沿用）**：启动器两行结果行 36px 盒装 27 行墨 / `ui/tooltip.go:133` 禁用元素不弹
   提示 / §6.111 / 113-5 / 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾 / §6.129 的 129-5。
4. **新记一条（本轮量到，未改）**：`chooseSelect` 的 `MinWidth(220)` 与 `paletteRow` 的
   `MinWidth(220)` 现在同值，但**来源**是两个常量字面量。如果哪天参考的 `max-width: 220px`
   （`settings.css:919-940`）改了，两处会各自漂。**现在不改**：`220` 在仓里只有这两处，
   提成一个常量属于重构而不是细节打磨，等第三次出现再说。
