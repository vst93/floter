# P13-di 报告 · 半成品的那一行：缺的是一格键，不是整行的亮度

方向一句话：**快捷键页里「正在做」的那一行不再被整行 62% 洗一遍**——它和一条已保存的行
是同一行、同一套墨；「还没写完」由它唯一缺的那一格（空的录制槽）说，而不是把一个乘法套在
**唯一能把这一行写完的控件**上。

## 1. 为什么是这一格

任务书：「下一轮 = 细节打磨；下一格从『状态反馈』的**另一半**（禁用 / 只读面）或空态满态里
挑，**先量再改**」。§6 差距表已无 `未开始` 行。禁用面一族（`disabledActionButton` /
`busyActionButton` / `extensionIconButton` / `lockedControl` / `disabledSwitch` / 卸载整卡 /
`commandPlate`）都已落地；**唯一没量过**的是「不是 `Disabled`、只是还没写完」的那一类——
`customDraftRow`（`internal/settingsui/custom.go:258`），参考的 `.settings-row--muted`
（`settings.css:564-569`）整行 `opacity: 0.62`，移植原样照做。

## 2. 先量（整行盒扫描，`litSum` 三通道和，1 亮度级 = 3）

行标题墨（`lo`，浅色取最暗 / 深色取最亮）与它到行地面的距离，四个因子各量一次
（`internal/settingsui/rows.go` 里那行 `.Opacity(·)` 的因子 swap 出来的，`md5sum -c` 逐字节还原）：

| 外观 | 因子 | 标题墨 | 差（级） | 提示墨（推导 `f·ink + (1−f)·面`） |
|---|---|---|---|---|
| 浅 | 1.00（已保存行 = 对照） | **83**（27.7 = `text`） | 659（220） | 128 = `textMuted` |
| 浅 | **0.62（改前）** | **322**（107.3 = `textSecondary` 107） | **420（140）** | **173（低过 `textTertiary` 142）** |
| 浅 | 0.75 | 238（79.3，两档之间） | 504 | 152 |
| 浅 | 0.50 | 398（132.7，两档之间） | 344 | 179 |
| 深 | 1.00（对照） | **713**（237.7 = `text`） | 669（223） | 133 = `textMuted` |
| 深 | **0.62（改前）** | **476**（158.7，`textSecondary` 154 +5） | **432（144）** | **99（低过 `textTertiary` 110）** |
| 深 | 0.75 | 560（186.7，两档之间） | 516 | 111 |
| 深 | 0.50 | 398（132.7，两档之间） | 354 | 88 |

阶梯的四个档（`internal/theme/theme.go:689-703` / `824-830`）：浅 `text` 27 · `textSecondary`
107 · `textMuted` 128 · `textTertiary` 142；深 `text` 237 · `textSecondary` 154 · `textMuted` 133
· `textTertiary` 110（浅色 107.3 与 `textSecondary` 107 差 0.3 —— 那个巧合正是问题所在）。

**两条结论**：

1. **一个乘法不可能同时是两个档。** 0.62 让标题恰好冒充 `textSecondary`，同时把这一行最安静
   的一档墨推到阶梯之外（浅 173 > `textTertiary` 142，深 99 < `textTertiary` 110 ——两头都
   没有 token）。0.75 / 0.50 只是把「落在两档之间」从提示挪到标题：**没有哪个因子同时成立**。
2. **它洗掉的正是唯一能把这一行写完的控件。** 选择器的受光边（`LitFace`）、命令输入框的面、
   录制键的键帽、`Bind` 键的面，全被合成为 62% 亮的面。§3：受光边是「这块面可以被碰」的记号；
   这是整页里唯一**必须**被碰的那一块，而它成了唯一没有完整受光边的面。

## 3. 落地（三处，视觉）

- `internal/settingsui/rows.go` `rowFace`：删 `Muted bool`，字段位置改成一条注释，
  把上表的四个因子与结论写进代码里（`P13-di`，指向 §6.78）。→ 这一格**故意不存在**。
- `internal/settingsui/rows.go` `cardRow`：删掉消费点 `if face.Muted { row = row.Opacity(0.62) }`。
- `internal/settingsui/custom.go` `customDraftRow:258`：`face.Muted = !complete` → 零值
  `rowFace{}`，注释改成「同一行、同一套墨；没写完由空的录制槽说」。
- 新增 `internal/settingsui/draftrowink_test.go`：两条 pin + 一条 opt-in 出图。
  ① 未写完的行与已保存的行的**最强一档墨**差 ≤ 1 级；② 它到地面的**对比**也差 ≤ 1 级；
  ③ 写完整的草稿行（对照臂）同样与已保存的行同墨。深浅各一遍。盒由 `Bind`/`Resolve` 取，
  pin 里没有一个像素坐标。

**台账：V 0 / B 0 / S 0。** 没有放宽断言、加容差、`t.Skip`、弱化为「非空」。覆盖是**涨的**：
这一行此前没有任何像素级的 pin（`TestCustomShortcutDraftRowMuting` 只量 `customRowComplete()`
的布尔值）；从本轮起它的墨有两个数钉着。

## 4. 前后对比图

`/tmp/floter-P13-di-draftrow-light-before.png` / `-after.png`、
`/tmp/floter-P13-di-draftrow-dark-before.png` / `-after.png`（整页 720×1100，快捷键页，
草稿行打开）。差分实测：

```
light  7430 px  bbox (200,692)-(690,721)  rows 702:274 701:272 710:272 711:272 704:271 …
dark   7422 px  bbox (200,692)-(690,721)  rows 702:274 701:272 710:272 711:272 704:271 …
```

