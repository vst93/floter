# P13-fp 报告：picker 的地板之上，得有一条天花板

**意图**：P13-fo 把调色板 picker 从 `Width(220)` 改成 `MinWidth(220)`（220 是地板，长值不再
被省略号切）。这一轮补上那句话的另一半：**地板没有天花板，长值就把预览块顶出行外。**

选题来源：P13-fo 报告「仍待」第 1 条 = `palettePreview` 自己的宽度来源（同一张卡的下半块）。
量它的过程中发现，预览块从来不会切自己的字；真正的病在它**左边**——上一轮刚放开的那个盒子
会把它推走。

## 1. 先量

行内容盒 515（卡内宽），四台阶 × 深浅。真画法：`a.paletteRow` 画在 `Padding(20) + Width(515)`
的列里，`tt.Find(copy.PalettePreview)` 读预览块，`leftmostInk/rightmostInk` 读值墨。

| 台阶 | 值长 | 值墨 | 预览块 x | 块右沿 | 行右沿 | 溢出 |
|---|---|---|---|---|---|---|
| default | 8 | 49…94 | 333.0 | 481.0 | 535.0 | −54.0 |
| default | 35 | 49…250 | 333.0 | 481.0 | 535.0 | −54.0 |
| default | 55 | 49…358 | 392.0 | 540.0 | 535.0 | **+5.0** |
| default | 75 | 49…479 | 511.0 | 659.0 | 535.0 | **+124.0** |
| large | 55 | 51…390 | 427.8 | 590.6 | 535.0 | **+55.6** |
| large | 75 | 51…522 | 558.8 | 721.6 | 535.0 | **+186.6** |
| default | 120 | 49…568 | 770.0 | 770.0 | 535.0 | 盒宽 0 |

出厂九个调色板名最宽 80px，碰不到这条线；但 `paletteRow` 的标签表是参数
（`i18n.Settings.Palettes`），P13-fo 自己就是用 55 字标签证明地板生效的。被推出去的是
**预览块**——这一行存在的理由（R43：让用户看见这个调色板长什么样）。

顺带回答了 P13-fo 留下的问题：预览块自己的 `Space(37)` **不会切字**。四个台阶 × 深浅，
最长那句 `floter --version` 距块内沿 25.6 / 26.8 / 29.0 / 32.2px。

## 2. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:4768` `paletteRow` | `.MinWidth(220)` → **`.MinWidth(220).Shrink(1)`**，注释写清「先借标签列的地、借完了在自己的盒子里收口」 |
| 2 | `internal/settingsui/paletteceiling_test.go`（新） | 三条 pin：块 rect 不变 / 值的墨不越过块左沿 / 块内那句命令留 ≥8px |
| 3 | `docs/liquid-glass.md` §6.135 | 这一轮的账 |

出厂内容零像素改动（九个调色板名远不到 50 字，`Shrink(1)` 不触发）。

## 3. 门槛（实测）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 全绿
PASS 3416（基线 3413）| FAIL 0 | SKIP 195 | 测试函数 2589（基线 2585）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

## 4. mutation（实跑）

- **M1**：删掉 `.Shrink(1)`（回到本轮之前）→ `TestThePickerFloorHasACeiling` **11 处红**，
  原始报错串：
  ```
  step=large: a 120-char value moves the preview block {316.80002 61.7 162.80002 87.40001} → {841.8 61.7 0 0} (+525.0px): the picker's floor has no ceiling, so it pushes the block out of the row
  ```
  `cp` 还原后 `md5sum -c`（`settingsui.go: 成功`，`3635f1c9b1c3f65272fbbd6594987a7b`）+ `touch` 复跑全绿。
- **M2**：预览块宽度 `Space(37)` → `Space(24)` → `TestThePreviewBlockStillHoldsItsThreeLines`
  **8 处红**，原始报错串：
  ```
  tiny/dark=false: the command line ends 0.0px before the block's inner edge (block 76.8 wide, inner padding 12.8): the block is clipping its own content
  ```
  `cp` 还原后 `md5sum -c`（`settingsui.go: 成功`）+ `touch` 复跑全绿。

