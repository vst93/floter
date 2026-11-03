# P13-fq 报告：两枚 picker 之间的那片空，是余数不是间距

**意图**：`paletteRow` 中间那片 89…160px 的空，要么是一条设计过的间距，要么是两个定值夹出来
的余数——**先量出来是哪一种，再决定动不动**。

选题来源：P13-fp 报告「仍待」第 1 条（原话见 `docs/rounds/P13-fq-spec.md` 开头）。
表面 = 设置页 General 页 Terminal appearance 卡的 `paletteRow`
（`internal/settingsui/settingsui.go:4756`）。合约新增 **§6.136**。

## 1. 先量（720 宽真实 General 页，四台阶 × 深浅）

行内容盒 = 卡内宽 − 2×`rowInset`（`rows.go:418-423`，`3.5u`，随台阶缩放）；picker 盒 =
`MinWidth(220)`（字面，不缩放，§6.133）；预览块 = `Space(37)`（随台阶缩放，§6.135）。

| 台阶 | 行内容盒 | picker 盒 | 预览块 | 空 | 空/块 |
|---|---|---|---|---|---|
| tiny | 498.6 | 220 | 118.4 | **160.2** | 1.35 |
| small | 489.8 | 220 | 133.2 | **136.6** | 1.03 |
| default | 481.0 | 220 | 148.0 | **113.0** | 0.76 |
| large | 472.2 | 220 | 162.8 | **89.4** | 0.55 |

- **它不是一条间距，是余数**：两个定值（220 字面地板、行内容盒）夹一个随台阶缩放的块，
  于是这片空跟着台阶从 160.2 摆到 89.4（**70.8px**），空/块的比例从 1.35 翻到 0.55。
  同一行在 tiny 上读作「左边一个控件、右边一块独立的预览」，在 large 上读作「一对」。
- **它和卡片其它两端对齐的行是同一条规则**：预览块的右沿与同卡「14 px」读数（`slider`
  标题行的第二件）的右沿**逐像素相同**（693.4 / 690.2 / 687.0 / 683.8，四台阶 × 深浅两边
  同值），都是行内容盒的右沿；picker 的左沿与行名同一条（194.8 / 200.4 / 206.0 / 211.6）。

## 2. 判：不改（零像素）

三条理由，缺一条结论就会变：

1. picker 的 220 是与同卡字体族 picker 共享的盒（§6.133/§6.134，P13-fo 刚钉死「两枚 picker
   同一个盒」）。让它 `Grow(1)` 吃余数，等于把上一轮刚统一的两条规则再拆开。
2. 预览块必须落在卡片控制列上（否则这一行的右沿从卡片右列里凹进去一块，右列不再是一条线）。
3. 于是余数只能待在两者之间。唯一另一种排法——让预览块吃余数（`Grow(1)`，空恒定、块变宽到
   250…280）——会把缩微终端画成一大片空底色（`contrast` 调色板下是一整块黑），是**可见的
   产品改动**，已进 `.pending-bugs.md`「需用户拍板」。

## 3. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/palettepair_test.go`（新） | 三条 pin（见下）+ 一条 opt-in dump |
| 2 | `docs/liquid-glass.md` §6.136 | 量的表 + 两条答案 + 为什么不改 |
| 3 | `.pending-bugs.md` | 「需拍板」新增「让预览块吃掉余数」 |

pin：

- `TestThePreviewBlockEndsOnTheCardColumn`——块右沿 = 卡片控制列（4 台阶 × 深浅，容差 0.5px）。
- `TestThePalettePickerSharesTheCardsPickerBox`——两枚 picker 同一个地板：源码两个 builder 都
  含 `.MinWidth(220)`（`paletteRow` 不许出现 `.Width(220)`）+ 画面 214…222px。
- `TestThePaletteHoleIsARemainderNotAToken`——四个余数值（容差 0.6px）+ 单调递减。

## 4. 门槛（实测）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 全绿
PASS 3419（基线 3416）| FAIL 0 | SKIP 196（基线 195，+1 = 本轮新增的 opt-in dump）| 测试函数 2593（基线 2589）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

台账 **V 0 / B 0 / S 0**（零像素、零行为；新增 pin 全是新文件）。
中途抓到自己的一次红：§6.136 表里第一行写了 `已量`，`internal/ci` 的合约测试
（`liquidglass_contract_test.go:94`）只认四个词——改成 `已落地（pin）` 后转绿。这是合约在
正确工作，不是放水。

## 5. mutation（实跑）

- **M1**：删掉 `paletteRow` 的 `.Justify(ui.SpaceBetween)` → `TestThePreviewBlockEndsOnTheCardColumn`
  **8 处红**，原始报错串：
  ```
  step=tiny/dark=false: the preview block ends at 533.2 but the card's control column is at 693.4 (-160.2px): the palette row's right end is not the card's right end
  ```
