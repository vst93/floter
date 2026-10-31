# P13-bi 报告：插件窗口里「同一个运行的结局」没有画出来

规格 `docs/rounds/P13-bi-spec.md`，合约 `docs/liquid-glass.md` **§6.28**。

## 这轮为什么让它更轻、更有空间感

**因为这一页此前根本没有「结局」这一层。** 用户说界面「平、糊、旧」——在插件窗口上，
「平」是字面的：一次失败、一次超时、一次成功，三帧**逐像素相同**（`Texts()` 都是
`["jv" "Close" "hello"]`）。屏幕上没有第二层可读的东西，只有 bar 和一段正文。
补上这条带之后，同一页第一次有了三个高度：**bar 的 chrome（118 / 113）→ 这一次运行的
结论（135 / 130，失败 177 / 144）→ 关于它的事实（104 / 90）→ 正文（body 档）**。
结论比它头顶的 chrome 重，事实比结论轻 31 / 40 级——这正是 §2 的「靠面差与墨档说层级」。

## 先量（HEAD `2930aca6`）

`ui.Tester` 720×460，`a.detachedView`，窗口地面 `tokens.Opaque` 浅 246 / 深 20：

| 臂 | 窗口自己的文字 | 结局墨 |
|---|---|---|
| 成功 `Finished  ·  400 ms` | `["jv" "Close" "hello"]` | **没有** |
| 失败 `Exit code 3  ·  400 ms` | `["jv" "Close" "hello"]` | **没有** |
| 超时 `Timed out  ·  300.0 s` | `["jv" "Close" "hello"]` | **没有** |

差不是「小」，是 **0**。根因在 `internal/launcher/output.go:453`（`runStatus`）：结局与事实
在**字符串这一层**就被拼成一句，所以就算窗口想画也只能整句一个墨——而它连这句都没拿到
（`internal/shell/detach.go:338-350` 的正文只读 `Text` / `Rows` / `failed` 三样）。

## 落地

- `internal/launcher/output.go`：`OutputView` 拆出 `Verdict` / `Facts`，`runStatusParts`
  是两段的唯一出处；`Status` 仍由同一段拼接逻辑产出（**逐字节不变**，§6.26 的 pin 与
  像素不动）。
- `internal/shell/detach.go`：`detachedView` 的 `Column` 从两个孩子变三个——
  `detachedBar` → **`detachedVerdict`** → `detachedBodyView`。
- 门：`ran && !running && failed == "" && Verdict != "" && detachedHasBody(view)`。
  四条状态行（running / refused / 空输出 / 未运行）本身就是这一页的结论（§6.24），
  再补一条会让同一页说两遍结局。
- 阶梯：结局 `TextSecondary` + 580（失败 → `t.Warning`，色相是语义）；中点
  `TextTertiary` 0.7；事实 `TextTertiary` + 500；两半同一个 `tokens.Caption`。

## 实测（改后，`verdictProbe` 读的是画出来的像素）

| 段 | 浅 | 深 |
|---|---|---|
| bar 标题（§6.24，不动） | 118 | 113 |
| 结局（成功） | **135** | **130** |
| 结局（失败 / 超时，`t.Warning`） | **177 / 176** | **144 / 144** |
| 事实（时长 / 截断） | **104** | **90** |

结局压事实 **31 / 40**（失败臂 **73 / 54**）。

**几何只加一行**：整帧像素差 bbox `light finished (12,55)-(102,91)`、`light failed
(12,55)-(114,91)`、`light timedout (12,55)-(111,91)`，dark 同（583 / 606 / 605 px）。
正文整体下移 27px（14u 上内边 12.6 + 一行 15）；`Scroll` 的 `MaxY` 由布局自己算——
**面板高度链、主窗口、设置页一个字节不动**。

**一处取舍**：结局带左右取正文自己的 `detachedBodyInset`（14u），上也取 14u、下取 0
——正文自己的 14u 上内边距就是它与带之间的呼吸。给两次这条带就读成「第二条 bar」。

## V / B / S 台账

**V 0 / B 0 / S 0**。没有任何旧 pin 变红：`Status` 逐字节不变（§6.26 的四条 pin 原样绿），
新加的是一条此前不存在的行，所以没有旧值需要重定基线。

## mutation（4 条实跑，全部真红，`md5sum -c` 逐字节还原后 `touch` 复跑绿）

1. **失败臂去掉 `t.Warning`** → `TestTheFailedRunKeepsTheWarningHue` 红：
   ```
   dark=false failed: the verdict lost the warning hue (r−b = -5, want ≥ 25; ink {109 109 114 255 ...})
   dark=true  failed: the verdict lost the warning hue (r−b = -6, want ≥ 25; ink {152 152 158 255 ...})
   ```
