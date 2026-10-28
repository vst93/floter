# P11-ac 报告：列表的分组头落在它自己的文字列上

规格：`docs/rounds/P11-ac-spec.md`。合约更新：`docs/raycast-design.md` §4.4、§6.9。

## §1 为什么是这一条

选题顺序（任务书）：用户实测反馈 → 合约未落地条目 → 上一轮「仍待」→ 界面质感与交互。

- 用户实测三条（F1/F2/F3）已由 P11-y/z/aa 收完，`.pending-bugs.md` 顶部三条都是「已修」。
- 合约 §6 全表 `已落地`，唯一剩下的未落地条目是 `⌘K` 动作面（§4.6）：页脚画着键帽但那个
  chord 没绑定。做它要新增一个动作面表面 + 绑定一个新 chord，属**需拍板**（`.pending-bugs.md`
  的拍板表里 P11-ab 的「仍待」第 1 条就是它），本轮不动。
- 用户 2026-10-28 的原话是「还是先优化当前的界面、质感、交互逻辑」。本轮取「质感」里最可量
  的一条：**对齐**。分组头（section heading）是每一屏列表都看得见的一行字。

证据（`ui.Tester.Find()` 的矩形，改动前 HEAD `ef95721` 实测）：

| 表面 | 元素 | 改前 x | 改后 x |
|---|---|---|---|
| 启动器列表 | 分组头 `Recently launched` | **62.0** | **54.0** |
| 启动器列表 | 它下面第一行标题 `Editor` | 54.0 | 54.0 |
| 启动器列表 | 内置行 `Shut down the computer` | 54.0 | 54.0 |
| 输出面板 | 分组头 `Group A` / `Group B` | **20.0** | **52.0** |
| 输出面板 | 它下面第一行标题 `Alpha` / `Gamma` | 52.0 | 52.0 |

**根因**：分组头的左缩进是两个**手写数**（行左 padding + 文字的 `Margin-left`），从来没有从
「行的标题列」这个和里推出来。P8-a 把行块缩进 4u→8u、行内 padding 12u→8u 之后，这两个手写
数一个都没跟着动，于是同一件事在两个表面上漂成 62 与 20。两处注释当时都声称对齐：

- `internal/launcher/launcher.go`（改前 `:1988-1990`）：「The inset is the row block's own
  gutter (12u) plus the row's 4u padding plus the icon column's 28u and the 10u gap, so the
  label sits on the same column a row's title does.」——算式本身就对不上代码：行的左 padding
  是 12u（不是 4u），行块 gutter 是 8u（不是 12u）。
- `internal/launcher/output.go`（改前 `:529`）：「with the 6u/12u/4u inset the rows use」——
  行没有 12u 的左 inset，也**没有**图标列 + gap 的偏移。

## §2 改了什么

| 文件 | 位置 | 改动 |
|---|---|---|
| `internal/launcher/launcher.go` | heading 分支（`item.heading`） | 左/右 padding `Space(3)`(12u) → `Space(2)`(8u)（与 `rowBody` 的 `Padding(0, t.Space(2))` 同档）；文字 `Margin-left` `Space(10.5)`(42u) → `Space(9.5)`(38u) = 图标列 28u（`iconPlate` 的 `Space(7)`）+ 行 gap 10u（`rowBody` 的 `Gap(t.Space(2.5))`）。注释重写为「从行里推出来的和」 |
| `internal/launcher/output.go` | group heading（`row.Group` 换组处） | 同上左/右 padding → `Space(2)`；文字新增 `Margin-left` `Space(9)`(36u) = 图标列 28u + 该行 gap 8u（`outputRow` 的 `Gap(t.Space(2))`） |
| `internal/launcher/headingcolumn_test.go` | 新增 | `TestTheGroupHeadingSitsOnTheRowsTextColumn`（两个表面、三个断言）+ `TestDumpP11ACGroupHeadingPNGs`（opt-in，四张 PNG） |

纵向一个数没动：`Padding` 的上下仍是 `Space(1.5)`/`Space(1)`（`headingPadding` 的 6u/4u band），
`WindowHeight` 的 band 不动。字阶（body 步）、weight（500 / 620）、墨（`TextSecondary` /
`TextMuted`）零改动。

## §3 V / B / S 台账

- **V = 0**：没有旧 pin 钉过分组头的横向位置（`grep` 过 `internal/launcher` 与 `internal/shell`
  的 `_test.go`，只有「heading 不可选/不可编号/Enter 无动作」这类行为断言）。所以本轮不需要
  重定任何既有基线。
- **B = 0**：高度链、键盘模型（`rowSelectable` 未动）、会话管理、设置写回、i18n 文案、命令接线
  零改动；`TestSectionHeadings`、`TestTheFieldAndTheRowsShareTheReferenceTextColumn` 等既有
  pin 零改动全绿。
- **S = 0**：没有结构变化（同一批元素，只改 padding/margin 的数值）。
- 新增测试函数 **2** 条（1 条 pin + 1 条 opt-in dump），删除 0 条：函数 1911 → **1913**。

