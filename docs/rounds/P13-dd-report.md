# P13-dd 报告：设置页的字号从「body ± N」搬到阶梯上

## 一句话

`internal/settingsui` 里 24 处「比正文大一点 / 小一点」的字，原本写成 `t.FontSize ± N`
（偏移量不随界面步长缩放），本轮全部换成阶梯档位，并把 `internal/ci` 的字阶账从
`pluginsheet.go` 一个文件扩到**整包**；顺带量到上一轮「会话行 56 满足参考地板」的结论
是拿库存字取景量的，生产真数是 **49**（欠 7px，下一格）。

## 先量（`ui.Tester`，参考字体，四个步长上的行盒）

| 表达式 | tiny 0.8 | small 0.9（出货） | default 1.0 | large 1.1 |
|---|---|---|---|---|
| `t.FontSize`（body） | 9.60 / **15** | 10.80 / **17** | 12.00 / **18** | 13.20 / **20** |
| `t.FontSize-1` | 8.60 / **13** | 9.80 / 15 | 11.00 / 17 | 12.20 / 19 |
| `tokens.Caption` | 8.80 / **14** | 9.90 / 15 | 11.00 / 17 | 12.10 / 19 |
| `t.FontSize+1` | 10.60 / **17** | 11.80 / 18 | 13.00 / 20 | 14.20 / 22 |
| `tokens.Emphasis` | 10.40 / **16** | 11.70 / 18 | 13.00 / 20 | 14.30 / 22 |
| `t.FontSize+2` | 11.60 / **18** | 12.80 / 19 | 14.00 / 22 | 15.20 / 23 |
| `tokens.Title` | 11.20 / **17** | 12.60 / 19 | 14.00 / 22 | 15.40 / 23 |
| `t.FontSize-3` | 6.60 / 10 | 7.80 / 13 | 9.00 / 14 | 10.20 / 15 |
| `t.FontSize-0.5` | 9.10 / 14 | 10.30 / 15 | 11.50 / 18 | 12.70 / 19 |

结论：**tiny 档上三级字同时脱离阶梯**（13 对 14、17 对 16、18 对 17）；出货档上偏移量
与阶梯几乎重合，差 0.1 点；`-3`（`●` 盘）与 `-0.5`（卸载对话框）根本没有对应档位。

## 改了什么

- `rows.go` 4 处、`settingsui.go` 18 处、`transfer.go` 1 处：`t.FontSize±N` → `tokens.<档>`
  （`+1`→`Emphasis`、`+2`→`Title`、`-1`→`Caption`）。
- 两个具名例外，各自写明理由并被新账点名接受：
  - `markDot(t)`（`settingsui.go`，`●`/`○` 两处）：一枚盘是字形不是文字档位，要装进
    caption 行盒，所以是 body − 3；
  - `uninstallSubBody(t)`（`uninstall.go`，原有函数，补注释）：参考把它定义成「正文减
    半格」（`ComponentizedUninstallDialog.css` 的 10.5px），阶梯上没有这一档。
- `transferNotice` 从自由函数改成 `(*App).transferNotice`：它需要 `a.tokens(c)` 取阶
  梯（原来只读 `c.Theme()`）。
- `internal/ci/fontladder_contract_test.go`：账从单文件扩到整包（`../settingsui/*.go`，
  非 `_test.go`），三条判据——① 任何 `….FontSize ± …` 的算式在整包内为红（例外只在
  两个具名函数体内）；② `FontSize(...)` 的参数只能是阶梯档、自己的正文档、或具名例外
  的调用；③ 整包 ≥30 处 `FontSize` 且例外名单里的函数仍然存在。

## 台账

- **V 2**：`sessionrowbox_test.go` 的会话行 56 → **49**（标题内边距 9 → **8**），
  `extensionsection_test.go` 的集成标题行改用装好设计主题的取景（断言不变，因为两条
  字的字号回到了同一阶梯）。两条的原因**同一个**：夹具没装设计主题。
- **S 2**：`internal/ci/deadcode_baseline.txt` 拿掉 `internal/settingsui/raycast_test.go
  renderThemed`（账面 24 → 23）——本轮在 `view_test.go` 里写了一个同名但不同作用的取景器
  （装好设计主题的 `render`），它有两个真实调用点；`sessionrowbox_test` 的 pin 改名
  `…MeetsTheReferencesFloor` → `…HeightAgainstTheReferencesFloor`（它现在钉的是真数，
  参考地板**没被满足**，名字不能再这么说）。
- **B 0**：没有一条行为/度量 pin 需要改。

## 取景：第四次现形（本轮最重要的发现）

`settingsui` 的测试都用 `render()`，它**没有** `c.SetTheme`；生产里 shell 在画面板前就
`c.SetTheme(surface.Tokens.Theme)`（`internal/shell/shell.go:4262`）。于是：

- 生产：`c.Theme().FontSize` = 阶梯 body（出货档 **10.8**）；
- 夹具：`c.Theme().FontSize` = 框架库存 **14**。

同一个函数里一半的字读 `a.tokens(c)`（10.8 一系）、一半读 `c.Theme()`（14 一系），
夹具里混排 11.7pt 与 14pt——量出来的行高既不是生产的、也不是参考的。P13-da 就是在这个
混合取景里得出「56 由内容自然满足」的；生产真数是 **49**（8 + 18 + 1 + 15 + 8），
比参考的 `.session-manager__row { min-height: 56px }` **少 7px**。

