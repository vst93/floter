# P11-bz 报告：第二把尺子——字盒 / 行盒的高度阶梯

规格 `docs/rounds/P11-bz-spec.md`。选题：用户实测反馈 F1–F4 已收完 → `docs/raycast-design.md`
§6 全 `已落地`（只剩拍板项）→ P11-by「仍待」第 2 条，**上一轮自己实测出来的产物**：

> 「字号暴涨 / 行距失控」这类回归不会被子压字尺子看见（重排 + 裁剪后的盒子），要单独一条
> 字盒高度 / 行距的臂。

## 1. 落地

新增 `internal/frameaudit/heights.go`（与第一条尺子同包，同一套 `Run`）：

| 函数 | 问的问题 |
| --- | --- |
| `Heights(tt, keep)` | 这一帧的**叶子** run（盒子不包含别的 run 的盒子）有哪几档高度，去重、升序、四舍到 0.1u |
| `Ladder(rungs, tol, heights)` | 有哪一档**不在**阶梯上 |
| `Missing(rungs, tol, heights)` | 阶梯上应有哪一档**没被画出来** |
| `AuditHeights(...)` | 三者的接线（`Heights` 为空 ⇒ 这一臂失败，不是通过） |
| `MinHeight = 10` | 裁剪碎片的默认下限：本仓最小的档是键帽图例 15u，10u 以下不可能是整行 |

两条断言抓的是不同的东西：`Ladder` 抓「某一档被改了」（字号、行高、控件高度换了数），
`Missing` 抓「某一档不见了」（字被顶出视口、某块停止绘制、被裁成碎片）。

表面 1 —— **启动器**（新 `internal/launcher/textheight_test.go`），六档
`15/18/22/24/25/28`，三页 × 2 语言 × 2 外观；每页 `must` 子集：
空页 `{18,22,25,28}`、命中 `六档全要`、无结果 `{18,25,28}`。**实测 en/zh × 深/浅
逐字节相同**（三页都是同一组数）。

表面 2 —— **设置页**（新 `internal/settingsui/textheight_test.go`），八档
`15/17/18/19/27/30/30.6/34`，五页 × 2 语言 × 2 外观 × 2 尺寸 = 40 臂，另加 General 页
一条 `must {17,18,19,27,30.6}` 覆盖臂。八档是**实测并集**：720 宽下被滚动口裁剩的碎片
（2.7 / 5.2 / 6.5u）由 `MinHeight` 挡在外面，那是裁剪不是行高。

规则本身的尺子 —— `internal/frameaudit/heights_test.go` 5 条：阶梯外必报、半格容差边界、
缺档必报、容器盒不入表、重复高度只算一档。

**产品代码零改动**：`git diff --stat` 只有新文件与 docs。

## 2. 台账

**V = 0 / B = 0 / S = 0**（新守卫，不是修复；没有任何既有 pin 被改）。

门槛（本轮实测）：`gofmt -l cmd internal` 空 · `go vet ./...` 净 ·
`GOOS=windows/darwin go vet ./...` 净 · `go test -count=1 ./...` 全绿 0 FAIL ·
**PASS 2601 → 2655**（+54 = 启动器 8 + 设置页 40 + General 2 + frameaudit 5 …… 实测 2655）·
测试函数 **2031 → 2039** · 三平台 build 过。

## 3. 变异（5 条实跑，2 红 3 绿，全部还原并 `md5sum` 逐字节核对）

**M1（设置页红、启动器绿）**：`theme.BodyUnits` 12 → 20。
设置页阶梯红，例：

```
--- FAIL: TestTheSettingsPagesStayOnTheHeightLadder/en_light_general_860x900
    textheight_test.go:61: height 28.0 is not on the ladder [15 17 18 19 27 30 30.6 34] (±0.50)
    textheight_test.go:61: height 54.0 is not on the ladder [...]
```

启动器阶梯**仍绿**——查下来是它自己的事，也是本轮一条有用的实测：梯子里 `BodyUnits` 只喂
`t.FontSize`，`tokens.Body/Emphasis/...` 是 `ForPlatform` 里另外的字面量（`theme.go:538-543`），
而启动器几乎全用 `tokens.*` 画。⇒ 改 `BodyUnits` 对启动器是无感的。

**M2（红，最关键的一条）**：`emphasis := round1(13 * f)` → `round1(20 * f)`（`theme.go:542`）。
两个表面的阶梯**同时红**，且两条断言都真的开火：

```
textheight_test.go:52: light en empty: height 27.0 is not on the ladder [15 18 22 24 25 28] (±0.50)
textheight_test.go:52: light en empty: no run of height 18.0 was painted (heights [22 25 27 28])
textheight_test.go:65: light en no results: height 27.0 is not on the ladder [...] 
```

**同一变异下，第一条尺子（`TestTheLauncherPaintsNoTextOverText` /
`TestTheSettingsPagesPaintNoTextOverText`）两个包都绿。** 这就是本轮存在的理由：同一次
「字号暴涨」，旧尺子全绿、新尺子红。

**M3（绿，诚实记账）**：`settingsui/rows.go:923` 的行内提示 `FontSize(t.FontSize*6)`。
**四个包里的设置页臂都没画到这句**（它要一行被拒 / 不支持的插件才会出现），所以绿。记在这里
免得下一轮把它当成「尺子看不见」。

**M4（绿，诚实记账）**：空页的最近启动整块不画（`search.go:879` 加 `false &&`）。空页的
`must {18,22,25,28}` 四档**仍然全在**——那四档不来自最近启动那几行。⇒ 空页这条 `must`
不是「最近启动还在不在」的守卫，别这么用；`Missing` 的活证据是 M2 里那条 18。

## 4. 仍待（P11-bz）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **第三个表面（`internal/terminalui` 的抽屉）还没进这把尺子**：它要自己量一遍阶梯
   （条带 + 行的盒子与其它两个表面不同族）。别照抄启动器或设置页的数。
3. 设置页的八档里有 **30 与 30.6 只差 0.6u**（半格容差是 0.5）：这两档目前是两条不同的
   行（滑块轨 / 多行输入框），但分辨力已经很薄；再有人把某档挪 0.3u，这把尺子会看不见。
4. 分辨力边界（M1/M3/M4 的教训）：**只有字号/行高真的改变了盒子高度才看得见**。
   `BodyUnits` 这类「只改 `Theme.FontSize`」的回归对启动器无感（M1）；被固定高度裁住的
   一行也可能高度不变（M3 的可疑同类）。想守「字变大了但盒子没变」得量**墨迹**，
   不是盒子——P11-bz 试过一版像素墨迹量法，实测被邻居元素污染（`git` 一行报 17u、
   容器 label 报 23u），这一轮没落地。
5. 上一轮留下的尺子盲点不变：只量字盒（图标 / 面 / 描边的互压看不见）；
   `Tester.Find` 按字符串去重；滚动容器里被裁掉的行不进 `Texts()`。
6. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；
   设置页 `actionSpinner` 仍是构建期转（`rows.go:580`）。