2. **删掉 `a.detachedVerdict(...)` 调用**（= 改前的状态）→ 两条红：
   ```
   finished: the window does not say the run's verdict "Finished": ["jv" "Close" "hello"]
   the band's "a.detachedVerdict(c, detached, tokens, t)" is missing from detach.go
   ```
3. **事实那段抬到 `TextSecondary`** → `TestTheVerdictOutranksItsFacts` 红：
   ```
   dark=false finished: the verdict does not outrank its facts (gap -4, want ≥ 20; ...)
   dark=true  failed:   the verdict does not outrank its facts (gap 10, want ≥ 20; ...)
   ```
4. **门去掉 `detached.failed != ""`** → `TestTheStateLinesHaveNoVerdictBand` 红：
   ```
   dark=false refused: a state-line body grew a verdict band ("Finished"):
     ["jv" "Close" "Finished" "Could not start /opt/bin/node: no such file"]
   ```

## 门槛（实测）

```
gofmt -l cmd internal                       空
go vet ./...                                净
GOOS=windows go vet ./...                   净
GOOS=darwin  go vet ./...                   净
go test -count=1 ./...                      全绿
--- PASS 计数                               3160 → 3165（+5）
--- FAIL                                    0
--- SKIP                                    109 → 110（+1，opt-in 出图）
测试函数                                    2327 → 2333（+6）
GOOS=linux/windows/darwin go build ./...    过
```

## 偷了什么（qiaomu-design · craft-loop.md 的工艺密度菜单）

逐条写「取了第 N 项 → 落成哪个具体数值」：

- **第 5 项「多层染色阴影」的反面——多层墨档**：这条带不是一层灰，是**三段各一个寄存器**
  （结局 580 / 中点 0.7 alpha / 事实 500），落成实测浅 135 / 中 — / 104、深 130 / — / 90。
  这是本 skill「工艺密度」里唯一能直接搬到自绘 UI 的一条：**同一个句子里没有同一个墨**。
- **第 6 项「编辑细节 / hairline 与大写字距标签」**：带与正文共用同一条 14u 内边，
  所以结局的第一个字母与正文的第一个字母**同列**（实测都从 x=12.6 起）——一条隐形的
  对齐线，不是画出来的线。
- **第 12 项「材质细节：内边框高光」的反面判据**：这条带**故意没有面**。它不是 E1、不是 E2，
  是「正文上方的一行注」——§1 的材质分档表里它落在「没有材质」那一档，因为它不承载点击。
  给它一块面就等于给一句陈述画一个框（正是 §2 退掉的画框分层）。

## preflight 逐条

- **A（反 AI 套路）**：过。无渐变、无纯黑、无外发光；无斜体（§6.27 已把全仓 `.Italic()`
  清零，本轮没有新增）；文案没有新串——`Verdict` / `Facts` 复用 `runStatus` 已有的
  `OutputSucceeded` / `OutputFailed` / `OutputTimedOut` / `OutputTruncated` 与
  `extensions.FormatRunDuration`。
- **B（设计读取一致性）**：过。这条带是 §6.26 那三档在第二个表面上的同一套数
  （结局 580 / 中点 tertiary 0.7 / 第三档 500），不是新发明的一套；第三档从「键位图例」
  换成「运行事实」，因为插件窗口的键在 bar 上，没有图例可给。
- **C（动效抽查）**：不适用——本轮零动效、零 transition、零 `Loop`。窗口是静态重绘。
- **D2（中文页）**：过。新增的可见文字全部来自 i18n 既有串（`Finished` / `退出码 3` /
  `已超时` / `输出已截断`），本轮没有写任何中文进产品；报告与合约里的中文按
  `chinese-typography.md` 的规则（全角标点、无斜体、数字与拉丁词两侧留半角空格）。

**过不了的项**：无。

## 出图（自己复核）

`/tmp/floter-P13-bi-band-{light,dark}-{finished,failed,timedout}-{before,after}.png`
（`FLOTER_P13BI_DUMP=1 FLOTER_P13BI_TAG=<before|after>`）。

**这张图上看到的一处变化**：`light-failed-after` 里，bar 下面多出一行**暖红色**的
`Exit code 3  ·  400 ms`，而它下面的正文 `hello` 仍是中性灰——**同一页第一次能一眼分出
「这次运行怎么了」与「它说了什么」**。`light-finished-after` 里同一位置是中性灰的
`Finished  ·  400 ms`，比下面的正文**深一档**、比上面的 bar 标题也**深一档**。
`before` 三张在这一段是空白：像素扫描 `y=40..55` 一行墨都没有。
