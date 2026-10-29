# P11-bw 报告 · 界面四档上的行盒、字盒与它报出的窗口

规格：`docs/rounds/P11-bw-spec.md`。分支 `mygo-rewrite`，本轮**产品代码零改动**，只新增一个测试文件。

## 1. 为什么选它

选题顺序：用户实测反馈 F1–F4 已收完（F4 三条在 P11-bb/bj/bk/bo）→ `docs/raycast-design.md` §6 全
`已落地`（只剩拍板项）→ P11-bv「仍待」第 1 条整张是老拍板项、第 2 条是新增拍板项（`AppIcons`
的 `Dark`/`Light` 漏译）、**第 3 条是尺子的第三处盲点**，正是本轮领的。

证据（本轮重新实测，不是抄旧报告）：`grep -rn "UIScale\|SetTheme" internal/launcher/*_test.go`
（P11-bv 之后）只有 `framememo_test.go:26`（只查 memo 键）与 `plate_lens_test.go:25`（深色主题）；
`height_test.go` 的 `Geometry` 夹具除 `launcher_test.go:157` 的收起态公式外全写死 `Spacing: 4`。
启动器的视图从 `c.Theme()` 读几何（`launcher.go:2242` 的 `t.Space(rowUnits)`），而把
`theme.ForPreferences` 装到窗口上的是外壳 —— 所以在启动器自己的测试里，界面档位**从来没到过画面**。
P11-bv 报告 §6 ① 里「四档审计把字号改成二次缩放仍全绿」就是这么来的。

## 2. 本轮实测（Linux，`InputWindowWidth = 720`，十个最近启动）

窗口高是启动器自己报给 `Actions.ResizeTo` 的那个数（`launcher.go:1235-1244` 的 `Geometry.Height()`）。

| 档位 | spacing | font | 行距（画出来） | 40u | title+subtitle 盒 | 报出的窗口 | 十行文字全在窗内 |
|------|---------|------|----------------|-----|-------------------|------------|------------------|
| tiny | 3.2 | 9.6 | 33 | 32 | 18+15 = 33 | 389 | 10/10 |
| small | 3.6 | 10.8 | 36 | 36 | 19+17 = 36 | 427 | 10/10 |
| default | 4.0 | 12.0 | 40 | 40 | 22+18 = 40 | 474 | 10/10 |
| large | 4.4 | 13.2 | 44 | 44 | 23+20 = 43 | 522 | 10/10 |

结论：**几何是按档位线性的，没有缺陷**。两件事实被四档逐档印证：
① 行距 `= max(rowUnits × spacing, TitleLine + SubtitleLine)` ——「行是它自己的盒，或者它装的两行字，
谁高算谁」这条规则在 tiny（字盒 33 > 40u 32）与 large（40u 44 > 字盒 43）**两个分支都真的被走到过**；
② 行距 / 字号在四档上是 3.4375 / 3.333 / 3.333 / 3.333（tiny 那 3% 是行盒按整像素栅格化，
18/9.6 ≠ 22/12，不是分歧）。

## 3. 落地项

新文件 `internal/launcher/uiscale_test.go`（唯一改动）：

- `measureStep` —— 一档要两个窗口：先用一个 1400px 高的窗口问出启动器想要的窗口高度
  （`Actions.ResizeTo`），再用**新的 `App`**（同一个 App 交给第二个 `ui.Tester` 会在
  `FocusSearch` panic，P11-bu 仍待第 2 条）按那个高度、装**该档自己的主题**画一帧，
  从帧里读 spacing / font / 行距 / 十行是否都还在。
- `TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep` —— 四档逐档：行距 == max(40u, 两行字盒)（容差 1px）；
  四档 `行距/字号` 极差 ≤ 5%；行距与窗口高都随档位严格递增。
- `TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep` —— 按报出的高度画，十行必须都还在帧里。

档位列表读 `valuedomain.Scales`，将来加第五档会自动进这两条 pin。

## 4. 台账

**V = 0 / B = 0 / S = 0**，产品代码（`launcher.go` / `height.go` / `view*.go`）**一个字节没动**：
`md5sum` 前后逐字节相同（`internal/launcher/launcher.go` `c2c9aebe…`、`internal/launcher/height.go`
`f65cfd38…`）。没有旧 pin 因为本轮变红，也没有断言被改写。

