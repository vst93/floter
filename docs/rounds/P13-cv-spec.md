# P13-cv 规格：探针列表的分隔线画在了错的边上——一条线贴着自己的上家，还替列表收了尾

选题：通用队列 `空间感`（`docs/liquid-glass.md` §6 已无 `未开始` 行）。
上行是 P13-cu 留的「下一格（先量）」：`healthProbeRow` 每行都戴一条线，先量它是不是与上面
那张卡的底边重复成双线。

## 0. 先量的结果（真设置页、720×1100、Integrations 页、两套外观）

`internal/settingsui/healthfaces.go:146` 是
`row.InsetShadow(0, -1, 0, 0, tokens.Hairline)`。`InsetShadow` 的 y 偏移在本仓框架里是
「把阴影往下推」，负值把**未覆盖的那一条**留在了盒子的**底边**（`mygo/ui/shadow_test.go`
的 `InsetShadow(0, 6, 0, 0)` 案例：正偏移只剩顶部那一条）。实测两套外观完全一致：

| 事实 | 浅色 | 深色 |
|---|---|---|
| 行数 / 分隔线条数（2 条探针） | 2 行 / **2 条线** | 2 行 / **2 条线** |
| 第一行上方 | **没有线**（不是双线，主线的猜测不成立） | 同 |
| 线到上一行文字的距离 | **3 px** | 3 px |
| 线到下一行文字的距离 | **8 px** | 8 px |
| 最后一行那条线下面 | 8 px 空气之后就是下一组标题（`Failures`）—— **一条孤立线** | 同 |
| 线的合成值 | 229 → 208（−63 三通道和 = **7 级**） | 39 → 176（+57 = **19 级**） |

一句话：**这条线不是「首行多一道」，是「每行都在自己的底边收一道」**。它带来两个毛病：

1. **贴着自己上家**：3 / 8 不是居中的分隔线，线看起来是「上一行的下划线」而不是「两行之间的分界」。
2. **替列表收了尾**：最后一条探针下面也有一条线，而它下面什么都没有（下一组是标题）。
   同页的诊断检查列表（`settingsui.go:2419` `diagnosisCheckRow`）恰好是反过来的正确写法——
   线是**两行之间的一个 1px 盒子**，**最后一行不画**。

## 1. 本轮意图

让探针列表的标点与同页诊断列表同一个语法：**线在两个条目之间，不在条目的边上，也不给列表收尾**。
这样线两侧的空气相等（4 + 4），列表结束时是空气而不是一条没有下家的线。

为什么现在做：这是健康页最后一块没有像素 pin 的排版（P13-cu 刚给了面档 pin，形状还没给），
而且它和「空间感」直接相关——用户说的「挤在一个 z 平面上」在列表里的表现正是**标点贴错边**。

## 2. 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/healthfaces.go:146`（`healthProbeRow`） | 行自己 `InsetShadow(0,-1,0,0,tokens.Hairline)`，画在**底边**，每行都画 | 删掉；探针行只管内容（mark / id / duration + 4px 竖 padding） |
| 2 | `internal/settingsui/settingsui.go:2310-2314`（`healthDetail` 的 checks 组） | `for _, probe := range report.Probes { a.healthProbeRow(...) }` | 逐条画之后，**只要后面还有探针**就补一个 `ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).Shrink(0)`（与 `diagnosisCheckRow` 同一写法） |

- 墨不动：仍是 `tokens.Hairline`（浅 208 / 深 176）。
- 组的 gap 不动：仍 `Space(1)`（4px），于是线两侧各 4px 空气。
- 字阶、字重、节点划分、`Glyph`、`i18n`、`row` 的 4px 竖 padding 全部不动。

## 3. 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
唯一可见变化是探针列表的高度（每两条探针之间的间距从 4 → 9：4 空气 + 1 线 + 4 空气）
与列表末尾少一条线。

## 4. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # ≥ 3292（P13-cu 实测）
GOOS=linux|windows|darwin go build ./...                # 过
```

## 5. mutation 计划

- **M1**：把 checks 组的 1px 盒子改成「每行都补一个」（即还原成「给列表收尾」）→
  `TestTheProbeListSeparatesItsRowsAndDoesNotCloseItself` 的「最后一行下面不许有线」变红。
- **M2**：把 1px 盒子换成 `Height(2)`（或删掉）→ 同一条 pin 的「线恰好 1px / 恰好 N−1 条」变红。
- **M3**：把 `tokens.Hairline` 换成 `tokens.Text` → 墨的合成值断言变红（±63 / ±57 级）。