差分框恰好是**那一行**（490 宽 × 30 行），一行之内每行 271~274 px 变了 —— 说明动的是
这一行里的**面**与**字**，不是外面的卡框；已保存的上一行（`Cmd+Shift+P`）一个像素都不在
框里。

**这张图上看到的一处变化**：草稿行不再是一块「隔着雾」的行——它的选择器面、`Bind` 键面与
上面那条已保存行站在同一层，字也回到了正墨；行里唯一还「空」的东西是录制槽那一格。
浅色前后同一个像素 `(200,692)` 从 `(196,206,219)` 变成 `(176,191,213)`，深色从
`(56,62,73)` 变成 `(66,75,93)`：两套外观上都是「面更实、字更清」，方向一致。

## 5. 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落到数值）

| 取第几项 | 菜单里的签名动作 | 本仓落成什么 |
|---|---|---|
| **#12** | 材质细节（内边框高光 / glass 折射边） | 这条是它的**反面**用法：受光边只在**该亮的面**上亮。落成「62% 亮**不是**一档」——一块面要么戴满 `LensRim`（选择器 / 输入框 / `Bind`），要么熄（§6.15 的 `DimFace`），不存在 0.62 亮的面。实测把这条从 `420/659`（浅）与 `432/669`（深）拉回 **满值** |
| **#6** | 编辑细节（hairline / 大写字距标签） | 行的两根线各管一件事：`Hairline` 是**画框**（分隔线，随行 inset），`LensRim`/`LensEdge` 是**材质**（这块面可以被碰）。本轮画框一个字节没动，只让材质不再被乘法打折 |
| **#5** | 多层染色阴影（拒绝单层灰） | 本仓的等价物是一条面的**一对**线（顶受光 / 底暗），不是「更亮的灰」。落成 pin 的第②条：不是「墨够黑」，而是**墨与地面之间那 220 级（浅）/ 223 级（深）的差**必须原样在（洗过只剩 140 / 144） |

不取：`#1` 氛围层、`#8` 流体字号、`#9` 滚动条、`#11` 图形资产——Go 自绘、无 DOM 层，
强凑就等于 §0 的「假装有 backdrop-filter」。

## 6. preflight 逐条结论

- **A 反套路**：无渐变、无发光、无斜体；本轮**减**了一道整体淡（少了一个乘法），没有加任何
  装饰。P-38（禁左竖线强调）：未加竖线，行仍靠面与一对线说话。**通过。**
- **B 设计读取一致性**：「轻便液态玻璃 + 空间感」这一读法在改后更成立：同一行里的两块面
  （选择器、`Bind`）与行地面之间的差从 `0.62×` 回到 `1.00×`，正是这一支要的 elevation 差；
  没有引入 glass-over-glass、没有组件级自造透明。**通过。**
- **C 动效抽查**：本轮零动效改动（未加 `Transition`、未改 `Scale`）；`render` 的 pin 都在
  `ReduceMotion: true` 下跑。**通过（不适用面已声明）。**
- **D2 中文页**：本轮无中文文案改动；存在的 `tokens.Caption` 9.9px 与 D2「正文 ≥ 14px」的
  冲突仍是**需拍板**项（沿用，见 §6 需拍板清单），不属本轮。**通过（附挂起项）。**
- **E 三秒测试**：改后这一页的第一眼读数变成「两行快捷键，其中一行还差一格键」，而不是
  「一行正常、一行坏掉」。**通过。**

## 7. mutation（实跑，`md5sum -c` 逐字节还原 + `touch` 后复跑 `ok`）

- **M1** 把 `0.62` 那道洗搬回来（`rows.go` `if face.Muted` + `custom.go`
  `face.Muted = !a.customRowComplete()`）→ 红 4 条：

  ```
  dark=false: the unfinished row's ink is 322, the saved row's is 83 (+239, 79.7 levels apart).
  dark=false: the unfinished row's ink sits 420 units off its plane, a saved row's sits 659 (-239).
  dark=true:  the unfinished row's ink is 476, the saved row's is 713 (-237, -79.0 levels apart).
  dark=true:  the unfinished row's ink sits 432 units off its plane, a saved row's sits 669 (-237).
  ```

- **M2** 保留机制、因子换成 `0.75` → 同一条 pin 红（`draft 238/504` 对 `saved 83/659`；
  `draft 560/516` 对 `saved 713/669`）——证明这条 pin 钉的是「乘法」本身，不是 `0.62` 这个数。
- 还原后 `md5sum -c` 两份文件均 `成功`，`touch` 再跑 → `ok`。

## 8. 门槛（实测，改后）

```
gofmt -l cmd internal              → 空
go vet ./...                       → 净
GOOS=windows go vet ./...          → 净
GOOS=darwin  go vet ./...          → 净
go test -count=1 ./...             → 全绿
--- PASS 3312（基线 3311，+1）      --- FAIL 0      --- SKIP 149（基线 148，+1 opt-in dump）
测试函数 2469（基线 2467，+2）
GOOS=linux/windows/darwin go build ./... → 过
```

### 仍待（P13-di）

1. **下一格（细节打磨，状态反馈的第三半：只读面）**：设置页 → About 页的只读事实行
   （§6.11 只换过墨，没量过只读面自己的材质）；先量它的面与地面差是否也落在阶梯外。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口（§6.75 / §6.76 /
   §6.77 的顺序表），四包一字未动，仍在队里。
3. **需拍板（沿用 + 第 34 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第三十六次）**：`stash@{0..2}`（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