## §4 mutation（实跑，还原一律 `cp` + `md5sum -c`）

备份：`cp internal/launcher/launcher.go /tmp/p11ac-launcher.go.bak`、
`cp internal/launcher/output.go /tmp/p11ac-output.go.bak`，
`md5sum … > /tmp/p11ac-md5.txt`（`56989bdf…` / `08753eea…`）。

**M1 · 启动器侧把 `Margin-left` 改回 42u（`Space(9.5)` → `Space(10.5)`）**

```
--- FAIL: TestTheGroupHeadingSitsOnTheRowsTextColumn (0.35s)
    headingcolumn_test.go:44: launcher: the section heading starts at x=58.0, its row's title at x=54.0: a heading sits on the column of the rows it heads
    headingcolumn_test.go:51: launcher: the section heading starts at x=58.0, a built-in row's title at x=54.0
```

**M2 · 输出面板侧把 `Margin-left` 归零（`Space(9)` → `0`）**

```
--- FAIL: TestTheGroupHeadingSitsOnTheRowsTextColumn (0.25s)
    headingcolumn_test.go:68: output: the group heading "Group A" starts at x=16.0, its row's title "Alpha" at x=52.0
    headingcolumn_test.go:68: output: the group heading "Group B" starts at x=16.0, its row's title "Gamma" at x=52.0
```

**M3 · 启动器侧把左 padding 改回 12u（`Space(2)` → `Space(3)`）**

```
--- FAIL: TestTheGroupHeadingSitsOnTheRowsTextColumn (0.14s)
    headingcolumn_test.go:44: launcher: the section heading starts at x=58.0, its row's title at x=54.0: a heading sits on the column of the rows it heads
    headingcolumn_test.go:51: launcher: the section heading starts at x=58.0, a built-in row's title at x=54.0
```

三条都 `cp` 备份还原后 `md5sum -c /tmp/p11ac-md5.txt` 报「成功」，`touch` 两个文件后复跑
（`TestTheGroupHeadingSitsOnTheRowsTextColumn` + `TestSectionHeadings` +
`TestTheFieldAndTheRowsShareTheReferenceTextColumn`）全绿。**没用 `git checkout`**。

## §5 图上实测（`FLOTER_PNG=1 go test -run TestDumpP11ACGroupHeadingPNGs`）

四张：`/tmp/floter-P11-ac-list-{light,dark}.png`（720×517）、
`/tmp/floter-P11-ac-output-{light,dark}.png`（720×488）。卡片是半透明的，所以 dump 里合成到
中性桌面（浅 `#DFDFE4` / 深 `#1C1C20`）上。

用 PIL 逐像素量「第一个非填充像素的列」（阈值：与同一行 x=4 处的填充色差 > 40）：

| 图 | 元素 | 墨的 x 区间 | 起点 |
|---|---|---|---|
| `list-light` | 分组头 `Recently launched` | 55..177（y 347..362） | **55** |
| `list-light` | 行标题 `Editor` | 55..103（y 382..394） | **55** |
| `output-light` | 分组头 `Group B` | 53..108（y 200..210） | **53** |
| `output-light` | 行标题 `Gamma` | 53..102（y 236..245） | **53** |

**这张图上看到的一处变化**：输出面板里 `Group B` 这行字现在与它下面的 `Gamma` 从**同一列**
起笔（改前量到 20 与 52，差 32px）；启动器列表里 `Recently launched` 与 `Editor` 从同一列
起笔（改前 62 与 54）。两处都是「标签浮在标题旁边」，现在标签就是标题列的第一行。

（深色两张的 `list-dark` 行标题窗口量不到墨：深色下 `Editor` 那一行的标题墨与面板底的
亮度差落在阈值内侧，属测量窗口问题，不是像素问题——深色分组头本身量到 x=56，与浅色同列。）

## §6 一条查过但没有成立的线索（不写进 pin）

量输出面板时发现 `Group A`（第一个分组头）的 `Find()` 矩形与面板的头部状态行（`Finished · ↑↓
choose · Enter runs · Esc closes`，`x=12 y=82`）在 y 上重叠，怀疑「第一个分组头压在头部那一行
下面」。**实测否掉了**：同一窗口（y 86..100、x 52..112）里，带分组的 dump 有 284 个墨像素、
不带分组的 P8-e dump 有 263 个——只差 21 个，说明 `Group A` 并没有画在那个位置，`Find()` 给的
是布局矩形而非绘制位置。**所以没有重叠 bug**，本轮不做，也不写 pin。

## §7 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2351（基线 2350，P11-ab 实测；本轮新增 2 条函数里 1 条是 opt-in dump，故 PASS +1）
测试函数                                              → 1913（基线 1911）
GOOS=linux/windows/darwin go build ./...               → 三平台过
SKIP                                                  → 20（P11-ab 是 19；本轮新增的 dump 也是 opt-in）
```
