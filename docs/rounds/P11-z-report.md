# P11-z 报告：红线只回答语义 + 终端画布不再套第二圈框（用户实测 F2 + F3）

## §1 意图与证据

用户实测原话（2026-10-28，最高优先级）：

- **F2**「整体红色线条太扎眼」——红色（边框/强调线）视觉上过重。
- **F3**「终端页面外圈那圈类玻璃边偏离『轻』的质感」——终端视图外围一圈类玻璃边框。

两者是同一片界面质感，本轮一起做。动手前的证据（`git log` = `6c38930`）：

| 位置 | 旧值 / 旧结构 |
|---|---|
| `internal/theme/theme.go:648` | 浅色 `strokeActive: ui.RGBA(229, 72, 77, 0.55)` |
| `internal/theme/theme.go:727` | 深色 `strokeActive: ui.RGBA(245, 84, 78, 0.55)` |
| `internal/shell/shell.go:4260-4264`、`:4345` | `cardFocused()` 为真 → 整张启动器卡的 `Border(1, InputStrokeActive)` |
| `internal/terminalui/terminalui.go:873-876`、`:888` | 终端画布同样的「有焦点 → `InputStrokeActive`」+ `Border(1, edge)`（面板 12 里套画布 10） |
| `internal/shell/cardedge.go:96-100` | Linux 聚焦卡另加 `inset 0 0 0 1px AccentGlow` |

判据是 `docs/raycast-design.md` §5 自己那句话：「accent 只能回答『哪个被选中 / 哪个
是开的』」。「整个窗口有键盘焦点」两件都不是——启动器在用时永远有焦点，那圈红因此是
**常量装饰**。

**动手前实测（像素，`/tmp/floter-P11-z-before-red-light.png`）**：卡边 1px 是
`(227,141,145)`（粉红），整张卡里 `R−B>16` 的暖像素 **2237** 个，其中 4 条边占
`2×(702+402)=2208`。

## §2 改了什么

### F2 红线

1. `internal/theme/theme.go`：`strokeActive` 浅色 `ui.RGBA(229,72,77,0.55)` →
   `ui.RGBA(0,0,0,0.16)`；深色 `ui.RGBA(245,84,78,0.55)` → `ui.RGBA(255,255,255,0.16)`。
   静息 `stroke` 是 0.09，所以聚焦仍是「亮一档」——**靠明度差说，不靠色相说**（§5 的
   「预算只约束色相，不约束明度」）。
2. `internal/shell/cardedge.go`：删掉 Linux 聚焦卡的 `inset 0 0 0 1px AccentGlow`。
   同一个「窗口有焦点」语义的第二圈红；焦点保留 `EdgeHighlightActive` 的亮顶边与
   `SheenAtFocus` 的亮面。
3. `internal/theme/theme.go` 的 `Tokens.AccentGlow` 标 `Deprecated`、**字段不删**
   （同 P8-e 对 `AccentTintQuiet` 的处理）。
4. **没动**（合法用途，逐条点名）：`AccentEdge`/`AccentEdgeStrong`（分段控件选中段
   keyline `internal/control/control.go:124`、主 CTA `internal/settingsui/rows.go:481`）、
   侧栏前导图标、开关 ON、权限 ●、`Warning` 语义色、框架 `FocusRing`（控件级）、
   `Selection`（文本选择）。

### F3 终端画布

5. `internal/terminalui/terminalui.go`：删掉画布的 `Border(1, edge)` 与 `edge` 计算。
   **面板是这一页唯一的框**，画布只留 `Radius(mountCorner)` 的填充；没加 rim/sheen/
   高光/渐变，也没做 floating/overlay 观感。

### 顺手查到的旧账（写进 §6.6）

`theme.For` **从未给 `Tokens.AccentGlow` 赋值**（构造块只有 `AccentEdge`/
`AccentEdgeStrong`/`AccentHalo` 三行），所以 Linux 那圈 inset 环一直用零值颜色画，
**从来没真的画出来过**；旧 pin `cardedge_test.go:72` 拿 `tokens.AccentGlow`（零值）
去比，因此永远绿。本轮删掉那条路径，并在新 pin 里改成「任何 accent 系 token 与任何
inset 环都不许出现」。