- **M2**：预览块宽度 `Space(37)` → `Space(30)` → `TestThePaletteHoleIsARemainderNotAToken`
  **8 处红**，原始报错串：
  ```
  step=tiny: the leftover is 182.6px, the pinned baseline is 160.2px: the row's spacing changed — if the leftover became a token, say so in the contract and re-pin it
  ```
- **M3**：`paletteRow` 的 `.MinWidth(220)` → `.Width(220)` → `TestThePalettePickerSharesTheCardsPickerBox`
  **2 处红**：
  ```
  paletteRow no longer floors its picker at 220: the card's two pickers are back to two rules
  paletteRow pins its picker to a literal 220 again: 220 is a floor, not a ceiling (P13-fn, §6.133)
  ```
- 三条均 `cp` 还原 → `md5sum -c`（`settingsui.go: 成功`，`3635f1c9b1c3f65272fbbd6594987a7b`）→
  `touch` → 复跑全绿。

## 6. 出图

`FLOTER_PNG=1 go test ./internal/settingsui -run TestDumpP13FQPaletteGap` →
`/tmp/floter-P13-fq-palettegap-{tiny,small,default,large}-{light,dark}.png`（720×320，只画这一行）。

**这张图上量到的一处变化**（像素读，零像素改动所以是「同一张图上的两个台阶」而不是前后）：
tiny/light 下，胶囊的受光面在 x=249 结束、预览块的左沿在 x=365.4，中间 **115 列纯底面无墨**；
large/light 下同一段从 x=253 到 x=316.8，只剩 **63 列**。同一行在两个台阶上的「空」差了近一倍
——这就是「余数」在画面上的样子。

## 7. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **2** 项 空间节奏（一个盒子只占它需要的地） | 把「空」拆成三个已知量：220 字面 + `Space(37)` + 行内容盒（498.6→472.2）；余数 160.2/136.6/113.0/89.4 全部写进合约，不再有「说不清的大空」 |
| 第 **4** 项 布局与让位顺序（谁先让） | 两端各有锚：picker 左沿 = 行左沿、块右沿 = 卡片控制列（与 `14 px` 读数逐像素同值）。余数被夹在两个锚之间，谁都不许先让 |
| 第 **12** 项 材质细节（盒与面分开量） | 断言量的是**布局 rect**（块右沿、行左沿）而不是墨；胶囊的宽度用「受光面并集 + 两端抗锯齿容差 214…222」单独量，不把抗锯齿当成盒宽 |
| 第 **6** 项 编辑细节（hairline 与刻度） | 余数不是「差不多」而是四个钉死的值 + 单调递减：任何把它变成 token 的改动都会红 |

只取 4 项：这一轮是**一个余数**的判定，凑第 5 项就得往产品里加东西（比如给这一行加分隔线），
而「细节打磨」的减法优先。

## 8. preflight 逐条（A / B / C / D2）

- **A 反套路**：零像素、零文案、零渐变/发光/纯黑/斜体（`noitalic_test.go` 仍绿）；没有新增卡片
  或三等分横排。唯一的产物是 pin 与文档。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」。
  本轮判据一条——**两件属于一对的东西，两端的锚必须各自是真的**：picker 的锚是行的左沿（与行名
  同一条），预览块的锚是卡片的控制列（与读数同一条）。量完两条都成立，所以这一行不需要动。
- **C 动效抽查**：不动效；无 `transition: all` 可违反；picker 开合、内容过渡仍是框架原样。
- **D2 中文页**：不新增一个汉字（i18n 红线）。zh 的调色板名（`跟随应用` 等）比 en 短，行的两个
  锚与 en 逐值相同；`paletteRow` 的标签表是参数，量值与语言无关。既有缺口仍是
  `tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」——**需拍板**，本轮不动。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动，两条不受影响。

## 9. 仍待

1. **下一格（首选）**：`chooseSelect` 的 `Shrink(1)`——P13-fp「仍待」第 4 条。现在它没病
   （尾随行由标签列 `Grow(1)` 让位），但本轮已把「两枚 picker 同一个地板」写成源码 pin；
   下一个「两个孩子两端对齐」的调用点出现时同一条病会复发。**先量**：给它补 `Shrink(1)` 在
   出厂值上是否零像素（大概率是），再把「让位顺序」也钉成源码 pin。
2. **需拍板（本轮新增）**：让预览块吃掉余数（`Grow(1)`，空恒定、块 250…280）——可见的产品
   改动，见 `.pending-bugs.md`。
3. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2；启动器两行结果行
   36px 盒装 27 行墨；`ui/tooltip.go:133` 禁用元素不弹提示；§6.111 / 113-5 / 114-4；浮层页头
   字号上阶梯（`104-2`）；便条行 rank 1.28；终端会话抽屉失败只加字不换墨；多选箱宽度；
   路径截断保头弃尾；会话行 56 地板；终端弧 S1b / S1c / S2 尾；§6.129 的 129-5。
