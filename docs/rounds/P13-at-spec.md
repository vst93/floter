# P13-at 规格：运行输出块的两端——一块 180px 的凹槽不该在两条边上刀切

## 0. 意图（一句话）

扩展页「最近一次输出」那个 `ui.Scroll` 的凹槽（`MaxHeight 45u` ≈ 180px）上下两条边
都是**平刀切**：内容在视口第一行像素与最后一行像素上被水平切断，墨一点没退；
给它两道**按滚动位置自门控**的软边（`glass.ScrollEdge`，底色取凹槽自己的面），
让这块小凹槽也有「面 / 浮」的先后，而不是一块被画框圈住的玻璃窗。

为什么是这一轮：`docs/liquid-glass.md` §6 **没有 `未开始` 行**，选题回到通用队列；
P13-as 报告「仍待 1」点名的三处同族容器里，这是**唯一一个两端都硬**的（另外两处
`config.go` 多选箱 66px 太矮、`pluginsheet.go` 是浮层，各占一轮）。按「一轮一个表面」
纪律，本轮只做运行输出块。

## 1. 实测现状（动手前，2026-10-31，HEAD = `151d9ab2`）

装置：`runOutputScrollApp(t)`（60 行 `line NN`）+ `integrationPage(t, a, 720, 2400)`
（窗口够高，页面体不溢出，滚的只可能是这块），`scroll.Y = scroll.MaxY/2` 后取像素。

视口框 `(209,564) 475×180`，`MaxY = 735`，`Y = 367.5`。

| 量 | 浅色 | 深色 |
|---|---|---|
| 视口**第一行**的墨展开（`spreadAt`） | 193 | 180 |
| 视口**最内**（y0+40…56）的墨展开 | 202 | 188 |
| 第一行 / 最内 = | **96%** | **96%** |
| 视口**最后一行**的墨展开 | 194 | 181 |
| 最内 / 最后一行 = | **96%** | **96%** |
| 框外一行（y0−2 / y1+2） | 0 | 0 |

逐行读数（浅）：`first rows: 0 0 193 189 179 202 0 0 …` / `last rows: … 170 170 178 194 0 0`
—— 上下两头都是满对比的字，中间毫无退让；框外立刻归 0，说明这道切完全由
`.Clip()` 造成，不是内容本身淡出。

## 2. 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/settingsui.go` `runOutput`（`ui.Scroll(c.Key("settings.runoutput."+id))` 那一段，动手前恰在 `:6812`） | 滚动视口直接贴在凹槽里，无任何边 | 把滚动视口包进一层 `FillWidth` 的 `ui.Column`，在**同一层**放两块 `PassThrough` 的绝对定位 `ui.Box`：顶边 `Height(3u)`、`Material(glass.ScrollEdge{Background: runOutputFace(tokens)})`；底边同值加 `Bottom: true` |
| 顶边的门 | — | `if scroll.Y > 0` 才画：停在开头时第一行是用户正在读的那一行，不许被淡 |
| 底边的门 | — | `if scroll.Y < scroll.MaxY` 才画：滚到底时最后一行同样是正在读的那一行 |
| 边的底色 | — | `runOutputFace(tokens)`（= `tokens.Sunken`），**不是** `t.Background`。P13-as 仍待 2 记的就是「边淡向错的底色会在静止帧留一道痕」，这里不再重犯 |
| 边高 | — | `t.Space(3)` = 12px：小于一行行盒（caption 11px × 1.45 ≈ 16px），因此最多软化一行的边缘，不会吞掉一整行；不用 4u 是因为 4u 正好等于一个行盒 |

## 3. 行为契约零改动红线

- 高度链：`MaxHeight(t.Space(45))`、凹槽 padding/gap、`runOutputScroll(id)` 的
  `*ui.ScrollState` 全部不动；`MaxY` 必须仍是 735，视口必须仍是 475×180 —— 这三点进 pin。
- 键盘模型 / 滚轮：`runoutputwheel_test.go` 的三条 pin（滚轮到达、两端 clamp、重绘保位）零改动必须绿。
- 会话管理、设置写回、i18n、命令接线：不碰。
- 只改墨与面：新增的两块只是 paint，`PassThrough()` 保证不吃指针事件。

## 4. 门槛命令与 PASS 基线（实测，HEAD `151d9ab2`）

```
gofmt -l cmd internal                 # 空（实测）
go vet ./...                          # 净
GOOS=windows|darwin go vet ./...      # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3065
grep -c -- "--- FAIL"                                # 0
grep -c -- "--- SKIP"                                # 99
测试函数（只数 cmd/ + internal/）                     # 2272
三平台 go build ./...                                 # 过
```

## 5. mutation 计划（≥2 条，各说清会让哪个 pin 变红）

1. **M1：边高 3u → 0**（等于删掉这两道边）→ pin ① 红：视口第一行/最后一行回到
   96% 对最内。
2. **M2：去掉门、无条件画**（`if scroll.Y > 0` 恒真）→ pin ② 红：停在开头时第一行被
   压低（预期 ≤58%），而 pin ② 要求 ≥90%。
3. **M3：底色换回 `t.Background`**（P13-as 仍待 2 的那个错）→ pin ③ 红：框外那一带
   均值会偏离凹槽自己的面（浅色 229 / 深色 39）超过 ±6 级。

## 6. 出图

`/tmp/floter-P13-at-runoutput-{light,dark}-{before,after}.png`，opt-in（`FLOTER_P13AT_DUMP=1`），
`FLOTER_P13AT_TAG=before|after` 取对比对。
