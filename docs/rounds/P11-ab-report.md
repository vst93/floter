# P11-ab 报告：搜索行补上前导放大镜——字段与结果行共用一条文字列

规格：`docs/rounds/P11-ab-spec.md`。合约更新：`docs/raycast-design.md` §4.2、§6 末尾、§6.8。

## §1 为什么是这一条

选题顺序（任务书）：用户实测反馈 → 合约未落地条目 → 上一轮「仍待」→ 界面质感与交互。

- 用户实测三条已经收完：F1（终端重放上一个会话的 VT 状态，`P11-y`）、F2 + F3
  （红线扎眼 / 终端玻璃边，`P11-z`）、F2 验收里的「可读性达标」（`P11-aa`）。
  `.pending-bugs.md` 顶部三条现在都是「已修」。
- 合约 §6 全表 `已落地`，但 §4.2 与 §6 末尾各留了一处「启动器自己没做」：
  **搜索行的前导放大镜**、**`⌘K` 动作面**。后者要动键盘模型（合约明写「键盘模型不动」），
  属拍板项；前者只是墨与几何的补完，且在主表面上**每次打开启动器都看得见**。
- 旧理由（「插标记会把文本列推右 22u」）只在「把标记塞进字段行的 16u padding 里」时成立。
  本轮换落法：标记占**结果行那 28u 的图标列 + 10u gap**，于是查询落到 54u——
  结果行标题的列。**这不是新开一列，是把两列并成一列。**

## §2 改了什么

| 文件 | 位置 | 改动 |
|---|---|---|
| `internal/launcher/launcher.go` | `1266-1295`（field row，scope mark 分支之前） | 无 scope 时画前导放大镜：`ui.Box` 28u 方框（`Space(7)`）、居中 16u（`Space(4)`）`Glyph("search")`、墨 `t.TextMuted`、右 margin `Space(2.5)`（10u）。注释从「本片不画前导标记」改写成「槽里只有一样东西」 |
| `internal/launcher/launcher.go` | 同段 scope mark 分支 | **零改动**（保持自然宽度；scope 拥有字段时放大镜让位） |
| `internal/launcher/actionbarfooter_test.go` | `TestTheFieldAndTheRowsShareTheReferenceTextColumn` | V 重定基线：caret `16` → `54` |
| `internal/launcher/actionbarfooter_test.go` | 新增 `TestTheSearchRowLeadsWithANeutralMark` | 墨落在 16..44 的图标格内、44..53 的 10u gap 为空、墨的暖像素为 0 |
| `internal/launcher/searchrowpng_test.go` | 新增 | opt-in PNG dump（深/浅各一张，卡合成到中性桌面） |
| `internal/shell/searchcolumn_test.go` | 三条 pin | V 重定基线：查询 `16u` → `54u`（两条）；`TestThePlatformInsetIsTheReferenceUnitLadder` 的**取样点**改到卡内左缘（占位符现在从 54u 起，正好盖住原来的卡片中缝取样点） |

产品代码只动了一处：`internal/launcher/launcher.go` 的 field row。

## §3 V / B / S 台账

- **V = 4 条**（视觉数值，断言钉到新值，注释写「P11-ab 重定基线：旧值 16(u) → 新值 54(u)」）：
  1. `internal/launcher` `TestTheFieldAndTheRowsShareTheReferenceTextColumn`（caret 16 → 54）
  2. `internal/shell` `TestTheSearchColumnIsTheReferenceUnitsAtTheShippedStep`（查询 16u → 54u）
  3. `internal/shell` `TestTheSearchPageFillsAWiderWindow`（同上，900/1200/1600 三个宽度）
  4. `internal/shell` `TestThePlatformInsetIsTheReferenceUnitLadder` —— **不是放宽断言**：
     它量的是「字段行的填充段 == 卡片内容盒」，而取样点原来是卡片中缝的像素；占位符
     从 54u 起之后中缝那个像素落在了字形里，取到的「填充色」变成了字的抗锯齿色。
     取样点改到卡内左缘 `left+2`，断言本身一个字没动。
- **B = 0 条**：高度链、键盘模型（`fieldKeys` / `pendingCaret` / `HandleInput`）、
  会话管理、设置写回、i18n 文案、命令接线零改动；`TestScopeMarker` 等行为 pin 零改动全绿。
- **S = 0 条**：没有结构变化。
- 新增测试函数 2 条（1 条 pin + 1 条 opt-in dump），删除 0 条。

## §4 mutation（实跑，还原一律 `cp`）

备份：`cp internal/launcher/launcher.go /tmp/p11ab-launcher.go.bak` +
`md5sum > /tmp/p11ab-md5.txt`（`dd17805882e4eebe3f9866133afe8752`）。

**M1 · 删掉放大镜的 10u 右 margin**（`:1292` `t.Space(2.5)` → `0`）：

```
--- FAIL: TestTheFieldAndTheRowsShareTheReferenceTextColumn (0.08s)
    actionbarfooter_test.go:259: the field's caret starts at x=44, want the rows' title column 54
--- FAIL: TestTheSearchRowLeadsWithANeutralMark (0.00s)
    actionbarfooter_test.go:315: 76 ink pixels in the 10u gap (x=44..53): the mark is not in the icon cell
```