## §3 V / B / S 台账

- **V = 2**（视觉数值重定基线，断言钉到新值）：
  - `internal/theme/theme_test.go`：`TestStrokes` 的注释「The focused step takes the
    accent in both.」→ 新基线说明（断言本身仍成立：聚焦档 ≠ 静息档）。
  - `internal/shell/cardedge_test.go`：`TestTheCardEdgeBrightensWithTheKeyboard` 的
    Linux 段，旧「3 条、首条是 `Spread:1` 的 `AccentGlow`」→ 新「2 条、顶边是
    `EdgeHighlightActive`」；注释写 `P11-z 重定基线：旧值 → 新值`。
- **B = 0**：其余测试零改动全绿——含 §5 的四条正面 pin
  （`TestTheSidebarChosenGlyphKeepsTheAccent`、`TestTheChosenSegmentKeepsItsAccentKeyline`、
  `TestTheSwitchOnStateCarriesTheAccent`、`TestThePermissionDotAndTheRegisterChipKeepTheAccent`）、
  `TestTheTerminalViewsPaintFromTokensNotLiterals`、`panelCorner`/`mountCorner` 几何 pin、
  `TestTheFocusFaceAndTheFocusStrokeMoveTogether`。
- **S = 0**：结构只少了两条描边调用，没有新结构。

新增 pin（正面 + 反面成对）：

| pin | 文件 | 正面 | 反面 |
|---|---|---|---|
| `TestTheFocusedCardEdgeIsNeutral` | `internal/theme/theme_test.go` | 聚焦档与静息档不同（0.16 vs 0.09） | 三通道差 ≤ 8（无彩度） |
| `TestTheFocusedCardDrawsNoAccentRing` | `internal/shell/cardedge_test.go` | 三平台 × 两外观的聚焦卡仍是 2 条边 | 没有 `Spread==1` 的 inset 环、没有 accent 系颜色 |
| `TestTheTerminalCanvasDrawsNoSecondFrame` | `internal/terminalui/redframe_test.go` | 页面最外一列 == 画布填充（无描边） | `R−B ≤ 16`（不是 accent 红） |

测试函数：1902 → **1907**（+5：3 条 pin + 2 条 opt-in dump）。无 `t.Skip` 新增以外的删减。

## §4 门槛（实测，2026-10-28）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2346
--- SKIP                                  # 17（15 + 两条新 dump）
测试函数数                                 # 1907
GOOS=linux/windows/darwin go build ./...  # 三平台过
```

**基线核对**：任务书写的起点是 2344（P11-y 报告）。我用 `git worktree add /tmp/p11z-head
HEAD` 在**未改动的 HEAD（`6c38930`）上重跑同一条命令，实测 2343**；本轮 +3 条会 PASS
的新 pin = 2346，两条 dump 是 SKIP。所以报告里的基线是**实测 2343**，不是抄来的 2344
（P11-y 那个数比实测多 1）。

## §5 mutation（实跑，全部变红并逐字节还原）

M1 把 `strokeActive` 改回 accent 红（`internal/theme/theme.go`）：

```
--- FAIL: TestTheFocusedCardEdgeIsNeutral (0.00s)
    theme_test.go:176: dark=false: the focused edge {229 72 77 140 [0 0 0]} has channels 157 levels
    apart: it carries a hue, and §5 lets the accent answer only 「哪个被选中 / 哪个是开的」
```

M2 把 Linux 的 `AccentGlow` inset 环加回 `cardEdge`（`internal/shell/cardedge.go`）：

```
--- FAIL: TestTheCardEdgeBrightensWithTheKeyboard (0.00s)
    cardedge_test.go:74: dark=false: the focused Linux card edge = 3 strokes, want the rim's 2: [...]
--- FAIL: TestTheFocusedCardDrawsNoAccentRing (0.00s)
    cardedge_test.go:110: dark=false linux: the focused card draws an inset ring: {X:0 Y:0 Spread:1 ...}