门槛（实测）：`gofmt -l cmd internal` 空；`go vet ./...`、`GOOS=windows go vet ./...`、
`GOOS=darwin go vet ./...` 全净；`go test -count=1 ./...` 全绿；
PASS **2560 → 2562**（+2，就是这两条 pin）；测试函数 **2026 → 2028**；
`GOOS=linux/windows/darwin go build ./...` 三平台过。

## 5. mutation（实跑，原始报错串照抄）

| # | 注入 | 结果 |
|---|------|------|
| M1 | `launcher.go:2242` `floor := t.Space(rowUnits)` → `* float32(tokens.Scale)`（档位缩放两次） | **两条 pin 都红** |
| M2 | 同上 → `floor := float32(40)`（钉死不跟档位走） | **两条 pin 都红** |
| M3 | `height.go` `chrome` 去掉 `breathUnits`（少算 4u） | **仍绿 —— 见 §6，这是本轮的实测边界** |

M1 原始报错：

```
--- FAIL: TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep
    uiscale_test.go:157: [large] the rows are 48.4px apart, want its own 40u: 44.0px = 10 × the theme's 4.400 spacing
    uiscale_test.go:175: the row box and the type do not scale together: [tiny 3.437 (33.0px rows on 9.6px type) small 3.333 (36.0px rows on 10.8px type) default 3.333 (40.0px rows on 12.0px type) large 3.667 (48.4px rows on 13.2px type)]
--- FAIL: TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep
    uiscale_test.go:206: [large] the window the launcher asked for (522px) holds 9 of the budget's 10 rows
```

M2 原始报错：

```
    uiscale_test.go:157: [tiny] the rows are 40.0px apart, want its two line boxes: 33.0px = 10 × the theme's 3.200 spacing
    uiscale_test.go:157: [small] the rows are 40.0px apart, want its own 40u: 36.0px = 10 × the theme's 3.600 spacing
    uiscale_test.go:181: tiny and small draw their rows 40.0px and 40.0px apart: a bigger interface size is a bigger row
--- FAIL: TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep
    uiscale_test.go:206: [tiny] the window the launcher asked for (389px) holds 8 of the budget's 10 rows
    uiscale_test.go:206: [small] the window the launcher asked for (427px) holds 9 of the budget's 10 rows
```

每条变异跑完都还原，`md5sum -c` 逐字节核对通过（`launcher.go: 成功` / `height.go: 成功`）。

## 6. 实测到的两处边界（都写进 `.pending-bugs.md`，别当 TODO 重领）

1. **M3 不红的原因**：框架把被裁的文本 run 夹进裁剪框，所以窗口少算 4u 时十行**还在帧里**，
   只是第十行的文字落到窗口下沿之外 16px —— `Tester.Find` 返回的是夹过之后的盒子，
   这条 pin 看不见「小数目短算」。它看得见的是**整行消失**（M1/M2 都看见了，报错就是
   `holds 9 of the budget's 10 rows`）。要守住 4u 这一档，得有一把能读「帧的裁剪前的几何」的尺子，
   `ui.Tester` 现在不给。
2. **新发现（拍板项，不是本轮该改的）**：分组头是被算在十行预算里的（`Geometry` 的
   `Rows`/`Heading` 注释写着「charged inside the ten-row budget」），于是带分组头的页面上
   **第十行的底板**比报出的窗口多出 14px（`default`：底板 y=448..488，窗口 474）。
   十行的**文字**都在窗内，最后一块**板**不在。把窗口加高一个分组头的带会改变可见行为
   （P11-bb 的 `last` 判据正是围着「只露一半的第十行」定的），所以进拍板表。

## 7. 没有出图的原因

本轮墨一个字节没改（§4 的 md5），不是视觉轮，`/tmp/floter-P11-bw-*.png` 会与 P11-bj 的逐字节相同。
本轮的证据是 §2 那张四档表 —— 它比图更能说明「档位改了、行跟着走了」。

## 8. 仍待（P11-bw）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light`）。
2. **新增拍板项**：十行预算里分组头吃掉一行，第十行的底板被窗口裁掉 14px（§6.2）。
3. 尺子的第四处盲点：`ui.Tester` 看不到裁剪前的几何（§6.1），所以「窗口少算一两 u」这一类
   回归仍然只能靠整行消失来发现。
4. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；
   设置页 `actionSpinner` 仍是上游构建期缺口。
