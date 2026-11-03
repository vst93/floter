# P13-fn 报告：picker 的盒子是**地板**，不是上限

**意图**：设置页的 picker 盒子是**字面 220px**，而它装的东西（内边距、字形、箭头）全部跟着
界面台阶缩放——于是同一个盒子的空气占比从 **56.7%**（tiny）滑到 **35.9%**（large），而一个比
盒子长的值被 `SingleLine` 用省略号切掉。这一轮把 220 从**上限**改成**地板**：出厂值永远画同一个
盒子（右列仍是一条线），更长的值借标签列的地，**标签一字不切**。

选题来源：P13-fk 报告的「下一格（首选）」= §6.132 的 **132-5**，状态 `需拍板（先量）`。

## 1. 先量（探针跑完即删，数字抄在这里）

720×900，General 页，`launcher.InputWindowWidth`，四个界面台阶（`valuedomain.ScaleFactors`）：

| 台阶 | u = `Space(1)` | picker 盒面（实测） | 盒内家具 11u | 盒内净宽 | 最宽出厂族名墨 | 空气占比 |
|---|---|---|---|---|---|---|
| tiny 0.8 | 3.2 | **212** | 35.2 | 184.8 | 80 | **56.7%** |
| small 0.9（出厂） | 3.6 | **214** | 39.6 | 180.4 | 90 | 50.1% |
| default 1.0 | 4.0 | **212** | 44.0 | 176.0 | 100 | 43.2% |
| large 1.1 | 4.4 | **212** | 48.4 | 171.6 | 110 | **35.9%** |

三句话：

1. **盒子不缩放**：pill 自己的面（不含两侧 1px 发丝边与抗锯齿）在四个台阶上是 **212…214**，
   而它里面的家具从 35.2 长到 48.4（+37.5%）——同一个盒子，两种缩放。
2. **空气在滑**：出厂最宽的那枚族名（`DejaVu Sans Mono`）在盒内占的面积从 43.3% 长到 64.1%，
   也就是盒子在 tiny 上「空得发飘」、在 large 上「挤」。
3. **长值被切**（把同一个 picker 画在 220 与 400 两个盒里逐列对比，差异列就是被切掉的地方）：

| 台阶 | 值自然墨宽（39 字族名） | 与 400 盒的差异列 | 第一处差异列 |
|---|---|---|---|
| tiny | 174 | 1（盒边，值装得下） | 209 |
| small | 199 | **19** | **189** |
| default | 221 | **21** | 187 |
| large | 239 | **27** | 178 |

出厂台阶上，长族名从**第 189 列**起被换成省略号——而它旁边就有 **260u** 的标签列可以借。

## 2. 判：盒子该跟谁缩放

三个候选，逐条否掉两个：

- **A. 让盒子跟界面台阶缩放（220 → 55u = 220·f）**：空气占比变成常数，但出厂台阶（small 0.9）
  上盒子会从 220 缩到 **198**——把**内容**（任意长度的族名）的余量又削掉 22px，切得更早。
  盒子装的是内容，不是台阶。
- **B. 只把注释/文档改成「字面 220px，故意不缩放」**：零像素，但把上面那张表的「病」写成
  「设计」——长值照样被切，而标签列明明有 260u 的空。
- **C（选它）。220 是地板**：出厂值（8 个族名全在内）永远画同一个盒子 → 右列仍是一条线；
  值比地板长多少，盒子长多少 → 切点回到「真的放不下」为止，让位的是**弹性**的标签列。

实测（改后，盒面 / 标签列）：

| 台阶 | 出厂族名（= Menlo） | 39 字族名 | 标签列 短值 → 长值 |
|---|---|---|---|
| tiny | 212 | 212（值 174 < 净宽 184.8，不用长） | 270 → 270 |
| small | 214 | **237** | 260 → **236.4** |
| default | 212 | **261** | 250 → **201.0** |
| large | 212 | **284** | 240 → **168.6** |

标签墨**一字未少**（`Font family` 66 列，长值/短值逐列相同）；最紧的一档（large）标签列仍有
168.6u，是标签墨的 **2.1 倍**。深浅两套逐值相差 ≤2px（AA），四个台阶 × 两套外观共 8 组。