```

M3 把画布的 `Border(1, InputStrokeActive)` 加回（`internal/terminalui/terminalui.go`）：

```
--- FAIL: TestTheTerminalCanvasDrawsNoSecondFrame (0.11s)
    redframe_test.go:83: dark=false: the canvas's column 0 is {207 207 208 255 ...} but its face is
    {255 255 255 255 ...} — a stroke is drawn there
```

三条都 `cp` 备份还原后 `md5sum -c /tmp/p11z-md5.txt` 三个文件全部「成功」，并
`touch` 后复跑一次全绿（避免测试缓存）。

> 教训（写在这里，下一轮别再踩）：第一次做 M1–M3 时我用 `git checkout <file>` 还原，
> 而改动**未入索引**，于是它把文件回退到了 HEAD——本轮三处产品改动当场丢失、md5 三条
> 全「失败」。重做后改成 `cp` 到 `/tmp` 再 `cp` 回来。**mutation 的还原不许用
> `git checkout`。**

## §6 出图与「我实测看到了什么」

`FLOTER_PNG=1` 跑 dump（深/浅各一张）：

| 文件 | 表面 |
|---|---|
| `/tmp/floter-P11-z-red-light.png` / `-dark.png` | 启动器卡（有键盘焦点，已查 `ls`） |
| `/tmp/floter-P11-z-terminal-light.png` / `-dark.png` | 终端页（有会话） |
| `/tmp/floter-P11-z-before-red-*.png` / `-before-terminal-*.png` | 改动前同两帧（对照） |

启动器卡的 dump 先把透明帧合成到一块中性桌面（浅 `#DFDFE4` / 深 `#1C1C20`），否则卡
是半透明的、像素是预乘 alpha，读不出边的颜色。

**我实测看到的（像素读数，不是形容词）**：

1. 卡的左边框中点（x=9, y=210）：浅色 `(227,141,145)` → `(187,187,192)`；深色
   `(148,59,57)` → `(64,64,68)`。即那条 1px 从「粉红/暗红」变成了**无彩度的灰**，
   并且仍比卡面亮一档（浅色卡面 `(236,236,240)`、深色 `(28,28,31)`），所以「有焦点」
   还看得出来。
2. 整张卡里 `R−B>16` 的暖像素：**2237 → 25**。剩下那 25 个全在 `x=37, y=26..50`
   一列 1px 宽的竖线上——那是**搜索框的文本光标**（框架 `ui/editor_layout.go` 用
   `Theme.Accent` 画 caret），不是描边。它是否也改中性已写进拍板项。
3. 终端页最外一列（x=0, y=310）：浅色 `(225,225,226)` → `(255,255,255)`（== 画布填充
   `x=4` 的颜色）；深色 `(36,36,38)` → `(24,24,27)`（== 画布填充）。也就是说画布那圈
   1px 描边、以及描边与面板之间那条 1px 的面板底色带**一起消失了**：终端页现在是
   一整块填充，面板的圆角只在四个角上留下半径 12 与 10 之间的那一小段弧。
4. 四个角：改动前 `y=0` 整行是面板底色（浅 `(247,247,248)`），改动后 `y=0` 就是画布
   填充（浅 `(255,255,255)`）——**双层框的「第二层」在像素上已经不存在**。

## §7 仍待 / 需拍板

1. **拍板项（新增）**：搜索框的文本光标要不要也改中性？现在它是
   `ui.Theme.Accent`（红），是全仓最后一处「常量红线」。它不是描边、只有 1px 宽，
   而且回答「文字从这里进去」；但用户说的「整体红色线条」里它确实算一条。改它要动
   `theme.For` 里 `t.Focus`/`t.Accent` 的分工（框架的 caret 用 `Theme.Accent`，同一个
   token 还喂着开关 ON / 主 CTA），属**行为/语汇**改动，不擅自做。
2. 上一轮「仍待」第 2 条起（需拍板清单）原样不变。
3. `docs/raycast-design.md` §5 第 4 条已按本轮定审改写（焦点环只到**控件级**），
   §6.6 记了本轮的两条改动与那笔 `AccentGlow` 旧账。