本轮只为需要真数的两条 pin 加了 `renderThemed`（`view_test.go`），`render()` 本身要不要
整体收口（会影响四十多条在量库存字的 pin）留给下一轮。

## 视觉证据

`/tmp/floter-P13-dd-settings-{tiny,small}-{light,dark}-{before,after}.png`（`General` 页，
760×900，`renderThemed` 取景；before 来自 HEAD 的 worktree，同一份取景器）。

- 出货档差分：浅 **50019** px / 深 **51402** px，bbox 覆盖整页——这不是布局变了，是
  0.1 点的字宽差遍布全页（`RunValues` 一类的 12.8 → 12.6）。
- 文本盒对照（112 个盒子）：**位置移动 60 个、行盒高度变 14 个，全部是 −1 或 −2px**，
  没有一处换行变化（段落盒 H 只有 section 容器 −4，是行盒 −1 累计）；
  其中 `Hide when focus is lost` 19 → 18、提示行 17 不动。
- tiny 档差分：浅 **154327** px / 深 **157726** px——这一档是本轮真正要修的那一档
  （三级字从 13/17/18 回到阶梯的 14/16/17）。
- 眼睛复核：`/tmp/floter-P13-dd-settings-small-light-{before,after}.png` 里
  「Launch at startup」这一行，after 的标题比 before 矮 1px、与它下面的说明行贴得更紧
  ——这正是 `Ellipsis`/行盒回到阶梯 18 的结果，不是控制槽或卡片框的变化。

## 门槛（实测）

```
gofmt -l cmd internal                     空
go vet ./... / GOOS=windows / GOOS=darwin 净
GOOS=linux|windows|darwin go build ./...  过
go test -count=1 ./...                    FAIL 0
PASS 3309（基线 3308）| SKIP 147（145 + 2 条 opt-in dump）| 顶层测试函数 2461 → 2464（+2 dump、+1 例外账；HEAD worktree 同口径实测）
```

## mutation（3 条实跑，逐字节还原 + `touch` 复跑绿）

1. M1：`rows.go:181` 退回 `t.FontSize + 2` →
   `../settingsui/rows.go:181:31: the size is an expression ('t.FontSize + 2'), not a rung of the ladder …`
2. M2：`settingsui.go` 的 `issueUnder` 改成 `tokens.Caption + 1` →
   `../settingsui/settingsui.go:7242:2: the size is an expression ('tokens.Caption + 1'), not a rung of the ladder …`
   （这条第一次跑**没红**——账的 `BinaryExpr` 臂当时只盯着 `.FontSize`，本轮补上「偏移
   阶梯档」的那一半才算真钉住。）
3. M3：把 `markDot(t)` 换成调用点上的 `t.FontSize - 3` →
   `../settingsui/settingsui.go:1361:32: the size is an expression ('t.FontSize - 3') …`
   （例外只让**函数体**里有偏移，调用点写偏移照样红。）

`md5sum -c` 三次全部「成功」，`touch` 后复跑 `-run "Rungs|Ladder"` 绿。

## 偷了什么（qiaomu-design）

- 读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md`、`preflight.md`
  （本轮是「已有方向，只要打磨」车道，不走四方向预览，也不出 HTML）。
- `craft-loop.md` 的工艺密度菜单：取**第 3 项「把量出来的数写进合约再改」**——本轮先把
  四个步长的行盒列成表（上表）再动代码，落成 `internal/ci` 的整包 AST 账；
  取**第 7 项「不要用近似值代替名字」**——落成 `markDot`/`uninstallSubBody` 两个具名
  例外（偏移只能出现在有理由的函数体里）；取**第 12 项「一次只收一族」**——本轮只收
  `internal/settingsui`，`internal/launcher` 的同一族（`launcher.go:1535,1633,2646,2743`、
  `output.go` 五处、`filters.go:235`、`triggerhint.go:153`）留给下一轮，不跨表面。
- `preflight.md`：A（反 AI 套路）过——本轮没有「渐变白边/毛玻璃/noise」这类替代品；
  B（设计读取一致性）部分过——阶梯与 `theme.Tokens` 一致，但**取景**不一致（见上）；
  C（动效抽查）不适用（字号不动效）；D2（中文页 ≥14px）**不能过**：本仓正文 10.8、
  caption 9.9，这是长期悬着的那份证据（第 30 份），仍需拍板。

## 仍待（P13-dd）

1. **下一格（不需拍板，首选）**：会话行比参考的 56 地板少 7px（生产 49）。补一条
   `min-height = Space(14)`（56u）让行回到参考高度；顺带把 `render()` 的取景收口
   ——两者同一轮做，因为补地板之后 pin 的数才有意义。
2. **同族尾巴（不需拍板）**：`internal/launcher` 的同一族偏移量（`launcher.go:1535/1633/2646/2743`、
   `output.go:345/695/980`、`filters.go:235`、`triggerhint.go:153`），量法与本轮相同；
   注意 `output.go:324` 的注释写着「字阶不动因为改行盒就改高度链」，要逐处量过再动。
3. **需拍板（沿用 + 第 30 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight
   D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端页会话抽屉 / 标签对失败只加字不换墨；扩展页空态只有一行结论
   没有出路；会话页空态说错对象；终端外观抽屉行要不要面；`ui/tooltip.go:133` 禁用元素
   不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾；终端弧 S1b /
   S1c / S2 尾。
