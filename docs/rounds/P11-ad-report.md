# P11-ad 报告：同一个窗口里的两种列表只有一条文字列

规格：`docs/rounds/P11-ad-spec.md`。合约更新：`docs/raycast-design.md` §4.3、§4.4、§6.10。

## §1 为什么是这一条

选题顺序（任务书）：用户实测反馈 → 合约未落地条目 → 上一轮「仍待」→ 界面质感与交互。

- 用户实测三条（F1 终端异常字符 / F2 红线扎眼 / F3 类玻璃边）已由 P11-y、P11-z、P11-aa
  收完，`.pending-bugs.md` 顶部三条都写着「已修」。
- 合约 §6 全表 `已落地`，唯一未落地的 `⌘K` 动作面（§4.6）要新增一个表面 + 绑一个新
  chord → 动键盘模型 → **拍板项**，本轮不动。
- 上一轮（P11-ac）的「仍待」第 1 条就是它；第 4 条是本轮这个缺口，原话：「输出面板的行
  gap 是 8u、启动器列表是 10u，于是两个表面的标题列差 2u（52 vs 54）」。

P11-ac 把**分组头**钉到了各自的标题列上，但没有统一**两种列表**的标题列。启动器列表与
它自己的输出面板是同一张卡里的两种列表：同一个窗口、同一个 8u 卡片内缩、同一个 28u
图标列、同一个 8u 行内 padding——横向分量里唯一不同的是**行自己的 gap**（列表 10u，
面板 8u）。于是用户在列表与输出之间来回看时，文字列会无缘无故错 2u。

## §2 改了什么

| 文件 | 位置 | 改动 |
|---|---|---|
| `internal/launcher/output.go` | `outputRow` 的行盒 | `Gap(t.Space(2))` 8u → `Gap(t.Space(2.5))` 10u |
| `internal/launcher/output.go` | `outputRow` 的分组头 | `Margin-left` `t.Space(9)` 36u → `t.Space(9.5)` 38u |

和只有一个：面板/卡片内缩 8u + 行内 padding 8u + 图标列 28u + 行自己的 gap = 标题列。
gap 与启动器列表取同一个数之后，两种列表的整条横向节奏就是同一个数：

| 表面 | 行标题 x（改前） | 行标题 x（改后） | 分组头 x（改前） | 分组头 x（改后） |
|---|---|---|---|---|
| 启动器列表 | 54.0 | 54.0 | 54.0 | 54.0 |
| 输出面板 | **52.0** | **54.0** | **52.0** | **54.0** |

（`ui.Tester.Find()` 的矩形，`TestTheTwoListsShareOneTextColumn` 的 `t.Logf` 实测：
`text column: list title 54.0 / heading 54.0, output title 54.0 / heading 54.0`。）

字阶、weight、墨、band（6u/4u）、行高、图标列 28u、keycap 槽、选中/悬停档、滚动与键盘
模型一律未动——**纯几何**。

## §3 台账

- **V = 0**：没有旧 pin 钉过输出面板行的横向位置（`TestTheGroupHeadingSitsOnTheRowsTextColumn`
  断言的是「头 == 本表面标题列」这个**相等**，不是字面数，所以两处一起动之后它仍然绿）。
- **B = 0**：`go test ./internal/launcher/` 全绿，无一条行为断言改动。
- **S = 0**：结构没变。
- 新增 pin `TestTheTwoListsShareOneTextColumn`（`internal/launcher/textcolumn_test.go`，
  断言两个表面的标题 x **相等**、两个表面的分组头 x **相等**，并打印四个实测值）
  + 1 条 opt-in dump（`TestDumpP11ADOutputPanelPNGs`）→ 函数 1913 → **1915**。

## §4 mutation（3 条，实跑）

1. 行 gap 回 `t.Space(2)`（8u），分组头保持 38u：
   ```
   --- FAIL: TestTheGroupHeadingSitsOnTheRowsTextColumn
       headingcolumn_test.go:68: output: the group heading "Group A" starts at x=54.0, its row's title "Alpha" at x=52.0
   --- FAIL: TestTheTwoListsShareOneTextColumn
       textcolumn_test.go:48: the launcher list's title starts at x=54.0, the output panel's at x=52.0: two lists in one card share one text column
   ```
