# P13-cq 规格 · 文本动作的三个状态：一盏灯，不是两盏

**意图一句话**：设置页共享文本动作（`.settings-reset`，`Restore defaults` / `Add shortcut` /
两个深链复制键）的 hover 与 press 各自被**两个方向相反的旋钮**同时画——墨抬一档 **加** 整棵
子树不透明度降一档；结果是 hover 的抬墨被吃掉一半，而 **press 比静息还淡**（按下那一刻，
这个动作在这个页面上读起来最像「不可用」）。

**为什么现在做**：P13-cp「仍待 1」点名了这一处，并把它归到 P13-s / P13-aq / P13-u / P13-y /
P13-cp 同一族——**一个意图画两遍**（§6.59 判据 1）。本轮是这一族的最后一处（`rows.go`
里剩下的 `Opacity(0.78/0.6)`）。

## 1. 现状（`internal/settingsui/rows.go:1663-1695`）

```go
ink := tokens.TextSecondary
switch {
case row.Pressed():
    row.Opacity(0.6)   // `.settings-reset:active { opacity: .6 }`
case row.Hovered():
    row.Opacity(0.78)  // `.settings-reset:not(:disabled):hover { opacity: .78 }`
    ink = t.Text
}
```

三件事：

1. **hover = 抬墨 + 淡下去**：`TextSecondary → Text` 是 80 级的抬升，`Opacity(0.78)` 又把它
   按回 0.78，净抬升只剩一半左右（实测见 §3）。
2. **press = 墨退回 `TextSecondary` + `Opacity(0.6)`**：`row.Pressed()` 蕴含指针在这条动作上，
   但 `switch` 的第一臂把 hover 抬的那一档**退回去**了，再乘 0.6——按下比静息更淡。
   「按住不放」在这条动作上的读数方向与整个应用相反（§5：按下是第三档，不是第一档）。
3. 同一个 `switch` 里 hover 抬墨、press 退墨：三档墨不单调（静息 < hover，press < 静息）。

参考 CSS 只有**一个**旋钮（`opacity`），P7-gk 照抄了那两个数；P8-d 又加上了「抬墨」这一档，
两个旋钮就都留在了源码里。合约这一代的说法（§5 + `settingsui.go:1419` 的注释）是：
**第三档换的不是墨，是面 / 几何**——而这条动作没有面（`.settings-reset` 透明底，P7-gk 专门
把底带删掉了），所以它的第三档只能是几何：§5 的 `Scale(0.986)`（与启动器结果行
`launcher.go:2511` 同一句话）。

## 2. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `rows.go` `actionWithGlyph` 的 `row.Hovered()` 臂 | `Opacity(0.78)` + `ink = t.Text` | 只留 `ink = t.Text`（一个旋钮：墨阶的一步） |
| 2 | 同函数 `row.Pressed()` 臂 | `Opacity(0.6)`，墨退回 `TextSecondary` | `ink = t.Text`（与 hover 同档，同 `sidebarRow` 的第三档说法）+ `row.Scale(0.986, 0.986)`（§5 的按下几何） |
| 3 | 同上，注释 | 引用 CSS 的两个 opacity | 写明「旧值 → 新值」与为什么第三档是几何不是墨 |

**不改**：盒子（24u 下限 / 2u 内边距 / 5u gap / `RadiusXS`）、字阶、字重、i18n 文案、
`Clicked()` 的接线、`Focusable()`。

## 3. 先量再改（pin 在改产品代码之前跑出来的原始串）

（由 `internal/settingsui/actionrung_test.go` 在 HEAD `e6bed3b7` 上跑出，原样抄进来。）

## 4. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线——本轮一个字节都不碰。
`Scale` 是渲染变换，布局盒不动（§6.7b-10 的教训：改盒子会动命中框，这里不动盒子）。

## 5. 门槛命令 + PASS 基线

基线（P13-cp 实测，HEAD `e6bed3b7`）：**PASS 3280 / FAIL 0 / SKIP 134 / 2428 测试函数**。

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## 6. mutation 计划（≥2 条，每条说明哪个 pin 会红）

- **M1**：把 `Opacity(0.78)` 放回 hover 臂 → `TestTheTextActionTakesOneRungPerState` 的
  「hover 墨 = `Text` token」那条红（墨被按回 blend）。
- **M2**：press 臂墨退回 `tokens.TextSecondary`（保留 Scale）→ 「press 的墨 = `Text` token」
  与「press 不比静息淡」两条红。
- **M3**：删掉 `row.Scale(0.986)` → 「press 帧与 hover 帧必须不同」红（两帧逐像素相同，
  正是 P13-cn 修过的那种「按下去一声不响」）。
