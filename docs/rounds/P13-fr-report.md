# P13-fr 报告：字体族 picker 的天花板，不在 picker 上，在槽上

**意图**：`chooseSelect`（`internal/settingsui/settingsui.go:4718`）是同一张 Terminal appearance
卡上唯一没有「天花板」的那一枚 picker。P13-fp 给 `paletteRow` 补过 `Shrink(1)`（§6.135），
P13-fq 的「仍待」第 1 条于是问：这一枚要不要也补一条？本轮把它量成一句话——**补了也没用，
而且它根本不出卡**。

选题来源：P13-fq 报告「仍待」第 1 条。表面 = 设置页 General 页字体族 picker。合约新增 **§6.137**。

## 1. 先量（720 宽真实 General 页，四台阶 × 深浅；92 字族名对出厂 `JetBrains Mono`）

| 台阶 | 出厂：标签列右沿 | picker 盒 | 92 字：标签列右沿 | picker 盒 |
|---|---|---|---|---|
| tiny | 464.8 | 215 | 194.8 | 497 |
| small | 460.4 | 218 | 200.4 | 491 |
| default | 456.0 | 216 | 206.0 | 485 |
| large | 451.6 | 220 | 211.6 | 480 |

（深浅两套逐值相同；picker 盒按面读，`pillBox`。）

## 2. 判一：`Shrink(1)` 在这一处是**空操作**

把 `MinWidth(220)` 改成 `MinWidth(220).Shrink(1)`，八个 arm（四台阶 × 深浅）的**整帧 CRC32
逐位相同**：`6f4e0060` / `840884d8` / `7a497533` / `48dd95d3` / `3f73e637` / `25242ea6` /
`5139c72c` / `b0df1161`（改前改后同值）。

结构原因：picker 是控制槽（`rows.go:1617` `ui.Row(c).Shrink(0)`）的**唯一孩子**——槽的盒子就是
picker 的盒子，槽不收缩，picker 收缩也没人压它。`paletteRow` 那一处之所以有效，是因为 picker 与
预览块是**同一个 `Row(FillWidth)` 的两个孩子**，`Shrink(1)` 是在兄弟之间让位。

所以**不加**：加了就是一行永远不生效的代码。改完又原样退回，`settingsui.go` 的 md5 与本轮开工时
逐字节相同（`3635f1c9b1c3f65272fbbd6594987a7b`）。

## 3. 判二：长值不出卡

出厂帧与 92 字帧逐列比（picker 自己那条带的 y 区间内），**最后一列不同的位置**是
**693 / 690 / 687 / 684**——正好是同一张卡「14 px」读数的右沿（行内容盒右沿，§6.136 量过
693.4 / 690.2 / 687.0 / 683.8）。再往右，**含卡片自己的右框 703.8，两帧逐像素相同**。

## 4. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/pickerceil_test.go`（新） | 三条 pin + 一条 opt-in dump |
| 2 | `docs/liquid-glass.md` §6.137 | 量的表 + 两条答案 + 一条 `需拍板` |
| 3 | `.pending-bugs.md` | 「需拍板」新增「让控制槽在标签列让尽之后能收缩到 220」 |

pin：

- `TestThePickerNeverPaintsPastTheRowColumn`——出厂帧 vs 92 字帧的最后一列差 = 行内容盒右沿
  （四台阶 × 深浅，`±0.5px` 上界；下界 `-3px` 防止这条 pin 变成空断言）。
- `TestTheLabelColumnYieldsFirstAndThePickerHasNoCeilingOfItsOwn`——标签列右沿 8 个值、picker 盒
  8 个值（±1.5 / ±6），外加「两半都动了 ≥200px」。
- `TestChooseSelectHasNoShrinkOfItsOwn`——源码 pin：`chooseSelect` 里**不许**出现 `.Shrink(`
  （它没有兄弟可让），`MinWidth(220)` 必须在。

## 5. 门槛（实测）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 全绿
PASS 3422（基线 3419）| FAIL 0 | SKIP 196（基线 196）| 测试函数 2596（基线 2593）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

台账 **V 0 / B 0 / S 0**（零像素、零行为；新增 pin 全在新文件）。

## 6. mutation（实跑）

- **M1**：`chooseSelect` 的 `MinWidth(220)` → `MinWidth(220).Shrink(1)` →
  `TestChooseSelectHasNoShrinkOfItsOwn` 红：
  ```
  chooseSelect grew a Shrink of its own: it is the only child of the control slot (`rows.go` `ui.Row(c).Shrink(0)`), so a shrink here is a no-op — the ceiling, if the row needs one, belongs on the slot (§6.137)
  ```