**M2 · 把放大镜的墨改成 accent**（`:1294` `t.TextMuted` → `t.Accent`）：

```
--- FAIL: TestTheSearchRowLeadsWithANeutralMark (0.07s)
    actionbarfooter_test.go:318: 84 of the leading mark's 95 ink pixels are warm (R−B beyond 12): it carries accent ink
```

**M3 · 让标记不画**（`:1289` 的条件换成 `Glyph("nope")`，未知名字返回 `nil,false`）：

```
--- FAIL: TestTheFieldAndTheRowsShareTheReferenceTextColumn (0.07s)
    actionbarfooter_test.go:259: the field's caret starts at x=16, want the rows' title column 54
--- FAIL: TestTheSearchRowLeadsWithANeutralMark (0.01s)
    actionbarfooter_test.go:315: 65 ink pixels in the 10u gap (x=44..53): the mark is not in the icon cell
    actionbarfooter_test.go:318: 25 of the leading mark's 227 ink pixels are warm (R−B beyond 12): it carries accent ink
```

三条都 `cp /tmp/p11ab-launcher.go.bak` 还原后 `md5sum -c /tmp/p11ab-md5.txt` 报「成功」，
`touch internal/launcher/launcher.go` 后复跑六条相关 pin（`internal/launcher` 两条 +
`internal/shell` 三条 + `TestScopeMarker`）全绿。**没用 `git checkout`**（P11-z 的教训）。
M3 的注入态顺手拿来当「改动前」帧出图（见 §6）。

## §5 门槛（全部实测）

```
gofmt -l cmd internal          → 空
 go vet ./...                  → 净
 GOOS=windows go vet ./...     → 净
 GOOS=darwin  go vet ./...     → 净
 go test -count=1 -v ./...     → EXIT=0，FAIL 0
 PASS 2350（起点记录 2349，P11-aa 实测；+1 = 本轮新增 pin）
 SKIP 19（起点 18；+1 = 本轮新增的 opt-in dump）
 测试函数 1911（起点 1909；+2 = 1 pin + 1 dump）
 GOOS=linux/windows/darwin go build ./...  → 三平台全过
```

## §6 出图与「我实测看到了什么」

`FLOTER_PNG=1 go test -run TestDumpP11ABSearchRowPNGs ./internal/launcher/`（720×488，
卡合成到中性桌面）：

| 文件 | 表面 |
|---|---|
| `/tmp/floter-P11-ab-searchrow-light.png` / `-dark.png` | 改动后的搜索行（无 query，占位符在 54u） |
| `/tmp/floter-P11-ab-before-light.png` / `-dark.png` | 改动前同两帧（M3 注入态，标记不画） |

**像素读数（不是形容词）**：

1. **放大镜真的画出来了，且就在图标格里**：浅色 `x=23..36, y=21..34`（95 个像素，
   行高 0..55 的垂直中心 27.5 —— 与 56u 行的中心对齐）。深色同一个盒子、同样 95 个像素。
2. **它不是 accent**：两套外观下这块墨的暖像素（R−B 超出 ±12）都是 **0**。浅色最深的墨
   `(113,113,122)` = `TextMuted`（填充 `(250,250,251)`），深色 `(161,161,170)`（填充
   `(28,28,31)`)——两套外观都读得清，且与占位符同档。
3. **查询列挪到了结果行标题的列上**：改动前，`y=28` 行上第一个非填充像素是 **x=16**
   （占位符的首字）；改动后是 **x=54**（文本框的光标），与结果行标题的 54 同列。
   改动前图标格 16..53 里全是占位符的字（292 个像素、其中 25 个暖像素是蓝光标），
   改动后 44..53 这 10u gap **一个像素都没有**。

**一个我不确定、已查过的点**：放大镜所在的那 38u 现在不可点。查过框架：这张卡的搜索框
没有 `FocusBind`/`AutoFocus`，它是窗口里唯一的 focusable，鼠标点到非 focusable 区不会
改变焦点——所以「点放大镜焦点丢了」不成立，也就没有加 `OnClick` 去抢焦点（那会引入
一个新的 focus 目标，属键盘模型改动）。


## §7 仍待（P11-ab）

1. **`⌘K` 动作面（合约 §4.6）现在是合约里唯一未落地的一处**，也是 §6 末尾最后一条。
   它要动键盘模型（合约明写「键盘模型不动，动作面属于 P8-b」）→ **需拍板**，本轮没动。
2. `docs/raycast-design.md` §4.6 写着页脚右侧的 `⌘K`/`Ctrl+K` 键帽「在本 build 里没有
   绑定」——**键帽画着、键不响应**是交互上的一处空承诺，与上一条是同一件事的两面。
3. 拍板清单原样有效：P11-aa 的「默认浅色安静档要不要推到 4.5:1」、P11-z 的「搜索框
   caret 要不要中性」、P11-y 的「上次会话的滚动缓冲要不要回来」，以及 P9-g 表那一批。
4. 侦察结论（别当 TODO 重领）：`internal/shell/searchcolumn_test.go` 里「在卡片中缝取
   字段行填充色」的取样手法本轮一并改掉了两处（`TestThePlatformInsetIsTheReferenceUnitLadder`
   与 `TestTheSearchPageFillsAWiderWindow`）——同一类脆弱点，同族一次收完。
