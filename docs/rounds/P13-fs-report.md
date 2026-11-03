# P13-fs 报告：空态那一对「标记与它那句」，三个调用点各量一次

## 0. 这轮做了什么（一句话）

§4.3 的「标记与它那句」只在会话页那一套字形上量过；本轮把**三个调用点各自真发的标记**
（`square-terminal` / `keyboard` / `package`）各量一次，量出 `keyboard` 那一处两档**相等**
（11 对 11，规则不成立），并第一次量了理由折行后的段内密度。**零像素改动**，两条 pin 进仓库。

## 1. 先量（`emptyState` 真画法，720 宽，深浅两套逐值相同）

| 调用点 | 标记 | 墨高 / 盒 | 标记→句子 | 句子→理由 | 结论 |
|---|---|---|---|---|---|
| 会话页（`PageSessions` 的 face） | `square-terminal` | 17 / 19.8 | 9 | 12 | 一对更近 3px |
| 自定义快捷键 `custom.go:50` | `keyboard` | 15 / 19.8 | 11 | 11 | **相等 → 标记读成第三个平级邻居** |
| 集成页 `settingsui.go:1861` | `package` | 17 / 19.8 | 9 | —（无理由） | 只有一对 |

机制：盒是一个数（`empty.go:52` `Space(5.5)` = 19.8px），墨按**字形自己的纵横比**落进去。
`keyboard` 的墨只有 15 行（`square-terminal` / `package` 17 行），同一个 4u 盒间隙在墨上多 2px，
正好落在理由那一对的 11px 上。

理由折行（本轮第一次量，en 折两行、zh 一行）：

| 量 | 值 |
|---|---|
| 段内墨缝 | **4px** |
| 行距（墨起点→墨起点） | **16.0px** |
| 盒高（2 行） | **30.0** |
| 句子→理由 | **11px** |
| zh 那句宽 / 度量 | 267 / 288 |

段内 4 < 11：折行的那段话仍是一段，这条成立。

**副产品（重要）**：`LineHeight(1.5)` → `LineHeight(1.2)` 后墨距仍 16.0、盒高仍 30.0——
`ui.Text` 的折行排版吃的是**字体自己的行进给**，`LineHeight` 在这两处是空操作。
这既让 M3 首版变成了一次**空变异**（见 §5），也成为合约 §6.138 的 138-4（删/留需拍板）。

## 2. 落地项

| # | 位置 | 落成 |
|---|---|---|
| 1 | `internal/settingsui/emptypara_test.go` | 新增两条 pin + 一条 opt-in dump（`FLOTER_P13FS_DIR`） |
| 2 | `docs/liquid-glass.md` | 新增 §6.138（138-1 / 138-2 已落地，138-3 / 138-4 需拍板） |
| 3 | 产品代码 | **零像素改动**：`empty.go` 的 md5 与 P13-fr 收尾时逐字节相同（`git status` 里没有它） |

pin 的判据（不是「非空」这种弱断言，是实测数）：

- `TestTheMarkHangsOffItsSentenceOnEveryShippedMark`：逐 arm 钉**墨高**（17 / 15 / 17）与
  **两档差**（会话 3px / 快捷键 0px），±1.1px；集成页无理由时不比较第二档。
- `TestAWrappedWhyIsStillOneParagraph`：`hintLineCount == 2`（en）与 `== 1`（zh）——
  度量没被改动；行距 16.0 ± 0.6；盒高 30.0 ± 0.6；段内墨缝比句子→理由**至少近 4px**。

## 3. 台账（V / B / S）

- **V 0**：没有重定任何视觉数值的基线（本轮零像素）。
- **B 0**：行为/度量/接线一条没动，既有 3422 条 pin 零改动全绿。
- **S 0**：结构未变。

## 4. 门槛（实测，本轮收尾）

```
gofmt -l cmd internal                        # 空
go vet ./...                                 # 净
GOOS=windows go vet ./...                    # 净
GOOS=darwin  go vet ./...                    # 净
go test -count=1 ./...                       # 3424 PASS / 0 FAIL（基线 3422 / 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3424（基线 3422）
GOOS=linux/windows/darwin go build ./...     # 三平台过（BUILDS_OK）
```

函数数 2596 → **2599**（新增 2 条 pin + 1 条 opt-in dump，dump 是 `t.Skip` 的截图导出，
已标注 opt-in）。SKIP 196 → 197（就是那条 dump）。

## 5. mutation（3 条实跑，逐条抄原始报错）

- **M1**：`empty.go` 的 `head` `Gap(t.Space(1))` → `Gap(t.Space(2))`（把「同一块」退回区域那一档）
  → `TestTheMarkHangsOffItsSentenceOnEveryShippedMark` 红（三个 arm 全红）：
  ```
  emptypara_test.go:139: sessions (dark=false): the mark hangs 12.0px off its sentence, measured 9.0 — the shipped mark's ink moved (it is 17 rows tall in a 19.8px box)
  emptypara_test.go:153: sessions (dark=false): the mark hangs 12.0px off its sentence and the why 0.0px further — this arm's pinned ladder is 3px (mark ink 17 rows in a 19.8px box)
  emptypara_test.go:139: shortcuts (dark=false): the mark hangs 14.0px off its sentence, measured 11.0 — ...
  ```
- **M2**：`iconSize` 默认 `t.Space(5.5)` → `t.Space(4)`（标记变小）
  → 同一 pin 红：
  ```
  emptypara_test.go:158: sessions dark=false: mark ink 12 rows, mark→sentence 8.0px, sentence→why 12.0px
  emptypara_test.go:139: shortcuts (dark=false): the mark hangs 9.0px off its sentence, measured 11.0 — the shipped mark's ink moved (it is 12 rows tall in a 19.8px box)
  emptypara_test.go:158: integrations dark=false: mark ink 13 rows, mark→sentence 8.0px, sentence→why 0.0px
  ```