- **M2**：`MinWidth(220)` → `MinWidth(2200)` → 两条 pin 红：
  ```
  step=tiny/dark=false: a 92-character family paints exactly like the shipped one — the value never reached the picker
  step=tiny/dark=false: the label column ends at 194.8, pinned at 464.8 (shipped family)
  step=tiny/dark=false: the shipped family draws a 497px picker, pinned at 215…220 (the 220 floor): the picker's box stopped coming from its floor
  ```
- **M3**：`MinWidth(220)` → `Width(220)` → `TestThePickerNeverPaintsPastTheRowColumn` 红（8 处）：
  ```
  step=tiny/dark=false: the long value's frame stops at x=665, 28.4px short of the row's content edge (693.4): the picker no longer uses the whole column
  ```
- 三条均 `cp` 还原 → `md5sum` 核对（`settingsui.go: 3635f1c9b1c3f65272fbbd6594987a7b`、
  `rows.go: 11fdeeef25958268606195faea653304`）→ `touch` 复跑全绿。

## 7. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP13FRPickerCeiling` →
`/tmp/floter-P13-fr-pickerceiling-{default,large}-{light,dark}-{ship,long}.png`（720×1400 整页）。

**这张图上看到的一处变化**：default/light 的同一行，出厂帧的胶囊从 x=462 起、92 字帧从 x=212 起
（左沿移动 **250px**），而**两者的右沿都在 x=687**——也就是同一张图上，左边那一半让开了多少，
右边那一半就吃了多少，卡片的右列一列不动。

## 8. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **2** 项 空间节奏（一个盒子只占它需要的地） | 标签列让出的 270 / 260 / 250 / 240px 与 picker 吃进的 282 / 273 / 269 / 260px 全部写进合约；「一半让、一半吃」不再是感觉 |
| 第 **4** 项 布局与让位顺序（谁先让） | 量成一句话：标签列先让到自己的墨为止（464.8 → 194.8），picker 永远不收缩（`Shrink(0)` 槽的唯一孩子）；顺序写进 §6.137，pin 守着 |
| 第 **12** 项 材质细节（盒与面分开量） | 盒按**面**读（`pillBox`），帧差按**列**读（最后一列不同 = 693/690/687/684），两个量分开，不把抗锯齿当成盒宽 |
| 第 **6** 项 编辑细节（hairline 与刻度） | 帧差的下界也钉住（不得短于行内容盒右沿 3px）：只钉上界的话，「picker 变窄了」这条回归会静默通过 |

只取 4 项：这一轮是一个**上限**的判定，第 5 项就要往产品里加东西，而细节打磨的减法优先。

## 9. preflight 逐条（A / B / C / D2）

- **A 反套路**：零像素、零文案、零渐变/发光/纯黑/斜体（`noitalic_test.go` 仍绿）；没有新增卡片或
  三等分横排。产物是 pin 与文档。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」。
  本轮判据一条——**卡片的右沿是这一页唯一的墙**：所有控件都必须停在它左边。量完成立
  （出厂帧与 92 字帧在墙右侧逐像素相同）。
- **C 动效抽查**：不动效；picker 开合、行色过渡仍是框架原样。
- **D2 中文页**：不新增一个汉字（i18n 红线）。zh 的族名与 en 同为 `JetBrains Mono` 这类 ASCII
  值，标签列与 picker 盒的几何与语言无关。既有缺口仍是 `tokens.Caption` 9.9px / 正文 10.8px 对
  「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动。

## 10. 仍待

1. **下一格（首选）**：**空态的密度**——通用队列里只剩这一项（满态已由 P13-ex/fj 收官）。
   表面 = 会话页 / 自定义快捷键页的空态块；P13-fj 已量过它们的上下空气（一个密度），
   没量过的是「空态里那几句话自己的行距」。
2. **需拍板（本轮新增）**：让控制槽在标签列让尽之后可以收缩到 220——把 92 字的值改成
   **省略号结尾**而不是被行边切断（`ui/layout.go:200` 用元素自己的 width 排版，框 600+ 而可见
   493 时文字被行边硬切）。这是 §6.132 `Shrink(0)`（「控件不收缩，让标签列先让」）的语义改动，
   影响设置页每一行，见 `.pending-bugs.md`。
3. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2；启动器两行结果行
   36px 盒装 27 行墨；`ui/tooltip.go:133` 禁用元素不弹提示；§6.111 / 113-5 / 114-4；浮层页头
   字号上阶梯（`104-2`）；便条行 rank 1.28；终端会话抽屉失败只加字不换墨；多选箱宽度；
   路径截断保头弃尾；会话行 56 地板；终端弧 S1b / S1c / S2 尾；§6.129 的 129-5；§6.136 的 136-4。
