# P13-co 报告 · 终端外观抽屉：读数住进自己的槽

**这轮为什么让它更有空间感**：抽屉尾部那簇东西此前是「数字多宽、簇就多宽」——读数是
**尺寸的主人**，滑轴是它的随从。给读数一个固定的槽之后，尺寸关系反过来了：滑轴钉在
簇的左端不动，只有槽里的数字在变。空间感来自**不动的东西**：一条永远在同一处的轨道，
比一条会跟着数字挪位的轨道更像一件「装好的仪器」。

## 1. 证据（先量后改）

pin 在改动之前跑出来的原始报错串（HEAD `712ac7ac`，产品代码未动）：

```
--- FAIL: TestTheDrawerReadoutHasItsOwnSlot (0.08s)
    readoutslot_test.go:62: the readout is not in a slot: its box goes
    {419.90002 28 22 17} → {413.90002 28 28 17} (x 419.9 → 413.9, w 22.0 → 28.0)
```

- 读数盒子 `w`：`8 px` = **22.0**，`48 px` = **28.0** —— 多一个字形就是多 6px。
- 尾部簇是 `Grow(1)` + `Justify(ui.End)` 的右对齐（`drawerRow`，`drawer.go:352-359`），
  所以读数变宽 = **滑轴左移同样的 6px**。
- 同一件事在拖动中每次跨过一位数都会发生一次（`9 px` → `10 px`）：手指按着的那根
  轨道在指下自己挪位。
- 行高读数 `%.2f×`（`0.50×`…`2.00×`）恒 5 个字形，它「稳」是格式碰巧给的，不是槽给的
  —— 所以这一轮把三个读数一起收进同一条规矩。

## 2. 改法（只改墨与盒子宽度）

`internal/terminalui/drawer.go` 的 `drawerReadout`：

```
- return ui.Text(c, text).Font(...).FontSize(tokens.Body).
-     TextColor(tokens.TextStrong).FontWeight(600).FontFeatures("tnum")
+ return ui.Text(c, text).Font(...).FontSize(tokens.Body).
+     TextColor(tokens.TextStrong).FontWeight(600).FontFeatures("tnum").
+     Grow(1).TextAlign(ui.End)
```

- **槽不是量出来的常数**：`Grow(1)` 让读数吃掉尾部剩下的宽度，于是滑轴被钉在簇的左端
  （它的 `drawerRangeWidth` 27.5u 是固定的，§6.3 钉死），读数盒子也随之固定；
  `TextAlign(ui.End)` 让数字仍贴在这一行的右边缘——单位对齐不破。
- **故意不挂 `SingleLine`**：槽被压窄时，读数折成两行会被 pin 抓到（盒子高度变）；
  裁掉一个字的数字是看不见的错。这一支宁可吵，不要静默截断。
- 三个调用点**零改动**（签名不变）。行高那一条从「碰巧稳」变成「按规矩稳」。

改后实测（同一个 pin 的日志）：

```
readout slot: 8 px → {396.65002 28 45.25 17}, 48 px → {396.65002 28 45.25 17}
```

盒子的 `x` 与 `w` 在两端逐字节相同（`45.25`），右边缘仍是 **441.9**（与改前一致）。

## 3. V / B / S 台账

- **V 0**：没有任何一条视觉 pin 因为「旧值 → 新值」而改写。
- **B 0**：`internal/terminalui` 与 `internal/ci` 全绿，零测试改动。
- **S 0**：结构没变（一个 helper 的方法链多了两个调用）。

## 4. 门槛（实测，HEAD 前后同一棵树）

```
gofmt -l cmd internal                            # 空
go vet ./...                                    # 净
GOOS=windows go vet ./...                       # 净
GOOS=darwin  go vet ./...                       # 净
go test -count=1 ./...                          # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3276 → 3277（基线 3276，P13-cn）
                                                    # FAIL 0 | SKIP 132 → 133（新增的 dump 是 opt-in）
                                                    # 测试函数 2424 → 2426
GOOS=linux/windows/darwin go build ./...        # 三平台过
```

## 5. mutation（实跑，逐条还原并逐字节核对）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 去掉 `Grow(1)`（只留 `TextAlign(ui.End)`） | 红：`the readout is not in a slot: its box goes {419.90002 28 22 17} → {413.90002 28 28 17} (x 419.9 → 413.9, w 22.0 → 28.0)` |
| M2 | `Grow(1)` → `Width(c.Theme().Space(2.5))`（10px 的窄槽） | 红：`the widest readout wrapped instead of fitting its slot: box height 34.0 → 51.0` |