## 5. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP13FPPaletteCeiling` →
`/tmp/floter-P13-fp-ceiling-{small,large}-{light,dark}-{before,after}.png`（720×320，只画这一行）。
before 是同一条 dump 在「没有 `Shrink(1)`」的行上重跑的（`FLOTER_P13FP_ARM=before`），
不是同一张图改名字——四张 before 与四张 after 的 md5 全不相同。

**这张图上看到的一处变化**（像素读，`contrast` 调色板的黑块）：large/light 下，before 的
黑块右沿落在 x=624（距窗口右沿 95px，已经越过行的右沿 535）；after 的黑块右沿落在 x=479
（距窗口右沿 240px）——块**向左挪回 145px**，正是它该在的位置。small/light 下 before
x=518 / after x=481。

## 6. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **4** 项 布局与让位顺序（谁先让） | 行内两个孩子的让位顺序：预览块 `Shrink(0)`（不可让），picker `Shrink(1)`（先让）。一句话定序，不靠宽度常数 |
| 第 **9** 项 边界与截断（收口处的手艺） | 截断点从「行外」推回「自己的盒边」：120 字的值从画到 x=770（行外 235px）→ 收在块左沿之前 |
| 第 **12** 项 材质细节（盒与面分开量） | 断言量的是**块的 rect**（`ui.Rect` 逐字段相等），不是「差不多在同一带」——宽度、x、高度任一漂移都红 |
| 第 **2** 项 空间节奏（一个盒子只占它需要的地） | 出厂值零像素改动：`Shrink(1)` 在 8 字的标签上根本不触发（实测四个台阶 rect 与改前逐字段相同） |

只取 4 项：这一轮是**一个让位顺序**，凑第 5 项就得往产品里加东西，而「细节打磨」的减法优先。

## 7. preflight 逐条（A / B / C / D2）

- **A 反套路**：不新增渐变、发光、纯黑、斜体（`noitalic_test.go` 仍绿）、三等分卡片；
  零文案改动（i18n 红线）。唯一的新视觉行为是「长值在盒边收口，预览块不挪」。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」。
  本轮判据一条——**同一行里不可让的东西先保住**：预览块显示的是这个选择的后果，它比值的完整
  更长更重要；所以让位的必须是 picker。
- **C 动效抽查**：不动效；没有 `transition: all` 可违反；picker 的开合仍是框架原样。
- **D2 中文页**：不新增一个汉字（i18n 红线）；`zh` 的调色板名（`跟随应用` / `高对比` / `纸张` /
  `墨黑` / …）比 en 短，量值与 en 的块 rect 逐值相同。既有缺口仍是 `tokens.Caption` 9.9px /
  正文 10.8px 对「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动，两条不受影响。

## 8. 仍待

1. **下一格（首选）**：`paletteRow` 的**行内间距**——`Justify(ui.SpaceBetween)` 把 picker 与
   预览块推到行的两端，出厂值下两者之间是 175…300px 的空；而参考的 `.terminal-palette` 是
   `justify-content:space-between` 同款，所以这是**照抄**，不是漏改。**先量**：这个空在四个
   台阶上的实际值，以及它和卡片其它「两端对齐」的行是不是同一条规则，再决定动不动。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2。
3. **需拍板（沿用）**：启动器两行结果行 36px 盒装 27 行墨 / `ui/tooltip.go:133` 禁用元素不弹
   提示 / §6.111 / 113-5 / 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾 / §6.129 的 129-5。
4. **新记一条（本轮量到，未改）**：`chooseSelect`（字体族 picker）**没有** `Shrink(1)`——
   它在尾随行（`cardRow`）里由标签列 `Grow(1)` 让位，所以现在没有溢出；但如果哪天它也进了
   一行「两个孩子的两端对齐」布局，同一个病会复发。**现在不改**：没有第二个调用点需要它，
   提前加是把一句话写两遍（§6.130 的 130-6 同款理由）。