## 3. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:4718-4730` `chooseSelect` | `Width(220)` → 分两支：`cardUnit > 0`（扩展面板，`extensions.css` 是字面 px）**保持** `Width(220)`；设置页（`cardUnit == 0`）`MinWidth(220)` = 地板。注释写清为什么是地板（附实测切点 189 列） |
| 2 | `internal/settingsui/pickerfloor_test.go`（新） | `TestALongPickerValueGrowsTheBoxInsteadOfBeingCut`：A 地板（出厂 8 个族名画同一个盒子，且盒面 216…222）；B 成长（盒增长 = 值溢出，±6）；C 标签不被切（标签墨列数长值/短值相同、标签列 ≥1.5× 标签墨） |
| 3 | 同上 | opt-in dump `TestDumpP13FNPickerFloor`（`FLOTER_PNG=1`） |
| 4 | `docs/liquid-glass.md` | 新增 **§6.133**，并把 §6.132 的 **132-5** 从 `需拍板（先量）` 结账成 `已落地（P13-fn）` |

**没动的两处，写清理由**：

- `paletteRow`（`settingsui.go:4757`）另一枚字面 `Width(220)`：调色板名都短（盒内净宽
  171.6…184.8 都装得下），而且它旁边是 `Space(37)`=148u 的预览块——那是**另一个表面**（终端外观
  卡）的一轮，按「一个表面一轮」不动。
- 扩展面板（`cardUnit > 0`）：`extensions.css` 是字面 px，卡片更窄、字段标签可能和选项一样长；
  让它长会把标签挤掉。保持 `Width(220)`。

## 4. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改（`go test ./internal/settingsui`
在改后第一次跑就全绿，266s）。新增 2 个测试函数（1 pin + 1 opt-in dump）。

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿（0 FAIL）
--- PASS                                    # 3412（基线 3411，+1）
--- SKIP                                    # 193（基线 192，+1 = 本轮 opt-in dump）
测试函数（静态计数）                          # 2583（基线 2581，+2）
GOOS=linux/windows/darwin go build ./...     # 全过
```

### mutation（实跑，原始报错抄在下面）

**M1** — `settingsui.go` 的 `e.MinWidth(220)` → `e.Width(220)`（回到本轮之前）：

```
--- FAIL: TestALongPickerValueGrowsTheBoxInsteadOfBeingCut (2.56s)
    pickerfloor_test.go:57: dark=false step=small: the box grew 0px for a value 20.600006px longer than its room (box 218→218, room 178.4, value 199.0): the value is being cut instead of the box growing
    pickerfloor_test.go:57: dark=false step=default: the box grew 0px for a value 49px longer than its room (box 216→216, room 172.0, value 221.0): ...
    pickerfloor_test.go:57: dark=false step=large: the box grew 0px for a value 67.399994px longer than its room (box 220→220, room 171.6, value 239.0): ...
    ... dark=true 同样三处，共 6 处红
```

**M2** — `e.MinWidth(220)` → `e.MinWidth(120)`（地板调低）：

```
--- FAIL: TestALongPickerValueGrowsTheBoxInsteadOfBeingCut (2.38s)
    pickerfloor_test.go:36: dark=false: "JetBrains Mono" paints a 119px box where "Menlo" paints 118px: the floor is not governing the shipped names
    pickerfloor_test.go:36: dark=false: "DejaVu Sans Mono" paints a 132px box where "Menlo" paints 118px: ...
    pickerfloor_test.go:36: dark=false: "Liberation Mono" paints a 124px box where "Menlo" paints 118px: ...
    ... dark=true 同样三处，共 6 处红
```

M2 顺带量到一件本轮之前不知道的事：**地板一旦降到 120，右列立刻按内容各画各的盒子**
（118 / 119 / 132 / 124）——「右列是一条线」这句话，靠的正是这个地板。

两条都 `cp` 还原，`md5sum -c` 逐字节核对（`settingsui.go: 成功`、`pickerfloor_test.go: 成功`），
`touch internal/settingsui/pickerfloor_test.go` 后复跑全绿。

## 5. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP13FNPickerFloor` →
`/tmp/floter-P13-fn-picker-{small,large}-{light,dark}-{before,after}.png`（General 页，装 39 字族名）。
before 是同一份 dump 在 `Width(220)` 上重跑的，不是同一张图改名字。