两条都 `cp` 备份 → `md5sum -c` 逐字节还原（`b0fa101660d77ee660490020e799d768`）→
`touch` 后复跑绿（避免测试缓存）。

## 6. 出图

`/tmp/floter-P13-co-readout-{light,dark}-{8,48}-{before,after}.png`（opt-in dump，
`internal/terminalui/readoutslot_dump_test.go`，`FLOTER_PNG=1`）。

前后差分（逐像素，`image/png` 解码后比对）：

```
light  before/after: 8px 643 px, 48px 768 px
dark   before/after: 8px 1021 px, 48px 950 px
```

**图上看到的一处变化**：字号那一行的轨道（含滑轴）在 `-after` 里比 `-before` 靠左
23.25px，而 `-before` 的两张图之间轨道自己相差 6px（8px 那帧在 301.9，48px 那帧在
295.9）——`-after` 的两张图轨道 x 完全相同，只有槽里的数字在变。

## 7. 「偷了什么」（craft-loop 工艺密度菜单，逐条落地）

- **第 6 项「编辑细节」（数字/单位对齐）** → 落成 `Grow(1)` + `TextAlign(ui.End)` 的
  固定读数槽：盒子宽 **45.25px**、右边缘 **441.9** 在两套外观、两个极值下都不动。
- **第 5 项「多层染色阴影」/ 第 12 项「材质细节」** → **本轮不取**，理由：读数不是一个
  *面*，它没有 z；给它加受光边就变成 §2 禁令里的「给不是面的东西画框」。
- **DNA 注入**：这一支的 DNA 供体**不是** 58 站库，而是本仓 `docs/liquid-glass.md` §4
  （间距/圆角刻度）与 `docs/raycast-design.md` 的控件度量——因为方向已由用户钉死，
  craft-loop 的「参考 DNA」在本仓的等价物就是这两份合约。偷的具体值：`drawerRangeWidth`
  27.5u（不改）、`Gap(2)`（不改）、读数槽吃剩余宽度（新）。

## 8. preflight 逐条结论

- **A 反 AI 套路**：无渐变/无纯黑/无斜体/无卡片海/无左侧竖线。本轮只动一个读数盒子的
  宽度与对齐，A 节全部条目维持原状（不新增任何装饰）。
- **B 设计读取一致性**：设计读取 = 「功能型控件（终端外观条），面向重度终端用户，
  用轻便液态玻璃的**平**那一支（条带内不加材质），DENSITY 7」。记忆点一句话：
  **「轨道钉住，数字在槽里长」**。产出与该读取一致（无新增面、无新增线）。
- **C 动效抽查**：本轮无动效、无过渡。C 节的「高频操作不加动画」在这里的等价物是
  **跟手**：被拖的轨道在拖动过程中不许自己移动——这正是本轮修的东西。
- **D2 中文页**：**零 i18n 改动**（红线段）；读数 `48 px` 数字与单位之间有空格 ✓；
  全角标点不涉及。`tokens.Caption` 11px 对 D2「正文 ≥ 14px」的拍板项**仍悬着**，
  本轮不碰（沿用第 17 份证据）。
- **不过/不适用项**：A2.1 K3 门 —— 用户未点名 K3，按 P-46 默认由当前执行者做，
  不调用外部模型；A5 预览门 —— 本轮不是方向选择，无预览页（用户已把方向钉死）。

## 9. 仍待（下一格）

1. **需拍板（沿用）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
2. **需拍板（沿用）**：会话页空态说错了对象（「持久化会话」vs 进程内会话）；i18n 红线。
3. **需拍板（沿用）**：终端页会话抽屉/标签对失败只加字不换墨。
4. **需拍板（沿用）**：`launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带 chip
   的按下档；终端外观抽屉行要不要面（**这一条正是本轮绕开的那一格**——本轮只动读数，
   没有给抽屉行加面）。
5. **下一格（不需拍板）**：通用队列「空态满态」——扩展页空态只有一行结论、没有出路
   （`settingsui.go:1838` 的 `hint` 传空串、`action` 传 nil），与同页会话页空态
   （结论 + 理由 + 一块板）不是同一种说法；但补文案要动 i18n，**先量再定**。