2. 只把分组头的 `Margin-left` 回 `t.Space(9)`（36u）：
   ```
   --- FAIL: TestTheGroupHeadingSitsOnTheRowsTextColumn
       headingcolumn_test.go:68: output: the group heading "Group B" starts at x=52.0, its row's title "Gamma" at x=54.0
   --- FAIL: TestTheTwoListsShareOneTextColumn
       textcolumn_test.go:57: the launcher's heading starts at x=54.0, the output panel's at x=52.0
   ```
3. 行 gap 改 `t.Space(3)`（12u）、分组头不动：
   ```
   --- FAIL: TestTheTwoListsShareOneTextColumn
       textcolumn_test.go:48: the launcher list's title starts at x=54.0, the output panel's at x=56.0
   ```

每条都 `cp` 备份还原后 `md5sum -c` 逐字节核对（`68a4bf51c0107e0c9541ca9bae35d80e` →
改后 `…成功`），再 `touch` + 复跑绿。**教训记一笔**：第一次 mutation 用
`git checkout` 还原，把未提交的改动一起冲掉了（M2/M3 因此在旧代码上跑）——改用 `cp`
备份 + `md5sum -c` 之后重跑，三条报错串是重跑后的真值。

## §5 门槛（实测）

```
gofmt -l cmd internal        空
go vet ./...                 净
GOOS=windows go vet ./...    净
GOOS=darwin  go vet ./...    净
go test -count=1 ./...       全绿
PASS 2352（基线 2351，HEAD be3357d 实测）| SKIP 21（基线 20，新增的 opt-in dump）| FAIL 0
测试函数 1913 → 1915
GOOS=linux/windows/darwin go build ./...  过
```

## §6 我实测看到了什么（图）

`/tmp/floter-P11-ad-{list,output}-{light,dark}.png`（720×517 / 720×488，四张）。

- **`/tmp/floter-P11-ad-output-light.png`**：分组头 `Group A` 那一带的墨从 **x=54** 起
  （`y=200..211` 的 `ink x=54..110`），改前是 x=52；同一张图上行图标列仍是 `x=16..43`
  （8u 内缩 + 8u padding + 28u 图标列），所以动的是「图标之后的那一份 gap」，图标列本身
  一个像素没挪。
- **`/tmp/floter-P11-ad-list-light.png`**：启动器列表的标题列仍在 **x=54**（`y=16` 上
  搜索行光标那一列），两张图的文字列第一次是同一个数。
- 深色两张同理（同一个几何，只有墨变），半径、band、行高在四张图上都没有变化。

## §7 仍待

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定。
   要新增表面 + 新 chord → **需拍板**（原样有效）。
2. 拍板清单原样有效（P11-aa 的默认浅色安静档、P11-z 的搜索框 caret、P11-y 的滚动缓冲、
   P9-g 表那一批）。
3. 侦察结论（本轮实测否掉，别当 TODO 重领）：`internal/launcher` 的行标题/副标题只写了
   `SingleLine()` 没写 `.Ellipsis("…")`，但 mygo 的 `text.Params.Ellipsis` **空值就是
   `…`**（`internal/text/layout.go:1084-1096`，`ellipsis` 变量是默认值，参数只是替换标记），
   所以注释里「single-line and ellipsized」是**准确**的，没有截断缺陷。
4. 侦察结论（本轮实测否掉）：终端页标题栏是一块**没有底**的浮条，窗口外观与终端配色
   相反时（浅色窗口 + `ink` 等深色 palette），它的墨会落在调色板底色上。量到的两个墨
   （浅 `#6B6B70` / 深 `#9A9AA0`）在默认 46% 透明度的合成底上**都**不到 2:1，所以这不是
   「选错一套墨」的问题，而是「这块浮条没有自己的面」——要修得给它一个背衬或改成 inline，
   属**视觉语汇 + 布局**决定，已记进 `.pending-bugs.md` 的拍板表，本轮不动。