逐像素对比 before/after（差异边界框）：

| 图 | 差异像素 | 差异边界框 |
|---|---|---|
| small-light | 1857 | (448, 965) – (662, 995) |
| small-dark | 1858 | (448, 965) – (662, 995) |
| large-light | 3976 | (393, 1168) – (650, 1203) |
| large-dark | 3981 | (393, 1168) – (650, 1203) |

**这张图上看到的一处变化**：差异边界框**正好是 picker 自己那一块**（小台阶 214×30、大台阶
257×35），页面上其它一个像素都没动——picker 的左沿向左让出 17px（small）/ 64px（large），
值是完整字形结尾，不再是省略号；右侧那条列沿（691.2）一个像素没挪。

## 6. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **8** 项 流体字号（`clamp(min, vw, max)` 的**容器**版） | 容器的宽度要么跟内容、要么跟台阶，**不许固定**：220 从 `Width` 变 `MinWidth`——地板 220，内容更长时跟内容（small 237 / default 261 / large 284） |
| 第 **6** 项 编辑细节（hairline / 一列一基线） | 右列仍是一条线：8 个出厂族名画**同一个**盒（216…220）；地板降到 120 时立刻变成 118/119/132/124 的锯齿（M2 的实测） |
| 第 **12** 项 材质细节（内边框高光 / 玻璃折射边） | 「盒」与「面」分开量：盒 220，面 212…214——差的 6…8px 就是两侧 1px 发丝边 + 抗锯齿。pin 量的是**面**，所以断言写成 216…222 而不是 220 |
| 第 **9** 项 边界与截断（`#9` 自定义滚动条同族：收口处的手艺） | 截断点从「盒边」推到「真的放不下」：39 字族名从**第 189 列**被切 → 完整；tiny 台阶值本来就装得下（174 < 184.8），盒子一动不动 |

只取 4 项：这一轮是**一块面**的宽度来源，凑第 5 项就得往产品里加东西，而「细节打磨」的减法优先。

## 7. preflight 逐条（A / B / C / D2）

- **A 反套路**：不新增渐变、发光、纯黑、斜体（`noitalic_test.go` 仍绿）、三等分卡片；
  零文案改动（i18n 红线）。唯一的新视觉行为是「长值不再被省略号切」。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」；
  本轮判据一条——**右列是一条线，内容长了就借标签列，不切值**。与 §6.132（`Shrink(0)` +
  标签列 `Grow(1)`）同一条分工，本轮只是把「控件是固有宽度」这半句从 220 **固定**放宽成 220 **地板**。
- **C 动效抽查**：不动效；没有 `transition: all` 可违反；picker 的开合仍是框架原样。
- **D2 中文页**：不新增一个汉字（i18n 红线）；`zh` 的族名与 en 是同一批 ASCII 字体名，量值与 en
  逐值相同。既有缺口仍是 `tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动，两条不受影响。

## 8. 仍待

1. **下一格（首选）**：`paletteRow` 那枚字面 `Width(220)`（终端外观卡）。**要先量**它旁边的
   预览块（`Space(37)`=148u）在 large 台阶上还剩多少，以及调色板名最长到多少列——本轮按
   「一个表面一轮」把它留给终端外观卡自己那一轮。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2。
3. **需拍板（沿用）**：启动器两行结果行 36px 盒装 27 行墨 / `ui/tooltip.go:133` 禁用元素不弹
   提示 / §6.111 / 113-5 / 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾 / §6.129 的 129-5。
4. **新记一条（本轮量到，未改）**：`a.row` 的标签列在长值下会缩到 168.6u（large）。如果哪天
   插件表单出现「长标签 + 长选项」，标签会先被切——那时该给标签列一条地板（例如 120u），
   而不是让值无限借地。**现在不改**：出厂内容够不到。