- **M3（首版，记录为一次空变异）**：理由的 `LineHeight(1.5)` → `LineHeight(1.2)`
  → **一个 pin 都没红**（`ok  floter/internal/settingsui 0.092s`），墨距 16.0、盒高 30.0 逐值不变。
  这不是 pin 弱，是那条 `LineHeight` 本来就不参与折行排版——结论写进 §6.138 的 138-4。
- **M3′（换成量尺本身的变异）**：理由的 `MaxWidth(t.Space(80))` → `MaxWidth(t.Space(40))`
  → `TestAWrappedWhyIsStillOneParagraph` 红：
  ```
  emptypara_test.go:173: dark=false: the en why resolved 3 ink lines, want 2 — the measure or the copy moved, and this pin's subject is the wrap
  ```
- 三条（M1 / M2 / M3′）均 `git checkout internal/settingsui/empty.go` 还原 →
  `md5sum -c` 核对 `internal/settingsui/empty.go: 成功` → `touch internal/settingsui/emptypara_test.go`
  后复跑全绿。

## 6. 出图

`FLOTER_P13FS_DIR=/tmp/p13fs go test ./internal/settingsui -run TestDumpP13FSEmptyParagraphPNGs` →
`/tmp/p13fs/floter-P13-fs-emptyparagraph-{light,dark}.png`（720×560，三个 arm 竖排）。

**这张图上看到的一处变化**：本轮零像素改动，所以图是**证据**而不是 before/after——三块区域的
标记与它那句之间的空气，在 `square-terminal` / `package` 两块上读成一条**更紧**的缝，
而 `keyboard` 那一块的标记在图上明显比前两块离句子远一点（9 对 11px），正是 §6.138 记下的
那一处「两档相等」。

## 7. 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **12** 项 材质细节（盒与面分开量） | 「盒是一个数、墨是字形自己的」：`Space(5.5)` = 19.8px 的盒里，`keyboard` 墨 15 行、`square-terminal` / `package` 17 行；9 / 11 / 9 三个墨距全部写进合约 |
| 第 **6** 项 编辑细节（hairline / 编号表） | 三个调用点写成一张**编号表**（arm × 墨高 / 盒 / 两档缝），每行一个实测数，深浅两套逐值相同 |
| 第 **2** 项 空间节奏（一个盒子只占它需要的地） | 段内墨缝 4px、行距 16.0px、盒高 30.0、句子→理由 11px——一段话的三个数一次列清，不再只说「LineHeight(1.5)」 |
| 第 **4** 项 签名交互（一个足矣） | 一个决定：**判据落在「这一对」上，不落在「这一块」上**——同一个 4u 在不同字形上落在 9 与 11，规则必须按对量 |

只取 4 项：这一轮是**量尺轮**（零像素），第 5 项就要往空态里加东西，而 §0 的减法禁令优先。

## 8. preflight 逐条（A / B / C / D2）

- **A 反套路**：零像素、零文案、零渐变/发光/纯黑/斜体（`noitalic_test.go` 仍绿）；没有新增
  卡片或三等分横排。产物是两条 pin 与一节合约。
- **B 设计读取一致**：读取为「桌面命令面板的设置页，面向每天用键盘的人，轻便液态玻璃的语言」。
  本轮判据一条——**一条规则必须在它管的每个表面都成立**：量完发现它在 2/3 的表面上成立，
  第三处被写成 `需拍板` 而不是「看起来还行」。
- **C 动效抽查**：不动效。`ReduceMotion` 在探针里是开的；没有 `transition: all` 可违反。
- **D2 中文页**：本轮**不新增一个汉字**（i18n 红线）。zh 那句理由被量到 267px / 一行，
  是 en 折两行的边界；既有缺口仍是 `tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」——
  **需拍板**，本轮不动。
- **A4.1（Carbon 决策门）「disabled / read-only / hidden 语义没有混淆」**：本轮不碰状态面
  （P13-fi 已收口）；`138-3` 的两条备选路都写清了代价，没有偷偷选一条。
- **P-1（禁斜体）/ P-5（可读性优先）**：零墨色改动。

## 9. 仍待

1. **下一格（首选）**：**边界情况**——通用队列里 `间距节奏 / 状态反馈 / 空态满态 / 文案密度`
   四类都已收口，只剩「边界情况」与「像素细节」没有专轮。第一格建议
   `internal/settingsui/rows.go` 的 `cardRow` 控制槽（`Shrink(0)` 已在 §6.132 / P13-fk 钉住，
   剩下的是**窄窗 + 长标签 + 长值同时成立**的边界），不需要拍板。
2. **需拍板（本轮新增 2 条，见 `.pending-bugs.md`）**：138-3（按字形给 head gap / 整条收到 2u）、
   138-4（`LineHeight` 在这两处是空操作，删还是留）。
3. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2；启动器两行结果行
   36px 盒装 27 行墨；`ui/tooltip.go:133` 禁用元素不弹提示；§6.111 / 113-5 / 114-4；
   浮层页头字号上阶梯（`104-2`）；便条行 rank 1.28；终端会话抽屉失败只加字不换墨；
   `cardRow` 控制槽 `Shrink(0)` 的语义改动（§6.137 的 137-4）；多选箱宽度；路径截断保头弃尾；
   会话行 56 地板；§6.129 的 129-5；§6.136 的 136-4；终端弧 S1b / S1c / S2 尾。
