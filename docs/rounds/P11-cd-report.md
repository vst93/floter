# P11-cd 报告 · 什么都没问的空页不报「没有匹配项」

轮号 P11-cd｜规格 `docs/rounds/P11-cd-spec.md`｜分支 `mygo-rewrite`
前 HEAD `e0f78d57`（P11-cc）｜基线 PASS **2752** / 0 FAIL / 55 SKIP

## 1. 选题与证据（本轮实测）

选题顺序走完：用户实测反馈 F1–F4 已收完 → 合约 §6 全 `已落地`（只剩拍板项）→ P11-cc「仍待」
第 1、2 条都是拍板项（13 条老决策项 + 「带子里要不要显示全文」）→ 按用户优先级自己找
（界面质感与交互 → 基础功能 → …），落在一处**用户会直接读到的错话**。

**这不是内部一致性，是页面对用户说的第一句话就是错的**：`internal/launcher/launcher.go:887`
的 `default: return copy.NoResults` 把「空查询 + 一条最近启动都没有」也当成「搜索没命中」。
实测（`720` 宽，`testApp()`，无最近启动）：

```
["No results" "Search" "Type to search" "Settings"]
```

全新安装的 floter 一打开，卡片就在否认一次没发生的搜索。P11-bh 已经为「没有拖入文件」的
页面立过同一条规则（`emptyMessage` 的 `files` 分支注释：「nothing was asked」），本轮把这条
规则补到最后一个 `default` 上。

### 顺手核掉的一条旧账（P11-cc 仍待第 3 条，**结论：量的是夹具自摆的状态，不是产品缺陷**）

P11-cc 报告说「`tt.SetSize` 把窗口长高 30u 之后滚到末行，最低行的字盒与带子重叠」。
本轮照它的描述复跑（20 个工具、查询 `app`、`Feedback` 抖出带子、`SetSize(720, 518)`、
`ScrollIntoView(last)`），逐像素量与逐盒子量**都是干净的**：

| 量 | 实测 |
|---|---|
| 带子的面（x=300 那一列扫像素） | `y=443..472`（30u），下沿 `y=473` 一条发丝，`y≥474` 是面板 / 动作栏 |
| 列表最低行 `app-20`（行盒 `H=46`，两行字） | 标题 `y=395..419`，行盒下沿 `y≈431` |
| 列表内容底 + 底部内边距（`2u + statusBand 30u`） | 最低行比带子上沿还高 12u |

真正的差别是**窗口有没有为带子长高**：手动把窗口留在旧高度（不长那 30u）而带子在场，行当然
落在带子底下——那是夹具的状态，不是用户能走到的路径（`Geometry.Feedback` 与
`FeedbackVisible` 是同一个谓词，shell 一定为它长 30u）。这条写进「仍待」时已改成侦察结论。

## 2. 改了什么（两处产品代码）

| 文件 | 改动 |
|---|---|
| `internal/launcher/launcher.go:886-897` | `emptyMessage` 的 `default`：`strings.TrimSpace(a.Query) == ""` → 返回 `""`（空页没有话要说）；有 needle 的空命中照旧 `copy.NoResults` |
| `internal/launcher/launcher.go:1321-1331` | 空态列只在句非空时才建 `ui.Text`（空串不再建一个空 text 元素） |

墨、面、字号、内边距、高度链、键盘模型、会话、设置写回、命令接线**一个字节没动**；
i18n 文案**没有新增也没有改字**（`NoResults` / `Hint` / `Placeholder` 三句原样，只是不再让
没有 needle 的页面借用 `NoResults`）。

## 3. 门槛（本轮实测）

```
gofmt -l cmd internal                             → 空
go vet ./... ; GOOS=windows ; GOOS=darwin          → 净
go test -count=1 ./...                            → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS" → 2754（基线 2752，+2）
GOOS=linux|windows|darwin go build ./...           → 过
测试函数数 grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2048（基线记 2046）
```

## 4. V/B/S 台账

- **V**：无（没有数值口径变了）。
- **B**：无（没有「测试零改动仍必须绿」的接线被改）。
- **S（结构）**：`internal/launcher/filters_test.go:326` 的「ordinary page」臂原本在**空查询**上
  断言 `NoResults`，空查询不再是有 needle 的空结果，夹具补 `a.Query = "zzzz"`；
  `internal/launcher/textheight_test.go` 的空页臂 `must` 去掉 `18`（那一页不再画句子，
  `18` 由同测试的「有 needle 的空结果」与 `match` 臂守着）。
- **新 pin**：`TestAnEmptyQueryNeverClaimsNoResults`（两臂，见下）。

## 5. mutation（实跑，`md5sum -c` 逐字节还原，`touch` 后复跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 删掉 `emptyMessage` 的 `TrimSpace` 守卫（回到无条件 `NoResults`） | **红**：`an empty query with no recents has no sentence to say, got "No results"` / `the page denies a search nobody made: ["No results" "Search" "Type to search" "Settings"]` |
| M2 | 守卫之后 `return copy.NoResults` → `return ""` | **红**：`a real miss says "", want "No results"` / `a real miss drew no sentence: ["Search" "Clear" "Settings"]` |

两条都 `cp` 回原文件后 `md5sum -c /tmp/p11cd.md5` 成功（`15092ee399c38d8552a574fe91591a50`），
`touch` 后复跑两条 pin 全绿。

## 6. 仍待（P11-cd）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文）。
2. **P11-cc 仍待第 3 条作废**（§1 末段）：带子与最低行在 shell 真的长高 30u 时是干净的
   （最低行比带子上沿高 12u）；别再照那条找「带子压行」。
3. **空页要不要一句引导**是新的**文案拍板项**：本轮按「没有 needle 就不说话」处理（与
   P11-bh 同一条规则，且不新增 copy）。若用户希望全新安装时有一句「还没有最近启动」，
   那是新增 i18n 文案，属拍板项。
4. 本轮**没出图**：改动是「某一帧少一句话」，`ui.Tester` 的字树足以证明（`Texts()` 里
   `No results` 消失、`Type to search` 仍在），像素层没有可比的形态；要出图的话
   `TestDumpP11CDEmptyPagePNGs` 属可选 dump，未加。
5. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器里被裁的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
